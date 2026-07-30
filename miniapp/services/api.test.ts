import { describe, expect, it, vi } from 'vitest'
import { createAPI } from './api'

describe('API client', () => {
  it('creates a project and returns its collection plan', async () => {
    const request = vi.fn().mockResolvedValue({
      statusCode: 201,
      data: {
        id: 'project-1',
        state: 'collecting',
        display_name: '林奶奶',
        collection_plan: [{ id: 'plan-1', category: 'childhood', prompt: '童年', status: 'pending' }]
      }
    })
    const api = createAPI({ baseURL: 'http://localhost:8080', request })
    const project = await api.createProject({
      display_name: '林奶奶',
      birth_year: 1948,
      birth_place: '江苏苏州',
      long_term_residence: '江苏苏州',
      primary_occupation: '纺织工人',
      target_edition: 'standard'
    })

    expect(project.collection_plan).toHaveLength(1)
    expect(request).toHaveBeenCalledWith(expect.objectContaining({
      method: 'POST',
      url: 'http://localhost:8080/v1/staff/projects'
    }))
  })

  it('rejects non-success status codes with APIError', async () => {
    const request = vi.fn().mockResolvedValue({ statusCode: 422, data: { error: { code: 'validation_failed', message: 'invalid' } } })
    const api = createAPI({ baseURL: 'http://localhost:8080', request })
    await expect(api.createProject({
      display_name: '', birth_year: 1948, birth_place: '苏州',
      long_term_residence: '苏州', primary_occupation: '', target_edition: 'brief'
    })).rejects.toMatchObject({ code: 'validation_failed', statusCode: 422 })
  })

  it('uploads bytes and completes the asset with SHA-256', async () => {
    const request = vi.fn()
      .mockResolvedValueOnce({ statusCode: 200, data: '' })
      .mockResolvedValueOnce({ statusCode: 200, data: { state: 'uploaded' } })
    const readFile = vi.fn().mockResolvedValue(new TextEncoder().encode('abc').buffer)
    const api = createAPI({ baseURL: 'http://localhost:8080', request, readFile })

    await api.uploadAsset({ asset_id: 'asset-1', upload_url: 'http://storage/upload', expires_at: '' }, 'local.file', 'audio/wav')

    expect(request).toHaveBeenNthCalledWith(2, expect.objectContaining({
      url: 'http://localhost:8080/v1/staff/assets/asset-1:complete',
      data: { sha256: 'ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad' }
    }))
  })

  it('refreshes an expired session once and replays the request', async () => {
    let token = 'expired-token'
    const session = {
      getToken: () => token,
      ensure: vi.fn(async () => token),
      refresh: vi.fn(async () => {
        token = 'fresh-token'
        return token
      }),
      clear: vi.fn()
    }
    const request = vi.fn()
      .mockResolvedValueOnce({ statusCode: 401, data: { error: { code: 'staff_unauthorized' } } })
      .mockResolvedValueOnce({ statusCode: 200, data: { id: 'project-1', collection_plan: [] } })
    const api = createAPI({ baseURL: 'http://localhost:8080', request, session })

    const project = await api.getProject('project-1')

    expect(project.id).toBe('project-1')
    expect(session.refresh).toHaveBeenCalledTimes(1)
    expect(request).toHaveBeenNthCalledWith(1, expect.objectContaining({
      header: expect.objectContaining({ Authorization: 'Bearer expired-token' })
    }))
    expect(request).toHaveBeenNthCalledWith(2, expect.objectContaining({
      header: expect.objectContaining({ Authorization: 'Bearer fresh-token' })
    }))
  })

  it('lists projects from the server with encoded filters', async () => {
    const request = vi.fn().mockResolvedValue({ statusCode: 200, data: { items: [], next_cursor: '' } })
    const api = createAPI({ baseURL: 'http://localhost:8080', request })

    await api.listProjects({ limit: 20, query: '林 奶奶', state: 'collecting' })

    expect(request).toHaveBeenCalledWith(expect.objectContaining({
      method: 'GET',
      url: 'http://localhost:8080/v1/staff/projects?limit=20&query=%E6%9E%97%20%E5%A5%B6%E5%A5%B6&state=collecting'
    }))
  })
})

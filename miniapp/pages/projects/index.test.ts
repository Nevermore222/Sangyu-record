import { beforeEach, describe, expect, it, vi } from 'vitest'

type ArchivePage = {
  onShow?: () => void
}

describe('archive page lifecycle', () => {
  let page: ArchivePage

  beforeEach(async () => {
    vi.resetModules()
    vi.stubGlobal('wx', {
      getStorageSync: vi.fn(),
      setStorageSync: vi.fn(),
      removeStorageSync: vi.fn()
    })
    vi.stubGlobal('Page', (definition: ArchivePage) => { page = definition })
    await import('./index')
  })

  it('reloads the first archive page whenever the tab becomes visible', () => {
    const load = vi.fn()

    expect(page.onShow).toEqual(expect.any(Function))
    page.onShow?.call({ load })

    expect(load).toHaveBeenCalledWith(true)
  })
})

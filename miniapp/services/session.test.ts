import { describe, expect, it, vi } from 'vitest'
import { createSession } from './session'

describe('staff session', () => {
  it('restores a stored session without logging in again', async () => {
    const storage = memoryStorage({ token: 'stored-token', staff: { id: 'staff-1', display_name: '采集员' } })
    const authenticate = vi.fn()
    const session = createSession({ storage, authenticate })

    await expect(session.ensure()).resolves.toBe('stored-token')
    expect(authenticate).not.toHaveBeenCalled()
  })

  it('deduplicates concurrent refreshes and stores the new identity', async () => {
    const storage = memoryStorage()
    const authenticate = vi.fn().mockResolvedValue({
      token: 'new-token',
      staff: { id: 'staff-1', display_name: '采集员', team_name: '口述史组', state: 'active' }
    })
    const session = createSession({ storage, authenticate })

    const [first, second] = await Promise.all([session.refresh(), session.refresh()])

    expect(first).toBe('new-token')
    expect(second).toBe('new-token')
    expect(authenticate).toHaveBeenCalledTimes(1)
    expect(storage.value()?.staff.display_name).toBe('采集员')
  })
})

function memoryStorage(initial?: unknown) {
  let stored = initial
  return {
    load: () => stored,
    save: (value: unknown) => { stored = value },
    clear: () => { stored = undefined },
    value: () => stored as { staff: { display_name: string } } | undefined
  }
}

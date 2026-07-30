import { describe, expect, it } from 'vitest'
import { canSubmitVisit } from './capture'

describe('visit capture state', () => {
  it('blocks submission while any local upload is incomplete', () => {
    expect(canSubmitVisit(
      [{ state: 'completed' }, { state: 'failed' }],
      [{ state: 'uploaded' }]
    )).toBe(false)
  })

  it('requires at least one uploaded server asset', () => {
    expect(canSubmitVisit([], [])).toBe(false)
    expect(canSubmitVisit([], [{ state: 'uploaded' }])).toBe(true)
  })
})

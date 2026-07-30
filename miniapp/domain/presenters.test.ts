import { describe, expect, it } from 'vitest'
import { presentStatus } from './presenters'

describe('status presenter', () => {
  it('maps backend states to stable Chinese labels and tones', () => {
    expect(presentStatus('needs_material')).toEqual({ label: '待补材料', tone: 'warning' })
    expect(presentStatus('completed')).toEqual({ label: '已完成', tone: 'success' })
    expect(presentStatus('unexpected')).toEqual({ label: '状态待确认', tone: 'neutral' })
  })
})

import { describe, expect, it } from 'vitest'
import { presentDashboard, presentStatus } from './presenters'

describe('status presenter', () => {
  it('maps backend states to stable Chinese labels and tones', () => {
    expect(presentStatus('needs_material')).toEqual({ label: '待补材料', tone: 'warning' })
    expect(presentStatus('completed')).toEqual({ label: '已完成', tone: 'success' })
    expect(presentStatus('unexpected')).toEqual({ label: '状态待确认', tone: 'neutral' })
  })

  it('maps dashboard projects to the next staff action', () => {
    const rows = presentDashboard({
      actionable: [
        { id: 'p1', display_name: '周奶奶', birth_year: 1948, state: 'needs_material', updated_at: '2026-07-30T00:00:00Z' },
        { id: 'p2', display_name: '陈爷爷', birth_year: 1942, state: 'completed', updated_at: '2026-07-29T00:00:00Z' },
        { id: 'p3', display_name: '林奶奶', birth_year: 1950, state: 'processing', updated_at: '2026-07-28T00:00:00Z' }
      ],
      recent: [],
      counts: { collecting: 0, needs_material: 1, processing: 1, completed: 1 }
    })

    expect(rows.actionable.map((row) => row.action)).toEqual(['补充采集', '查看成果', '继续处理'])
  })
})

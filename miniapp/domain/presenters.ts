export type StatusTone = 'neutral' | 'info' | 'warning' | 'danger' | 'success'

export interface StatusPresentation {
  label: string
  tone: StatusTone
}

const statuses: Record<string, StatusPresentation> = {
  collecting: { label: '采集中', tone: 'info' },
  processing: { label: '处理中', tone: 'info' },
  needs_material: { label: '待补材料', tone: 'warning' },
  generating: { label: '编写中', tone: 'info' },
  quality_check: { label: '质检中', tone: 'info' },
  pdf_rendering: { label: '排版中', tone: 'info' },
  exception: { label: '需处理', tone: 'danger' },
  completed: { label: '已完成', tone: 'success' },
  draft: { label: '草稿', tone: 'neutral' },
  submitted: { label: '已提交', tone: 'info' },
  analyzing: { label: '分析中', tone: 'info' },
  failed: { label: '处理失败', tone: 'danger' },
  pending: { label: '待采集', tone: 'neutral' },
  collected: { label: '已采集', tone: 'success' },
  insufficient: { label: '需补充', tone: 'warning' },
  not_needed: { label: '无需采集', tone: 'neutral' },
  queued: { label: '排队中', tone: 'info' },
  running: { label: '处理中', tone: 'info' },
  succeeded: { label: '已完成', tone: 'success' },
  pending_upload: { label: '待上传', tone: 'neutral' },
  uploaded: { label: '已上传', tone: 'success' }
}

export function presentStatus(status: string): StatusPresentation {
  return statuses[status] || { label: '状态待确认', tone: 'neutral' }
}

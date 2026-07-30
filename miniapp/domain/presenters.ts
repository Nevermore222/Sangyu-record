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

interface DashboardProject {
  id: string
  display_name: string
  birth_year: number
  state: string
  updated_at: string
}

interface DashboardInput {
  counts: { collecting: number; needs_material: number; processing: number; completed: number }
  actionable: DashboardProject[]
  recent: DashboardProject[]
}

export interface ProjectRowPresentation extends DashboardProject {
  statusLabel: string
  statusTone: StatusTone
  action: string
}

export function presentProjectRow(project: DashboardProject): ProjectRowPresentation {
  const status = presentStatus(project.state)
  const action = project.state === 'needs_material'
    ? '补充采集'
    : project.state === 'completed'
      ? '查看成果'
      : '继续处理'
  return { ...project, statusLabel: status.label, statusTone: status.tone, action }
}

export function presentDashboard(input: DashboardInput) {
  return {
    counts: input.counts,
    actionable: input.actionable.map(presentProjectRow),
    recent: input.recent.map(presentProjectRow)
  }
}

const categories: Record<string, string> = {
  childhood: '童年记忆',
  education: '求学经历',
  work: '工作生涯',
  family: '家庭生活',
  turning_points: '人生转折',
  photos: '老照片',
  occupation: '职业记忆'
}

export function presentCategory(category: string): string {
  return categories[category] || '其他记忆'
}

const workflowNodes: Record<string, string> = {
  transcribe: '整理访谈录音',
  understand_photo: '识别照片内容',
  build_memory: '串联个人记忆',
  retrieve_shared_memory: '补充时代背景',
  plan_book: '规划篇章结构',
  write_book: '编写回忆录正文',
  render_pdf: '生成回忆录 PDF',
  visit_transcribe: '整理本轮录音',
  visit_understand_photo: '识别本轮照片',
  visit_assess_material: '核对采集任务',
  visit_plan_followup: '生成补采问题',
  visit_persist_analysis: '保存走访报告'
}

export function presentWorkflowNode(name: string): string {
  return workflowNodes[name] || '处理资料'
}

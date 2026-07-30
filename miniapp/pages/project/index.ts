import type { Artifact, Project, Visit, VisitAnalysis, WorkflowRun } from '../../services/api'
import { presentCategory, presentStatus, type StatusTone } from '../../domain/presenters'
import { api } from '../../services/client'

interface PlanView {
  id: string
  categoryLabel: string
  prompt: string
  gapReason?: string
  statusLabel: string
  statusTone: StatusTone
}

interface VisitView extends Visit {
  statusLabel: string
  statusTone: StatusTone
  dateLabel: string
}

Page({
  data: {
    projectID: '',
    loading: true,
    finalizing: false,
    error: '',
    project: null as Project | null,
    planItems: [] as PlanView[],
    visits: [] as VisitView[],
    latestAnalysis: null as VisitAnalysis | null,
    workflow: null as WorkflowRun | null,
    artifact: null as Artifact | null,
    readyToFinalize: false,
    activeWorkflow: false
  },

  onLoad(options: Record<string, string | undefined>) {
    this.setData({ projectID: options.id || '' })
  },

  onShow() { void this.refresh() },

  async refresh() {
    this.setData({ loading: true, error: '' })
    try {
      const [project, visitsResponse] = await Promise.all([
        api.getProject(this.data.projectID),
        api.listVisits(this.data.projectID)
      ])
      const visits = visitsResponse.items
      const assetResponses = await Promise.all(visits.map(async (visit) => {
        try { return await api.listVisitAssets(visit.id) } catch { return { items: [] } }
      }))
      const assets = assetResponses.flatMap((response) => response.items)
      const latestCompleted = visits.find((visit) => visit.state === 'completed')
      const [workflow, artifact, latestAnalysis] = await Promise.all([
        this.optional(() => api.getWorkflow(this.data.projectID)),
        this.optional(() => api.getLatestArtifact(this.data.projectID)),
        latestCompleted ? this.optional(() => api.getVisitAnalysis(latestCompleted.id)) : Promise.resolve(null)
      ])
      const hasDraft = visits.some((visit) => visit.state === 'draft')
      const hasAudio = assets.some((asset) => asset.kind === 'audio' && asset.state === 'uploaded')
      const hasPhoto = assets.some((asset) => asset.kind === 'photo' && asset.state === 'uploaded')
      const planItems = project.collection_plan.map((item) => {
        const status = presentStatus(item.status)
        return {
          id: item.id,
          categoryLabel: presentCategory(item.category),
          prompt: item.prompt,
          gapReason: item.gap_reason,
          statusLabel: status.label,
          statusTone: status.tone
        }
      })
      const visitViews = visits.map((visit) => {
        const status = presentStatus(visit.state)
        return {
          ...visit,
          statusLabel: status.label,
          statusTone: status.tone,
          dateLabel: new Date(visit.visited_at).toLocaleDateString('zh-CN')
        }
      })
      const activeWorkflow = workflow?.state === 'queued' || workflow?.state === 'running'
      this.setData({
        project,
        planItems,
        visits: visitViews,
        workflow,
        artifact,
        latestAnalysis,
        activeWorkflow,
        readyToFinalize: Boolean(project.consent) && !hasDraft && hasAudio && hasPhoto && !activeWorkflow,
        loading: false
      })
    } catch (error) {
      this.setData({ loading: false, error: error instanceof Error ? error.message : '档案加载失败' })
    }
  },

  async optional<T>(load: () => Promise<T>): Promise<T | null> {
    try { return await load() } catch { return null }
  },

  startVisit() {
    void wx.navigateTo({ url: `/pages/visit-prepare/index?projectID=${this.data.projectID}` })
  },

  openVisit(event: WechatMiniprogram.BaseEvent) {
    const visitID = event.currentTarget.dataset.id as string
    const visit = this.data.visits.find((item) => item.id === visitID)
    const page = visit?.state === 'completed' ? 'visit-report' : 'visit-capture'
    void wx.navigateTo({ url: `/pages/${page}/index?visitID=${visitID}&projectID=${this.data.projectID}` })
  },

  async finalize() {
    if (!this.data.readyToFinalize || this.data.finalizing) return
    const confirmed = await new Promise<boolean>((resolve) => {
      wx.showModal({
        title: '确认开始成书',
        content: '请确认所有录音、照片与补充信息均已上传。提交后系统将自动编写并排版回忆录。',
        confirmText: '确认成书',
        success: (result) => resolve(result.confirm),
        fail: () => resolve(false)
      })
    })
    if (!confirmed) return
    this.setData({ finalizing: true, error: '' })
    try {
      const workflow = await api.finalizeProject(this.data.projectID)
      this.setData({ workflow, activeWorkflow: true, readyToFinalize: false, finalizing: false })
      void wx.navigateTo({ url: `/pages/workflow/index?projectID=${this.data.projectID}` })
    } catch (error) {
      this.setData({ finalizing: false, error: error instanceof Error ? error.message : '成书任务启动失败' })
    }
  },

  openResult() {
    void wx.navigateTo({ url: `/pages/result/index?id=${this.data.projectID}` })
  }
})

import type { Project, Visit, VisitAnalysis } from '../../services/api'
import { presentCategory } from '../../domain/presenters'
import { api } from '../../services/client'

let reportTimer: number | undefined

Page({
  data: {
    projectID: '', visitID: '', loading: true, retrying: false, error: '',
    visit: null as Visit | null,
    analysis: null as VisitAnalysis | null,
    covered: [] as Array<{ category: string; evidenceCount: number }>,
    gaps: [] as Array<{ category: string; reason: string }>,
    questions: [] as Array<{ category: string; question: string }>
  },

  onLoad(options: Record<string, string | undefined>) {
    this.setData({ projectID: options.projectID || '', visitID: options.visitID || '' })
    void this.load()
  },
  onUnload() { if (reportTimer !== undefined) clearTimeout(reportTimer) },

  async load() {
    try {
      const [visit, project] = await Promise.all([api.getVisit(this.data.visitID), api.getProject(this.data.projectID)])
      this.setData({ visit, loading: visit.state !== 'completed' && visit.state !== 'failed', error: '' })
      if (visit.state === 'failed') return
      if (visit.state !== 'completed') {
        reportTimer = setTimeout(() => void this.load(), 2000) as unknown as number
        return
      }
      this.applyAnalysis(project, await api.getVisitAnalysis(visit.id))
    } catch (error) {
      this.setData({ loading: false, error: error instanceof Error ? error.message : '走访报告加载失败' })
    }
  },

  applyAnalysis(project: Project, analysis: VisitAnalysis) {
    const plan = new Map(project.collection_plan.map((item) => [item.id, item]))
    this.setData({
      analysis, loading: false,
      covered: analysis.covered_items.map((item) => ({
        category: presentCategory(plan.get(item.plan_item_id)?.category || ''),
        evidenceCount: item.evidence_refs.length
      })),
      gaps: analysis.gaps.map((item) => ({
        category: presentCategory(plan.get(item.plan_item_id)?.category || ''), reason: item.reason
      })),
      questions: analysis.followup_questions.map((item) => ({
        category: presentCategory(plan.get(item.plan_item_id)?.category || ''), question: item.question
      }))
    })
  },

  async retry() {
    if (this.data.retrying) return
    this.setData({ retrying: true, error: '' })
    try {
      await api.retryVisit(this.data.visitID)
      this.setData({ retrying: false, loading: true })
      void this.load()
    } catch (error) {
      this.setData({ retrying: false, error: error instanceof Error ? error.message : '重试失败' })
    }
  },

  startFollowup() { void wx.navigateTo({ url: `/pages/visit-prepare/index?projectID=${this.data.projectID}` }) }
})

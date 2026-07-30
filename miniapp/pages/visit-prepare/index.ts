import type { Project } from '../../services/api'
import { presentCategory, presentStatus, type StatusTone } from '../../domain/presenters'
import { api } from '../../services/client'
import { wxDrafts } from '../../services/drafts'

interface PlanChoice {
  id: string
  categoryLabel: string
  prompt: string
  statusLabel: string
  statusTone: StatusTone
  selected: boolean
}

Page({
  data: {
    projectID: '',
    loading: true,
    submitting: false,
    error: '',
    project: null as Project | null,
    planChoices: [] as PlanChoice[],
    selectedIDs: [] as string[],
    visitedDate: new Date().toISOString().slice(0, 10),
    location: '',
    notes: ''
  },

  onLoad(options: Record<string, string | undefined>) {
    this.setData({ projectID: options.projectID || '' })
    void this.load()
  },

  async load() {
    try {
      const project = await api.getProject(this.data.projectID)
      const selectedIDs = project.collection_plan
        .filter((item) => item.status === 'pending' || item.status === 'insufficient')
        .map((item) => item.id)
      const planChoices = project.collection_plan.map((item) => {
        const status = presentStatus(item.status)
        return {
          id: item.id,
          categoryLabel: presentCategory(item.category),
          prompt: item.prompt,
          statusLabel: status.label,
          statusTone: status.tone,
          selected: selectedIDs.includes(item.id)
        }
      })
      this.setData({ project, selectedIDs, planChoices, loading: false })
    } catch (error) {
      this.setData({ loading: false, error: error instanceof Error ? error.message : '采集计划加载失败' })
    }
  },

  onPlanChange(event: WechatMiniprogram.CheckboxGroupChange) {
    const selectedIDs = event.detail.value
    this.setData({
      selectedIDs,
      planChoices: this.data.planChoices.map((item) => ({ ...item, selected: selectedIDs.includes(item.id) }))
    })
  },

  onDate(event: WechatMiniprogram.PickerChange) { this.setData({ visitedDate: String(event.detail.value) }) },
  onLocation(event: WechatMiniprogram.Input) { this.setData({ location: event.detail.value }) },
  onNotes(event: WechatMiniprogram.TextareaInput) { this.setData({ notes: event.detail.value }) },

  async createVisit() {
    if (this.data.submitting) return
    if (!this.data.location.trim()) {
      this.setData({ error: '请填写本次走访地点' })
      return
    }
    if (this.data.selectedIDs.length === 0) {
      this.setData({ error: '请至少选择一项采集任务' })
      return
    }
    this.setData({ submitting: true, error: '' })
    try {
      const visit = await api.createVisit(this.data.projectID, {
        visited_at: new Date(`${this.data.visitedDate}T12:00:00`).toISOString(),
        location: this.data.location.trim(),
        notes: this.data.notes.trim(),
        plan_item_ids: this.data.selectedIDs
      })
      wxDrafts.save({
        projectID: this.data.projectID,
        visitID: visit.id,
        location: visit.location,
        notes: visit.notes,
        planItemIDs: visit.plan_item_ids,
        updatedAt: Date.now()
      })
      await wx.redirectTo({ url: `/pages/visit-capture/index?visitID=${visit.id}&projectID=${this.data.projectID}` })
    } catch (error) {
      this.setData({ submitting: false, error: error instanceof Error ? error.message : '走访创建失败' })
    }
  }
})

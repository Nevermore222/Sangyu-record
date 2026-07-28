import type { Project, WorkflowRun } from '../../services/api'
import { api } from '../../services/client'

let pollTimer: number | undefined

Page({
  data: {
    projectID: '', loading: true, error: '', processing: false,
    project: null as Project | null,
    workflow: null as WorkflowRun | null
  },

  onLoad(options: Record<string, string | undefined>) {
    this.setData({ projectID: options.id || '' })
  },

  onShow() {
    void this.refresh()
  },

  onUnload() {
    if (pollTimer !== undefined) clearInterval(pollTimer)
  },

  async refresh() {
    try {
      const project = await api.getProject(this.data.projectID)
      let workflow: WorkflowRun | null = null
      try { workflow = await api.getWorkflow(this.data.projectID) } catch { workflow = null }
      this.setData({ project, workflow, loading: false, error: '', processing: workflow?.state === 'running' || workflow?.state === 'queued' })
      if (this.data.processing && pollTimer === undefined) {
        pollTimer = setInterval(() => void this.refresh(), 1500) as unknown as number
      } else if (!this.data.processing && pollTimer !== undefined) {
        clearInterval(pollTimer)
        pollTimer = undefined
      }
    } catch (error) {
      this.setData({ error: error instanceof Error ? error.message : '项目加载失败', loading: false })
    }
  },

  collect() {
    void wx.navigateTo({ url: `/pages/upload/index?id=${this.data.projectID}` })
  },

  async startWorkflow() {
    try {
      await api.startWorkflow(this.data.projectID)
      this.setData({ processing: true })
      await this.refresh()
    } catch (error) {
      this.setData({ error: error instanceof Error ? error.message : '自动处理启动失败' })
    }
  },

  openResult() {
    void wx.navigateTo({ url: `/pages/result/index?id=${this.data.projectID}` })
  }
})

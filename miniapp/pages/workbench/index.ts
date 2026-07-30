import type { ProjectSummary } from '../../services/api'
import { presentDashboard, type ProjectRowPresentation } from '../../domain/presenters'
import { api } from '../../services/client'

Page({
  data: {
    loading: true,
    error: '',
    counts: [] as Array<{ label: string; value: number; tone: string }>,
    actionable: [] as ProjectRowPresentation[],
    recent: [] as ProjectRowPresentation[]
  },

  onShow() { void this.load() },

  async load() {
    this.setData({ loading: true, error: '' })
    try {
      const view = presentDashboard(await api.getDashboard())
      this.setData({
        loading: false,
        counts: [
          { label: '采集中', value: view.counts.collecting, tone: 'green' },
          { label: '待补材料', value: view.counts.needs_material, tone: 'burgundy' },
          { label: '处理中', value: view.counts.processing, tone: 'ink' },
          { label: '已完成', value: view.counts.completed, tone: 'muted' }
        ],
        actionable: view.actionable,
        recent: view.recent
      })
    } catch (error) {
      this.setData({ loading: false, error: error instanceof Error ? error.message : '工作台加载失败' })
    }
  },

  openProject(event: WechatMiniprogram.CustomEvent<{ id: string }>) {
    void wx.navigateTo({ url: `/pages/project/index?id=${event.detail.id}` })
  },

  createProject() { void wx.navigateTo({ url: '/pages/create/index' }) },
  openArchives() { void wx.switchTab({ url: '/pages/projects/index' }) }
})

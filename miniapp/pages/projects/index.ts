import type { Project } from '../../services/api'
import { api } from '../../services/client'

Page({
  data: {
    loading: true,
    error: '',
    projects: [] as Project[]
  },

  onShow() {
    void this.loadProjects()
  },

  async loadProjects() {
    this.setData({ loading: true, error: '' })
    try {
      const ids = (wx.getStorageSync('recentProjectIDs') || []) as string[]
      const projects = await Promise.all(ids.map((id) => api.getProject(id)))
      this.setData({ projects, loading: false })
    } catch (error) {
      this.setData({ error: error instanceof Error ? error.message : '项目加载失败', loading: false })
    }
  },

  createProject() {
    void wx.navigateTo({ url: '/pages/create/index' })
  },

  openProject(event: WechatMiniprogram.BaseEvent) {
    const id = event.currentTarget.dataset.id as string
    void wx.navigateTo({ url: `/pages/project/index?id=${id}` })
  }
})

import type { ProjectSummary } from '../../services/api'
import { api } from '../../services/client'

let searchTimer: number | undefined

Page({
  data: {
    loading: true,
    loadingMore: false,
    error: '',
    query: '',
    state: '',
    nextCursor: '',
    projects: [] as ProjectSummary[],
    filters: [
      { label: '全部', value: '' },
      { label: '采集中', value: 'collecting' },
      { label: '待补材料', value: 'needs_material' },
      { label: '已完成', value: 'completed' }
    ]
  },

  onShow() { void this.load(true) },
  onUnload() { if (searchTimer !== undefined) clearTimeout(searchTimer) },
  onPullDownRefresh() { void this.load(true).finally(() => wx.stopPullDownRefresh()) },
  onReachBottom() { if (this.data.nextCursor) void this.load(false) },

  onSearch(event: WechatMiniprogram.Input) {
    const query = event.detail.value
    this.setData({ query })
    if (searchTimer !== undefined) clearTimeout(searchTimer)
    searchTimer = setTimeout(() => void this.load(true), 320) as unknown as number
  },

  selectFilter(event: WechatMiniprogram.BaseEvent) {
    this.setData({ state: event.currentTarget.dataset.value as string })
    void this.load(true)
  },

  async load(reset: boolean) {
    if (this.data.loadingMore) return
    this.setData(reset ? { loading: true, error: '' } : { loadingMore: true, error: '' })
    try {
      const page = await api.listProjects({
        limit: 20,
        query: this.data.query.trim() || undefined,
        state: this.data.state || undefined,
        cursor: reset ? undefined : this.data.nextCursor
      })
      this.setData({
        projects: reset ? page.items : [...this.data.projects, ...page.items],
        nextCursor: page.next_cursor || '',
        loading: false,
        loadingMore: false
      })
    } catch (error) {
      this.setData({
        loading: false,
        loadingMore: false,
        error: error instanceof Error ? error.message : '档案加载失败'
      })
    }
  },

  openProject(event: WechatMiniprogram.CustomEvent<{ id: string }>) {
    void wx.navigateTo({ url: `/pages/project/index?id=${event.detail.id}` })
  },

  createProject() { void wx.navigateTo({ url: '/pages/create/index' }) }
})

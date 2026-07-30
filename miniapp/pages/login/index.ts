import { session } from '../../services/client'

Page({
  data: {
    loading: true,
    error: ''
  },

  onLoad() {
    void this.login()
  },

  async login() {
    this.setData({ loading: true, error: '' })
    try {
      await session.ensure()
      await wx.reLaunch({ url: '/pages/workbench/index' })
    } catch (error) {
      this.setData({
        loading: false,
        error: error instanceof Error ? error.message : '身份核验失败'
      })
    }
  }
})

import { api, session } from '../../services/client'

const initialStaff = session.getStaff()

Page({
  data: {
    staff: initialStaff,
    staffInitial: initialStaff?.display_name.slice(0, 1) || '员',
    loggingOut: false
  },
  onShow() {
    const staff = session.getStaff()
    this.setData({ staff, staffInitial: staff?.display_name.slice(0, 1) || '员' })
  },
  async logout() {
    if (this.data.loggingOut) return
    this.setData({ loggingOut: true })
    try { await api.logout() } catch { /* local logout still applies */ }
    session.clear()
    await wx.reLaunch({ url: '/pages/login/index' })
  }
})

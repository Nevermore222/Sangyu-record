import { session } from './services/session-client'

App({
  globalData: {
    staff: session.getStaff()
  },
  onLaunch() {
    void session.ensure().then(() => {
      this.globalData.staff = session.getStaff()
    }).catch(() => undefined)
  }
})

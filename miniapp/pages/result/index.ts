import type { Artifact } from '../../services/api'
import { api } from '../../services/client'

Page({
  data: { projectID: '', loading: true, error: '', artifact: null as Artifact | null, opening: false },

  onLoad(options: Record<string, string | undefined>) {
    this.setData({ projectID: options.id || '' })
    void this.load()
  },

  async load() {
    try {
      const artifact = await api.getLatestArtifact(this.data.projectID)
      this.setData({ artifact, loading: false, error: '' })
    } catch (error) {
      this.setData({ error: error instanceof Error ? error.message : '成果加载失败', loading: false })
    }
  },

  async openPDF() {
    if (!this.data.artifact || this.data.opening) return
    this.setData({ opening: true })
    try {
	  const result = await new Promise<WechatMiniprogram.DownloadFileSuccessCallbackResult>((resolve, reject) => {
	    wx.downloadFile({ url: this.data.artifact!.download_url, success: resolve, fail: reject })
	  })
	  await new Promise<void>((resolve, reject) => {
	    wx.openDocument({ filePath: result.tempFilePath, fileType: 'pdf', showMenu: true, success: () => resolve(), fail: reject })
	  })
    } catch (error) {
      this.setData({ error: error instanceof Error ? error.message : 'PDF打开失败' })
    } finally {
      this.setData({ opening: false })
    }
  }
})

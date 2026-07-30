import type { Artifact, Project } from '../../services/api'
import { api } from '../../services/client'

Page({
  data: { projectID: '', loading: true, opening: false, error: '', project: null as Project | null, artifact: null as Artifact | null, sizeLabel: '' },
  onLoad(options: Record<string, string | undefined>) {
    this.setData({ projectID: options.id || '' })
    wx.showShareMenu({ menus: ['shareAppMessage'] })
    void this.load()
  },
  async load() {
    this.setData({ loading: true, error: '' })
    try {
      const [project, artifact] = await Promise.all([api.getProject(this.data.projectID), api.getLatestArtifact(this.data.projectID)])
      this.setData({ project, artifact, sizeLabel: formatBytes(artifact.size_bytes), loading: false })
    } catch (error) {
      this.setData({ loading: false, error: error instanceof Error ? error.message : '成果加载失败' })
    }
  },
  async openPDF() {
    if (!this.data.artifact || this.data.opening) return
    this.setData({ opening: true, error: '' })
    try {
      const artifact = await api.getLatestArtifact(this.data.projectID)
      this.setData({ artifact, sizeLabel: formatBytes(artifact.size_bytes) })
      const result = await new Promise<WechatMiniprogram.DownloadFileSuccessCallbackResult>((resolve, reject) => {
        wx.downloadFile({ url: artifact.download_url, success: resolve, fail: reject })
      })
      if (result.statusCode < 200 || result.statusCode >= 300) {
        throw new Error(`PDF 下载失败（${result.statusCode}）`)
      }
      await new Promise<void>((resolve, reject) => {
        wx.openDocument({ filePath: result.tempFilePath, fileType: 'pdf', showMenu: true, success: () => resolve(), fail: reject })
      })
    } catch (error) {
      this.setData({ error: error instanceof Error ? error.message : 'PDF 打开失败' })
    } finally {
      this.setData({ opening: false })
    }
  },
  onShareAppMessage() {
    return { title: `${this.data.project?.display_name || '老人'}的回忆录`, path: `/pages/result/index?id=${this.data.projectID}` }
  }
})

function formatBytes(size: number): string {
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`
  return `${(size / 1024 / 1024).toFixed(1)} MB`
}

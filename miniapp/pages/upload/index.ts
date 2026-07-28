import { api } from '../../services/client'

const recorder = wx.getRecorderManager()
let handleRecorderStop: Parameters<typeof recorder.onStop>[0] | undefined
recorder.onStop((result) => handleRecorderStop?.(result))

Page({
  data: {
    projectID: '', recording: false, uploading: false, error: '',
    audioPath: '', audioSize: 0, photoPath: '', photoSize: 0,
    progressText: '等待采集'
  },

  onLoad(options: Record<string, string | undefined>) {
    this.setData({ projectID: options.id || '' })
    handleRecorderStop = (result) => {
      const info = wx.getFileSystemManager().statSync(result.tempFilePath) as WechatMiniprogram.Stats
      this.setData({ recording: false, audioPath: result.tempFilePath, audioSize: info.size, progressText: '录音已就绪' })
    }
  },

  onUnload() {
    handleRecorderStop = undefined
    if (this.data.recording) recorder.stop()
  },

  toggleRecord() {
    if (this.data.recording) {
      recorder.stop()
    } else {
      recorder.start({ format: 'mp3', sampleRate: 16000, numberOfChannels: 1, encodeBitRate: 48000 })
      this.setData({ recording: true, progressText: '正在录音' })
    }
  },

  async choosePhoto() {
    const result = await wx.chooseMedia({ count: 1, mediaType: ['image'], sourceType: ['camera', 'album'] })
    const photo = result.tempFiles[0]
    this.setData({ photoPath: photo.tempFilePath, photoSize: photo.size, progressText: '照片已就绪' })
  },

  async upload() {
    if (!this.data.audioPath || !this.data.photoPath || this.data.uploading) {
      this.setData({ error: '请先完成一段录音并选择一张照片' })
      return
    }
    this.setData({ uploading: true, error: '', progressText: '上传录音' })
    try {
      const audioTicket = await api.initiateAsset(this.data.projectID, { kind: 'audio', filename: 'interview.mp3', content_type: 'audio/mpeg', size_bytes: this.data.audioSize })
      await api.uploadAsset(audioTicket, this.data.audioPath, 'audio/mpeg')
      this.setData({ progressText: '上传照片' })
      const photoTicket = await api.initiateAsset(this.data.projectID, { kind: 'photo', filename: 'photo.jpg', content_type: 'image/jpeg', size_bytes: this.data.photoSize })
      await api.uploadAsset(photoTicket, this.data.photoPath, 'image/jpeg')
      this.setData({ progressText: '物料上传完成', uploading: false })
      setTimeout(() => void wx.navigateBack(), 600)
    } catch (error) {
      this.setData({ error: error instanceof Error ? error.message : '上传失败', uploading: false, progressText: '上传中断' })
    }
  }
})

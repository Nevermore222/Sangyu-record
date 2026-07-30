import { APIError, type Asset, type Visit } from '../../services/api'
import { presentCategory } from '../../domain/presenters'
import { canSubmitVisit } from '../../domain/capture'
import { api } from '../../services/client'
import { wxDrafts } from '../../services/drafts'
import { saveCapturedFile, removeSavedFile, type UploadQueueItem, type UploadSource } from '../../services/upload-queue'
import { createLocalID, uploadQueue, visitQueueItems } from '../../services/uploads'

const recorder = wx.getRecorderManager()
let recorderStop: Parameters<typeof recorder.onStop>[0] | undefined
let recorderError: Parameters<typeof recorder.onError>[0] | undefined
let unsubscribeUploadQueue: (() => void) | undefined
let capturePageActive = false
recorder.onStop((result) => {
  const handler = recorderStop
  handler?.(result)
  if (!capturePageActive) recorderStop = undefined
})
recorder.onError((error) => recorderError?.(error))

Page({
  data: {
    projectID: '',
    visitID: '',
    loading: true,
    recording: false,
    uploading: false,
    submitting: false,
    error: '',
    notes: '',
    visit: null as Visit | null,
    queueItems: [] as UploadQueueItem[],
    audioItems: [] as UploadQueueItem[],
    photoItems: [] as UploadQueueItem[],
    assets: [] as Asset[],
    planOptions: [] as string[],
    planOptionIDs: [] as string[],
    canSubmit: false
  },

  onLoad(options: Record<string, string | undefined>) {
    capturePageActive = true
    this.setData({ projectID: options.projectID || '', visitID: options.visitID || '' })
    recorderStop = (result) => void this.handleRecordedFile(result.tempFilePath)
    recorderError = (error) => {
      if (!capturePageActive) return
      this.setData({ recording: false, error: error.errMsg || '录音启动失败，请检查麦克风权限' })
      this.syncUnloadWarning(visitQueueItems(this.data.visitID))
    }
    unsubscribeUploadQueue = uploadQueue.subscribe(() => this.refreshQueueView())
    void this.load()
  },

  onShow() {
    if (!this.data.visitID) return
    void this.resumeUploads()
  },

  onUnload() {
    capturePageActive = false
    unsubscribeUploadQueue?.()
    unsubscribeUploadQueue = undefined
    if (this.data.recording) {
      recorder.stop()
    } else {
      recorderStop = undefined
    }
    recorderError = undefined
    wx.disableAlertBeforeUnload()
  },

  async load() {
    try {
      const [visit, project] = await Promise.all([api.getVisit(this.data.visitID), api.getProject(this.data.projectID)])
      const draft = wxDrafts.load(visit.id)
      const selectedPlan = project.collection_plan.filter((item) => visit.plan_item_ids.includes(item.id))
      this.setData({
        visit,
        notes: draft?.notes ?? visit.notes,
        planOptions: ['全部本轮任务', ...selectedPlan.map((item) => presentCategory(item.category))],
        planOptionIDs: selectedPlan.map((item) => item.id),
        loading: false
      })
      await this.refreshAssets()
    } catch (error) {
      this.setData({ loading: false, error: error instanceof Error ? error.message : '走访加载失败' })
    }
  },

  async resumeUploads() {
    this.setData({ uploading: true })
    await uploadQueue.resume()
    this.setData({ uploading: false })
    await this.refreshAssets()
  },

  async refreshAssets() {
    const response = await api.listVisitAssets(this.data.visitID)
    this.setData({ assets: response.items })
    this.refreshQueueView()
  },

  refreshQueueView() {
    const queueItems = visitQueueItems(this.data.visitID).map((item) => {
      const selectedID = item.planItemIDs?.length === 1 ? item.planItemIDs[0] : ''
      const selectedIndex = selectedID ? this.data.planOptionIDs.indexOf(selectedID) : -1
      return { ...item, associationLabel: selectedIndex >= 0 ? this.data.planOptions[selectedIndex + 1] : '全部本轮任务' }
    })
    this.setData({
      queueItems,
      audioItems: queueItems.filter((item) => item.kind === 'audio'),
      photoItems: queueItems.filter((item) => item.kind === 'photo'),
      canSubmit: canSubmitVisit(queueItems, this.data.assets)
    })
    this.syncUnloadWarning(queueItems)
  },

  syncUnloadWarning(items: UploadQueueItem[]) {
    if (items.length > 0 || this.data.recording) {
      const message = this.data.recording
        ? '录音尚未结束，离开页面会先保存当前录音。'
        : '仍有本地物料尚未上传，离开后可回来继续。'
      wx.enableAlertBeforeUnload({ message })
    } else {
      wx.disableAlertBeforeUnload()
    }
  },

  toggleRecord() {
    if (this.data.uploading || this.data.submitting) return
    if (this.data.recording) {
      recorder.stop()
      this.setData({ recording: false })
      return
    }
    recorder.start({ duration: 600000, format: 'mp3', sampleRate: 16000, numberOfChannels: 1, encodeBitRate: 48000 })
    this.setData({ recording: true, error: '' })
    wx.enableAlertBeforeUnload({ message: '录音尚未结束，离开页面会先保存当前录音。' })
  },

  async handleRecordedFile(tempFilePath: string) {
    try {
      await this.addFile(tempFilePath, `现场录音-${Date.now()}.mp3`, 'audio/mpeg', 'audio', 'direct')
      if (capturePageActive) this.setData({ recording: false })
    } catch (error) {
      if (capturePageActive) {
        this.setData({ recording: false, error: error instanceof Error ? error.message : '录音保存失败' })
      }
    }
  },

  async chooseAudio() {
    if (this.data.recording || this.data.uploading || this.data.submitting) return
    try {
      const result = await wx.chooseMessageFile({ count: 9, type: 'file', extension: ['mp3', 'm4a', 'wav'] })
      for (const file of result.tempFiles) {
        const extension = file.name.toLowerCase().split('.').pop()
        const contentType = extension === 'wav' ? 'audio/wav' : extension === 'm4a' ? 'audio/mp4' : 'audio/mpeg'
        await this.addFile(file.path, file.name, contentType, 'audio', 'wechat_file', file.size)
      }
    } catch (error) {
      if ((error as { errMsg?: string }).errMsg?.includes('cancel')) return
      this.setData({ error: error instanceof Error ? error.message : '音频选择失败' })
    }
  },

  async choosePhoto(event: WechatMiniprogram.BaseEvent) {
    if (this.data.recording || this.data.uploading || this.data.submitting) return
    const source = event.currentTarget.dataset.source as 'camera' | 'album'
    try {
      const result = await wx.chooseMedia({ count: source === 'camera' ? 1 : 9, mediaType: ['image'], sourceType: [source] })
      for (const [index, file] of result.tempFiles.entries()) {
        const isPNG = file.tempFilePath.toLowerCase().split('?')[0].endsWith('.png')
        await this.addFile(
          file.tempFilePath,
          `老照片-${Date.now()}-${index + 1}.${isPNG ? 'png' : 'jpg'}`,
          isPNG ? 'image/png' : 'image/jpeg',
          'photo',
          source,
          file.size
        )
      }
    } catch (error) {
      if ((error as { errMsg?: string }).errMsg?.includes('cancel')) return
      this.setData({ error: error instanceof Error ? error.message : '照片选择失败' })
    }
  },

  async addFile(
    tempFilePath: string,
    filename: string,
    contentType: string,
    kind: 'audio' | 'photo',
    source: UploadSource,
    knownSize?: number
  ) {
    const filePath = await saveCapturedFile(tempFilePath)
    const sizeBytes = knownSize || (wx.getFileSystemManager().statSync(filePath) as WechatMiniprogram.Stats).size
    uploadQueue.add({
      localID: createLocalID(),
      projectID: this.data.projectID,
      visitID: this.data.visitID,
      filePath,
      filename,
      contentType,
      sizeBytes,
      kind,
      source,
      planItemIDs: this.data.visit?.plan_item_ids || [],
      state: 'local',
      progress: 0
    })
    if (capturePageActive) await this.refreshAssets()
  },

  renameItem(event: WechatMiniprogram.CustomEvent<{ localID: string; name: string }>) {
    const item = visitQueueItems(this.data.visitID).find((value) => value.localID === event.detail.localID)
    const name = event.detail.name.trim()
    if (!item || !name) return
    uploadQueue.add({ ...item, filename: name })
    void this.refreshAssets()
  },

  async removeItem(event: WechatMiniprogram.CustomEvent<{ localID: string }>) {
    const item = visitQueueItems(this.data.visitID).find((value) => value.localID === event.detail.localID)
    if (!item) return
    if (item.assetID) {
      try {
        await api.deleteAsset(item.assetID)
      } catch (error) {
        const converged = error instanceof APIError && (error.statusCode === 404 || error.code === 'asset_state_conflict')
        if (!converged) {
          this.setData({ error: error instanceof Error ? error.message : '物料删除失败，请稍后重试' })
          return
        }
      }
    }
    try { await removeSavedFile(item.filePath) } catch { /* file may already be absent */ }
    uploadQueue.remove(item.localID)
    await this.refreshAssets()
  },

  associateItem(event: WechatMiniprogram.CustomEvent<{ localID: string; index: number }>) {
    const item = visitQueueItems(this.data.visitID).find((value) => value.localID === event.detail.localID)
    if (!item) return
    const planItemIDs = event.detail.index === 0
      ? (this.data.visit?.plan_item_ids || [])
      : [this.data.planOptionIDs[event.detail.index - 1]]
    uploadQueue.add({ ...item, planItemIDs })
    void this.refreshAssets()
  },

  onNotes(event: WechatMiniprogram.TextareaInput) {
    const notes = event.detail.value
    this.setData({ notes })
    if (this.data.visit) {
      wxDrafts.save({
        projectID: this.data.projectID,
        visitID: this.data.visitID,
        location: this.data.visit.location,
        notes,
        planItemIDs: this.data.visit.plan_item_ids,
        updatedAt: Date.now()
      })
    }
  },

  async uploadAll() {
    if (this.data.recording || this.data.uploading || this.data.submitting) return
    await this.resumeUploads()
  },

  async submitVisit() {
    if (!this.data.canSubmit || this.data.recording || this.data.uploading || this.data.submitting) return
    this.setData({ submitting: true, error: '' })
    try {
      await api.updateVisit(this.data.visitID, { notes: this.data.notes.trim() })
      await api.submitVisit(this.data.visitID)
      wxDrafts.remove(this.data.visitID)
      wx.disableAlertBeforeUnload()
      await wx.redirectTo({ url: `/pages/visit-report/index?visitID=${this.data.visitID}&projectID=${this.data.projectID}` })
    } catch (error) {
      this.setData({ submitting: false, error: error instanceof Error ? error.message : '本轮分析提交失败' })
    }
  }
})

export type UploadKind = 'audio' | 'photo'
export type UploadSource = 'direct' | 'wechat_file' | 'album' | 'camera'
export type UploadState = 'local' | 'uploading' | 'completed' | 'failed'

export interface UploadQueueItem {
  localID: string
  projectID: string
  visitID: string
  assetID?: string
  filePath: string
  filename: string
  contentType: string
  sizeBytes: number
  kind: UploadKind
  source: UploadSource
  planItemIDs?: string[]
  state: UploadState
  progress: number
  error?: string
}

export interface UploadQueueStorage {
  load(): UploadQueueItem[] | undefined
  save(items: UploadQueueItem[]): void
}

export interface QueueUploader {
  upload(item: UploadQueueItem, onProgress: (progress: number) => void): Promise<void>
  removeLocalFile(filePath: string): Promise<void> | void
}

export function createUploadQueue(storage: UploadQueueStorage, uploader: QueueUploader) {
  let items: UploadQueueItem[] = (storage.load() || []).map((item) => ({
    ...item,
    state: item.state === 'uploading' ? 'local' as const : item.state
  }))
  let resumePromise: Promise<void> | undefined

  const persist = () => storage.save(items.map((item) => ({ ...item })))
  const resume = (): Promise<void> => {
    if (resumePromise) return resumePromise
    resumePromise = (async () => {
      for (const item of [...items]) {
        if (item.state !== 'local' && item.state !== 'failed') continue
        item.state = 'uploading'
        item.error = undefined
        persist()
        try {
          await uploader.upload(item, (progress) => {
            item.progress = Math.max(0, Math.min(100, Math.round(progress)))
            persist()
          })
          item.state = 'completed'
          item.progress = 100
          persist()
          try { await uploader.removeLocalFile(item.filePath) } catch { /* best-effort cleanup */ }
          items = items.filter((value) => value.localID !== item.localID)
          persist()
        } catch (error) {
          item.state = 'failed'
          item.error = error instanceof Error ? error.message : '上传失败'
          persist()
        }
      }
    })().finally(() => { resumePromise = undefined })
    return resumePromise
  }

  return {
    add: (item: UploadQueueItem) => {
      items = [...items.filter((value) => value.localID !== item.localID), { ...item }]
      persist()
    },
    remove: (localID: string) => {
      items = items.filter((item) => item.localID !== localID)
      persist()
    },
    retry: (localID: string) => {
      const item = items.find((value) => value.localID === localID)
      if (item) {
        item.state = 'local'
        item.error = undefined
        persist()
      }
      return resume()
    },
    resume,
    snapshot: () => items.map((item) => ({ ...item }))
  }
}

export function createWXUploadQueueStorage(key = 'sangyu.upload-queue'): UploadQueueStorage {
  return {
    load: () => wx.getStorageSync(key) || [],
    save: (items) => wx.setStorageSync(key, items)
  }
}

export function saveCapturedFile(tempFilePath: string): Promise<string> {
  return new Promise((resolve, reject) => {
    wx.getFileSystemManager().saveFile({
      tempFilePath,
      success: (result) => resolve(result.savedFilePath),
      fail: reject
    })
  })
}

export function removeSavedFile(filePath: string): Promise<void> {
  return new Promise((resolve, reject) => {
    wx.getFileSystemManager().removeSavedFile({ filePath, success: () => resolve(), fail: reject })
  })
}

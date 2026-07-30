import { api } from './client'
import {
  createUploadQueue,
  createWXUploadQueueStorage,
  removeSavedFile,
  type UploadQueueItem
} from './upload-queue'
import { uploadQueuedAsset } from './asset-upload'

export const uploadQueue = createUploadQueue(createWXUploadQueueStorage(), {
  upload: async (item, onProgress) => {
    await uploadQueuedAsset(item, api, assertLocalFile, onProgress)
  },
  removeLocalFile: removeSavedFile
})

export function visitQueueItems(visitID: string): UploadQueueItem[] {
  return uploadQueue.snapshot().filter((item) => item.visitID === visitID)
}

export function createLocalID(): string {
  return `${Date.now()}-${Math.random().toString(16).slice(2)}`
}

function assertLocalFile(filePath: string): Promise<void> {
  return new Promise((resolve, reject) => {
    wx.getFileSystemManager().access({
      path: filePath,
      success: () => resolve(),
      fail: () => reject(new Error('本地文件已失效，请重新选择'))
    })
  })
}

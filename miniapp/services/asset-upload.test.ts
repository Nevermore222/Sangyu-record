import { describe, expect, it, vi } from 'vitest'
import { uploadQueuedAsset } from './asset-upload'
import type { UploadQueueItem } from './upload-queue'

describe('queued asset upload', () => {
  it('treats an already uploaded server asset as completed without reading the local file', async () => {
    const api = {
      listVisitAssets: vi.fn().mockResolvedValue({
        items: [{ id: 'asset-1', visit_id: 'visit-1', kind: 'audio', display_name: 'recording', state: 'uploaded' }]
      }),
      renewAssetUpload: vi.fn(),
      initiateAsset: vi.fn(),
      uploadAsset: vi.fn()
    }
    const assertLocalFile = vi.fn()

    await uploadQueuedAsset(queuedItem(), api, assertLocalFile, vi.fn())

    expect(assertLocalFile).not.toHaveBeenCalled()
    expect(api.renewAssetUpload).not.toHaveBeenCalled()
    expect(api.uploadAsset).not.toHaveBeenCalled()
  })
})

function queuedItem(): UploadQueueItem {
  return {
    localID: 'local-1', projectID: 'project-1', visitID: 'visit-1', assetID: 'asset-1',
    filePath: 'missing-file', filename: 'recording.wav', contentType: 'audio/wav', sizeBytes: 10,
    kind: 'audio', source: 'direct', state: 'failed', progress: 100
  }
}

import { describe, expect, it, vi } from 'vitest'
import { createUploadQueue, type UploadQueueItem } from './upload-queue'

describe('upload queue', () => {
  it('keeps failed items while completed items are removed', async () => {
    const items: UploadQueueItem[] = [item('ok'), item('bad')]
    let persisted = items
    const storage = {
      load: () => persisted,
      save: (value: UploadQueueItem[]) => { persisted = value }
    }
    const removeLocalFile = vi.fn()
    const queue = createUploadQueue(storage, {
      upload: async (value) => {
        if (value.localID === 'bad') throw new Error('network unavailable')
      },
      removeLocalFile
    })

    await queue.resume()

    expect(queue.snapshot().map((value) => value.state)).toEqual(['failed'])
    expect(queue.snapshot()[0].error).toBe('network unavailable')
    expect(removeLocalFile).toHaveBeenCalledWith('saved-ok')
    expect(removeLocalFile).not.toHaveBeenCalledWith('saved-bad')
  })
})

function item(localID: string): UploadQueueItem {
  return {
    localID, projectID: 'project-1', visitID: 'visit-1', filePath: `saved-${localID}`,
    filename: `${localID}.wav`, contentType: 'audio/wav', sizeBytes: 10,
    kind: 'audio', source: 'direct', state: 'local', progress: 0
  }
}

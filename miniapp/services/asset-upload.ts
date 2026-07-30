import type { Asset, InitiateAssetInput, UploadTicket } from './api'
import type { UploadQueueItem } from './upload-queue'

interface AssetUploadAPI {
  listVisitAssets(visitID: string): Promise<{ items: Asset[] }>
  renewAssetUpload(assetID: string): Promise<UploadTicket>
  initiateAsset(projectID: string, input: InitiateAssetInput): Promise<UploadTicket>
  uploadAsset(ticket: UploadTicket, filePath: string, contentType: string): Promise<void>
}

export async function uploadQueuedAsset(
  item: UploadQueueItem,
  api: AssetUploadAPI,
  assertLocalFile: (filePath: string) => Promise<void>,
  onProgress: (progress: number) => void
): Promise<void> {
  let ticket: UploadTicket
  if (item.assetID) {
    const remote = await api.listVisitAssets(item.visitID)
    if (remote.items.some((asset) => asset.id === item.assetID && asset.state === 'uploaded')) return
    await assertLocalFile(item.filePath)
    ticket = await api.renewAssetUpload(item.assetID)
  } else {
    await assertLocalFile(item.filePath)
    ticket = await api.initiateAsset(item.projectID, {
      visit_id: item.visitID,
      kind: item.kind,
      source: item.source,
      filename: item.filename,
      display_name: item.filename,
      content_type: item.contentType,
      size_bytes: item.sizeBytes,
      plan_item_ids: item.planItemIDs
    })
    item.assetID = ticket.asset_id
    onProgress(15)
  }
  onProgress(35)
  await api.uploadAsset(ticket, item.filePath, item.contentType)
  onProgress(100)
}

export function canSubmitVisit(
  queueItems: Array<{ state: string }>,
  serverAssets: Array<{ state: string }>
): boolean {
  const hasIncomplete = queueItems.some((item) => item.state !== 'completed')
  return !hasIncomplete && serverAssets.some((asset) => asset.state === 'uploaded')
}

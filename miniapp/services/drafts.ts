export interface VisitDraft {
  projectID: string
  visitID: string
  location: string
  notes: string
  planItemIDs: string[]
  updatedAt: number
}

export interface DraftStorage {
  get(key: string): VisitDraft | undefined
  set(key: string, value: VisitDraft): unknown
  remove(key: string): unknown
}

export function createDraftStore(storage: DraftStorage) {
  const key = (visitID: string) => `sangyu.visit-draft.${visitID}`
  return {
    load: (visitID: string) => storage.get(key(visitID)),
    save: (draft: VisitDraft) => storage.set(key(draft.visitID), { ...draft }),
    remove: (visitID: string) => storage.remove(key(visitID))
  }
}

export const wxDrafts = createDraftStore({
  get: (key) => wx.getStorageSync(key) || undefined,
  set: (key, value) => wx.setStorageSync(key, value),
  remove: (key) => wx.removeStorageSync(key)
})

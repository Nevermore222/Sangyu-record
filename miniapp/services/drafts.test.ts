import { describe, expect, it } from 'vitest'
import { createDraftStore, type VisitDraft } from './drafts'

describe('visit drafts', () => {
  it('keeps separate drafts per visit and removes only the submitted visit', () => {
    const values = new Map<string, VisitDraft>()
    const drafts = createDraftStore({
      get: (key) => values.get(key),
      set: (key, value) => values.set(key, value),
      remove: (key) => values.delete(key)
    })
    drafts.save({ projectID: 'project-1', visitID: 'visit-1', location: '公园', notes: '', planItemIDs: [], updatedAt: 1 })
    drafts.save({ projectID: 'project-1', visitID: 'visit-2', location: '家中', notes: '', planItemIDs: [], updatedAt: 2 })

    drafts.remove('visit-1')

    expect(drafts.load('visit-1')).toBeUndefined()
    expect(drafts.load('visit-2')?.location).toBe('家中')
  })
})

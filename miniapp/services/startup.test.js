import { readFileSync } from 'node:fs'

import { describe, expect, it } from 'vitest'

function source(path) {
  return readFileSync(new URL(path, import.meta.url), 'utf8')
}

describe('miniapp startup dependency boundary', () => {
  it('keeps app launch and login independent from the full API client', () => {
    expect(source('../app.ts')).toContain("from './services/session-client'")
    expect(source('../pages/login/index.ts')).toContain("from '../../services/session-client'")
    expect(source('./session-client.ts')).not.toMatch(/\.\/api|js-sha256/)
  })
})

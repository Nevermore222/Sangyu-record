import assert from 'node:assert/strict'
import { after, before, test } from 'node:test'
import { createApp } from './server.mjs'

let server
let baseURL

before(async () => {
  server = createApp()
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve))
  baseURL = `http://127.0.0.1:${server.address().port}`
})

after(async () => {
  await new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()))
})

test('runs the supported versioned skill', async () => {
  const response = await fetch(`${baseURL}/v1/invocations`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({
      invocation_id: 'inv-1',
      contract_version: '1.0',
      skill: { name: 'mock-memoir', version: '0.1.0' },
      input: { project_id: 'project-1' },
      allowed_resources: ['asset-1'],
      deadline: '2026-07-28T10:00:00Z'
    })
  })
  assert.equal(response.status, 200)
  const result = await response.json()
  assert.equal(result.invocation_id, 'inv-1')
  assert.equal(result.status, 'succeeded')
  assert.deepEqual(result.evidence_refs, ['asset-1#0-12'])
})

test('rejects an unregistered skill', async () => {
  const response = await fetch(`${baseURL}/v1/invocations`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({
      invocation_id: 'inv-2',
      contract_version: '1.0',
      skill: { name: 'unknown', version: '1.0.0' },
      input: {},
      allowed_resources: [],
      deadline: '2026-07-28T10:00:00Z'
    })
  })
  assert.equal(response.status, 404)
})

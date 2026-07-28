import { createServer } from 'node:http'
import { fileURLToPath } from 'node:url'
import { resolve } from 'node:path'

const maxBodyBytes = 1024 * 1024

function send(response, status, value) {
  response.writeHead(status, { 'content-type': 'application/json; charset=utf-8' })
  response.end(`${JSON.stringify(value)}\n`)
}

async function readJSON(request) {
  const chunks = []
  let size = 0
  for await (const chunk of request) {
    size += chunk.length
    if (size > maxBodyBytes) throw new Error('request_too_large')
    chunks.push(chunk)
  }
  return JSON.parse(Buffer.concat(chunks).toString('utf8'))
}

export function createApp() {
  return createServer(async (request, response) => {
    if (request.method !== 'POST' || request.url !== '/v1/invocations') {
      send(response, 404, { error: { code: 'not_found', message: 'route not found' } })
      return
    }
    try {
      const invocation = await readJSON(request)
      if (invocation.contract_version !== '1.0' || !invocation.invocation_id) {
        send(response, 422, { error: { code: 'invalid_invocation', message: 'contract_version 1.0 and invocation_id are required' } })
        return
      }
      if (invocation.skill?.name !== 'mock-memoir' || invocation.skill?.version !== '0.1.0') {
        send(response, 404, { error: { code: 'skill_not_found', message: 'skill name or version is not registered' } })
        return
      }
      const firstResource = invocation.allowed_resources?.[0] ?? 'asset-1'
      send(response, 200, {
        invocation_id: invocation.invocation_id,
        status: 'succeeded',
        output: { title: '岁月留声', project_id: invocation.input?.project_id ?? '' },
        evidence_refs: [`${firstResource}#0-12`],
        warnings: [],
        metrics: { duration_ms: 1 },
        error: null
      })
    } catch (error) {
      const status = error.message === 'request_too_large' ? 413 : 400
      send(response, status, { error: { code: 'invalid_json', message: error.message } })
    }
  })
}

if (process.argv[1] && fileURLToPath(import.meta.url) === resolve(process.argv[1])) {
  const port = Number(process.env.PORT ?? 8090)
  createApp().listen(port, '0.0.0.0', () => {
    process.stdout.write(`mock skill runner listening on ${port}\n`)
  })
}

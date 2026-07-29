import { randomUUID } from 'node:crypto'
import { createServer } from 'node:http'
import { fileURLToPath } from 'node:url'
import { resolve } from 'node:path'

const maxBodyBytes = 1024 * 1024
const taskTypes = new Set([
  'audio_transcription',
  'speaker_diarization',
  'photo_ocr',
  'photo_understanding',
  'collection_plan',
  'material_assessment',
  'followup_plan',
  'memoir_positioning',
  'timeline_builder',
  'chapter_planner',
  'chapter_writer',
  'chapter_review',
  'book_consistency_review',
  'shared_memory_retrieval'
])

function send(response, status, value) {
  const headers = value === null ? undefined : { 'content-type': 'application/json; charset=utf-8' }
  response.writeHead(status, headers)
  response.end(value === null ? undefined : `${JSON.stringify(value)}\n`)
}

async function readJSON(request) {
  const chunks = []
  let size = 0
  for await (const chunk of request) {
    size += chunk.length
    if (size > maxBodyBytes) throw new Error('request_too_large')
    chunks.push(chunk)
  }
  const encoded = Buffer.concat(chunks).toString('utf8')
  return encoded ? JSON.parse(encoded) : {}
}

function validateSubmission(value) {
  return value?.contract_version === '1.0' &&
    typeof value.request_id === 'string' && value.request_id !== '' &&
    typeof value.idempotency_key === 'string' && value.idempotency_key !== '' &&
    taskTypes.has(value.task_type) &&
    value.input_schema_version === '1.0' &&
    value.output_schema_version === '1.0' &&
    value.input && typeof value.input === 'object' &&
    Array.isArray(value.resource_urls) &&
    typeof value.callback_url === 'string' &&
    typeof value.deadline === 'string'
}

function outputFor(taskType) {
  switch (taskType) {
    case 'audio_transcription':
      return { segments: [{ start_seconds: 12, end_seconds: 20, text: '\u6211\u57281978\u5e74\u8fdb\u5165\u4e86\u5f53\u5730\u7eba\u7ec7\u5382\u3002', source_ref: 'audio-fixture#12-20' }] }
    case 'speaker_diarization':
      return { turns: [{ speaker: 'elder', start_seconds: 12, end_seconds: 20, source_ref: 'audio-fixture#12-20' }] }
    case 'photo_ocr':
      return { text_blocks: [{ text: '1978', source_ref: 'photo-fixture#ocr-1' }] }
    case 'photo_understanding':
      return { description: '\u4e00\u5f20\u5de5\u4f5c\u65f6\u671f\u7684\u96c6\u4f53\u7167\u7247', source_ref: 'photo-fixture#image-1', confidence: 'inferred' }
    case 'collection_plan':
      return { items: [{ kind: 'audio', topic: 'childhood' }] }
    case 'material_assessment':
      return { complete: true, missing: [] }
    case 'followup_plan':
      return { questions: ['What happened next?'] }
    case 'memoir_positioning':
      return { voice: 'first_person', tone: 'warm' }
    case 'timeline_builder':
      return { memories: [{ event: '\u8fdb\u5165\u5f53\u5730\u7eba\u7ec7\u5382\u5de5\u4f5c', year: 1978, evidence_refs: ['audio-fixture#12-20'] }] }
    case 'chapter_planner':
      return { title: '\u5c81\u6708\u7559\u58f0', chapters: [{ title: '\u7eba\u7ec7\u5382\u7684\u65e5\u5b50', target_words: 1200 }] }
    case 'chapter_writer':
      return {
        title: '\u5c81\u6708\u7559\u58f0',
        chapters: [{
          title: '\u7eba\u7ec7\u5382\u7684\u65e5\u5b50',
          paragraphs: [{ text: '1978\u5e74\uff0c\u5979\u8fdb\u5165\u4e86\u5f53\u5730\u7eba\u7ec7\u5382\u3002', evidence_refs: ['audio-fixture#12-20', 'K-1978-001'] }]
        }]
      }
    case 'chapter_review':
      return { approved: true, issues: [] }
    case 'book_consistency_review':
      return { approved: true, issues: [] }
    case 'shared_memory_retrieval':
      return {
        entries: [{
          reference_id: 'K-1978-001',
          text: '\u6539\u9769\u5f00\u653e\u521d\u671f\u7684\u57ce\u9547\u5de5\u4e1a\u8bb0\u5fc6',
          source: 'mock-local-corpus',
          year_from: 1978,
          year_to: 1982,
          region: '\u6c5f\u82cf',
          confidence: 'high',
          license: 'CC0-1.0'
        }]
      }
    default:
      throw new Error(`unsupported task type: ${taskType}`)
  }
}

function snapshot(job, completionDelayMs) {
  if (job.cancelled) return { request_id: job.request.request_id, provider_job_id: job.id, state: 'cancelled' }
  const elapsed = Date.now() - job.createdAt
  if (elapsed < completionDelayMs) {
    return { request_id: job.request.request_id, provider_job_id: job.id, state: 'processing' }
  }
  const scenario = job.request.input.mock_scenario
  if (['retryable_failed', 'permanently_failed', 'timed_out'].includes(scenario)) {
    return {
      request_id: job.request.request_id,
      provider_job_id: job.id,
      state: scenario,
      error_code: `mock_${scenario}`,
      error_message: `mock scenario ${scenario}`
    }
  }
  const output = scenario === 'malformed_output' ? { unexpected: true } : outputFor(job.request.task_type)
  return { request_id: job.request.request_id, provider_job_id: job.id, state: 'succeeded', output }
}

export function createApp({ completionDelayMs = 20 } = {}) {
  const jobs = new Map()
  const jobsByKey = new Map()

  return createServer(async (request, response) => {
    const url = new URL(request.url, 'http://mock-provider')
    try {
      if (request.method === 'POST' && url.pathname === '/v1/jobs') {
        const submission = await readJSON(request)
        if (!validateSubmission(submission)) {
          send(response, 422, { error: { code: 'invalid_submission', message: 'canonical Provider submission is required' } })
          return
        }
        let job = jobsByKey.get(submission.idempotency_key)
        if (!job) {
          job = { id: randomUUID(), request: submission, createdAt: Date.now(), cancelled: false }
          jobs.set(job.id, job)
          jobsByKey.set(submission.idempotency_key, job)
        }
        send(response, 202, { provider_job_id: job.id, state: job.cancelled ? 'cancelled' : 'submitted' })
        return
      }

      const cancelMatch = url.pathname.match(/^\/v1\/jobs\/([^/]+):cancel$/)
      if (request.method === 'POST' && cancelMatch) {
        const job = jobs.get(cancelMatch[1])
        if (!job) {
          send(response, 404, { error: { code: 'job_not_found', message: 'Provider job was not found' } })
          return
        }
        await readJSON(request)
        job.cancelled = true
        send(response, 204, null)
        return
      }

      const statusMatch = url.pathname.match(/^\/v1\/jobs\/([^/]+)$/)
      if (request.method === 'GET' && statusMatch) {
        const job = jobs.get(statusMatch[1])
        if (!job) {
          send(response, 404, { error: { code: 'job_not_found', message: 'Provider job was not found' } })
          return
        }
        send(response, 200, snapshot(job, completionDelayMs))
        return
      }

      send(response, 404, { error: { code: 'not_found', message: 'route not found' } })
    } catch (error) {
      const status = error.message === 'request_too_large' ? 413 : 400
      send(response, status, { error: { code: 'invalid_json', message: error.message } })
    }
  })
}

if (process.argv[1] && fileURLToPath(import.meta.url) === resolve(process.argv[1])) {
  const port = Number(process.env.PORT ?? 8090)
  createApp().listen(port, '0.0.0.0', () => {
    process.stdout.write(`mock Provider listening on ${port}\n`)
  })
}

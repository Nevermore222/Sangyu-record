import assert from 'node:assert/strict'
import { test } from 'node:test'
import { createApp } from './server.mjs'

async function usingServer(server, run) {
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve))
  try {
    await run(`http://127.0.0.1:${server.address().port}`)
  } finally {
    await new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()))
  }
}

async function requestJSON(url, options) {
  const response = await fetch(url, options)
  const text = await response.text()
  return { status: response.status, body: text ? JSON.parse(text) : null }
}

function postJSON(url, body) {
  return requestJSON(url, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(body)
  })
}

function getJSON(url) {
  return requestJSON(url, { method: 'GET' })
}

function validSubmission(taskType) {
  return {
    request_id: '11111111-1111-1111-1111-111111111111',
    idempotency_key: `test:${taskType}`,
    contract_version: '1.0',
    task_type: taskType,
    input_schema_version: '1.0',
    output_schema_version: '1.0',
    input: { project_id: '22222222-2222-2222-2222-222222222222' },
    resource_urls: [],
    callback_url: 'http://api:8080/v1/provider-callbacks/media/job-1',
    deadline: '2026-07-30T12:10:00Z'
  }
}

test('reports readiness for container health checks', async () => {
  const app = createApp()
  await usingServer(app, async (baseURL) => {
    const result = await getJSON(`${baseURL}/healthz`)
    assert.equal(result.status, 200)
    assert.deepEqual(result.body, { status: 'ok' })
  })
})

test('runs a canonical asynchronous provider job', async () => {
  const app = createApp({ completionDelayMs: 5 })
  await usingServer(app, async (baseURL) => {
    const submitted = await postJSON(`${baseURL}/v1/jobs`, validSubmission('audio_transcription'))
    assert.equal(submitted.status, 202)
    assert.equal(submitted.body.state, 'submitted')

    await new Promise((resolve) => setTimeout(resolve, 10))
    const result = await getJSON(`${baseURL}/v1/jobs/${submitted.body.provider_job_id}`)
    assert.equal(result.body.state, 'succeeded')
    assert.equal(result.body.output.segments[0].source_ref, 'audio-fixture#12-20')
  })
})

test('exposes selected terminal and retry states', async () => {
  const app = createApp({ completionDelayMs: 1 })
  await usingServer(app, async (baseURL) => {
    for (const expected of ['retryable_failed', 'permanently_failed', 'timed_out']) {
      const request = validSubmission('audio_transcription')
      request.idempotency_key = `scenario-${expected}`
      request.input.mock_scenario = expected
      const submitted = await postJSON(`${baseURL}/v1/jobs`, request)
      await new Promise((resolve) => setTimeout(resolve, 5))
      const result = await getJSON(`${baseURL}/v1/jobs/${submitted.body.provider_job_id}`)
      assert.equal(result.body.state, expected)
    }
  })
})

test('reuses idempotent submissions and supports cancellation', async () => {
  const app = createApp({ completionDelayMs: 1000 })
  await usingServer(app, async (baseURL) => {
    const request = validSubmission('chapter_writer')
    const first = await postJSON(`${baseURL}/v1/jobs`, request)
    const second = await postJSON(`${baseURL}/v1/jobs`, request)
    assert.equal(first.body.provider_job_id, second.body.provider_job_id)
    const cancelled = await postJSON(`${baseURL}/v1/jobs/${first.body.provider_job_id}:cancel`, {})
    assert.equal(cancelled.status, 204)
    const result = await getJSON(`${baseURL}/v1/jobs/${first.body.provider_job_id}`)
    assert.equal(result.body.state, 'cancelled')
  })
})

test('can return malformed successful output for adapter validation tests', async () => {
  const app = createApp({ completionDelayMs: 1 })
  await usingServer(app, async (baseURL) => {
    const request = validSubmission('audio_transcription')
    request.input.mock_scenario = 'malformed_output'
    const submitted = await postJSON(`${baseURL}/v1/jobs`, request)
    await new Promise((resolve) => setTimeout(resolve, 5))
    const result = await getJSON(`${baseURL}/v1/jobs/${submitted.body.provider_job_id}`)
    assert.equal(result.body.state, 'succeeded')
    assert.deepEqual(result.body.output, { unexpected: true })
  })
})

test('accepts every canonical task type', async () => {
  const taskTypes = [
    'audio_transcription', 'speaker_diarization', 'photo_ocr', 'photo_understanding',
    'collection_plan', 'material_assessment', 'followup_plan', 'memoir_positioning',
    'timeline_builder', 'chapter_planner', 'chapter_writer', 'chapter_review',
    'book_consistency_review', 'shared_memory_retrieval'
  ]
  const app = createApp({ completionDelayMs: 1 })
  await usingServer(app, async (baseURL) => {
    for (const taskType of taskTypes) {
      const submitted = await postJSON(`${baseURL}/v1/jobs`, validSubmission(taskType))
      assert.equal(submitted.status, 202, taskType)
      await new Promise((resolve) => setTimeout(resolve, 2))
      const result = await getJSON(`${baseURL}/v1/jobs/${submitted.body.provider_job_id}`)
      assert.equal(result.body.state, 'succeeded', taskType)
    }
  })
})

test('returns normalized visit analysis structures', async () => {
	const app = createApp({ completionDelayMs: 1 })
	await usingServer(app, async (baseURL) => {
		const assessmentRequest = validSubmission('material_assessment')
		assessmentRequest.input.selected_plan_items = [{ id: 'plan-1' }, { id: 'plan-2' }]
		const assessment = await postJSON(`${baseURL}/v1/jobs`, assessmentRequest)
		await new Promise((resolve) => setTimeout(resolve, 5))
		const assessmentResult = await getJSON(`${baseURL}/v1/jobs/${assessment.body.provider_job_id}`)
		assert.equal(assessmentResult.body.output.covered_items[0].plan_item_id, 'plan-1')
		assert.equal(assessmentResult.body.output.gaps[0].plan_item_id, 'plan-2')

		const followupRequest = validSubmission('followup_plan')
		followupRequest.idempotency_key = 'visit-followup-structure'
		followupRequest.input.selected_plan_items = [{ id: 'plan-1' }]
		const followup = await postJSON(`${baseURL}/v1/jobs`, followupRequest)
		await new Promise((resolve) => setTimeout(resolve, 5))
		const followupResult = await getJSON(`${baseURL}/v1/jobs/${followup.body.provider_job_id}`)
		assert.equal(followupResult.body.output.questions[0].plan_item_id, 'plan-1')
	})
})

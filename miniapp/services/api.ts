import { sha256 } from 'js-sha256'
import type { SessionClient } from './session'

export interface PlanItem {
  id: string
  category: string
  prompt: string
  status: 'pending' | 'collected' | 'insufficient' | 'not_needed'
  gap_reason?: string
}

export interface Project {
  id: string
  display_name: string
  birth_year: number
  birth_place?: string
  long_term_residence?: string
  primary_occupation?: string
  target_edition?: 'brief' | 'standard' | 'long'
  state: string
  collection_plan: PlanItem[]
  consent?: Consent
}

export interface Consent {
  id: string
  confirmed_by: 'elder' | 'guardian'
  confirmation_method: 'onsite'
  confirmed_at: string
}

export interface ProjectSummary {
  id: string
  display_name: string
  birth_year: number
  birth_place?: string
  long_term_residence?: string
  primary_occupation?: string
  target_edition?: 'brief' | 'standard' | 'long'
  state: string
  updated_at: string
}

export interface ProjectPage {
  items: ProjectSummary[]
  next_cursor?: string
}

export interface Dashboard {
  counts: { collecting: number; needs_material: number; processing: number; completed: number }
  actionable: ProjectSummary[]
  recent: ProjectSummary[]
}

export interface Visit {
  id: string
  project_id: string
  sequence: number
  visited_at: string
  location: string
  notes: string
  state: string
  error_code?: string
  plan_item_ids: string[]
}

export interface Asset {
  id: string
  visit_id?: string
  kind: 'audio' | 'photo'
  display_name: string
  state: 'pending_upload' | 'uploaded'
}

export interface VisitAnalysis {
  id: string
  visit_id: string
  summary: string
  covered_items: Array<{ plan_item_id: string; evidence_refs: string[] }>
  gaps: Array<{ plan_item_id: string; reason: string }>
  followup_questions: Array<{ plan_item_id: string; question: string }>
}

export interface CreateVisitInput {
  visited_at: string
  location: string
  notes: string
  plan_item_ids: string[]
}

export interface InitiateAssetInput {
  visit_id?: string
  kind: 'audio' | 'photo'
  source?: 'direct' | 'wechat_file' | 'album' | 'camera'
  filename: string
  display_name?: string
  content_type: string
  size_bytes: number
  plan_item_ids?: string[]
}

export interface CreateProjectInput {
  display_name: string
  birth_year: number
  birth_place: string
  long_term_residence: string
  primary_occupation: string
  target_edition: 'brief' | 'standard' | 'long'
}

export interface UploadTicket {
  asset_id: string
  upload_url: string
  expires_at: string
}

export interface WorkflowNode {
  name: string
  state: string
  error_code?: string
}

export interface WorkflowRun {
  id: string
  state: string
  error_code?: string
  nodes: WorkflowNode[]
}

export interface Artifact {
  id: string
  version: number
  size_bytes: number
  download_url: string
}

export interface ListProjectsInput {
  limit?: number
  query?: string
  state?: string
  cursor?: string
}

interface RequestOptions {
  url: string
  method: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  data?: unknown
  header?: Record<string, string>
}

interface RequestResponse {
  statusCode: number
  data: unknown
}

type Request = (options: RequestOptions) => Promise<RequestResponse>
type ReadFile = (filePath: string) => Promise<ArrayBuffer>

export class APIError extends Error {
  constructor(
    public readonly statusCode: number,
    public readonly code: string,
    message: string
  ) {
    super(message)
    this.name = 'APIError'
  }
}

function wxRequest(options: RequestOptions): Promise<RequestResponse> {
  return new Promise((resolve, reject) => {
    wx.request({
      url: options.url,
      method: options.method as any,
      data: options.data as string | ArrayBuffer | WechatMiniprogram.IAnyObject | undefined,
      header: options.header,
      success: (response) => resolve({ statusCode: response.statusCode, data: response.data }),
      fail: reject
    })
  })
}

function wxReadFile(filePath: string): Promise<ArrayBuffer> {
  return new Promise((resolve, reject) => {
    wx.getFileSystemManager().readFile({
      filePath,
      success: (result) => resolve(result.data as ArrayBuffer),
      fail: reject
    })
  })
}

export function createAPI({
  baseURL,
  request = wxRequest,
  readFile = wxReadFile,
  session
}: {
  baseURL: string
  request?: Request
  readFile?: ReadFile
  session?: Pick<SessionClient, 'ensure' | 'refresh' | 'clear'>
}) {
  const call = async <T>(method: RequestOptions['method'], path: string, data?: unknown, replayed = false): Promise<T> => {
    const token = session ? await session.ensure() : undefined
    const header: Record<string, string> = {}
    if (data !== undefined) header['content-type'] = 'application/json'
    if (token) header.Authorization = `Bearer ${token}`
    const response = await request({
      method,
      url: `${baseURL}${path}`,
      data,
      header: Object.keys(header).length > 0 ? header : undefined
    })
    if (response.statusCode === 401 && session && !replayed) {
      session.clear()
      await session.refresh()
      return call<T>(method, path, data, true)
    }
    if (response.statusCode < 200 || response.statusCode >= 300) {
      const body = response.data as { error?: { code?: string; message?: string } }
      throw new APIError(
        response.statusCode,
        body.error?.code ?? 'request_failed',
        body.error?.message ?? '请求失败'
      )
    }
    return response.data as T
  }

  return {
    getDashboard: () => call<Dashboard>('GET', '/v1/staff/dashboard'),
    listProjects: (input: ListProjectsInput = {}) => {
      const pairs: string[] = []
      if (input.limit) pairs.push(`limit=${encodeURIComponent(String(input.limit))}`)
      if (input.query) pairs.push(`query=${encodeURIComponent(input.query)}`)
      if (input.state) pairs.push(`state=${encodeURIComponent(input.state)}`)
      if (input.cursor) pairs.push(`cursor=${encodeURIComponent(input.cursor)}`)
      return call<ProjectPage>('GET', `/v1/staff/projects${pairs.length > 0 ? `?${pairs.join('&')}` : ''}`)
    },
    createProject: (input: CreateProjectInput) => call<Project>('POST', '/v1/staff/projects', input),
    getProject: (projectID: string) => call<Project>('GET', `/v1/staff/projects/${projectID}`),
    confirmConsent: (projectID: string, confirmedBy: 'elder' | 'guardian') =>
      call<Consent>('POST', `/v1/staff/projects/${projectID}/consents`, { confirmed_by: confirmedBy }),
    createVisit: (projectID: string, input: CreateVisitInput) =>
      call<Visit>('POST', `/v1/staff/projects/${projectID}/visits`, input),
    listVisits: (projectID: string) =>
      call<{ items: Visit[] }>('GET', `/v1/staff/projects/${projectID}/visits`),
    getVisit: (visitID: string) => call<Visit>('GET', `/v1/staff/visits/${visitID}`),
    updateVisit: (visitID: string, input: Partial<CreateVisitInput>) =>
      call<Visit>('PATCH', `/v1/staff/visits/${visitID}`, input),
    listVisitAssets: (visitID: string) =>
      call<{ items: Asset[] }>('GET', `/v1/staff/visits/${visitID}/assets`),
    getVisitAnalysis: (visitID: string) =>
      call<VisitAnalysis>('GET', `/v1/staff/visits/${visitID}/analysis`),
    submitVisit: (visitID: string) =>
      call<WorkflowRun>('POST', `/v1/staff/visits/${visitID}:submit`, {}),
    retryVisit: (visitID: string) =>
      call('POST', `/v1/staff/visits/${visitID}:retry`, {}),
    initiateAsset: (projectID: string, input: InitiateAssetInput) =>
      call<UploadTicket>('POST', `/v1/staff/projects/${projectID}/assets:initiate`, input),
    renewAssetUpload: (assetID: string) =>
      call<UploadTicket>('POST', `/v1/staff/assets/${assetID}:renew-upload`, {}),
    deleteAsset: (assetID: string) => call<void>('DELETE', `/v1/staff/assets/${assetID}`),
    completeAsset: (assetID: string, digest: string) =>
      call('POST', `/v1/staff/assets/${assetID}:complete`, { sha256: digest }),
    startWorkflow: (projectID: string) =>
      call<WorkflowRun>('POST', `/v1/staff/projects/${projectID}/workflow:start`),
    getWorkflow: (projectID: string) =>
      call<WorkflowRun>('GET', `/v1/staff/projects/${projectID}/workflow`),
    getLatestArtifact: (projectID: string) =>
      call<Artifact>('GET', `/v1/staff/projects/${projectID}/artifacts/latest`),
    finalizeProject: (projectID: string) =>
      call<WorkflowRun>('POST', `/v1/staff/projects/${projectID}:finalize`, { confirm_materials_ready: true }),
    logout: () => call<void>('POST', '/v1/staff/logout', {}),
    uploadAsset: async (ticket: UploadTicket, filePath: string, contentType: string): Promise<void> => {
      const data = await readFile(filePath)
      const upload = await request({
        method: 'PUT',
        url: ticket.upload_url,
        data,
        header: { 'content-type': contentType }
      })
      if (upload.statusCode < 200 || upload.statusCode >= 300) {
        throw new APIError(upload.statusCode, 'upload_failed', '材料上传失败')
      }
      await call('POST', `/v1/staff/assets/${ticket.asset_id}:complete`, { sha256: sha256(data) })
    }
  }
}

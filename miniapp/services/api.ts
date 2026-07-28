import { sha256 } from 'js-sha256'

export interface PlanItem {
  id: string
  category: string
  prompt: string
  status: 'pending' | 'collected' | 'insufficient' | 'not_needed'
}

export interface Project {
  id: string
  display_name: string
  birth_year: number
  state: string
  collection_plan: PlanItem[]
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

interface RequestOptions {
  url: string
  method: 'GET' | 'POST' | 'PUT'
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
	  method: options.method,
	  data: options.data as string | ArrayBuffer | WechatMiniprogram.IAnyObject | undefined,
	  header: options.header,
      success: (response) => resolve({ statusCode: response.statusCode, data: response.data }),
      fail: reject
    })
  })
}

function wxReadFile(filePath: string): Promise<ArrayBuffer> {
  return new Promise((resolve, reject) => {
    wx.getFileSystemManager().readFile({ filePath, success: (result) => resolve(result.data as ArrayBuffer), fail: reject })
  })
}

export function createAPI({ baseURL, request = wxRequest, readFile = wxReadFile }: { baseURL: string; request?: Request; readFile?: ReadFile }) {
  const call = async <T>(method: RequestOptions['method'], path: string, data?: unknown): Promise<T> => {
    const response = await request({
      method,
      url: `${baseURL}${path}`,
      data,
      header: data === undefined ? undefined : { 'content-type': 'application/json' }
    })
    if (response.statusCode < 200 || response.statusCode >= 300) {
      const body = response.data as { error?: { code?: string; message?: string } }
      throw new APIError(response.statusCode, body.error?.code ?? 'request_failed', body.error?.message ?? '请求失败')
    }
    return response.data as T
  }

  return {
    createProject: (input: CreateProjectInput) => call<Project>('POST', '/v1/staff/projects', input),
    getProject: (projectID: string) => call<Project>('GET', `/v1/staff/projects/${projectID}`),
    initiateAsset: (projectID: string, input: { kind: 'audio' | 'photo'; filename: string; content_type: string; size_bytes: number }) =>
      call<UploadTicket>('POST', `/v1/staff/projects/${projectID}/assets:initiate`, input),
    completeAsset: (assetID: string, sha256: string) => call('POST', `/v1/staff/assets/${assetID}:complete`, { sha256 }),
    startWorkflow: (projectID: string) => call<WorkflowRun>('POST', `/v1/staff/projects/${projectID}/workflow:start`),
    getWorkflow: (projectID: string) => call<WorkflowRun>('GET', `/v1/staff/projects/${projectID}/workflow`),
    getLatestArtifact: (projectID: string) => call<Artifact>('GET', `/v1/staff/projects/${projectID}/artifacts/latest`),
    uploadAsset: async (ticket: UploadTicket, filePath: string, contentType: string): Promise<void> => {
      const data = await readFile(filePath)
      const upload = await request({ method: 'PUT', url: ticket.upload_url, data, header: { 'content-type': contentType } })
      if (upload.statusCode < 200 || upload.statusCode >= 300) {
        throw new APIError(upload.statusCode, 'upload_failed', '物料上传失败')
      }
      await call('POST', `/v1/staff/assets/${ticket.asset_id}:complete`, { sha256: sha256(data) })
    }
  }
}

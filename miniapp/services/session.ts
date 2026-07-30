export type AuthMode = 'dev' | 'wechat'

export interface StaffSummary {
  id: string
  display_name: string
  team_name?: string
  state?: 'active' | 'disabled'
}

export interface LoginResult {
  token: string
  staff: StaffSummary
}

export interface SessionState extends LoginResult {}

export interface SessionStorage {
  load(): unknown
  save(value: SessionState): void
  clear(): void
}

export interface SessionClient {
  getToken(): string | undefined
  getStaff(): StaffSummary | undefined
  ensure(): Promise<string>
  refresh(): Promise<string>
  clear(): void
}

export function createSession({
  storage,
  authenticate
}: {
  storage: SessionStorage
  authenticate: () => Promise<LoginResult>
}): SessionClient {
  let current = parseSession(storage.load())
  let refreshPromise: Promise<string> | undefined

  const refresh = (): Promise<string> => {
    if (refreshPromise) return refreshPromise
    refreshPromise = authenticate()
      .then((result) => {
        current = result
        storage.save(result)
        return result.token
      })
      .finally(() => { refreshPromise = undefined })
    return refreshPromise
  }

  return {
    getToken: () => current?.token,
    getStaff: () => current?.staff,
    ensure: async () => current?.token || refresh(),
    refresh,
    clear: () => {
      current = undefined
      storage.clear()
    }
  }
}

export function createWXSession({
  baseURL,
  mode,
  devDisplayName = '本地采集员'
}: {
  baseURL: string
  mode: AuthMode
  devDisplayName?: string
}): SessionClient {
  const storageKey = 'sangyu.staff-session'
  return createSession({
    storage: {
      load: () => wx.getStorageSync(storageKey),
      save: (value) => wx.setStorageSync(storageKey, value),
      clear: () => wx.removeStorageSync(storageKey)
    },
    authenticate: async () => {
      const path = mode === 'dev' ? '/v1/auth/dev' : '/v1/auth/wechat'
      const data: Record<string, string> = mode === 'dev'
        ? { display_name: devDisplayName }
        : { code: await wxLoginCode() }
      const response = await requestLogin(`${baseURL}${path}`, data)
      if (response.statusCode < 200 || response.statusCode >= 300) {
        const body = response.data as { error?: { message?: string } }
        throw new Error(body.error?.message || '工作人员登录失败')
      }
      return response.data as unknown as LoginResult
    }
  })
}

function parseSession(value: unknown): SessionState | undefined {
  if (!value || typeof value !== 'object') return undefined
  const candidate = value as Partial<SessionState>
  if (typeof candidate.token !== 'string' || !candidate.staff || typeof candidate.staff.id !== 'string') {
    return undefined
  }
  return candidate as SessionState
}

function wxLoginCode(): Promise<string> {
  return new Promise((resolve, reject) => {
    wx.login({
      success: (result) => result.code ? resolve(result.code) : reject(new Error('微信登录未返回 code')),
      fail: reject
    })
  })
}

function requestLogin(url: string, data: Record<string, string>): Promise<WechatMiniprogram.RequestSuccessCallbackResult> {
  return new Promise((resolve, reject) => {
    wx.request({ url, method: 'POST', data, header: { 'content-type': 'application/json' }, success: resolve, fail: reject })
  })
}

import { API_BASE_URL } from '../env'
import { createAPI } from './api'
import { session } from './session-client'

export { session }
export const api = createAPI({ baseURL: API_BASE_URL, session })

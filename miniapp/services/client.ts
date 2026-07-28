import { API_BASE_URL } from '../env'
import { createAPI } from './api'

export const api = createAPI({ baseURL: API_BASE_URL })

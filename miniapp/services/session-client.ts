import { API_BASE_URL, AUTH_MODE, DEV_STAFF_NAME } from '../env'
import { createWXSession } from './session'

export const session = createWXSession({
  baseURL: API_BASE_URL,
  mode: AUTH_MODE,
  devDisplayName: DEV_STAFF_NAME
})

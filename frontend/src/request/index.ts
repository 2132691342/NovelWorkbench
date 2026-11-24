// 请求实例：注入令牌、解包后端包络并处理会话过期。
import { ElMessage } from 'element-plus'
import { createRequest, isCancel, type AxiosResponse, type RequestConfig } from '@novelworkbench/request'
import {
  API_BASE_PATH,
  REQUEST_TIMEOUT,
  RESP_CODE_SUCCESS,
  RESP_CODE_UNAUTHORIZED
} from '@/constants'
import { readToken } from '@/utils/storage'
import { useUserStoreWithOut } from '@/store/modules/user'
import router from '@/router'

/** 后端统一响应包络 */
export interface ApiResponse<Data = unknown> {
  resp_code: string
  resp_message: string
  trace_id: string
  data: Data
}

/** 请求选项：silent 为 true 时失败不弹统一提示（调用方自行处理） */
export interface RequestOptions extends RequestConfig {
  silent?: boolean
}

/** 业务/传输统一错误：resp_code 为字符串业务码，http_status 为传输层状态 */
export class ApiError extends Error {
  resp_code: string
  trace_id: string
  http_status: number

  constructor(resp_code: string, message: string, trace_id: string, http_status: number) {
    super(message || resp_code)
    this.name = 'ApiError'
    this.resp_code = resp_code
    this.trace_id = trace_id
    this.http_status = http_status
  }
}

/** 判定响应体是否为后端包络（网关异常时可能是 HTML 或空体） */
function isEnvelope(value: unknown): value is ApiResponse {
  if (typeof value !== 'object' || value === null) return false
  return 'resp_code' in value && typeof value.resp_code === 'string'
}

/** 把 axios 传输错误归一为 ApiError（携带后端包络时保留业务码与文案） */
function toApiError(error: unknown): ApiError {
  if (error instanceof ApiError) return error
  if (typeof error === 'object' && error !== null && 'response' in error) {
    const response = error.response
    if (typeof response === 'object' && response !== null && 'status' in response) {
      const status = Number(response.status)
      const body = 'data' in response ? response.data : undefined
      if (isEnvelope(body)) {
        return new ApiError(body.resp_code, body.resp_message, body.trace_id, status)
      }
      return new ApiError('ERR_INTERNAL', `请求失败（HTTP ${status}）`, '', status)
    }
  }
  return new ApiError('ERR_INTERNAL', '网络异常，请检查网络连接', '', 0)
}

const client = createRequest({
  axiosConfig: {
    baseURL: API_BASE_PATH,
    timeout: REQUEST_TIMEOUT
  },
  beforeRequest(config) {
    const token = readToken()
    // FormData 必须交给 axios 自行生成带 boundary 的 multipart 头，写死 JSON 会让后端收不到任何字段
    const isFormData = typeof FormData !== 'undefined' && config.data instanceof FormData
    return {
      ...config,
      headers: {
        ...(isFormData ? {} : { 'Content-Type': 'application/json' }),
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        ...config.headers
      }
    }
  },
  transformResponse(response: AxiosResponse) {
    if (response.config.responseType === 'blob') return response

    const envelope = response.data
    if (isEnvelope(envelope) && envelope.resp_code === RESP_CODE_SUCCESS) return envelope.data

    throw new ApiError(
      isEnvelope(envelope) ? envelope.resp_code : 'ERR_INTERNAL',
      isEnvelope(envelope) ? envelope.resp_message : '响应格式异常',
      isEnvelope(envelope) ? envelope.trace_id : '',
      response.status
    )
  }
})

/** 登录相关接口：401 表示凭据错误，不能当成会话过期处理 */
const AUTH_ENTRY_URL = '/auth/login'

/** 会话过期处理：清本地会话并跳登录页；只清一次，避免登出接口 401 造成递归 */
function handleSessionExpired() {
  const userStore = useUserStoreWithOut()
  if (!userStore.isAuthenticated) return
  userStore.clearSession()
  if (router.currentRoute.value.path !== '/login') {
    void router.replace({ path: '/login', query: { redirect: router.currentRoute.value.fullPath } })
  }
}

/** 统一出口：解包后的业务数据直接返回；失败时按 silent 决定是否弹全局提示 */
async function send<Data>(config: RequestOptions, method: 'get' | 'post'): Promise<Data> {
  try {
    return method === 'get'
      ? await client.get<Data>(config)
      : await client.post<Data>(config)
  } catch (rawError) {
    if (isCancel(rawError)) throw rawError
    const apiError = toApiError(rawError)
    if (apiError.resp_code === RESP_CODE_UNAUTHORIZED && config.url !== AUTH_ENTRY_URL) {
      handleSessionExpired()
      ElMessage.error('登录已过期，请重新登录')
      throw apiError
    }
    if (!config.silent) ElMessage.error(apiError.message)
    throw apiError
  }
}

export default {
  get: <Data = unknown>(config: RequestOptions) => send<Data>(config, 'get'),
  post: <Data = unknown>(config: RequestOptions) => send<Data>(config, 'post')
}

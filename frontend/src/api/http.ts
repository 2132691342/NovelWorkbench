// Axios 客户端：统一注入 Bearer Token、解包响应包络、归一错误并弹出提示。

import axios, { AxiosError, type AxiosRequestConfig, type AxiosResponse } from 'axios'
import { ElMessage } from 'element-plus'
import i18n from '@/i18n'
import { clearToken, clearUserVo, readToken } from '@/utils/storage'

/** 业务成功码 */
export const NW_RESP_CODE_SUCCESS = 'SUCCESS'
/** 未授权业务码 */
export const NW_RESP_CODE_UNAUTHORIZED = 'ERR_UNAUTHORIZED'
/** 资源不存在业务码 */
export const NW_RESP_CODE_NOT_FOUND = 'ERR_NOT_FOUND'

/** 后端统一响应包络 */
export interface NWResponseEnvelope<T> {
  resp_code: string
  resp_message: string
  trace_id: string
  data: T
}

/** 请求配置：silent_error 为 true 时失败不弹统一错误提示 */
export interface NWRequestConfig extends AxiosRequestConfig {
  silent_error?: boolean
}

/** 业务与传输统一错误对象 */
export class NWApiError extends Error {
  resp_code: string
  resp_message: string
  trace_id: string
  http_status: number
  notified_flag: boolean

  constructor(resp_code: string, resp_message: string, trace_id: string, http_status: number) {
    super(resp_message || resp_code)
    this.name = 'NWApiError'
    this.resp_code = resp_code
    this.resp_message = resp_message
    this.trace_id = trace_id
    this.http_status = http_status
    this.notified_flag = false
  }
}

const http = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
})

http.interceptors.request.use((config) => {
  const token = readToken()
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

/** 将任意异常转换为统一错误对象 */
function toNWApiError(error: unknown): NWApiError {
  if (error instanceof NWApiError) {
    return error
  }
  const axiosError = error as AxiosError<NWResponseEnvelope<unknown>>
  const envelope = axiosError?.response?.data
  const httpStatus = axiosError?.response?.status ?? 0
  if (envelope && typeof envelope.resp_code === 'string' && envelope.resp_code.length > 0) {
    return new NWApiError(
      envelope.resp_code,
      envelope.resp_message,
      envelope.trace_id ?? '',
      httpStatus,
    )
  }
  if (!axiosError?.response) {
    return new NWApiError('ERR_INTERNAL', i18n.global.t('http.errorNetwork'), '', 0)
  }
  return new NWApiError('ERR_INTERNAL', axiosError.message, '', httpStatus)
}

/** 统一错误提示；未授权时清 token 并跳转登录页 */
function notifyApiError(error: NWApiError, silentError: boolean): void {
  if (error.resp_code === NW_RESP_CODE_UNAUTHORIZED) {
    clearToken()
    clearUserVo()
    ElMessage.error(error.resp_message || i18n.global.t('http.errorUnauthorized'))
    void import('@/router').then((routerModule) => {
      routerModule.default.push('/login')
    })
    return
  }
  if (!silentError) {
    ElMessage.error(error.resp_message || i18n.global.t('http.errorUnknown'))
  }
}

/** 统一失败出口：保证同一错误只提示一次后 reject */
function rejectWithError(error: NWApiError, config: NWRequestConfig | undefined): Promise<never> {
  if (!error.notified_flag) {
    error.notified_flag = true
    notifyApiError(error, config?.silent_error ?? false)
  }
  return Promise.reject(error)
}

http.interceptors.response.use(
  (response: AxiosResponse<NWResponseEnvelope<unknown>>) => {
    const envelope = response.data
    if (!envelope || envelope.resp_code !== NW_RESP_CODE_SUCCESS) {
      return rejectWithError(
        new NWApiError(
          envelope?.resp_code ?? 'ERR_INTERNAL',
          envelope?.resp_message ?? i18n.global.t('http.errorUnknown'),
          envelope?.trace_id ?? '',
          response.status,
        ),
        response.config as NWRequestConfig,
      )
    }
    // 包络解包：业务数据直接作为调用方拿到的结果（axios 拦截器类型无法表达解包，这里做边界转换）
    return envelope.data as unknown as AxiosResponse
  },
  (error: AxiosError<NWResponseEnvelope<unknown>>) => {
    return rejectWithError(toNWApiError(error), error.config as NWRequestConfig | undefined)
  },
)

/** GET 请求：返回包络中的 data */
export async function get<T>(url: string, config?: NWRequestConfig): Promise<T> {
  return http.get<unknown, T>(url, config)
}

/** POST 请求：返回包络中的 data */
export async function post<T>(url: string, data?: unknown, config?: NWRequestConfig): Promise<T> {
  return http.post<unknown, T>(url, data, config)
}

/** 判断错误是否属于"接口尚未就绪"（404） */
export function isApiNotReadyError(error: unknown): boolean {
  return (
    error instanceof NWApiError &&
    (error.resp_code === NW_RESP_CODE_NOT_FOUND || error.http_status === 404)
  )
}


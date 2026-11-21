// 业务请求薄封装：在统一请求客户端之上提供 get/post 便捷函数与静默错误开关。
// 所有业务 api 模块只依赖本文件，不直接触碰 axios。

import request, { ApiError } from '@/request'

export { ApiError }

/** 业务成功码 */
export const NW_RESP_CODE_SUCCESS = 'SUCCESS'
/** 未授权业务码 */
export const NW_RESP_CODE_UNAUTHORIZED = 'ERR_UNAUTHORIZED'
/** 资源不存在业务码 */
export const NW_RESP_CODE_NOT_FOUND = 'ERR_NOT_FOUND'

/** 上传进度回调 */
export type NWUploadProgressHandler = (percent: number) => void

/** 请求配置：silent_error 为 true 时失败不弹统一错误提示 */
export interface NWRequestConfig {
  silent_error?: boolean
  params?: Record<string, unknown>
  responseType?: 'json' | 'blob' | 'text'
  timeout?: number
  onUploadProgress?: NWUploadProgressHandler
}

/** GET 请求，返回解包后的业务数据 */
export function get<T>(url: string, config: NWRequestConfig = {}): Promise<T> {
  return request.get<T>({
    url,
    params: config.params,
    responseType: config.responseType,
    timeout: config.timeout,
    silent: config.silent_error === true
  })
}

/** POST 请求，返回解包后的业务数据（支持 FormData 上传与进度回调） */
export function post<T>(url: string, data?: unknown, config: NWRequestConfig = {}): Promise<T> {
  return request.post<T>({
    url,
    data,
    params: config.params,
    responseType: config.responseType,
    timeout: config.timeout,
    silent: config.silent_error === true,
    onUploadProgress: config.onUploadProgress
      ? (event) => {
          if (event.total) config.onUploadProgress!(Math.round((event.loaded / event.total) * 100))
        }
      : undefined
  })
}

/** 判断错误是否属于「接口尚未就绪」（404） */
export function isApiNotReadyError(error: unknown): boolean {
  return (
    error instanceof ApiError &&
    (error.resp_code === NW_RESP_CODE_NOT_FOUND || error.http_status === 404)
  )
}

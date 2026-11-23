// 请求客户端工厂：前置钩子、响应解包与错误转交。
// 只实现业务实际用到的 GET / POST；流式对话走独立的 SSE 通道，不在此取消。
import axios, {
  type AxiosRequestConfig,
  type AxiosResponse,
  type CreateAxiosDefaults
} from 'axios'

type MaybePromise<Value> = Value | Promise<Value>

export type RequestConfig<Data = unknown> = AxiosRequestConfig<Data>

export interface CreateRequestOptions {
  axiosConfig?: CreateAxiosDefaults
  beforeRequest?: <Data>(config: RequestConfig<Data>) => MaybePromise<RequestConfig<Data>>
  transformResponse?: (response: AxiosResponse) => MaybePromise<unknown>
  onError?: (error: unknown) => MaybePromise<void>
}

export interface RequestClient {
  request: <Response = unknown, Data = unknown>(config: RequestConfig<Data>) => Promise<Response>
  get: <Response = unknown>(config: RequestConfig) => Promise<Response>
  post: <Response = unknown, Data = unknown>(config: RequestConfig<Data>) => Promise<Response>
}

/** createRequest 构造请求客户端：注入前置钩子、解包响应并转交错误。 */
export const createRequest = (options: CreateRequestOptions = {}): RequestClient => {
  const instance = axios.create(options.axiosConfig)

  const request = async <Response = unknown, Data = unknown>(
    config: RequestConfig<Data>
  ): Promise<Response> => {
    let prepared: RequestConfig<Data> = config
    try {
      prepared = options.beforeRequest ? await options.beforeRequest(config) : config
    } catch (error) {
      await options.onError?.(error)
      throw error
    }

    try {
      const response = await instance.request(prepared)
      return (
        options.transformResponse ? await options.transformResponse(response) : response.data
      ) as Response
    } catch (error) {
      await options.onError?.(error)
      throw error
    }
  }

  return {
    request,
    get: (config) => request({ ...config, method: 'get' }),
    post: (config) => request({ ...config, method: 'post' })
  }
}

export type {
  AxiosError,
  AxiosRequestConfig,
  AxiosResponse,
  InternalAxiosRequestConfig,
  RawAxiosRequestHeaders
} from 'axios'
export { isCancel } from 'axios'

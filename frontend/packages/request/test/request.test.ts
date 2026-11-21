// 请求客户端链路的回归测试：前置钩子 → 响应解包 → 错误转交 → 并发取消。
// 所有业务请求都经过这一层，头部注入或错误转交失效会波及每个页面，因此整体守住。

import { describe, expect, it } from 'vitest'
import { CanceledError } from 'axios'
import { createRequest, type AxiosResponse } from '../src/index'

describe('请求客户端链路', () => {
  it('前置钩子注入头部，响应被解包，业务错误转交 onError，同键并发请求可一并取消', async () => {
    // 前置钩子与响应解包：请求头被注入，返回值是解包后的业务数据。
    let receivedHeader: unknown
    const client = createRequest({
      axiosConfig: {
        adapter: async (config) => {
          receivedHeader = config.headers.Authorization
          return { data: { code: 0, data: 'ok' }, status: 200, statusText: 'OK', headers: {}, config }
        }
      },
      beforeRequest: (config) => ({
        ...config,
        headers: { ...config.headers, Authorization: 'token' }
      }),
      transformResponse: (response: AxiosResponse) => response.data.data
    })

    expect(await client.get<string>({ url: '/users' })).toBe('ok')
    expect(receivedHeader).toBe('token')

    // 业务错误：transformResponse 抛出的错误原样交给 onError 并向调用方冒泡。
    const businessError = new Error('business failed')
    let handledError: unknown
    const failingClient = createRequest({
      axiosConfig: {
        adapter: async (config) => ({
          data: { code: 1 },
          status: 200,
          statusText: 'OK',
          headers: {},
          config
        })
      },
      transformResponse: () => {
        throw businessError
      },
      onError: (error) => {
        handledError = error
      }
    })

    await expect(failingClient.get({ url: '/failed' })).rejects.toBe(businessError)
    expect(handledError).toBe(businessError)

    // 并发取消：同键请求共享取消信号，cancelRequest 让在途请求一起以取消码失败。
    const signals: AbortSignal[] = []
    const cancelClient = createRequest({
      axiosConfig: {
        adapter: (config) =>
          new Promise((_resolve, reject) => {
            const signal = config.signal as AbortSignal
            signals.push(signal)
            signal.addEventListener('abort', () => reject(new CanceledError()), { once: true })
          })
      }
    })

    const first = cancelClient.get({ url: '/slow' })
    const second = cancelClient.get({ url: '/slow' })
    cancelClient.cancelRequest('/slow')

    await expect(first).rejects.toMatchObject({ code: 'ERR_CANCELED' })
    await expect(second).rejects.toMatchObject({ code: 'ERR_CANCELED' })
    expect(signals.length).toBe(2)
    expect(signals.every((signal) => signal.aborted)).toBe(true)
  })
})

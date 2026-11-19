// SSE 流式解析与请求（POST /chat/send 用 fetch + ReadableStream，axios 不支持流式响应）
import { ref, type Ref } from 'vue'
import type { NWChatStreamEventVo } from '@/api/contracts'
import { readToken } from '@/utils/storage'

/** SSE data 行前缀 */
const SSE_DATA_PREFIX = 'data:'

/** 解析结果：本轮完整事件 + 留待下一轮的半行缓冲 */
export interface SseParseResult {
  eventList: NWChatStreamEventVo[]
  restText: string
}

/**
 * 纯函数：从累积文本中解析 SSE 事件。
 * 只消费完整行（以 \n 结尾），末尾半行留在 restText，跨 chunk 断行与多事件粘包都能正确处理。
 */
export function parseSseBuffer(bufferText: string): SseParseResult {
  const eventList: NWChatStreamEventVo[] = []
  let restText = bufferText
  let lineEndIndex = restText.indexOf('\n')
  while (lineEndIndex >= 0) {
    const lineText = restText.slice(0, lineEndIndex).replace(/\r$/, '')
    restText = restText.slice(lineEndIndex + 1)
    if (lineText.startsWith(SSE_DATA_PREFIX)) {
      const dataText = lineText.slice(SSE_DATA_PREFIX.length).trimStart()
      if (dataText.length > 0) {
        try {
          eventList.push(JSON.parse(dataText) as NWChatStreamEventVo)
        } catch {
          // 非法 JSON 行跳过
        }
      }
    }
    lineEndIndex = restText.indexOf('\n')
  }
  return { eventList, restText }
}

/** 流式请求回调 */
export interface SseStreamHandlers {
  onEvent: (event: NWChatStreamEventVo) => void
  onError: (error: Error) => void
}

/**
 * 以 fetch 流式 POST 发起 SSE 请求并逐块解析。
 * 网络异常/HTTP 异常走 onError，主动 abort 静默结束。
 */
export async function streamSsePost(
  urlPath: string,
  body: unknown,
  handlers: SseStreamHandlers,
  signal: AbortSignal,
): Promise<void> {
  try {
    const response = await fetch(`/api/v1${urlPath}`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${readToken()}`,
      },
      body: JSON.stringify(body),
      signal,
    })
    if (!response.ok || !response.body) {
      throw new Error(`SSE request failed with status ${response.status}`)
    }
    const reader = response.body.getReader()
    const decoder = new TextDecoder()
    let bufferText = ''
    for (;;) {
      const { done, value } = await reader.read()
      if (done) {
        break
      }
      bufferText += decoder.decode(value, { stream: true })
      const parseResult = parseSseBuffer(bufferText)
      bufferText = parseResult.restText
      parseResult.eventList.forEach(handlers.onEvent)
    }
    // 流结束：末尾可能没有换行的 data 行补一个换行再解析一次
    parseSseBuffer(`${bufferText}\n`).eventList.forEach(handlers.onEvent)
  } catch (error) {
    // 主动取消（AbortController）不视为失败
    if (error instanceof DOMException && error.name === 'AbortError') {
      return
    }
    handlers.onError(error instanceof Error ? error : new Error(String(error)))
  }
}

/**
 * SSE 流式请求 composable：管理流式状态与中止（停止生成按钮）。
 * 重复调用 startStream 会先中止上一条流。
 */
export function useSSE(): {
  streamingFlag: Ref<boolean>
  startStream: (
    urlPath: string,
    body: unknown,
    handlers: SseStreamHandlers,
  ) => Promise<void>
  stopStream: () => void
} {
  const streamingFlag = ref(false)
  let abortController: AbortController | null = null

  async function startStream(
    urlPath: string,
    body: unknown,
    handlers: SseStreamHandlers,
  ): Promise<void> {
    stopStream()
    abortController = new AbortController()
    streamingFlag.value = true
    try {
      await streamSsePost(urlPath, body, handlers, abortController.signal)
    } finally {
      streamingFlag.value = false
      abortController = null
    }
  }

  function stopStream(): void {
    abortController?.abort()
    abortController = null
    streamingFlag.value = false
  }

  return { streamingFlag, startStream, stopStream }
}

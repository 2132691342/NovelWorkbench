// 流式事件到消息展示态的归并规则：thinking 累加思考、chunk 累加正文、
// done 用服务端全量覆盖并补上完整思考与召回依据、error 记录错误文案。
// 抽成纯函数后，界面表现与「后端事件契约」可以被一条用例同时守住。

import type { NWChatStreamEventVo } from '@/api/contracts'
import type { ChatDisplayMessage } from './types'

/**
 * 把一条流式事件归并到消息展示态上。
 * 返回 false 表示该事件不改写消息（未知事件类型）。
 */
export function applyStreamEvent(
  message: ChatDisplayMessage,
  event: NWChatStreamEventVo,
  fallbackErrorText: string,
): boolean {
  if (event.event_type === 'thinking') {
    message.reasoning_text += event.content_text
    return true
  }
  if (event.event_type === 'chunk') {
    message.content_text += event.content_text
    return true
  }
  if (event.event_type === 'done') {
    message.content_text = event.content_text
    message.reasoning_text = event.reasoning_text || message.reasoning_text
    message.reference_list = event.reference_list ?? []
    return true
  }
  if (event.event_type === 'error') {
    message.error_message = event.error_message || fallbackErrorText
    return true
  }
  return false
}

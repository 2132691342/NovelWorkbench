// 流式事件归并链路的回归测试：thinking 累加 → chunk 累加 → done 全量覆盖 → error 兜底文案。
// 前端据此切换「等待 → 思考 → 正文 → 完成」，归并偏差会让正文重复累加或思考被清空，因此整体守住。
import { describe, expect, it } from 'vitest'
import { applyStreamEvent } from '@/views/workspace/chatStreamState'
import type { NWChatDisplayMessage } from '@/views/workspace/chatTypes'
import type { NWChatStreamEventVo } from '@/api/contracts'

/** 构造一条空白助手消息。 */
function blankMessage(): NWChatDisplayMessage {
  return {
    message_ulid: 'MSG_1',
    role_code: 'assistant',
    content_text: '',
    reasoning_text: '',
    skill_code: '',
    created_at: '',
    reference_list: [],
    error_message: '',
    streaming_flag: true,
    waited_ms: 0,
    retry_payload: null,
  }
}

/** 构造一条流式事件（只填关心的字段）。 */
function streamEvent(overrides: Partial<NWChatStreamEventVo>): NWChatStreamEventVo {
  return {
    seq: 1,
    event_type: 'chunk',
    session_ulid: 'SES_1',
    message_ulid: 'MSG_1',
    content_text: '',
    reasoning_text: '',
    reference_list: [],
    error_message: '',
    ...overrides,
  }
}

describe('applyStreamEvent 流式事件归并链路', () => {
  it('思考与正文分别累加，done 用服务端全量覆盖并补齐召回依据，error 落兜底文案', () => {
    const message = blankMessage()

    // 思考增量与正文增量分别累加，互不污染。
    applyStreamEvent(message, streamEvent({ event_type: 'thinking', content_text: '先想' }), '发送失败')
    applyStreamEvent(message, streamEvent({ event_type: 'thinking', content_text: '一步' }), '发送失败')
    applyStreamEvent(message, streamEvent({ event_type: 'chunk', content_text: '正文' }), '发送失败')
    expect(message.reasoning_text).toBe('先想一步')
    expect(message.content_text).toBe('正文')

    // done 用服务端全量正文覆盖（不与累加值叠加），并带回完整思考与召回依据。
    applyStreamEvent(
      message,
      streamEvent({
        event_type: 'done',
        content_text: '最终正文',
        reasoning_text: '完整思考',
        reference_list: [
          {
            chunk_ulid: 'CHK_1',
            doc_ulid: 'DOC_1',
            doc_name: '设定集',
            chunk_text: '命中片段',
            chunk_index: 0,
            score_value: 0.5,
            source_mode: 'hybrid',
          },
        ],
      }),
      '发送失败',
    )
    expect(message.content_text).toBe('最终正文')
    expect(message.reasoning_text).toBe('完整思考')
    expect(message.reference_list).toHaveLength(1)
    expect(message.reference_list[0].chunk_ulid).toBe('CHK_1')

    // done 未带思考时保留已累加的思考，不清空。
    const second = blankMessage()
    applyStreamEvent(second, streamEvent({ event_type: 'thinking', content_text: '思考A' }), '发送失败')
    applyStreamEvent(second, streamEvent({ event_type: 'done', content_text: '正文B' }), '发送失败')
    expect(second.reasoning_text).toBe('思考A')

    // error 优先用服务端文案；服务端未给文案时回退到界面文案。
    const third = blankMessage()
    applyStreamEvent(third, streamEvent({ event_type: 'error', error_message: '模型不可用' }), '发送失败')
    expect(third.error_message).toBe('模型不可用')
    const fourth = blankMessage()
    applyStreamEvent(fourth, streamEvent({ event_type: 'error' }), '发送失败')
    expect(fourth.error_message).toBe('发送失败')

    // 未知事件类型不改写消息。
    const fifth = blankMessage()
    expect(applyStreamEvent(fifth, streamEvent({ event_type: 'unknown' }), '发送失败')).toBe(false)
    expect(fifth.content_text).toBe('')
  })
})

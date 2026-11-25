// 流式对话链路的回归：SSE 报文解析 → 事件归并到消息展示态。
// 解析偏差会丢事件或重复事件，归并偏差会让正文重复累加或思考被清空，两者直接决定 AI 回复能否正确落到界面。
import { describe, expect, it } from 'vitest'
import { parseSseBuffer } from '@/composables/useSSE'
import { applyStreamEvent } from './chatStream'
import type { ChatDisplayMessage } from './types'
import type { NWChatStreamEventVo } from '@/api/contracts'

/** 构造一条空白助手消息。 */
function blankMessage(): ChatDisplayMessage {
  return {
    message_ulid: 'MSG_1',
    role_code: 'assistant',
    content_text: '',
    reasoning_text: '',
    prompt_code: '',
    tool_code_list: [],
    created_at: '',
    reference_list: [],
    error_message: '',
    streaming_flag: true,
    waited_ms: 0,
    retry_payload: null
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
    tool_code_list: [],
    error_message: '',
    ...overrides
  }
}

describe('流式对话链路', () => {
  it('报文解析：跨 chunk 断行与多事件粘包应正确拆分，非法行跳过，半行留存', () => {
    // 第一段：一个完整事件 + 一个被截断的半行
    const first = parseSseBuffer('data: {"seq":1,"event_type":"chunk"}\ndata: {"seq":2,"event_ty')
    expect(first.eventList).toHaveLength(1)
    expect(first.eventList[0].seq).toBe(1)
    expect(first.restText).toBe('data: {"seq":2,"event_ty')

    // 第二段：半行补全 + 紧随其后的完整事件（粘包）+ CRLF 换行 + 非法 JSON 行
    const second = parseSseBuffer(
      `${first.restText}pe":"done","content_text":"完成"}\n` +
        'data: {"seq":3,"event_type":"chunk"}\r\n' +
        'data: {非法 JSON}\n' +
        ': ping\n'
    )
    expect(second.eventList.map((item) => item.seq)).toEqual([2, 3])
    expect(second.eventList[0].content_text).toBe('完成')
    expect(second.restText).toBe('')

    // 末尾半行必须留存，供下一段拼接
    const third = parseSseBuffer('data: {"seq":6')
    expect(third.eventList).toHaveLength(0)
    expect(third.restText).toBe('data: {"seq":6')
  })

  it('事件归并：思考与正文分别累加，done 全量覆盖并补齐依据与工具，error 落兜底文案', () => {
    const message = blankMessage()

    // 思考增量与正文增量分别累加，互不污染。
    applyStreamEvent(message, streamEvent({ event_type: 'thinking', content_text: '先想' }), '发送失败')
    applyStreamEvent(message, streamEvent({ event_type: 'thinking', content_text: '一步' }), '发送失败')
    applyStreamEvent(message, streamEvent({ event_type: 'chunk', content_text: '正文' }), '发送失败')
    expect(message.reasoning_text).toBe('先想一步')
    expect(message.content_text).toBe('正文')

    // done 用服务端全量正文覆盖（不与累加值叠加），并带回完整思考、召回依据与工具编码。
    applyStreamEvent(
      message,
      streamEvent({
        event_type: 'done',
        content_text: '最终正文',
        reasoning_text: '完整思考',
        tool_code_list: ['deai_scan', 'chapter_check'],
        reference_list: [
          {
            chunk_ulid: 'CHK_1',
            doc_ulid: 'DOC_1',
            doc_name: '设定集',
            chunk_text: '命中片段',
            chunk_index: 0,
            score_value: 0.5,
            source_mode: 'hybrid'
          }
        ]
      }),
      '发送失败'
    )
    expect(message.content_text).toBe('最终正文')
    expect(message.reasoning_text).toBe('完整思考')
    expect(message.reference_list).toHaveLength(1)
    expect(message.tool_code_list).toEqual(['deai_scan', 'chapter_check'])

    // done 未带思考时保留已累加的思考；未带工具时保留本地值。
    const second = blankMessage()
    second.tool_code_list = ['deai_scan']
    applyStreamEvent(second, streamEvent({ event_type: 'thinking', content_text: '思考A' }), '发送失败')
    applyStreamEvent(second, streamEvent({ event_type: 'done', content_text: '正文B' }), '发送失败')
    expect(second.reasoning_text).toBe('思考A')
    expect(second.tool_code_list).toEqual(['deai_scan'])

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

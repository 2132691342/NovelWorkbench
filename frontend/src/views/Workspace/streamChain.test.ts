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
    skill_code: '',
    tool_code_list: [],
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
    tool_code_list: [],
    error_message: '',
    ...overrides,
  }
}

describe('流式对话链路', () => {
  it('报文解析：跨 chunk 断行、多事件粘包、CRLF、非法 JSON 与尾部半行应被正确处理', () => {
    // 第一段：一个完整事件 + 一个被截断的半行
    const first = parseSseBuffer('data: {"seq":1,"event_type":"chunk"}\ndata: {"seq":2,"event_ty')
    expect(first.eventList).toHaveLength(1)
    expect(first.eventList[0].seq).toBe(1)
    expect(first.eventList[0].event_type).toBe('chunk')
    expect(first.restText).toBe('data: {"seq":2,"event_ty')

    // 第二段：半行补全 + 紧随其后的完整事件（粘包）+ CRLF 换行 + 非法 JSON 行
    const second = parseSseBuffer(
      `${first.restText}pe":"done","content_text":"完成"}\n` +
        'data: {"seq":3,"event_type":"chunk"}\r\n' +
        'data: {非法 JSON}\n',
    )
    expect(second.eventList).toHaveLength(2)
    expect(second.eventList[0].seq).toBe(2)
    expect(second.eventList[0].event_type).toBe('done')
    expect(second.eventList[0].content_text).toBe('完成')
    expect(second.eventList[1].seq).toBe(3)
    expect(second.restText).toBe('')

    // 非法 JSON 行被跳过且不中断后续解析
    const third = parseSseBuffer('data: not-json\ndata: {"seq":4,"event_type":"done"}\n')
    expect(third.eventList).toHaveLength(1)
    expect(third.eventList[0].seq).toBe(4)

    // 非 data 前缀行（心跳注释等）不产生事件
    const fourth = parseSseBuffer(': ping\ndata: {"seq":5,"event_type":"chunk"}\n')
    expect(fourth.eventList).toHaveLength(1)
    expect(fourth.eventList[0].seq).toBe(5)

    // 末尾半行必须留存，供下一段拼接
    const fifth = parseSseBuffer('data: {"seq":6')
    expect(fifth.eventList).toHaveLength(0)
    expect(fifth.restText).toBe('data: {"seq":6')
  })

  it('事件归并：思考与正文分别累加，done 用服务端全量覆盖并补齐召回依据，error 落兜底文案', () => {
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

    // done 携带的服务端工具编码覆盖本地载荷：关键词自动触发的工具只有服务端知道。
    const toolMessage = blankMessage()
    toolMessage.tool_code_list = ['deai_scan']
    applyStreamEvent(
      toolMessage,
      streamEvent({
        event_type: 'done',
        content_text: '正文',
        tool_code_list: ['deai_scan', 'chapter_check'],
      }),
      '发送失败',
    )
    expect(toolMessage.tool_code_list).toEqual(['deai_scan', 'chapter_check'])
    // 服务端未给工具时保留本地值，不清空。
    const keepMessage = blankMessage()
    keepMessage.tool_code_list = ['deai_scan']
    applyStreamEvent(keepMessage, streamEvent({ event_type: 'done', content_text: '正文' }), '发送失败')
    expect(keepMessage.tool_code_list).toEqual(['deai_scan'])

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

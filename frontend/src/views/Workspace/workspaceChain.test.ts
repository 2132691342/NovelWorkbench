// 写作台对话链路的回归：SSE 报文解析 → 事件归并到展示态 → 斜杠命令解析与注册表。
// 三者是同一次「作者发出指令、模型流式回话」的前后端两半：报文解析偏差会丢事件或重复事件，
// 归并偏差会让正文重复累加或思考被清空，命令解析偏差会让本地动作被误发给模型。

import { describe, expect, it } from 'vitest'
import { parseSseBuffer } from '@/composables/useSSE'
import { applyStreamEvent } from './chatStream'
import type { ChatDisplayMessage } from './types'
import type { NWChatStreamEventVo } from '@/api/contracts'
import {
  COMPOSER_ACTION_LIST,
  actionCodeSet,
  filterActions,
  matchActionByCode,
  parseLeadingSlashToken
} from './composerCommands'

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

describe('写作台对话链路', () => {
  it('流式输出：报文跨 chunk 拆分正确，事件归并后思考与正文互不污染', () => {
    // 环节1：报文解析。第一段是一个完整事件加一个被截断的半行。
    const first = parseSseBuffer('data: {"seq":1,"event_type":"chunk"}\ndata: {"seq":2,"event_ty')
    expect(first.eventList).toHaveLength(1)
    expect(first.eventList[0].seq).toBe(1)
    expect(first.restText).toBe('data: {"seq":2,"event_ty')

    // 环节2：半行补全 + 紧随其后的完整事件（粘包）+ CRLF 换行 + 非法 JSON 行。
    const second = parseSseBuffer(
      `${first.restText}pe":"done","content_text":"完成"}\n` +
        'data: {"seq":3,"event_type":"chunk"}\r\n' +
        'data: {非法 JSON}\n' +
        ': ping\n'
    )
    expect(second.eventList.map((item) => item.seq)).toEqual([2, 3])
    expect(second.eventList[0].content_text).toBe('完成')
    expect(second.restText).toBe('')

    // 环节3：末尾半行必须留存，供下一段拼接。
    const third = parseSseBuffer('data: {"seq":6')
    expect(third.eventList).toHaveLength(0)
    expect(third.restText).toBe('data: {"seq":6')

    // 环节4：事件归并。思考增量与正文增量分别累加，互不污染。
    const message = blankMessage()
    applyStreamEvent(message, streamEvent({ event_type: 'thinking', content_text: '先想' }), '发送失败')
    applyStreamEvent(message, streamEvent({ event_type: 'thinking', content_text: '一步' }), '发送失败')
    applyStreamEvent(message, streamEvent({ event_type: 'chunk', content_text: '正文' }), '发送失败')
    expect(message.reasoning_text).toBe('先想一步')
    expect(message.content_text).toBe('正文')

    // 环节5：done 用服务端全量正文覆盖（不与累加值叠加），并带回完整思考、召回依据与工具编码。
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

    // 环节6：done 未带思考或工具时保留已累加的本地值。
    const second2 = blankMessage()
    second2.tool_code_list = ['deai_scan']
    applyStreamEvent(second2, streamEvent({ event_type: 'thinking', content_text: '思考A' }), '发送失败')
    applyStreamEvent(second2, streamEvent({ event_type: 'done', content_text: '正文B' }), '发送失败')
    expect(second2.reasoning_text).toBe('思考A')
    expect(second2.tool_code_list).toEqual(['deai_scan'])

    // 环节7：error 优先用服务端文案，未给文案时回退到界面文案；未知事件不改写消息。
    const third2 = blankMessage()
    applyStreamEvent(third2, streamEvent({ event_type: 'error', error_message: '模型不可用' }), '发送失败')
    expect(third2.error_message).toBe('模型不可用')
    const fourth = blankMessage()
    applyStreamEvent(fourth, streamEvent({ event_type: 'error' }), '发送失败')
    expect(fourth.error_message).toBe('发送失败')
    const fifth = blankMessage()
    expect(applyStreamEvent(fifth, streamEvent({ event_type: 'unknown' }), '发送失败')).toBe(false)
    expect(fifth.content_text).toBe('')
  })

  it('斜杠命令：首 token 解析与正文剥离，注册表编码唯一且不与内置提示词撞车', () => {
    // 解析口径必须与后端一致，否则同一句 /polish 在前后端会走向不同分支。
    expect(parseLeadingSlashToken('/polish 加强画面感')).toEqual({
      actionCode: 'polish',
      remainderText: '加强画面感'
    })
    expect(parseLeadingSlashToken('  /clear  ')).toEqual({ actionCode: 'clear', remainderText: '' })
    expect(parseLeadingSlashToken('续写 /continue')).toBeNull()
    expect(parseLeadingSlashToken('/ 加强画面感')).toBeNull()
    expect(parseLeadingSlashToken('/po!lish')).toBeNull()
    expect(parseLeadingSlashToken('')).toBeNull()
    // 超长编码不匹配，避免把整段路径当命令
    expect(parseLeadingSlashToken(`/${'a'.repeat(33)} 正文`)).toBeNull()

    // 注册表：编码唯一、可按编码取回、关键词过滤命中编码与名称。
    const codeSet = actionCodeSet()
    expect(codeSet.size).toBe(COMPOSER_ACTION_LIST.length)
    for (const action of COMPOSER_ACTION_LIST) {
      expect(matchActionByCode(action.action_code)?.action_code).toBe(action.action_code)
    }
    expect(matchActionByCode('not_exist')).toBeNull()
    expect(filterActions('clear').map((action) => action.action_code)).toContain('clear')
    expect(filterActions('体检').map((action) => action.action_code)).toContain('check')
    expect(filterActions('不存在的东西')).toHaveLength(0)

    // 命名空间：动作编码一旦与内置提示词撞车，同一句输入会因实现顺序走向两种结果。
    for (const promptCode of ['continue', 'polish', 'deai', 'review', 'hitscan', 'summary']) {
      expect(codeSet.has(promptCode)).toBe(false)
    }
    expect(codeSet.has('snapshot')).toBe(true)
  })
})

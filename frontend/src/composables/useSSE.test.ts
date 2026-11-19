// SSE 事件解析链路的回归测试：跨 chunk 断行、多事件粘包、CRLF 兼容、非法 JSON 跳过、尾部半行留存。
// 这段解析直接决定 AI 流式回复能否正确落到界面，解析偏差会造成事件丢失或重复，因此整体作为一条用例守住。
import { describe, expect, it } from 'vitest'
import { parseSseBuffer } from '@/composables/useSSE'

describe('parseSseBuffer 事件解析链路', () => {
  it('跨 chunk 断行、多事件粘包、CRLF、非法 JSON 与尾部半行应被正确处理', () => {
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
})
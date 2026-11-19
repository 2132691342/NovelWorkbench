// 创作笔记双模式正文转换的回归测试。
// 契约：切换编辑器不能把 HTML 标签漏进 Markdown 编辑器，也不能把 Markdown 原文当 HTML 渲染。
import { describe, expect, it } from 'vitest'
import { convertNoteContent, looksLikeHtml } from '@/views/notes/noteContent'

describe('convertNoteContent 笔记正文格式转换', () => {
  it('富文本 HTML 转 Markdown 时不应残留标签，空段落应被清掉', () => {
    // WangEditor 空笔记的真实产物：只有空段落
    expect(convertNoteContent('<p><br></p>', 'markdown')).toBe('')

    const markdownText = convertNoteContent(
      '<h2>雾港</h2><p>常年被海雾笼罩。</p><ul><li>上城</li><li>下城</li></ul>',
      'markdown',
    )
    expect(markdownText).toContain('## 雾港')
    expect(markdownText).toContain('常年被海雾笼罩。')
    // turndown 的列表缩进宽度不固定，只校验是列表项且不含标签
    expect(markdownText).toMatch(/-\s+上城/)
    expect(markdownText).toMatch(/-\s+下城/)
    expect(markdownText).not.toMatch(/<\/?[a-z]/i)
  })

  it('Markdown 原文转富文本时应渲染为 HTML，且不重复包裹', () => {
    const htmlText = convertNoteContent('## 雾港\n\n常年被海雾笼罩。', 'rich')
    expect(htmlText).toContain('<h2')
    expect(htmlText).toContain('常年被海雾笼罩。')
    expect(looksLikeHtml(htmlText)).toBe(true)
  })

  it('已是目标格式时原样返回，避免二次转换丢格式', () => {
    const htmlSource = '<p>已经是富文本</p>'
    expect(convertNoteContent(htmlSource, 'rich')).toBe(htmlSource)

    const markdownSource = '# 已经是 Markdown'
    expect(convertNoteContent(markdownSource, 'markdown')).toBe(markdownSource)
  })

  it('空正文在两种格式下都保持为空串', () => {
    expect(convertNoteContent('', 'markdown')).toBe('')
    expect(convertNoteContent('   ', 'rich')).toBe('')
  })
})

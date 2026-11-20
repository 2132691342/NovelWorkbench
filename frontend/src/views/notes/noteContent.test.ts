// 创作笔记双模式正文转换的回归测试。
// 契约：切换编辑器不能把 HTML 标签漏进 Markdown 编辑器，也不能把 Markdown 原文当 HTML 渲染。
import { describe, expect, it } from 'vitest'
import { convertNoteContent, looksLikeHtml } from '@/views/notes/noteContent'

describe('convertNoteContent 笔记正文格式转换', () => {
  it('HTML 与 Markdown 双向转换不串格式，空正文与同格式输入保持原样', () => {
    // WangEditor 空笔记的真实产物：只有空段落
    expect(convertNoteContent('<p><br></p>', 'markdown')).toBe('')
    expect(convertNoteContent('', 'markdown')).toBe('')
    expect(convertNoteContent('   ', 'rich')).toBe('')

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

    const htmlText = convertNoteContent('## 雾港\n\n常年被海雾笼罩。', 'rich')
    expect(htmlText).toContain('<h2')
    expect(htmlText).toContain('常年被海雾笼罩。')
    expect(looksLikeHtml(htmlText)).toBe(true)

    // 已是目标格式时原样返回，避免二次转换丢格式
    expect(convertNoteContent('<p>已经是富文本</p>', 'rich')).toBe('<p>已经是富文本</p>')
    expect(convertNoteContent('# 已经是 Markdown', 'markdown')).toBe('# 已经是 Markdown')
  })
})

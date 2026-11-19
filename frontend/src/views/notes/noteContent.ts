// 创作笔记正文格式转换：rich（富文本 HTML）与 markdown（原文）双模式互转。
// 笔记以单一 content_text 字段存储，切换编辑器时必须转换，否则 HTML 标签会出现在 Markdown 编辑器里。
import TurndownService from 'turndown'
import { gfm } from 'turndown-plugin-gfm'
import { marked } from 'marked'

/** 笔记正文格式 */
export type NWNoteContentMode = 'rich' | 'markdown'

/** HTML → Markdown 转换器（GFM：表格、删除线、任务列表） */
const htmlToMarkdownConverter = new TurndownService({
  headingStyle: 'atx',
  codeBlockStyle: 'fenced',
  bulletListMarker: '-',
})
htmlToMarkdownConverter.use(gfm)

/** 判断正文是否含 HTML 标签（而非 Markdown 原文） */
export function looksLikeHtml(contentText: string): boolean {
  return /<\/?[a-z][\s\S]*>/i.test(contentText)
}

/**
 * 按目标格式转换正文。
 * rich→markdown 走 turndown（`<p><br></p>` 这类空段落会被清成空串，不残留标签）；
 * markdown→rich 走 marked；已是目标格式时原样返回，避免二次转换丢格式。
 */
export function convertNoteContent(contentText: string, targetMode: NWNoteContentMode): string {
  if (!contentText.trim()) {
    return ''
  }
  const isHtml = looksLikeHtml(contentText)
  if (targetMode === 'markdown') {
    return isHtml ? htmlToMarkdownConverter.turndown(contentText).trim() : contentText
  }
  return isHtml ? contentText : String(marked.parse(contentText, { async: false }))
}

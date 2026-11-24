// 输入区命令注册表：斜杠动作（本地执行，不消耗模型）的唯一事实来源。
// 斜杠还有第二类来源——提示词库的 command 类提示词（/polish 等），由后端资产驱动；
// 本文件只管「不经过模型、直接改界面或写作台状态」的动作，二者的边界见 doc/frontend.md。

/** 动作作用域：谁负责执行 */
export type ComposerActionScope = 'composer' | 'panel' | 'workspace'

/** 一个斜杠动作的定义 */
export interface ComposerAction {
  action_code: string // 动作编码（对话中输入 /code 触发）
  action_name: string // 动作名称
  action_desc: string // 一句话说明
  action_icon: string // 图标（mdi 名）
  action_scope: ComposerActionScope // composer=输入区自理；panel=AI 面板；workspace=写作台
}

/** 全部斜杠动作（顺序即弹层顺序；编码不可与提示词编码冲突，见下方 ASSERTS） */
export const COMPOSER_ACTION_LIST: ComposerAction[] = [
  {
    action_code: 'help',
    action_name: '命令速查',
    action_desc: '查看全部 / 动作、/ 提示词、# 工具与 @ 引用范围',
    action_icon: 'mdi:help-circle-outline',
    action_scope: 'panel'
  },
  {
    action_code: 'clear',
    action_name: '清空输入',
    action_desc: '清空正文、命令徽标与全部引用',
    action_icon: 'mdi:eraser-variant',
    action_scope: 'composer'
  },
  {
    action_code: 'new',
    action_name: '新会话',
    action_desc: '开一个空会话，旧会话保留可切回',
    action_icon: 'mdi:plus-box-outline',
    action_scope: 'panel'
  },
  {
    action_code: 'attach',
    action_name: '附带当前章',
    action_desc: '切换是否把当前章节正文并入本轮上下文',
    action_icon: 'mdi:paperclip',
    action_scope: 'panel'
  },
  {
    action_code: 'copy',
    action_name: '复制回复',
    action_desc: '复制上一条 AI 回复全文',
    action_icon: 'mdi:content-copy',
    action_scope: 'panel'
  },
  {
    action_code: 'check',
    action_name: '章节体检',
    action_desc: '对当前章跑量化门禁，列出问题清单',
    action_icon: 'mdi:clipboard-check-outline',
    action_scope: 'workspace'
  },
  {
    action_code: 'focus',
    action_name: '专注模式',
    action_desc: '收起目录树与 AI 面板，只留正文',
    action_icon: 'mdi:book-open-variant',
    action_scope: 'workspace'
  }
]

/** 首 token 斜杠命令：仅匹配 [a-z0-9_]{2,32}，且后面必须是空白或行尾。
 * 与后端 parseSlashToken 同一口径，避免前端把「/ 符号」这类正文当成命令。 */
const LEADING_SLASH_REGEXP = /^\/([a-z0-9_]{2,32})(?=\s|$)/

/** 解析正文首 token 的 /code；不是命令时返回 null */
export function parseLeadingSlashToken(contentText: string): {
  actionCode: string
  remainderText: string
} | null {
  const trimmed = contentText.trim()
  const matched = LEADING_SLASH_REGEXP.exec(trimmed)
  if (!matched) return null
  return {
    actionCode: matched[1],
    remainderText: trimmed.slice(matched[0].length).trim()
  }
}

/** 按编码取动作（未注册返回 null） */
export function matchActionByCode(actionCode: string): ComposerAction | null {
  return COMPOSER_ACTION_LIST.find((action) => action.action_code === actionCode) ?? null
}

/** 按关键词过滤动作（编码、名称、说明任一命中） */
export function filterActions(keywordText: string): ComposerAction[] {
  const keyword = keywordText.trim().toLowerCase()
  if (!keyword) return COMPOSER_ACTION_LIST
  return COMPOSER_ACTION_LIST.filter(
    (action) =>
      action.action_code.includes(keyword) ||
      action.action_name.toLowerCase().includes(keyword) ||
      action.action_desc.toLowerCase().includes(keyword)
  )
}

/** 动作编码集合（供冲突自检与文档生成） */
export function actionCodeSet(): Set<string> {
  return new Set(COMPOSER_ACTION_LIST.map((action) => action.action_code))
}

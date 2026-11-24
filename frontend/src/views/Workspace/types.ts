// 写作台的展示模型与发送载荷定义。
import type { NWKnowledgeChunkVo } from '@/api/contracts'

/** AI 面板单条消息的展示模型（组合后端 VO 形成界面态） */
export interface ChatDisplayMessage {
  message_ulid: string
  role_code: string
  content_text: string
  /** 推理模型返回的思考过程，生成期间逐字追加 */
  reasoning_text: string
  /** 命中的斜杠命令提示词编码（空为自由对话） */
  prompt_code: string
  /** 本轮执行的工具编码（模型自主调用或作者显式 #code） */
  tool_code_list: string[]
  created_at: string
  reference_list: NWKnowledgeChunkVo[]
  error_message: string
  streaming_flag: boolean
  /** 已等待时长（毫秒），仅用于等待态提示 */
  waited_ms: number
  /** 发送失败时保留的原始载荷，供「原样重发」 */
  retry_payload: ChatSendPayload | null
}

/** 输入区组装的发送载荷 */
export interface ChatSendPayload {
  content_text: string
  prompt_code: string
  tool_code_list: string[]
  ref_character_ulid_list: string[]
  ref_lore_ulid_list: string[]
  ref_chapter_ulid_list: string[]
  ref_summary_ulid_list: string[]
  ref_doc_ulid_list: string[]
  ref_session_ulid_list: string[]
  history_count: number
  system_prompt_code: string
}

/** @ 引用选择器的条目 */
export interface ChatRefItem {
  ref_ulid: string
  ref_name: string
  ref_desc: string
  /** 条目来源页签：聚合页签下选中时据此写回对应列表 */
  ref_tab_code: ChatRefTabCode
}

/** @ 引用选择器的页签编码：素材库五类 + 历史会话（all 为跨来源聚合检索） */
export type ChatRefTabCode =
  | 'all'
  | 'character'
  | 'lore'
  | 'chapter'
  | 'summary'
  | 'doc'
  | 'session'

/** 命令弹层候选项：本地动作与指令提示词归一成同一形状 */
export interface ChatCommandOption {
  code: string
  title: string
  desc: string
  active: boolean
  /** 来源：action=本地动作（不耗模型）；prompt=指令提示词（系统级注入） */
  source_code: 'action' | 'prompt'
  /** 图标（mdi 名），本地动作才有 */
  icon: string
}

// 写作台的展示模型与发送载荷定义。
import type { NWKnowledgeChunkVo } from '@/api/contracts'

/** AI 面板单条消息的展示模型（组合后端 VO 形成界面态） */
export interface ChatDisplayMessage {
  message_ulid: string
  role_code: string
  content_text: string
  /** 推理模型返回的思考过程，生成期间逐字追加 */
  reasoning_text: string
  skill_code: string
  /** 本轮执行的工具编码（显式 #code 与关键词自动触发） */
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
  skill_code: string
  tool_code_list: string[]
  ref_doc_ulid_list: string[]
  ref_chapter_ulid_list: string[]
  ref_summary_ulid_list: string[]
  ref_prompt_ulid_list: string[]
  ref_message_ulid_list: string[]
  history_count: number
  prompt_profile_ulid: string
}

/** @ 引用选择器的条目 */
export interface ChatRefItem {
  ref_ulid: string
  ref_name: string
  ref_desc: string
}

/** @ 引用选择器的页签编码 */
export type ChatRefTabCode = 'doc' | 'chapter' | 'summary' | 'prompt' | 'message'

/** 命令弹层候选项：技能与工具归一成同一形状 */
export interface ChatCommandOption {
  code: string
  title: string
  desc: string
  active: boolean
}

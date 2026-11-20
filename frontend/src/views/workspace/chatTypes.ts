// 写作台 AI 面板的前端展示模型（组合后端 VO 形成界面态，供 ChatComposer/ChatMessage/AiPanel 共用）
import type { NWKnowledgeChunkVo } from '@/api/contracts'

/** AI 面板单条消息的展示模型 */
export interface NWChatDisplayMessage {
  message_ulid: string
  role_code: string
  content_text: string
  /** 思考过程（推理模型返回，生成期间逐字追加） */
  reasoning_text: string
  skill_code: string
  created_at_ms: number
  reference_list: NWKnowledgeChunkVo[]
  error_message: string
  streaming_flag: boolean
  /** 已等待时长（毫秒），由面板计时器写入，仅用于等待态提示 */
  waited_ms: number
  /** 发送失败时保存的原始请求，供"原样重发" */
  retry_payload: NWChatSendPayload | null
}

/** 聊天输入区组装的发送载荷 */
export interface NWChatSendPayload {
  content_text: string
  skill_code: string
  ref_doc_ulid_list: string[]
  ref_chapter_ulid_list: string[]
  ref_message_ulid_list: string[]
  history_count: number
  prompt_profile_ulid: string
}

/** @引用选择器的条目（知识库文档、本项目章节或本会话历史消息） */
export interface NWChatRefItem {
  ref_ulid: string
  ref_name: string
  ref_desc: string
}

/** @引用选择器的页签编码 */
export type NWChatRefTabCode = 'doc' | 'chapter' | 'message'

// 后端契约中的共享视图对象类型（按 /api/v1 响应包络的 data 结构定义）

/** 用户视图对象 */
export interface NWUserVo {
  user_ulid: string
  user_name: string
  user_email: string
  avatar_url: string
  /** 账号状态编码（active/disabled，登录/用户信息接口返回） */
  user_status_code?: string
  /** 角色编码列表（登录/用户信息接口返回，用于菜单入口权限控制） */
  role_codes?: string[]
  created_at_ms: number
}

/** 登录结果视图对象 */
export interface NWLoginResultVo {
  access_token: string
  expires_in_sec: number
  user_vo: NWUserVo
}

/** 近 30 天字数趋势项 */
export interface NWTrendItemVo {
  stat_date: string
  word_delta: number
  ai_word_delta: number
}

/** 统计概览视图对象 */
export interface NWStatsOverviewVo {
  project_count: number
  total_word_count: number
  today_word_delta: number
  ai_word_total: number
  trend_list: NWTrendItemVo[]
}

/** 项目列表项（字段对齐后端 ProjectVO） */
export interface NWProjectItemVo {
  project_ulid: string
  project_name: string
  genre_code: string
  project_status: string
  total_word_count: number
  volume_count: number
  chapter_count: number
  created_at_ms: number
  updated_at_ms: number
}

/** 系统配置项 */
export interface NWSettingItemVo {
  setting_key: string
  setting_value: string
  setting_desc: string
}

/** 斜杠命令项（字段对齐后端 SlashCommandVO） */
export interface NWSlashCommandItemVo {
  command_ulid: string
  command_code: string
  command_name: string
  category_code: string
  description_text: string
  prompt_template: string
  enabled_flag: boolean
  builtin_flag: boolean
  sort_order: number
}

/** 斜杠命令保存请求（字段对齐后端 SlashCommandREQ） */
export interface NWSlashCommandSaveReqVo {
  command_ulid?: string
  command_code: string
  command_name: string
  category_code: string
  description_text: string
  prompt_template: string
  enabled_flag: boolean
  sort_order: number
}

// ============================================================
// 写作工作台 / 项目管理契约（wave B，字段对齐后端 domain）
// ============================================================

/** 分页结果（字段对齐后端 PageDTO） */
export interface NWPageDto<T> {
  total: number
  page_num: number
  page_size: number
  items: T[]
}

/** 项目视图对象（字段对齐后端 ProjectVO） */
export interface NWProjectVo {
  project_ulid: string
  project_name: string
  genre_code: string
  style_text: string
  synopsis_text: string
  project_status: string
  total_word_count: number
  volume_count: number
  chapter_count: number
  created_at_ms: number
  updated_at_ms: number
}

/** 卷视图对象（字段对齐后端 VolumeVO） */
export interface NWVolumeVo {
  volume_ulid: string
  volume_name: string
  volume_order: number
  summary_text: string
}

/** 章节摘要视图（目录树用，字段对齐后端 ChapterBriefVO） */
export interface NWChapterBriefVo {
  chapter_ulid: string
  chapter_name: string
  chapter_order: number
  word_count: number
  chapter_status: string
}

/** 目录树卷节点（字段对齐后端 VolumeTreeNodeVO） */
export interface NWVolumeTreeNodeVo {
  volume_vo: NWVolumeVo
  chapter_list: NWChapterBriefVo[]
}

/** 项目目录树（字段对齐后端 ProjectTreeVO） */
export interface NWProjectTreeVo {
  project_vo: NWProjectVo
  volume_list: NWVolumeTreeNodeVo[]
}

/** 章节详情视图（含正文，字段对齐后端 ChapterDetailVO） */
export interface NWChapterDetailVo extends NWChapterBriefVo {
  project_ulid: string
  volume_ulid: string
  content_text: string
  summary_text: string
  updated_at_ms: number
}

/** 章节快照视图（字段对齐后端 ChapterSnapshotVO） */
export interface NWChapterSnapshotVo {
  snapshot_ulid: string
  chapter_ulid: string
  snapshot_type: string
  change_note: string
  word_count: number
  created_at_ms: number
}

/** 知识库文档视图（字段对齐后端 KnowledgeDocumentVO） */
export interface NWKnowledgeDocumentVo {
  doc_ulid: string
  project_ulid: string
  doc_name: string
  doc_type_code: string
  doc_status_code: string
  chunk_count: number
  total_token_count: number
  error_message: string
  created_at_ms: number
}

/** 知识库切片视图（RAG 召回依据，字段对齐后端 KnowledgeChunkVO） */
export interface NWKnowledgeChunkVo {
  chunk_ulid: string
  doc_ulid: string
  doc_name: string
  chunk_index: number
  chunk_text: string
  score_value: number
  source_mode: string
}

/** AI 会话视图（字段对齐后端 ChatSessionVO） */
export interface NWChatSessionVo {
  session_ulid: string
  project_ulid: string
  session_name: string
  message_count: number
  updated_at_ms: number
}

/** AI 消息视图（字段对齐后端 ChatMessageVO） */
export interface NWChatMessageVo {
  message_ulid: string
  session_ulid: string
  role_code: string
  content_text: string
  skill_code: string
  ref_doc_ulid_list: string[]
  ref_chapter_ulid_list: string[]
  ref_message_ulid_list: string[]
  token_used_count: number
  created_at_ms: number
}

/** SSE 流式事件（字段对齐后端 ChatStreamEventDTO） */
export interface NWChatStreamEventVo {
  seq: number
  event_type: string
  session_ulid: string
  message_ulid: string
  content_text: string
  reference_list: NWKnowledgeChunkVo[]
  error_message: string
}

/** 创建项目请求（字段对齐后端 ProjectCreateREQ） */
export interface NWProjectCreateReqVo {
  project_name: string
  genre_code?: string
  style_text?: string
  synopsis_text?: string
}

/** 更新项目请求（字段对齐后端 ProjectUpdateREQ） */
export interface NWProjectUpdateReqVo {
  project_ulid: string
  project_name: string
  genre_code?: string
  style_text?: string
  synopsis_text?: string
  project_status?: string
}

/** 项目分页查询请求（字段对齐后端 ProjectListREQ） */
export interface NWProjectListReqVo {
  page_num: number
  page_size: number
  keyword_text?: string
  project_status?: string
}

/** 创建卷请求（字段对齐后端 VolumeCreateREQ） */
export interface NWVolumeCreateReqVo {
  project_ulid: string
  volume_name: string
  summary_text?: string
}

/** 更新卷请求（字段对齐后端 VolumeUpdateREQ） */
export interface NWVolumeUpdateReqVo {
  volume_ulid: string
  volume_name?: string
  summary_text?: string
}

/** 创建章节请求（字段对齐后端 ChapterCreateREQ） */
export interface NWChapterCreateReqVo {
  project_ulid: string
  volume_ulid: string
  chapter_name: string
}

/** 更新章节请求（字段对齐后端 ChapterUpdateREQ，内容整体覆盖） */
export interface NWChapterUpdateReqVo {
  chapter_ulid: string
  chapter_name?: string
  volume_ulid?: string
  content_text?: string
  summary_text?: string
  chapter_status?: string
}

/** 章节快照留档请求（字段对齐后端 ChapterSnapshotCreateREQ） */
export interface NWChapterSnapshotCreateReqVo {
  chapter_ulid: string
  change_note: string
}

/** 章节快照回滚请求（字段对齐后端 ChapterSnapshotRestoreREQ） */
export interface NWChapterSnapshotRestoreReqVo {
  snapshot_ulid: string
}

/** 发送聊天消息请求（字段对齐后端 ChatSendREQ，SSE 流式响应） */
export interface NWChatSendReqVo {
  session_ulid?: string
  project_ulid: string
  content_text: string
  skill_code?: string
  ref_doc_ulid_list?: string[]
  ref_chapter_ulid_list?: string[]
  ref_message_ulid_list?: string[]
  history_count?: number
}

/** 创建 AI 会话请求 */
export interface NWChatSessionCreateReqVo {
  project_ulid: string
  session_name?: string
}

/** 会话历史分页请求（字段对齐后端 ChatHistoryREQ） */
export interface NWChatHistoryReqVo {
  session_ulid: string
  page_num: number
  page_size: number
}

/** 知识库文档分页请求（字段对齐后端 KnowledgeListREQ） */
export interface NWKnowledgeListReqVo {
  project_ulid: string
  page_num: number
  page_size: number
  doc_status_code?: string
}

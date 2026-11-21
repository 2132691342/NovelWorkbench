// 本文件定义 AI 会话、聊天消息与 SSE 流式事件的持久化对象、请求与视图对象。

package domain

// ChatSessionDO AI 会话持久化对象。
type ChatSessionDO struct {
	BaseDO
	SessionULID string `json:"session_ulid" gorm:"column:session_ulid;primaryKey;size:40"` // 会话 ULID（SES_ 前缀）
	ProjectULID string `json:"project_ulid" gorm:"column:project_ulid;size:40;index"`      // 所属项目
	UserULID    string `json:"user_ulid" gorm:"column:user_ulid;size:40;index"`            // 所属用户
	SessionName string `json:"session_name" gorm:"size:128"`                               // 会话名（默认取首条消息摘要）
}

// TableName 指定表名。
func (ChatSessionDO) TableName() string { return "chat_sessions" }

// ChatMessageDO 聊天消息持久化对象。
type ChatMessageDO struct {
	BaseDO
	MessageULID    string `json:"message_ulid" gorm:"column:message_ulid;primaryKey;size:40"` // 消息 ULID（MSG_ 前缀）
	SessionULID    string `json:"session_ulid" gorm:"column:session_ulid;size:40;index"`      // 所属会话
	RoleCode       string `json:"role_code" gorm:"size:16"`                                   // 角色：user/assistant/system
	ContentText    string `json:"content_text" gorm:"type:text"`                              // 消息正文
	ReasoningText  string `json:"reasoning_text" gorm:"type:text"`                            // 思考过程（推理模型返回，可为空）
	SkillCode      string `json:"skill_code" gorm:"size:32"`                                  // 命中的斜杠命令编码（空为自由对话）
	RefDocJSON     string `json:"ref_doc_json" gorm:"type:text"`                              // @引用文档 ULID 列表（JSON 数组）
	RefChapterJSON string `json:"ref_chapter_json" gorm:"type:text"`                          // @引用章节 ULID 列表（JSON 数组）
	RefMessageJSON string `json:"ref_message_json" gorm:"type:text"`                          // 显式引用的历史消息 ULID 列表（JSON 数组）
	RefSummaryJSON string `json:"ref_summary_json" gorm:"type:text"`                          // @引用章节摘要 ULID 列表（JSON 数组）
	RefPromptJSON  string `json:"ref_prompt_json" gorm:"type:text"`                           // @引用提示词档案 ULID 列表（JSON 数组）
	TokenUsedCount int    `json:"token_used_count"`                                           // 本条消耗的估算 token
}

// TableName 指定表名。
func (ChatMessageDO) TableName() string { return "chat_messages" }

// ChatSendREQ 发送聊天消息请求（POST，流式以 SSE 响应）。
type ChatSendREQ struct {
	SessionULID        string            `json:"session_ulid" binding:"omitempty"`               // 会话 ULID，空则自动创建
	ProjectULID        string            `json:"project_ulid" binding:"required"`                // 所属项目
	ContentText        string            `json:"content_text" binding:"required,max=8000"`       // 消息正文
	SkillCode          string            `json:"skill_code" binding:"omitempty,max=32"`          // 技能编码
	SkillVariableMap   map[string]string `json:"skill_variable_map"`                             // 技能变量取值
	RefDocULIDList     []string          `json:"ref_doc_ulid_list"`                              // @引用文档
	RefChapterULIDList []string          `json:"ref_chapter_ulid_list"`                          // @引用章节
	RefMessageULIDList []string          `json:"ref_message_ulid_list"`                          // 显式引用的历史消息
	RefSummaryULIDList []string          `json:"ref_summary_ulid_list"`                          // @引用章节摘要
	RefPromptULIDList  []string          `json:"ref_prompt_ulid_list"`                           // @引用提示词档案
	HistoryCount       int               `json:"history_count" binding:"omitempty,min=0,max=50"` // 历史消息注入条数，0 用全局配置
	ModelULID          string            `json:"model_ulid" binding:"omitempty,max=40"`          // 指定模型，空用默认
	PromptProfileULID  string            `json:"prompt_profile_ulid" binding:"omitempty,max=40"` // 指定提示词档案，空用默认全局档案
}

// ChatHistoryREQ 拉取会话历史。
type ChatHistoryREQ struct {
	SessionULID string `json:"session_ulid" binding:"required"` // 会话 ULID
	PageREQ
}

// ChatSessionVO 会话视图对象。
type ChatSessionVO struct {
	SessionULID  string `json:"session_ulid"`  // 会话 ULID
	ProjectULID  string `json:"project_ulid"`  // 所属项目
	SessionName  string `json:"session_name"`  // 会话名
	MessageCount int    `json:"message_count"` // 消息条数
	UpdatedAt    string `json:"updated_at"`    // 最后活跃时间（RFC3339）
}

// ChatMessageVO 消息视图对象。
type ChatMessageVO struct {
	MessageULID        string   `json:"message_ulid"`          // 消息 ULID
	SessionULID        string   `json:"session_ulid"`          // 所属会话
	RoleCode           string   `json:"role_code"`             // 角色
	ContentText        string   `json:"content_text"`          // 正文
	ReasoningText      string   `json:"reasoning_text"`        // 思考过程（推理模型返回，可为空）
	SkillCode          string   `json:"skill_code"`            // 命中斜杠命令
	RefDocULIDList     []string `json:"ref_doc_ulid_list"`     // 引用文档
	RefChapterULIDList []string `json:"ref_chapter_ulid_list"` // 引用章节
	RefMessageULIDList []string `json:"ref_message_ulid_list"` // 引用历史消息
	RefSummaryULIDList []string `json:"ref_summary_ulid_list"` // 引用章节摘要
	RefPromptULIDList  []string `json:"ref_prompt_ulid_list"`  // 引用提示词档案
	TokenUsedCount     int      `json:"token_used_count"`      // 估算 token
	CreatedAt          string   `json:"created_at"`            // 发送时间（RFC3339）
}

// ChatStreamEventDTO SSE 流式事件（EventType 取值见常量）。
type ChatStreamEventDTO struct {
	Seq           int64              `json:"seq"`            // 单调递增事件序号
	EventType     string             `json:"event_type"`     // 事件类型
	SessionULID   string             `json:"session_ulid"`   // 会话 ULID
	MessageULID   string             `json:"message_ulid"`   // 助手消息 ULID
	ContentText   string             `json:"content_text"`   // 增量文本（chunk/done）或增量思考（thinking）
	ReasoningText string             `json:"reasoning_text"` // 完整思考过程（done 时给出，可为空）
	ReferenceList []KnowledgeChunkVO `json:"reference_list"` // RAG 召回依据（done 时给出）
	ErrorMessage  string             `json:"error_message"`  // 错误信息（error 时给出）
}

// SSE 事件类型常量。
const (
	StreamEventThinking = "thinking" // 增量思考过程（推理模型）
	StreamEventChunk    = "chunk"    // 增量正文
	StreamEventDone     = "done"     // 生成结束（含引用依据与完整思考）
	StreamEventError    = "error"    // 生成失败
)

// 聊天消息角色编码常量（chat_messages.role_code）。
const (
	ChatRoleUser      = "user"      // 用户
	ChatRoleAssistant = "assistant" // AI 助手
	ChatRoleSystem    = "system"    // 系统
)

// ChatSessionCreateREQ 创建 AI 会话请求。
type ChatSessionCreateREQ struct {
	ProjectULID string `json:"project_ulid" form:"project_ulid" binding:"required"`          // 所属项目
	SessionName string `json:"session_name" form:"session_name" binding:"omitempty,max=128"` // 会话名，空则取首条消息前 20 字
}

// ChatSessionListREQ AI 会话列表查询请求。
type ChatSessionListREQ struct {
	ProjectULID string `json:"project_ulid" form:"project_ulid" binding:"required"` // 所属项目
}

// ChatSessionDeleteREQ 删除 AI 会话请求（级联删消息）。
type ChatSessionDeleteREQ struct {
	SessionULID string `json:"session_ulid" form:"session_ulid" binding:"required"` // 会话 ULID
}

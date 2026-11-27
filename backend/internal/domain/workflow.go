// 本文件定义自动化工作流的模板、运行实例与步骤记录。
// 工作流把「素材 + 大纲 + 提示词 + 字数约束」变成可追踪的多步任务：AI 执行，人只做审核。

package domain

import "time"

// 工作流 ULID 业务前缀。
const (
	IDPrefixWorkflowTemplate = "WFT_" // 工作流模板
	IDPrefixWorkflowRun      = "WFR_" // 运行实例
	IDPrefixWorkflowStep     = "WFS_" // 步骤记录
	IDPrefixTrace            = "TRC_" // 链路追踪（一次运行一条，串联全部步骤审计）
)

// 内置工作流模板编码（模板决定一次任务跑哪些步骤）。
const (
	WorkflowTemplateChapterDraft  = "chapter_draft"  // 逐章成稿
	WorkflowTemplateChapterPolish = "chapter_polish" // 存量打磨
	WorkflowTemplateChapterDeai   = "chapter_deai"   // 批量去 AI 味
)

// 运行状态编码。
const (
	WorkflowRunStatusQueued        = "queued"         // 已创建待执行
	WorkflowRunStatusRunning       = "running"        // 执行中
	WorkflowRunStatusWaitingReview = "waiting_review" // 等待人工审核
	WorkflowRunStatusSucceeded     = "succeeded"      // 全部章节完成
	WorkflowRunStatusFailed        = "failed"         // 步骤失败
	WorkflowRunStatusCanceled      = "canceled"       // 用户取消
	WorkflowRunStatusPaused        = "paused"         // 用户暂停
)

// 步骤状态编码。
const (
	WorkflowStepStatusQueued   = "queued"   // 待执行
	WorkflowStepStatusRunning  = "running"  // 执行中
	WorkflowStepStatusSuccess  = "success"  // 成功
	WorkflowStepStatusFailed   = "failed"   // 失败
	WorkflowStepStatusSkipped  = "skipped"  // 条件不满足而跳过
	WorkflowStepStatusRejected = "rejected" // 人工审核退回
)

// 步骤类型编码（决定执行器走哪条分支）。
const (
	WorkflowStepTypeDraft       = "draft"        // 起草正文（AI）
	WorkflowStepTypeCheck       = "check"        // 量化体检（确定性）
	WorkflowStepTypeRevise      = "revise"       // 修订（AI，条件触发）
	WorkflowStepTypeReview      = "review"       // AI 复审（AI，输出严格 JSON）
	WorkflowStepTypeHumanReview = "human_review" // 人工审核门禁（阻塞运行）
	WorkflowStepTypeSummary     = "summary"      // 章节摘要（AI）
	WorkflowStepTypeApply       = "apply"        // 定稿入库（确定性）
)

// WorkflowStepDefDTO 模板中的步骤定义（落库为 JSON 数组）。
type WorkflowStepDefDTO struct {
	StepTypeCode  string `json:"step_type_code"`  // 步骤类型
	StepName      string `json:"step_name"`       // 步骤名称
	PromptCode    string `json:"prompt_code"`     // 该步骤使用的提示词编码（确定性步骤为空）
	MaxAttempts   int    `json:"max_attempts"`    // AI 步骤最大尝试次数（0 取默认 2）
	SkipWhenPass  bool   `json:"skip_when_pass"`  // 上一步体检通过时跳过（仅修订步骤）
	HumanGateFlag bool   `json:"human_gate_flag"` // 是否人工审核门禁
}

// WorkflowTemplateDO 工作流模板持久化对象（内置模板随代码升级同步）。
type WorkflowTemplateDO struct {
	BaseDO
	TemplateULID    string `json:"template_ulid" gorm:"column:template_ulid;primaryKey;size:40"`  // 模板 ULID（WFT_ 前缀）
	TemplateCode    string `json:"template_code" gorm:"column:template_code;size:32;uniqueIndex"` // 模板编码（唯一）
	TemplateName    string `json:"template_name" gorm:"column:template_name;size:64"`             // 模板名称
	DescriptionText string `json:"description_text" gorm:"column:description_text;size:256"`      // 一句话说明
	StepSchemaJSON  string `json:"step_schema_json" gorm:"column:step_schema_json;type:text"`     // 步骤定义（WorkflowStepDefDTO 数组）
	BuiltinFlag     bool   `json:"builtin_flag" gorm:"column:builtin_flag;default:false"`         // 是否内置
	EnabledFlag     bool   `json:"enabled_flag" gorm:"column:enabled_flag;default:true"`          // 是否启用
	SortOrder       int    `json:"sort_order" gorm:"column:sort_order"`                           // 排序号
}

// TableName 指定表名。
func (WorkflowTemplateDO) TableName() string { return "workflow_templates" }

// WorkflowRunDO 工作流运行实例持久化对象（一次任务覆盖多个章节）。
type WorkflowRunDO struct {
	BaseDO
	RunULID              string    `json:"run_ulid" gorm:"column:run_ulid;primaryKey;size:40"`                      // 运行 ULID（WFR_ 前缀）
	ProjectULID          string    `json:"project_ulid" gorm:"column:project_ulid;size:40;index"`                   // 所属项目
	UserULID             string    `json:"user_ulid" gorm:"column:user_ulid;size:40;index"`                         // 发起用户
	TemplateCode         string    `json:"template_code" gorm:"column:template_code;size:32;index"`                 // 模板编码
	TemplateName         string    `json:"template_name" gorm:"column:template_name;size:64"`                       // 模板名称（快照，模板改名不影响历史）
	RunStatusCode        string    `json:"run_status_code" gorm:"column:run_status_code;size:20;index"`             // 运行状态
	ChapterULIDListJSON  string    `json:"chapter_ulid_list_json" gorm:"column:chapter_ulid_list_json;type:text"`   // 目标章节列表（JSON 数组）
	ParamsJSON           string    `json:"params_json" gorm:"column:params_json;type:text"`                         // 运行参数（WorkflowRunParamsDTO）
	CurrentIndex         int       `json:"current_index"`                                                           // 当前章节下标（0 起）
	CurrentStepIndex     int       `json:"current_step_index"`                                                      // 当前章节内的步骤下标（0 起，审核恢复用）
	TotalChapterCount    int       `json:"total_chapter_count"`                                                     // 目标章节总数
	FinishedChapterCount int       `json:"finished_chapter_count"`                                                  // 已完成章节数
	TraceID              string    `json:"trace_id" gorm:"column:trace_id;size:40;index"`                           // 链路追踪 ID（串联步骤审计）
	FeedbackText         string    `json:"feedback_text" gorm:"column:feedback_text;type:text"`                     // 审核退回意见（下一轮起草注入）
	CandidateText        string    `json:"candidate_text" gorm:"column:candidate_text;type:text"`                   // 待入库正文（起草/修订产出，审核通过后写回章节）
	ErrorMessage         string    `json:"error_message" gorm:"column:error_message;size:512"`                      // 失败原因
	PauseRequestedFlag   bool      `json:"pause_requested_flag" gorm:"column:pause_requested_flag;default:false"`   // 已请求暂停
	CancelRequestedFlag  bool      `json:"cancel_requested_flag" gorm:"column:cancel_requested_flag;default:false"` // 已请求取消
	StartedAt            time.Time `json:"started_at" gorm:"column:started_at"`                                     // 开始时间
	FinishedAt           time.Time `json:"finished_at" gorm:"column:finished_at"`                                   // 结束时间
}

// TableName 指定表名。
func (WorkflowRunDO) TableName() string { return "workflow_runs" }

// WorkflowStepDO 工作流步骤记录持久化对象（每一步一条，构成可追溯的执行时间线）。
type WorkflowStepDO struct {
	BaseDO
	StepULID       string    `json:"step_ulid" gorm:"column:step_ulid;primaryKey;size:40"`          // 步骤 ULID（WFS_ 前缀）
	RunULID        string    `json:"run_ulid" gorm:"column:run_ulid;size:40;index"`                 // 所属运行
	ChapterULID    string    `json:"chapter_ulid" gorm:"column:chapter_ulid;size:40;index"`         // 目标章节
	StepIndex      int       `json:"step_index" gorm:"column:step_index"`                           // 步骤序号（0 起）
	StepTypeCode   string    `json:"step_type_code" gorm:"column:step_type_code;size:20"`           // 步骤类型
	StepName       string    `json:"step_name" gorm:"column:step_name;size:64"`                     // 步骤名称
	StepStatusCode string    `json:"step_status_code" gorm:"column:step_status_code;size:16;index"` // 步骤状态
	PromptCode     string    `json:"prompt_code" gorm:"column:prompt_code;size:32"`                 // 使用的提示词编码
	OutputText     string    `json:"output_text" gorm:"column:output_text;type:text"`               // 输出（体检/复审/摘要文本）
	MetricJSON     string    `json:"metric_json" gorm:"column:metric_json;type:text"`               // 量化指标（JSON 对象）
	AttemptCount   int       `json:"attempt_count"`                                                 // 已尝试次数
	LatencyMs      int64     `json:"latency_ms"`                                                    // 耗时（毫秒）
	ErrorMessage   string    `json:"error_message" gorm:"column:error_message;size:512"`            // 失败原因
	StartedAt      time.Time `json:"started_at" gorm:"column:started_at"`                           // 开始时间
	FinishedAt     time.Time `json:"finished_at" gorm:"column:finished_at"`                         // 结束时间
}

// TableName 指定表名。
func (WorkflowStepDO) TableName() string { return "workflow_steps" }

// WorkflowRunParamsDTO 运行参数（创建时确定，执行期只读）。
type WorkflowRunParamsDTO struct {
	ChapterULIDList      []string `json:"chapter_ulid_list"`      // 目标章节（空则取项目内空白章）
	NewChapterCount      int      `json:"new_chapter_count"`      // 无可用章节时新建的章数（0 且无空白章则报错）
	TargetWordCount      int      `json:"target_word_count"`      // 每章目标字数（0 取默认 2500）
	ModelULID            string   `json:"model_ulid"`             // 指定模型（空用账号默认模型）
	SystemPromptCode     string   `json:"system_prompt_code"`     // 身份提示词编码（可选，空用默认身份或内置作者人格）
	DraftPromptCode      string   `json:"draft_prompt_code"`      // 起草提示词编码（空用模板默认）
	RevisePromptCode     string   `json:"revise_prompt_code"`     // 修订提示词编码（空用模板默认）
	ReviewPromptCode     string   `json:"review_prompt_code"`     // 复审提示词编码（空用模板默认）
	GateScore            int      `json:"gate_score"`             // 体检通过分（0 取默认 80）
	ReviewGateFlag       *bool    `json:"review_gate_flag"`       // 是否保留人工审核门禁（缺省 true）
	ExtraRequirementText string   `json:"extra_requirement_text"` // 额外写作要求（作者补充）
}

// WorkflowQuickStartREQ 一键开跑请求：只给项目，其余参数取项目上的自动化默认值。
// 这是「AI 干活、人点头」的主入口——作者不必每次重填字数、门禁分与审核开关。
type WorkflowQuickStartREQ struct {
	ProjectULID          string   `json:"project_ulid" binding:"required,max=40"`                  // 所属项目
	TemplateCode         string   `json:"template_code" binding:"omitempty,max=32"`                // 模板（空则按章节状态自动选）
	ChapterULIDList      []string `json:"chapter_ulid_list" binding:"omitempty,max=100"`           // 指定章节（空则按模板默认范围）
	NewChapterCount      int      `json:"new_chapter_count" binding:"omitempty,min=0,max=50"`      // 无可用章节时新建章数
	TargetWordCount      int      `json:"target_word_count" binding:"omitempty,min=300,max=20000"` // 覆盖项目默认字数（0 用项目默认）
	GateScore            int      `json:"gate_score" binding:"omitempty,min=60,max=100"`           // 覆盖项目默认体检分（0 用项目默认）
	ReviewGateFlag       *bool    `json:"review_gate_flag"`                                        // 覆盖项目默认审核开关（nil 用项目默认）
	ExtraRequirementText string   `json:"extra_requirement_text" binding:"omitempty,max=1000"`     // 本次追加要求（与项目要求合并）
	ModelULID            string   `json:"model_ulid" binding:"omitempty,max=40"`                   // 指定模型（空用账号默认）
}

// WorkflowQuickStartVO 一键开跑结果（前端据此直接跳运行详情）。
type WorkflowQuickStartVO struct {
	RunVO            WorkflowRunVO `json:"run_vo"`             // 已创建的运行
	TemplateCode     string        `json:"template_code"`      // 实际选中的模板
	ChapterCount     int           `json:"chapter_count"`      // 本次将处理的章节数
	ReadyFlag        bool          `json:"ready_flag"`         // 素材是否齐备（false 仍已开跑，但已在就绪度里列出缺口）
	BlockingCount    int           `json:"blocking_count"`     // 未就绪的阻塞项数量
	BlockingHintList []string      `json:"blocking_hint_list"` // 未就绪项的可执行建议（前几条）
}

// WorkflowRunCreateREQ 创建工作流运行请求（创建后可立即执行）。
type WorkflowRunCreateREQ struct {
	ProjectULID  string `json:"project_ulid" binding:"required,max=40"`  // 所属项目
	TemplateCode string `json:"template_code" binding:"required,max=32"` // 模板编码
	WorkflowRunParamsDTO
	AutoStartFlag bool `json:"auto_start_flag"` // 创建后立即执行
}

// WorkflowRunListREQ 运行列表查询请求。
type WorkflowRunListREQ struct {
	PageREQ
	ProjectULID   string `json:"project_ulid" binding:"omitempty,max=40"`                                                                  // 项目过滤
	RunStatusCode string `json:"run_status_code" binding:"omitempty,oneof=queued running waiting_review succeeded failed canceled paused"` // 状态过滤
	KeywordText   string `json:"keyword_text" binding:"omitempty,max=64"`                                                                  // 运行 ULID / 模板名模糊
}

// WorkflowRunIDREQ 单次运行的操作请求（暂停/取消/重试）。
type WorkflowRunIDREQ struct {
	RunULID string `json:"run_ulid" binding:"required,max=40"` // 运行 ULID
}

// WorkflowRunApproveREQ 审核通过请求（通过后继续下一章）。
type WorkflowRunApproveREQ struct {
	RunULID  string `json:"run_ulid" binding:"required,max=40"`    // 运行 ULID
	NoteText string `json:"note_text" binding:"omitempty,max=512"` // 审核备注（记入步骤输出）
}

// WorkflowRunRejectREQ 审核退回请求（带意见重写当前章）。
type WorkflowRunRejectREQ struct {
	RunULID      string `json:"run_ulid" binding:"required,max=40"`              // 运行 ULID
	FeedbackText string `json:"feedback_text" binding:"required,min=2,max=1024"` // 退回意见（注入下一轮起草）
}

// WorkflowTemplateVO 模板视图（含步骤定义，供前端预览流程）。
type WorkflowTemplateVO struct {
	TemplateULID    string               `json:"template_ulid"`    // 模板 ULID
	TemplateCode    string               `json:"template_code"`    // 模板编码
	TemplateName    string               `json:"template_name"`    // 模板名称
	DescriptionText string               `json:"description_text"` // 说明
	StepList        []WorkflowStepDefDTO `json:"step_list"`        // 步骤定义
	BuiltinFlag     bool                 `json:"builtin_flag"`     // 是否内置
	EnabledFlag     bool                 `json:"enabled_flag"`     // 是否启用
	SortOrder       int                  `json:"sort_order"`       // 排序号
}

// WorkflowRunVO 运行视图对象（列表与详情共用）。
type WorkflowRunVO struct {
	RunULID              string `json:"run_ulid"`               // 运行 ULID
	ProjectULID          string `json:"project_ulid"`           // 所属项目
	ProjectName          string `json:"project_name"`           // 项目名（列表回填）
	TemplateCode         string `json:"template_code"`          // 模板编码
	TemplateName         string `json:"template_name"`          // 模板名称
	RunStatusCode        string `json:"run_status_code"`        // 运行状态
	CurrentIndex         int    `json:"current_index"`          // 当前章节下标
	TotalChapterCount    int    `json:"total_chapter_count"`    // 章节总数
	FinishedChapterCount int    `json:"finished_chapter_count"` // 已完成章节数
	CurrentChapterULID   string `json:"current_chapter_ulid"`   // 当前章节 ULID
	CurrentChapterName   string `json:"current_chapter_name"`   // 当前章节名
	TraceID              string `json:"trace_id"`               // 链路追踪 ID
	ErrorMessage         string `json:"error_message"`          // 失败原因
	FeedbackText         string `json:"feedback_text"`          // 最近一次退回意见
	TargetWordCount      int    `json:"target_word_count"`      // 目标字数
	ReviewGateFlag       bool   `json:"review_gate_flag"`       // 是否人工审核
	CreatedAt            string `json:"created_at"`             // 创建时间（RFC3339）
	StartedAt            string `json:"started_at"`             // 开始时间（RFC3339）
	FinishedAt           string `json:"finished_at"`            // 结束时间（RFC3339）
}

// WorkflowRunDetailVO 运行详情视图（含步骤时间线与当前章节正文）。
type WorkflowRunDetailVO struct {
	WorkflowRunVO
	StepList           []WorkflowStepVO `json:"step_list"`            // 步骤时间线（按章节与序号升序）
	ChapterContentText string           `json:"chapter_content_text"` // 当前章节库内正文（审核对比用）
	ChapterWordCount   int64            `json:"chapter_word_count"`   // 当前章节库内字数
	CandidateText      string           `json:"candidate_text"`       // 待入库候选正文（审核通过后写回章节）
	CandidateWordCount int64            `json:"candidate_word_count"` // 候选正文字数（与库内字数对比展示）
	CheckScore         int              `json:"check_score"`          // 最近一次体检得分
	CheckVerdictCode   string           `json:"check_verdict_code"`   // 最近一次体检结论
	ReviewVerdictCode  string           `json:"review_verdict_code"`  // 最近一次复审结论
	ReviewOutputText   string           `json:"review_output_text"`   // 最近一次复审正文
}

// WorkflowStepVO 步骤视图对象。
type WorkflowStepVO struct {
	StepULID       string `json:"step_ulid"`        // 步骤 ULID
	ChapterULID    string `json:"chapter_ulid"`     // 目标章节
	ChapterName    string `json:"chapter_name"`     // 章节名（回填）
	StepIndex      int    `json:"step_index"`       // 步骤序号
	StepTypeCode   string `json:"step_type_code"`   // 步骤类型
	StepName       string `json:"step_name"`        // 步骤名称
	StepStatusCode string `json:"step_status_code"` // 步骤状态
	PromptCode     string `json:"prompt_code"`      // 提示词编码
	OutputText     string `json:"output_text"`      // 输出文本
	MetricJSON     string `json:"metric_json"`      // 量化指标
	AttemptCount   int    `json:"attempt_count"`    // 尝试次数
	LatencyMs      int64  `json:"latency_ms"`       // 耗时（毫秒）
	ErrorMessage   string `json:"error_message"`    // 失败原因
	StartedAt      string `json:"started_at"`       // 开始时间（RFC3339）
	FinishedAt     string `json:"finished_at"`      // 结束时间（RFC3339）
}

// WorkflowRunStatsVO 工作流概览（顶部卡片）。
type WorkflowRunStatsVO struct {
	RunningCount       int64 `json:"running_count"`        // 执行中
	WaitingReviewCount int64 `json:"waiting_review_count"` // 待审核
	FinishedCount      int64 `json:"finished_count"`       // 已完成
	FailedCount        int64 `json:"failed_count"`         // 失败
}

// WorkflowReadinessItemVO 开跑就绪度检查项（一项素材/配置是否到位）。
type WorkflowReadinessItemVO struct {
	ItemCode     string `json:"item_code"`     // 检查项编码（固定值，前端按此映射图标）
	ItemName     string `json:"item_name"`     // 检查项名称
	ReadyFlag    bool   `json:"ready_flag"`    // 是否就绪
	BlockingFlag bool   `json:"blocking_flag"` // 未就绪是否阻塞开跑
	CurrentValue string `json:"current_value"` // 当前值（如「2 张」）
	TargetValue  string `json:"target_value"`  // 目标值（如「3 张」）
	HintText     string `json:"hint_text"`     // 一句可执行的补齐建议
	FixRoute     string `json:"fix_route"`     // 前端相对路由（点击直达补齐页面）
}

// WorkflowReadinessVO 项目开跑就绪度（三步开跑的第一步：先看清缺什么）。
type WorkflowReadinessVO struct {
	ProjectULID   string                    `json:"project_ulid"`   // 所属项目
	ScoreValue    int                       `json:"score_value"`    // 就绪项占比（0-100）
	ReadyFlag     bool                      `json:"ready_flag"`     // 阻塞项是否全部就绪
	BlockingCount int                       `json:"blocking_count"` // 未就绪的阻塞项数量
	ItemList      []WorkflowReadinessItemVO `json:"item_list"`      // 全部检查项（阻塞项在前）
}

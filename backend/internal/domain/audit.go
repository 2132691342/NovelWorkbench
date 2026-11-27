// 本文件定义审计日志的持久化对象、视图对象与查询请求。
// 审计记录一次调用链上的一个 span：谁、从哪来、做了什么、结果如何、耗时多少。

package domain

// 审计级别编码常量（由 HTTP 状态或调用结果推导，便于按严重程度筛查）。
const (
	AuditLevelInfo  = "info"  // 成功（2xx/3xx 或调用成功）
	AuditLevelWarn  = "warn"  // 客户端或业务拒绝（4xx）
	AuditLevelError = "error" // 服务端故障（5xx 或模型调用失败）
)

// 审计 span 类型编码常量：一次请求内按调用层级拆分，链路时间线据此着色与分组。
const (
	AuditSpanTypeHTTP     = "http"          // HTTP 入口请求（中间件写入）
	AuditSpanTypeWorkflow = "workflow_step" // 工作流步骤（后台执行）
	AuditSpanTypeAICall   = "ai_call"       // 模型调用（对话 / 工作流 / 开书）
	AuditSpanTypeJob      = "job"           // 异步任务
	AuditSpanTypeSystem   = "system"        // 系统维护动作（留存清理等）
)

// 后台链路使用的固定动作码：父 span 与系统动作按它串起时间线。
const (
	AuditActionWorkflowRunCreate = "workflow/run/create" // 创建工作流运行（工作流步骤的父 span）
	AuditActionAIChat            = "ai/chat"             // AI 对话模型调用
	AuditActionAIBootstrapPlan   = "ai/bootstrap/plan"   // AI 开书方案生成
	AuditActionSystemPurge       = "system/audit.purge"  // 审计留存清理
)

// AuditLogDO 审计日志持久化对象（落库在 Manticore 全文表）。
type AuditLogDO struct {
	BaseDO
	AuditULID      string `json:"audit_ulid" gorm:"column:audit_ulid;primaryKey;size:40"`    // 审计 ULID（AUD_ 前缀）
	LogLevelCode   string `json:"log_level_code" gorm:"column:log_level_code;size:16;index"` // 级别：info/warn/error
	SpanTypeCode   string `json:"span_type_code" gorm:"column:span_type_code;size:24;index"` // span 类型：http/workflow_step/ai_call/job/system
	TraceID        string `json:"trace_id" gorm:"column:trace_id;size:40;index"`             // 链路追踪 ID（同 X-Request-ID）
	ParentSpanCode string `json:"parent_span_code" gorm:"column:parent_span_code;size:128"`  // 父 span 的动作码（顶级为空）
	UserULID       string `json:"user_ulid" gorm:"column:user_ulid;size:40;index"`           // 操作用户 ULID（未登录为空）
	UserName       string `json:"user_name" gorm:"column:user_name;size:64;index"`           // 操作用户登录名
	ActionCode     string `json:"action_code" gorm:"column:action_code;size:128;index"`      // 动作码（接口路径去前缀 / AI 调用编码）
	ActionName     string `json:"action_name" gorm:"column:action_name;size:128"`            // 动作中文名（如「创建工作流」「AI 出稿」）
	ModelName      string `json:"model_name" gorm:"column:model_name;size:128"`              // AI 调用的模型名（非 AI 为空）
	TokenCount     int    `json:"token_count" gorm:"column:token_count"`                     // AI 调用消耗 token（非 AI 为 0）
	RequestPath    string `json:"request_path" gorm:"column:request_path;size:256"`          // 请求路径（后台链路为 pseudo URL）
	RequestMethod  string `json:"request_method" gorm:"column:request_method;size:8"`        // 请求方法（后台链路为 INTERNAL）
	RequestBody    string `json:"request_body" gorm:"column:request_body;type:text"`         // 请求体（脱敏截断，文件上传不记）
	RespBody       string `json:"resp_body" gorm:"column:resp_body;type:text"`               // 响应体（脱敏截断，SSE 流不采样）
	TargetType     string `json:"target_type" gorm:"column:target_type;size:32;index"`       // 目标类型（user/role 等）
	TargetULID     string `json:"target_ulid" gorm:"column:target_ulid;size:40;index"`       // 目标 ULID
	RespCode       string `json:"resp_code" gorm:"column:resp_code;size:32;index"`           // 业务响应码
	RespMessage    string `json:"resp_message" gorm:"column:resp_message;size:512"`          // 业务响应说明（失败时为原因）
	HTTPStatus     int    `json:"http_status" gorm:"column:http_status"`                     // HTTP 状态码（后台链路用 200/500 表达成败）
	LatencyMs      int64  `json:"latency_ms" gorm:"column:latency_ms"`                       // 处理耗时（毫秒）
	IPAddr         string `json:"ip_addr" gorm:"column:ip_addr;size:64"`                     // 客户端 IP
	UserAgentText  string `json:"user_agent_text" gorm:"column:user_agent_text;size:512"`    // 客户端 UA
}

// TableName 指定表名（历史 PG 表名，仅用于启动期清理遗留表）。
func (AuditLogDO) TableName() string { return "audit_logs" }

// AuditVO 审计日志视图对象（前端列表与链路时间线展示）。
type AuditVO struct {
	AuditULID      string `json:"audit_ulid"`       // 审计 ULID
	LogLevelCode   string `json:"log_level_code"`   // 级别
	SpanTypeCode   string `json:"span_type_code"`   // span 类型
	TraceID        string `json:"trace_id"`         // 链路追踪 ID
	ParentSpanCode string `json:"parent_span_code"` // 父 span 动作码
	UserULID       string `json:"user_ulid"`        // 操作用户 ULID
	UserName       string `json:"user_name"`        // 操作用户登录名
	ActionCode     string `json:"action_code"`      // 动作码
	ActionName     string `json:"action_name"`      // 动作中文名
	ModelName      string `json:"model_name"`       // AI 调用的模型名
	TokenCount     int    `json:"token_count"`      // AI 调用 token 数
	RequestPath    string `json:"request_path"`     // 请求路径
	RequestMethod  string `json:"request_method"`   // 请求方法
	TargetType     string `json:"target_type"`      // 目标类型
	TargetULID     string `json:"target_ulid"`      // 目标 ULID
	RespCode       string `json:"resp_code"`        // 业务响应码
	RespMessage    string `json:"resp_message"`     // 业务响应说明
	HTTPStatus     int    `json:"http_status"`      // HTTP 状态码
	LatencyMs      int64  `json:"latency_ms"`       // 处理耗时（毫秒）
	IPAddr         string `json:"ip_addr"`          // 客户端 IP
	UserAgentText  string `json:"user_agent_text"`  // 客户端 UA
	CreatedAt      string `json:"created_at"`       // 记录时间（RFC3339）
}

// AuditDetailVO 审计详情视图（含请求体与响应体原文）。
type AuditDetailVO struct {
	AuditVO
	RequestBody string `json:"request_body"` // 请求体（脱敏截断）
	RespBody    string `json:"resp_body"`    // 响应体（脱敏截断）
}

// AuditListREQ 审计日志分页查询请求（POST body）。
type AuditListREQ struct {
	PageREQ
	LogLevelCode string `json:"log_level_code" binding:"omitempty,oneof=info warn error"`                       // 级别过滤
	SpanTypeCode string `json:"span_type_code" binding:"omitempty,oneof=http workflow_step ai_call job system"` // span 类型过滤
	TraceID      string `json:"trace_id" binding:"omitempty,max=40"`                                            // 链路追踪 ID 精确过滤
	TargetType   string `json:"target_type" binding:"omitempty,max=32"`                                         // 目标类型过滤
	ActionCode   string `json:"action_code" binding:"omitempty,max=128"`                                        // 动作码模糊过滤
	KeywordText  string `json:"keyword_text" binding:"omitempty,max=128"`                                       // 路径/用户/说明/动作名模糊
	UserULID     string `json:"user_ulid" binding:"omitempty,max=40"`                                           // 操作用户过滤
	DateFrom     string `json:"date_from" binding:"omitempty,max=10"`                                           // 起始日期 YYYY-MM-DD
	DateTo       string `json:"date_to" binding:"omitempty,max=10"`                                             // 结束日期 YYYY-MM-DD
	TimeFrom     string `json:"time_from" binding:"omitempty,max=40"`                                           // 起始时刻 RFC3339（精确到秒，优先于日期）
	TimeTo       string `json:"time_to" binding:"omitempty,max=40"`                                             // 结束时刻 RFC3339（精确到秒）
}

// AuditDetailREQ 审计详情查询请求。
type AuditDetailREQ struct {
	AuditULID string `json:"audit_ulid" form:"audit_ulid" binding:"required,max=40"` // 审计 ULID
}

// AuditTraceREQ 按链路 ID 拉取时间线请求。
type AuditTraceREQ struct {
	TraceID string `json:"trace_id" form:"trace_id" binding:"required,max=40"` // 链路追踪 ID
}

// AuditTraceVO 一条链路下的完整调用时间线（同一 trace_id 内的全部 span）。
type AuditTraceVO struct {
	TraceID  string          `json:"trace_id"`  // 链路追踪 ID
	Total    int             `json:"total"`     // 记录条数
	ItemList []AuditDetailVO `json:"item_list"` // 记录列表（时间升序，含请求/响应摘要）
}

// AuditActionOptionVO 动作码筛选项（下拉用，带条数）。
type AuditActionOptionVO struct {
	ActionCode string `json:"action_code"` // 动作码
	ActionName string `json:"action_name"` // 动作中文名（历史行可能为空）
	Total      int64  `json:"total"`       // 该动作的记录条数
}

// AuditRetentionVO 审计留存策略视图（审计页顶部摘要与手动清理反馈）。
type AuditRetentionVO struct {
	RetentionDays       int    `json:"retention_days"`         // 审计记录留存天数
	PurgeHour           int    `json:"purge_hour"`             // 每日清理时刻（0–23，本地时区）
	TotalCount          int64  `json:"total_count"`            // 当前审计总条数
	LoginLogTotalCount  int64  `json:"login_log_total_count"`  // 当前登录日志总条数
	PurgedCount         int    `json:"purged_count"`           // 本次清理删除的审计条数
	LoginLogPurgedCount int    `json:"login_log_purged_count"` // 本次清理删除的登录日志条数
	LastPurgedAt        string `json:"last_purged_at"`         // 上次清理时间（RFC3339）
	NextPurgeAt         string `json:"next_purge_at"`          // 下次清理时间（RFC3339，真实计算）
}

// AuditTraceSummaryVO 链路摘要：一次入口请求及其下所有后台 span 的聚合视图。
// 审计页默认按链路展示——作者关心「这次点了按钮，系统到底做了什么」，而不是几十条平铺记录。
type AuditTraceSummaryVO struct {
	TraceID           string `json:"trace_id"`            // 链路追踪 ID
	UserULID          string `json:"user_ulid"`           // 触发用户 ULID
	UserName          string `json:"user_name"`           // 触发用户登录名
	RootActionCode    string `json:"root_action_code"`    // 根 span 动作码（入口请求）
	RootActionName    string `json:"root_action_name"`    // 根 span 动作中文名
	RootRequestPath   string `json:"root_request_path"`   // 根 span 请求路径
	LogLevelCode      string `json:"log_level_code"`      // 最严重级别（error > warn > info）
	HTTPStatus        int    `json:"http_status"`         // 根 span HTTP 状态
	RespCode          string `json:"resp_code"`           // 根 span 业务响应码
	RespMessage       string `json:"resp_message"`        // 根 span 业务响应说明
	SpanCount         int    `json:"span_count"`          // 链路内 span 总数
	AICallCount       int    `json:"ai_call_count"`       // 链路内模型调用次数
	TokenCount        int64  `json:"token_count"`         // 链路内模型消耗 token 合计
	TotalLatencyMs    int64  `json:"total_latency_ms"`    // 链路耗时（根 span 耗时，含后台步骤）
	SlowestSpanMs     int64  `json:"slowest_span_ms"`     // 最慢 span 耗时
	SlowestActionName string `json:"slowest_action_name"` // 最慢 span 的动作中文名
	StartedAt         string `json:"started_at"`          // 链路开始时间（RFC3339）
	FinishedAt        string `json:"finished_at"`         // 链路结束时间（RFC3339）
}

// AuditTraceListREQ 链路列表查询请求（审计页主视图）。
type AuditTraceListREQ struct {
	PageREQ
	LogLevelCode string `json:"log_level_code" binding:"omitempty,oneof=info warn error"` // 级别过滤（含子 span 的最严重级别）
	KeywordText  string `json:"keyword_text" binding:"omitempty,max=128"`                 // 动作名/路径/用户/说明/链路 ID 模糊
	UserULID     string `json:"user_ulid" binding:"omitempty,max=40"`                     // 触发用户过滤
	ActionCode   string `json:"action_code" binding:"omitempty,max=128"`                  // 根动作码精确过滤
	SpanTypeCode string `json:"span_type_code" binding:"omitempty,max=24"`                // 仅保留含该类型 span 的链路
	TimeFrom     string `json:"time_from" binding:"omitempty,max=40"`                     // 起始时刻 RFC3339
	TimeTo       string `json:"time_to" binding:"omitempty,max=40"`                       // 结束时刻 RFC3339
	DateFrom     string `json:"date_from" binding:"omitempty,max=10"`                     // 起始日期 YYYY-MM-DD
	DateTo       string `json:"date_to" binding:"omitempty,max=10"`                       // 结束日期 YYYY-MM-DD
}

// AuditStatsVO 审计概览统计（审计页顶部卡片，全部为区间内数据）。
type AuditStatsVO struct {
	TraceCount      int64 `json:"trace_count"`       // 链路数
	SpanCount       int64 `json:"span_count"`        // span 数
	ErrorTraceCount int64 `json:"error_trace_count"` // 含 error span 的链路数
	WarnTraceCount  int64 `json:"warn_trace_count"`  // 仅含 warn span 的链路数
	AICallCount     int64 `json:"ai_call_count"`     // 模型调用次数
	TokenCount      int64 `json:"token_count"`       // 模型消耗 token 合计
	AvgLatencyMs    int64 `json:"avg_latency_ms"`    // 平均链路耗时
	SlowTraceCount  int64 `json:"slow_trace_count"`  // 耗时超过 5 秒的链路数
}

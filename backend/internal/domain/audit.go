// 本文件定义审计日志的持久化对象、视图对象与查询请求。
// 审计记录一次写操作的完整证据链：谁、从哪来、改了什么、结果如何、耗时多少。

package domain

// 审计级别编码常量（由 HTTP 状态推导，便于按严重程度筛查）。
const (
	AuditLevelInfo  = "info"  // 成功（2xx/3xx）
	AuditLevelWarn  = "warn"  // 客户端或业务拒绝（4xx）
	AuditLevelError = "error" // 服务端故障（5xx）
)

// AuditLogDO 审计日志持久化对象（变更类请求的落库记录）。
type AuditLogDO struct {
	BaseDO
	AuditULID     string `json:"audit_ulid" gorm:"column:audit_ulid;primaryKey;size:40"`    // 审计 ULID（AUD_ 前缀）
	LogLevelCode  string `json:"log_level_code" gorm:"column:log_level_code;size:16;index"` // 级别：info/warn/error
	TraceID       string `json:"trace_id" gorm:"column:trace_id;size:40;index"`             // 链路追踪 ID（同 X-Request-ID）
	UserULID      string `json:"user_ulid" gorm:"column:user_ulid;size:40;index"`           // 操作用户 ULID（未登录为空）
	UserName      string `json:"user_name" gorm:"column:user_name;size:64;index"`           // 操作用户登录名
	ActionCode    string `json:"action_code" gorm:"column:action_code;size:128;index"`      // 动作码（接口路径去前缀）
	RequestPath   string `json:"request_path" gorm:"column:request_path;size:256"`          // 请求路径
	RequestMethod string `json:"request_method" gorm:"column:request_method;size:8"`        // 请求方法（GET/POST）
	RequestBody   string `json:"request_body" gorm:"column:request_body;type:text"`         // 请求体（脱敏截断，文件上传不记）
	TargetType    string `json:"target_type" gorm:"column:target_type;size:32;index"`       // 目标类型（user/role 等）
	TargetULID    string `json:"target_ulid" gorm:"column:target_ulid;size:40;index"`       // 目标 ULID
	RespCode      string `json:"resp_code" gorm:"column:resp_code;size:32;index"`           // 业务响应码
	RespMessage   string `json:"resp_message" gorm:"column:resp_message;size:512"`          // 业务响应说明（失败时为原因）
	HTTPStatus    int    `json:"http_status" gorm:"column:http_status"`                     // HTTP 状态码
	LatencyMs     int64  `json:"latency_ms" gorm:"column:latency_ms"`                       // 处理耗时（毫秒）
	IPAddr        string `json:"ip_addr" gorm:"column:ip_addr;size:64"`                     // 客户端 IP
	UserAgentText string `json:"user_agent_text" gorm:"column:user_agent_text;size:512"`    // 客户端 UA
}

// TableName 指定表名。
func (AuditLogDO) TableName() string { return "audit_logs" }

// AuditVO 审计日志视图对象（前端列表展示）。
type AuditVO struct {
	AuditULID     string `json:"audit_ulid"`      // 审计 ULID
	LogLevelCode  string `json:"log_level_code"`  // 级别
	TraceID       string `json:"trace_id"`        // 链路追踪 ID
	UserULID      string `json:"user_ulid"`       // 操作用户 ULID
	UserName      string `json:"user_name"`       // 操作用户登录名
	ActionCode    string `json:"action_code"`     // 动作码
	RequestPath   string `json:"request_path"`    // 请求路径
	RequestMethod string `json:"request_method"`  // 请求方法
	TargetType    string `json:"target_type"`     // 目标类型
	TargetULID    string `json:"target_ulid"`     // 目标 ULID
	RespCode      string `json:"resp_code"`       // 业务响应码
	RespMessage   string `json:"resp_message"`    // 业务响应说明
	HTTPStatus    int    `json:"http_status"`     // HTTP 状态码
	LatencyMs     int64  `json:"latency_ms"`      // 处理耗时（毫秒）
	IPAddr        string `json:"ip_addr"`         // 客户端 IP
	UserAgentText string `json:"user_agent_text"` // 客户端 UA
	CreatedAt     string `json:"created_at"`      // 记录时间（RFC3339）
}

// AuditDetailVO 审计详情视图（含请求体原文）。
type AuditDetailVO struct {
	AuditVO
	RequestBody string `json:"request_body"` // 请求体（脱敏截断）
}

// AuditListREQ 审计日志分页查询请求（POST body）。
type AuditListREQ struct {
	PageREQ
	LogLevelCode string `json:"log_level_code" binding:"omitempty,oneof=info warn error"` // 级别过滤
	TraceID      string `json:"trace_id" binding:"omitempty,max=40"`                      // 链路追踪 ID 精确过滤
	TargetType   string `json:"target_type" binding:"omitempty,max=32"`                   // 目标类型过滤
	ActionCode   string `json:"action_code" binding:"omitempty,max=128"`                  // 动作码模糊过滤
	KeywordText  string `json:"keyword_text" binding:"omitempty,max=128"`                 // 路径/用户/响应说明模糊
	UserULID     string `json:"user_ulid" binding:"omitempty,max=40"`                     // 操作用户过滤
	DateFrom     string `json:"date_from" binding:"omitempty,max=10"`                     // 起始日期 YYYY-MM-DD
	DateTo       string `json:"date_to" binding:"omitempty,max=10"`                       // 结束日期 YYYY-MM-DD
}

// AuditDetailREQ 审计详情查询请求。
type AuditDetailREQ struct {
	AuditULID string `json:"audit_ulid" form:"audit_ulid" binding:"required,max=40"` // 审计 ULID
}

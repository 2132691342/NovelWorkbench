// 本文件定义登录日志的持久化对象、查询请求与视图对象。

package domain

// 登录结果编码常量。
const (
	LoginResultSuccess  = "success"  // 登录成功
	LoginResultFailed   = "failed"   // 账号或密码错误
	LoginResultDisabled = "disabled" // 账号已禁用
)

// LoginLogDO 登录日志持久化对象（成功与失败均记录，用于安全审计）。
type LoginLogDO struct {
	BaseDO
	LoginLogULID     string `json:"login_log_ulid" gorm:"column:login_log_ulid;primaryKey;size:40"`     // 登录日志 ULID（LGL_ 前缀）
	UserULID         string `json:"user_ulid" gorm:"column:user_ulid;size:40;index"`                    // 命中的用户 ULID（失败时为空）
	LoginAccountText string `json:"login_account_text" gorm:"column:login_account_text;size:128;index"` // 登录名或邮箱
	LoginResultCode  string `json:"login_result_code" gorm:"column:login_result_code;size:16;index"`    // 结果：success/failed/disabled
	FailReasonText   string `json:"fail_reason_text" gorm:"column:fail_reason_text;size:256"`           // 失败原因
	IPAddr           string `json:"ip_addr" gorm:"column:ip_addr;size:64"`                              // 客户端 IP
	UserAgentText    string `json:"user_agent_text" gorm:"column:user_agent_text;size:512"`             // 客户端 UA
}

// TableName 指定表名。
func (LoginLogDO) TableName() string { return "login_logs" }

// LoginMetaDTO 登录请求的客户端元信息（由 API 层采集）。
type LoginMetaDTO struct {
	IPAddr        string // 客户端 IP
	UserAgentText string // 客户端 UA
}

// LoginLogListREQ 登录日志分页查询请求。
type LoginLogListREQ struct {
	PageREQ
	KeywordText     string `json:"keyword_text" binding:"omitempty,max=128"`                            // 账号/IP 模糊
	LoginResultCode string `json:"login_result_code" binding:"omitempty,oneof=success failed disabled"` // 结果过滤
	DateFrom        string `json:"date_from" binding:"omitempty,max=10"`                                // 起始日期 YYYY-MM-DD
	DateTo          string `json:"date_to" binding:"omitempty,max=10"`                                  // 结束日期 YYYY-MM-DD
}

// LoginLogVO 登录日志视图对象。
type LoginLogVO struct {
	LoginLogULID     string `json:"login_log_ulid"`     // 日志 ULID
	UserULID         string `json:"user_ulid"`          // 用户 ULID
	LoginAccountText string `json:"login_account_text"` // 登录账号
	LoginResultCode  string `json:"login_result_code"`  // 结果
	FailReasonText   string `json:"fail_reason_text"`   // 失败原因
	IPAddr           string `json:"ip_addr"`            // 客户端 IP
	UserAgentText    string `json:"user_agent_text"`    // 客户端 UA
	CreatedAtMs      int64  `json:"created_at_ms"`      // 登录时间（毫秒）
}

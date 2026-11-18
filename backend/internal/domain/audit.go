package domain

// AuditLogDO 审计日志持久化对象（变更类请求的落库记录）。
type AuditLogDO struct {
	BaseDO
	AuditULID  string `json:"audit_ulid" gorm:"column:audit_ulid;primaryKey;size:40"` // 审计 ULID（AUD_ 前缀）
	UserULID   string `json:"user_ulid" gorm:"column:user_ulid;size:40;index"`        // 操作用户 ULID（未登录为空）
	UserName   string `json:"user_name" gorm:"column:user_name;size:64;index"`        // 操作用户登录名
	ActionCode string `json:"action_code" gorm:"column:action_code;size:128;index"`   // 动作码（接口路径去前缀）
	TargetType string `json:"target_type" gorm:"column:target_type;size:32"`          // 目标类型（user/role 等）
	TargetULID string `json:"target_ulid" gorm:"column:target_ulid;size:40;index"`    // 目标 ULID
	DetailText string `json:"detail_text" gorm:"size:512"`                            // 补充信息（响应状态等）
	IPAddr     string `json:"ip_addr" gorm:"column:ip_addr;size:64"`                  // 客户端 IP
}

// TableName 指定表名。
func (AuditLogDO) TableName() string { return "audit_logs" }

// AuditVO 审计日志视图对象（前端展示）。
type AuditVO struct {
	AuditULID   string `json:"audit_ulid"`    // 审计 ULID
	UserULID    string `json:"user_ulid"`     // 操作用户 ULID
	UserName    string `json:"user_name"`     // 操作用户登录名
	ActionCode  string `json:"action_code"`   // 动作码
	TargetType  string `json:"target_type"`   // 目标类型
	TargetULID  string `json:"target_ulid"`   // 目标 ULID
	DetailText  string `json:"detail_text"`   // 补充信息
	IPAddr      string `json:"ip_addr"`       // 客户端 IP
	CreatedAtMs int64  `json:"created_at_ms"` // 记录时间（毫秒）
}

// AuditListREQ 审计日志分页查询请求（POST body）。
type AuditListREQ struct {
	PageREQ
	ActionCode string `json:"action_code" form:"action_code" binding:"omitempty,max=128"` // 动作码模糊过滤
	UserULID   string `json:"user_ulid" form:"user_ulid" binding:"omitempty,max=40"`      // 操作用户过滤
}

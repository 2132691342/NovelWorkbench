// 本文件定义个人中心的资料视图、资料更新与密码修改请求对象。

package domain

// UserProfileVO 个人中心资料视图（比登录用户视图多备注与更新时间）。
type UserProfileVO struct {
	UserULID       string   `json:"user_ulid"`        // 用户 ULID
	UserName       string   `json:"user_name"`        // 登录名（不可修改）
	UserEmail      string   `json:"user_email"`       // 邮箱
	AvatarURL      string   `json:"avatar_url"`       // 头像地址
	UserStatusCode string   `json:"user_status_code"` // 状态：active/disabled
	RoleCodes      []string `json:"role_codes"`       // 角色编码列表
	RemarkText     string   `json:"remark_text"`      // 个人备注
	CreatedAtMs    int64    `json:"created_at_ms"`    // 创建时间（毫秒）
	UpdatedAtMs    int64    `json:"updated_at_ms"`    // 更新时间（毫秒）
}

// UserProfileUpdateREQ 个人资料更新请求（只允许修改邮箱、头像与备注）。
type UserProfileUpdateREQ struct {
	UserEmail  string `json:"user_email" binding:"omitempty,email,max=128"` // 邮箱
	AvatarURL  string `json:"avatar_url" binding:"omitempty,max=256"`       // 头像地址
	RemarkText string `json:"remark_text" binding:"omitempty,max=256"`      // 个人备注
}

// UserPasswordChangeREQ 修改本人密码请求（需校验旧密码）。
type UserPasswordChangeREQ struct {
	OldPasswordText string `json:"old_password_text" binding:"required,min=6,max=64"` // 当前密码
	NewPasswordText string `json:"new_password_text" binding:"required,min=6,max=64"` // 新密码
}

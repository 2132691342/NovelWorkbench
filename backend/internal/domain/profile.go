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
	RoleNameList   []string `json:"role_name_list"`   // 角色名称列表（个人中心标签云用）
	PermCodes      []string `json:"perm_codes"`       // 权限码并集（"*" 表示全部权限）
	RemarkText     string   `json:"remark_text"`      // 个人备注
	CreatedAt      string   `json:"created_at"`       // 创建时间（RFC3339）
	UpdatedAt      string   `json:"updated_at"`       // 更新时间（RFC3339）
}

// UserProfileUpdateREQ 个人资料更新请求（只允许修改邮箱、头像与备注）。
type UserProfileUpdateREQ struct {
	UserEmail  string `json:"user_email" binding:"omitempty,email,max=128"` // 邮箱
	AvatarURL  string `json:"avatar_url" binding:"omitempty,max=256"`       // 头像地址
	RemarkText string `json:"remark_text" binding:"omitempty,max=256"`      // 个人备注
}

// UserPasswordChangeREQ 修改本人密码请求（需校验旧密码；旧密码保留 min=6 以兼容历史短密码）。
type UserPasswordChangeREQ struct {
	OldPasswordText string `json:"old_password_text" binding:"required,min=6,max=64"` // 当前密码
	NewPasswordText string `json:"new_password_text" binding:"required,min=8,max=64"` // 新密码（≥8 且含字母与数字）
}

// UserPasswordChangeDTO 改密结果：返回新令牌，避免本人被自己刚改的密码踢下线。
type UserPasswordChangeDTO struct {
	AccessToken  string `json:"access_token"`   // 新 JWT 访问令牌
	ExpiresInSec int64  `json:"expires_in_sec"` // 有效期（秒）
}

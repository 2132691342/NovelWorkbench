// 本文件定义用户账号的持久化对象、登录请求与用户视图对象。

package domain

// 用户状态码常量。
const (
	UserStatusActive   = "active"   // 正常
	UserStatusDisabled = "disabled" // 禁用（禁止登录）
)

// UserDO 用户账号持久化对象。
type UserDO struct {
	BaseDO
	UserULID       string `json:"user_ulid" gorm:"column:user_ulid;primaryKey;size:40"`                         // 用户 ULID（USR_ 前缀）
	UserName       string `json:"user_name" gorm:"size:64;uniqueIndex"`                                         // 登录名
	UserEmail      string `json:"user_email" gorm:"size:128;uniqueIndex"`                                       // 邮箱
	PasswordHash   string `json:"-" gorm:"size:128"`                                                            // bcrypt 密码哈希，JSON 不输出
	AvatarURL      string `json:"avatar_url" gorm:"size:256"`                                                   // 头像地址
	RemarkText     string `json:"remark_text" gorm:"size:256"`                                                  // 备注
	UserStatusCode string `json:"user_status_code" gorm:"column:user_status_code;size:16;index;default:active"` // 状态：active/disabled
}

// TableName 指定表名。
func (UserDO) TableName() string { return "users" }

// UserLoginREQ 登录请求。
type UserLoginREQ struct {
	AccountText  string `json:"account_text" form:"account_text" binding:"required"`   // 登录名或邮箱
	PasswordText string `json:"password_text" form:"password_text" binding:"required"` // 明文密码，仅入参
}

// UserVO 用户视图对象（登录/个人信息展示）。
type UserVO struct {
	UserULID       string   `json:"user_ulid"`        // 用户 ULID
	UserName       string   `json:"user_name"`        // 登录名
	UserEmail      string   `json:"user_email"`       // 邮箱
	AvatarURL      string   `json:"avatar_url"`       // 头像地址
	UserStatusCode string   `json:"user_status_code"` // 状态：active/disabled
	RoleCodes      []string `json:"role_codes"`       // 角色编码列表
	CreatedAtMs    int64    `json:"created_at_ms"`    // 注册时间（毫秒时间戳）
}

// UserAdminVO 用户管理视图对象（管理端列表/详情）。
type UserAdminVO struct {
	UserVO
	RemarkText  string `json:"remark_text"`   // 备注
	UpdatedAtMs int64  `json:"updated_at_ms"` // 更新时间（毫秒时间戳）
}

// UserAdminListREQ 用户管理分页查询请求（POST body）。
type UserAdminListREQ struct {
	PageREQ
	KeywordText    string `json:"keyword_text" form:"keyword_text" binding:"omitempty,max=64"`         // 登录名/邮箱模糊过滤
	UserStatusCode string `json:"user_status_code" form:"user_status_code" binding:"omitempty,max=16"` // 状态过滤
}

// UserAdminCreateREQ 管理端创建用户请求。
type UserAdminCreateREQ struct {
	UserName     string   `json:"user_name" form:"user_name" binding:"required,min=3,max=64"`         // 登录名
	UserEmail    string   `json:"user_email" form:"user_email" binding:"required,email,max=128"`      // 邮箱
	PasswordText string   `json:"password_text" form:"password_text" binding:"required,min=6,max=64"` // 初始密码，仅入参
	RoleCodes    []string `json:"role_codes" form:"role_codes" binding:"omitempty,max=16"`            // 角色编码列表
	RemarkText   string   `json:"remark_text" form:"remark_text" binding:"omitempty,max=256"`         // 备注
}

// UserAdminUpdateREQ 管理端更新用户请求。
type UserAdminUpdateREQ struct {
	UserULID       string   `json:"user_ulid" form:"user_ulid" binding:"required,max=40"`                // 用户 ULID
	UserEmail      string   `json:"user_email" form:"user_email" binding:"omitempty,email,max=128"`      // 邮箱
	AvatarURL      string   `json:"avatar_url" form:"avatar_url" binding:"omitempty,max=256"`            // 头像地址
	RemarkText     string   `json:"remark_text" form:"remark_text" binding:"omitempty,max=256"`          // 备注
	UserStatusCode string   `json:"user_status_code" form:"user_status_code" binding:"omitempty,max=16"` // 状态：active/disabled
	RoleCodes      []string `json:"role_codes" form:"role_codes" binding:"omitempty,max=16"`             // 角色编码列表（整体覆盖）
}

// UserAdminDeleteREQ 管理端删除用户请求。
type UserAdminDeleteREQ struct {
	UserULID string `json:"user_ulid" form:"user_ulid" binding:"required,max=40"` // 用户 ULID
}

// UserResetPasswordREQ 管理端重置密码请求。
type UserResetPasswordREQ struct {
	UserULID        string `json:"user_ulid" form:"user_ulid" binding:"required,max=40"`                       // 用户 ULID
	NewPasswordText string `json:"new_password_text" form:"new_password_text" binding:"required,min=6,max=64"` // 新密码，仅入参
}

// UserLoginDTO 登录结果（含令牌）。
type UserLoginDTO struct {
	AccessToken  string `json:"access_token"`   // JWT 访问令牌
	ExpiresInSec int64  `json:"expires_in_sec"` // 有效期（秒）
	UserVO       UserVO `json:"user_vo"`        // 用户信息
}

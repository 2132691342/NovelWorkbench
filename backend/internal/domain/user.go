package domain

// UserDO 用户账号持久化对象。
type UserDO struct {
	BaseDO
	UserULID    string `json:"user_ulid" gorm:"column:user_ulid;primaryKey;size:40"` // 用户 ULID（USR_ 前缀）
	UserName    string `json:"user_name" gorm:"size:64;uniqueIndex"` // 登录名
	UserEmail   string `json:"user_email" gorm:"size:128;uniqueIndex"` // 邮箱
	PasswordHash string `json:"-" gorm:"size:128"` // bcrypt 密码哈希，JSON 不输出
	AvatarURL   string `json:"avatar_url" gorm:"size:256"` // 头像地址
	RemarkText  string `json:"remark_text" gorm:"size:256"` // 备注
}

// TableName 指定表名。
func (UserDO) TableName() string { return "users" }

// UserRegisterREQ 注册请求。
type UserRegisterREQ struct {
	UserName     string `json:"user_name" form:"user_name" binding:"required,min=3,max=64"`         // 登录名
	UserEmail    string `json:"user_email" form:"user_email" binding:"required,email,max=128"`      // 邮箱
	PasswordText string `json:"password_text" form:"password_text" binding:"required,min=6,max=64"` // 明文密码，仅入参
}

// UserLoginREQ 登录请求。
type UserLoginREQ struct {
	AccountText  string `json:"account_text" form:"account_text" binding:"required"`    // 登录名或邮箱
	PasswordText string `json:"password_text" form:"password_text" binding:"required"` // 明文密码，仅入参
}

// UserVO 用户视图对象（前端展示）。
type UserVO struct {
	UserULID  string `json:"user_ulid"`  // 用户 ULID
	UserName  string `json:"user_name"`  // 登录名
	UserEmail string `json:"user_email"` // 邮箱
	AvatarURL string `json:"avatar_url"` // 头像地址
	CreatedAtMs int64 `json:"created_at_ms"` // 注册时间（毫秒时间戳）
}

// UserLoginDTO 登录结果（含令牌）。
type UserLoginDTO struct {
	AccessToken  string `json:"access_token"`  // JWT 访问令牌
	ExpiresInSec int64  `json:"expires_in_sec"` // 有效期（秒）
	UserVO       UserVO `json:"user_vo"`        // 用户信息
}

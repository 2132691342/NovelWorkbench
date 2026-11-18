package domain

// 角色与权限码常量：权限码为字符串，"*" 表示全部权限。
const (
	PermAdminAll   = "*"                  // 全部权限（内置 admin 角色）
	PermUserManage = "system:user:manage" // 用户管理
	PermRoleManage = "system:role:manage" // 角色管理
	PermAuditRead  = "system:audit:read"  // 审计日志查看
)

// 内置角色编码常量。
const (
	RoleCodeAdmin = "admin" // 管理员（全部权限）
	RoleCodeUser  = "user"  // 普通用户（无管理权限）
)

// RoleDO 角色持久化对象。
type RoleDO struct {
	BaseDO
	RoleULID      string `json:"role_ulid" gorm:"column:role_ulid;primaryKey;size:40"`    // 角色 ULID（ROL_ 前缀）
	RoleCode      string `json:"role_code" gorm:"column:role_code;size:64;uniqueIndex"`   // 角色编码（唯一）
	RoleName      string `json:"role_name" gorm:"column:role_name;size:64"`               // 角色名称
	RemarkText    string `json:"remark_text" gorm:"size:256"`                             // 备注
	PermCodesJSON string `json:"perm_codes_json" gorm:"column:perm_codes_json;type:text"` // 权限码 JSON 数组
	BuiltinFlag   bool   `json:"builtin_flag" gorm:"column:builtin_flag;default:false"`   // 内置角色（不可删除）
}

// TableName 指定表名。
func (RoleDO) TableName() string { return "roles" }

// UserRoleDO 用户-角色关联持久化对象。
type UserRoleDO struct {
	BaseDO
	UserRoleULID string `json:"user_role_ulid" gorm:"column:user_role_ulid;primaryKey;size:40"` // 关联 ULID（URO_ 前缀）
	UserULID     string `json:"user_ulid" gorm:"column:user_ulid;size:40;index"`                // 用户 ULID
	RoleULID     string `json:"role_ulid" gorm:"column:role_ulid;size:40;index"`                // 角色 ULID
}

// TableName 指定表名。
func (UserRoleDO) TableName() string { return "user_roles" }

// RoleVO 角色视图对象（前端展示）。
type RoleVO struct {
	RoleULID    string   `json:"role_ulid"`     // 角色 ULID
	RoleCode    string   `json:"role_code"`     // 角色编码
	RoleName    string   `json:"role_name"`     // 角色名称
	RemarkText  string   `json:"remark_text"`   // 备注
	PermCodes   []string `json:"perm_codes"`    // 权限码列表
	BuiltinFlag bool     `json:"builtin_flag"`  // 是否内置角色
	CreatedAtMs int64    `json:"created_at_ms"` // 创建时间（毫秒）
	UpdatedAtMs int64    `json:"updated_at_ms"` // 更新时间（毫秒）
}

// RoleListREQ 角色分页查询请求（POST body）。
type RoleListREQ struct {
	PageREQ
}

// RoleSaveREQ 角色创建/更新请求（role_ulid 为空表示创建）。
type RoleSaveREQ struct {
	RoleULID   string   `json:"role_ulid" form:"role_ulid" binding:"omitempty,max=40"`      // 角色 ULID，空为创建
	RoleCode   string   `json:"role_code" form:"role_code" binding:"required,max=64"`       // 角色编码
	RoleName   string   `json:"role_name" form:"role_name" binding:"required,max=64"`       // 角色名称
	RemarkText string   `json:"remark_text" form:"remark_text" binding:"omitempty,max=256"` // 备注
	PermCodes  []string `json:"perm_codes" form:"perm_codes" binding:"omitempty,max=32"`    // 权限码列表
}

// RoleDeleteREQ 角色删除请求。
type RoleDeleteREQ struct {
	RoleULID string `json:"role_ulid" form:"role_ulid" binding:"required,max=40"` // 角色 ULID
}

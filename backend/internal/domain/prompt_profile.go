// 本文件定义系统提示词档案（PromptProfile）的持久化对象、请求与视图对象。
// 档案决定 AI 对话的「身份与底线」：全局档案所有项目可用，项目档案只对所属项目生效。

package domain

// 提示词档案作用域编码常量。
const (
	PromptProfileScopeGlobal  = "global"  // 全局档案（所有项目可用）
	PromptProfileScopeProject = "project" // 项目档案（仅所属项目可用）
)

// PromptProfileDO 系统提示词档案持久化对象。
type PromptProfileDO struct {
	BaseDO
	PromptProfileULID   string `json:"prompt_profile_ulid" gorm:"column:prompt_profile_ulid;primaryKey;size:40"` // 档案 ULID（PMP_ 前缀）
	PromptProfileCode   string `json:"prompt_profile_code" gorm:"column:prompt_profile_code;size:32;index"`      // 档案编码（唯一识别用）
	PromptProfileName   string `json:"prompt_profile_name" gorm:"column:prompt_profile_name;size:64"`            // 档案名
	ScopeCode           string `json:"scope_code" gorm:"column:scope_code;size:16;index;default:global"`         // 作用域：global/project
	ProjectULID         string `json:"project_ulid" gorm:"column:project_ulid;size:40;index"`                    // 所属项目（全局档案为空）
	DescriptionText     string `json:"description_text" gorm:"column:description_text;size:256"`                 // 一句话说明
	SystemPromptText    string `json:"system_prompt_text" gorm:"column:system_prompt_text;type:text"`            // 系统提示词正文
	BuiltinFlag         bool   `json:"builtin_flag" gorm:"column:builtin_flag;default:false"`                    // 是否内置（内置不可删）
	EnabledFlag         bool   `json:"enabled_flag" gorm:"column:enabled_flag;default:true"`                     // 是否启用
	SortOrder           int    `json:"sort_order" gorm:"column:sort_order"`                                      // 排序号
}

// TableName 指定表名。
func (PromptProfileDO) TableName() string { return "prompt_profiles" }

// PromptProfileREQ 创建/更新档案请求（prompt_profile_ulid 为空表示创建）。
type PromptProfileREQ struct {
	PromptProfileULID string `json:"prompt_profile_ulid" binding:"omitempty,max=40"`                            // 档案 ULID
	PromptProfileCode string `json:"prompt_profile_code" binding:"required,min=2,max=32"`                       // 档案编码
	PromptProfileName string `json:"prompt_profile_name" binding:"required,min=1,max=64"`                       // 档案名
	ScopeCode         string `json:"scope_code" binding:"omitempty,oneof=global project"`                        // 作用域
	ProjectULID       string `json:"project_ulid" binding:"omitempty,max=40"`                                   // 项目 ULID（项目档案必填）
	DescriptionText   string `json:"description_text" binding:"omitempty,max=256"`                              // 说明
	SystemPromptText  string `json:"system_prompt_text" binding:"required,min=10,max=8000"`                     // 系统提示词正文
	EnabledFlag       *bool  `json:"enabled_flag" binding:"omitempty"`                                          // 是否启用（空=保持原值）
	SortOrder         int    `json:"sort_order" binding:"omitempty,min=0"`                                      // 排序号
}

// PromptProfileListREQ 档案列表查询。
type PromptProfileListREQ struct {
	ProjectULID string `json:"project_ulid" binding:"omitempty,max=40"` // 项目 ULID（空则只列全局档案）
	EnabledOnly bool   `json:"enabled_only"`                            // 仅返回启用的档案
}

// PromptProfileDeleteREQ 删除档案请求。
type PromptProfileDeleteREQ struct {
	PromptProfileULID string `json:"prompt_profile_ulid" binding:"required"` // 档案 ULID
}

// PromptProfileVO 档案视图对象。
type PromptProfileVO struct {
	PromptProfileULID string `json:"prompt_profile_ulid"` // 档案 ULID
	PromptProfileCode string `json:"prompt_profile_code"` // 档案编码
	PromptProfileName string `json:"prompt_profile_name"` // 档案名
	ScopeCode         string `json:"scope_code"`          // 作用域
	ProjectULID       string `json:"project_ulid"`        // 所属项目
	DescriptionText   string `json:"description_text"`    // 说明
	SystemPromptText  string `json:"system_prompt_text"`  // 系统提示词正文
	BuiltinFlag       bool   `json:"builtin_flag"`        // 是否内置
	EnabledFlag       bool   `json:"enabled_flag"`        // 是否启用
	SortOrder         int    `json:"sort_order"`          // 排序号
	UpdatedAtMs       int64  `json:"updated_at_ms"`       // 更新时间（毫秒时间戳）
}

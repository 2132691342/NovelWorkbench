// 本文件定义模型生态的持久化对象：供应商（网关凭据）、模型目录（可分配的最小单位）与角色授权。
// 分配粒度是「模型」：管理员把某个供应商下的具体模型授权给角色，用户按角色并集拿到可用模型。

package domain

import "time"

// 模型协议编码常量（仅支持 OpenAI 兼容与 Anthropic 原生两种对话协议）。
const (
	ProtocolCodeOpenAI    = "openai"    // OpenAI 兼容（/v1/chat/completions）
	ProtocolCodeAnthropic = "anthropic" // Anthropic 原生（/v1/messages）
)

// 供应商/模型连通性测试状态常量。
const (
	ProviderTestStatusUntested = "untested" // 未测试
	ProviderTestStatusSuccess  = "success"  // 连通
	ProviderTestStatusFailed   = "failed"   // 不通
)

// ModelProviderDO 模型供应商持久化对象（一个供应商 = 一套网关地址 + 凭据，可挂多个模型）。
type ModelProviderDO struct {
	BaseDO
	ProviderULID       string    `json:"provider_ulid" gorm:"column:provider_ulid;primaryKey;size:40"`      // 供应商 ULID（MPR_ 前缀）
	ProviderName       string    `json:"provider_name" gorm:"column:provider_name;size:64"`                 // 供应商名称
	ProtocolCode       string    `json:"protocol_code" gorm:"column:protocol_code;size:16;index"`           // 协议：openai/anthropic
	BaseURL            string    `json:"base_url" gorm:"column:base_url;size:256"`                          // 网关地址（OpenAI 需含 /v1）
	APIKeyCipher       string    `json:"-" gorm:"column:api_key_cipher;size:512"`                           // API Key 密文（AES-256-GCM）
	EnabledFlag        bool      `json:"enabled_flag" gorm:"column:enabled_flag;default:true"`              // 是否启用（停用则其下模型一律不可用）
	SortOrder          int       `json:"sort_order" gorm:"column:sort_order"`                               // 排序号
	RemarkText         string    `json:"remark_text" gorm:"column:remark_text;size:256"`                    // 备注
	LastTestStatusCode string    `json:"last_test_status_code" gorm:"column:last_test_status_code;size:16"` // 最近测试状态：untested/success/failed
	LastTestMessage    string    `json:"last_test_message" gorm:"column:last_test_message;size:512"`        // 最近测试结论
	LastTestAt         time.Time `json:"last_test_at" gorm:"column:last_test_at"`                           // 最近测试时间（RFC3339）
}

// TableName 指定表名。
func (ModelProviderDO) TableName() string { return "model_providers" }

// ModelCatalogDO 模型目录持久化对象（供应商下的具体模型，含调用参数与默认标记）。
type ModelCatalogDO struct {
	BaseDO
	ModelULID          string    `json:"model_ulid" gorm:"column:model_ulid;primaryKey;size:40"`            // 模型 ULID（MDL_ 前缀）
	ProviderULID       string    `json:"provider_ulid" gorm:"column:provider_ulid;size:40;index"`           // 所属供应商
	ModelName          string    `json:"model_name" gorm:"column:model_name;size:128"`                      // 模型名（请求体里的 model 字段）
	ModelLabel         string    `json:"model_label" gorm:"column:model_label;size:64"`                     // 展示名（空则用模型名）
	Temperature        float64   `json:"temperature" gorm:"column:temperature"`                             // 采样温度
	MaxTokens          int       `json:"max_tokens" gorm:"column:max_tokens"`                               // 单次输出上限
	ContextWindowCount int       `json:"context_window_count" gorm:"column:context_window_count"`           // 上下文窗口（token，0 未知）
	EnabledFlag        bool      `json:"enabled_flag" gorm:"column:enabled_flag;default:true"`              // 是否启用
	DefaultFlag        bool      `json:"default_flag" gorm:"column:default_flag;default:false;index"`       // 是否系统默认（全局唯一）
	SortOrder          int       `json:"sort_order" gorm:"column:sort_order"`                               // 排序号
	RemarkText         string    `json:"remark_text" gorm:"column:remark_text;size:256"`                    // 备注
	LastTestStatusCode string    `json:"last_test_status_code" gorm:"column:last_test_status_code;size:16"` // 最近测试状态
	LastTestMessage    string    `json:"last_test_message" gorm:"column:last_test_message;size:512"`        // 最近测试结论
	LastTestAt         time.Time `json:"last_test_at" gorm:"column:last_test_at"`                           // 最近测试时间（RFC3339）
}

// TableName 指定表名。
func (ModelCatalogDO) TableName() string { return "model_catalogs" }

// ModelRoleGrantDO 模型-角色授权持久化对象（模型 × 角色唯一；管理员不受限）。
type ModelRoleGrantDO struct {
	BaseDO
	GrantULID string `json:"grant_ulid" gorm:"column:grant_ulid;primaryKey;size:40"` // 授权 ULID（MGR_ 前缀）
	ModelULID string `json:"model_ulid" gorm:"column:model_ulid;size:40;index"`      // 模型 ULID
	RoleULID  string `json:"role_ulid" gorm:"column:role_ulid;size:40;index"`        // 角色 ULID
}

// TableName 指定表名。
func (ModelRoleGrantDO) TableName() string { return "model_role_grants" }

// ModelUserGrantDO 模型-用户授权持久化对象（模型 × 用户唯一）。
// 用户直授优先于角色授权：删角色不影响用户直授，删用户则级联清理其直授。
type ModelUserGrantDO struct {
	BaseDO
	GrantULID string `json:"grant_ulid" gorm:"column:grant_ulid;primaryKey;size:40"` // 授权 ULID（MUG_ 前缀）
	ModelULID string `json:"model_ulid" gorm:"column:model_ulid;size:40;index"`      // 模型 ULID
	UserULID  string `json:"user_ulid" gorm:"column:user_ulid;size:40;index"`        // 用户 ULID
}

// TableName 指定表名。
func (ModelUserGrantDO) TableName() string { return "model_user_grants" }

// ModelProviderREQ 供应商创建/更新请求（provider_ulid 为空表示创建）。
type ModelProviderREQ struct {
	ProviderULID string `json:"provider_ulid" form:"provider_ulid" binding:"omitempty,max=40"`                // 供应商 ULID，空为创建
	ProviderName string `json:"provider_name" form:"provider_name" binding:"required,min=1,max=64"`           // 供应商名称
	ProtocolCode string `json:"protocol_code" form:"protocol_code" binding:"required,oneof=openai anthropic"` // 协议
	BaseURL      string `json:"base_url" form:"base_url" binding:"required,url,max=256"`                      // 网关地址
	APIKeyText   string `json:"api_key_text" form:"api_key_text" binding:"omitempty,max=256"`                 // API Key 明文（空=保持原值）
	EnabledFlag  *bool  `json:"enabled_flag" form:"enabled_flag"`                                             // 启用状态（nil 保持）
	SortOrder    int    `json:"sort_order" form:"sort_order"`                                                 // 排序号
	RemarkText   string `json:"remark_text" form:"remark_text" binding:"omitempty,max=256"`                   // 备注
}

// ModelProviderDeleteREQ 删除供应商请求（存在模型时拒绝，先删模型）。
type ModelProviderDeleteREQ struct {
	ProviderULID string `json:"provider_ulid" form:"provider_ulid" binding:"required,max=40"` // 供应商 ULID
}

// ModelProviderTestREQ 供应商连通性测试请求（用其下首个启用模型探测）。
type ModelProviderTestREQ struct {
	ProviderULID string `json:"provider_ulid" form:"provider_ulid" binding:"required,max=40"` // 供应商 ULID
}

// ModelCatalogREQ 模型创建/更新请求（model_ulid 为空表示创建）。
type ModelCatalogREQ struct {
	ModelULID          string   `json:"model_ulid" form:"model_ulid" binding:"omitempty,max=40"`           // 模型 ULID，空为创建
	ProviderULID       string   `json:"provider_ulid" form:"provider_ulid" binding:"required,max=40"`      // 所属供应商
	ModelName          string   `json:"model_name" form:"model_name" binding:"required,min=1,max=128"`     // 模型名
	ModelLabel         string   `json:"model_label" form:"model_label" binding:"omitempty,max=64"`         // 展示名
	Temperature        *float64 `json:"temperature" form:"temperature" binding:"omitempty,min=0,max=2"`    // 采样温度
	MaxTokens          int      `json:"max_tokens" form:"max_tokens" binding:"omitempty,min=0,max=128000"` // 输出上限
	ContextWindowCount int      `json:"context_window_count" binding:"omitempty,min=0,max=10000000"`       // 上下文窗口
	EnabledFlag        *bool    `json:"enabled_flag" form:"enabled_flag"`                                  // 启用状态（nil 保持）
	DefaultFlag        *bool    `json:"default_flag" form:"default_flag"`                                  // 默认标记（nil 保持）
	SortOrder          int      `json:"sort_order" form:"sort_order"`                                      // 排序号
	RemarkText         string   `json:"remark_text" form:"remark_text" binding:"omitempty,max=256"`        // 备注
	RoleULIDList       []string `json:"role_ulid_list" binding:"omitempty,max=32"`                         // 保存时整体覆盖的角色授权
	UserULIDList       []string `json:"user_ulid_list" binding:"omitempty,max=200"`                        // 保存时整体覆盖的用户直授
}

// ModelCatalogListREQ 模型目录查询请求（provider_ulid 为空查全部）。
type ModelCatalogListREQ struct {
	ProviderULID string `json:"provider_ulid" form:"provider_ulid" binding:"omitempty,max=40"` // 供应商 ULID
}

// ModelCatalogDeleteREQ 删除模型请求。
type ModelCatalogDeleteREQ struct {
	ModelULID string `json:"model_ulid" form:"model_ulid" binding:"required,max=40"` // 模型 ULID
}

// ModelCatalogTestREQ 模型连通性测试请求。
type ModelCatalogTestREQ struct {
	ModelULID string `json:"model_ulid" form:"model_ulid" binding:"required,max=40"` // 模型 ULID
}

// ModelGrantREQ 模型授权请求（整体覆盖该模型的角色与用户直授）。
type ModelGrantREQ struct {
	ModelULID    string   `json:"model_ulid" form:"model_ulid" binding:"required,max=40"` // 模型 ULID
	RoleULIDList []string `json:"role_ulid_list" binding:"omitempty,max=32"`              // 角色 ULID 列表（空=取消角色分配）
	UserULIDList []string `json:"user_ulid_list" binding:"omitempty,max=200"`             // 用户 ULID 列表（空=取消用户直授）
}

// ModelProviderVO 供应商视图对象（密钥仅回显掩码）。
type ModelProviderVO struct {
	ProviderULID       string `json:"provider_ulid"`         // 供应商 ULID
	ProviderName       string `json:"provider_name"`         // 供应商名称
	ProtocolCode       string `json:"protocol_code"`         // 协议
	BaseURL            string `json:"base_url"`              // 网关地址
	APIKeyMasked       string `json:"api_key_masked"`        // 密钥掩码
	APIKeyConfigured   bool   `json:"api_key_configured"`    // 是否已配置密钥
	EnabledFlag        bool   `json:"enabled_flag"`          // 是否启用
	SortOrder          int    `json:"sort_order"`            // 排序号
	RemarkText         string `json:"remark_text"`           // 备注
	ModelCount         int    `json:"model_count"`           // 模型数量
	EnabledModelCount  int    `json:"enabled_model_count"`   // 启用模型数量
	LastTestStatusCode string `json:"last_test_status_code"` // 最近测试状态
	LastTestMessage    string `json:"last_test_message"`     // 最近测试结论
	LastTestAt         string `json:"last_test_at"`          // 最近测试时间（RFC3339）
	UpdatedAt          string `json:"updated_at"`            // 更新时间（RFC3339）
}

// ModelCatalogVO 模型视图对象（含供应商快照与角色授权）。
type ModelCatalogVO struct {
	ModelULID          string   `json:"model_ulid"`            // 模型 ULID
	ProviderULID       string   `json:"provider_ulid"`         // 所属供应商
	ProviderName       string   `json:"provider_name"`         // 供应商名称
	ProtocolCode       string   `json:"protocol_code"`         // 供应商协议
	BaseURL            string   `json:"base_url"`              // 供应商网关地址
	ProviderEnabled    bool     `json:"provider_enabled"`      // 供应商是否启用
	ModelName          string   `json:"model_name"`            // 模型名
	ModelLabel         string   `json:"model_label"`           // 展示名
	Temperature        float64  `json:"temperature"`           // 采样温度
	MaxTokens          int      `json:"max_tokens"`            // 输出上限
	ContextWindowCount int      `json:"context_window_count"`  // 上下文窗口
	EnabledFlag        bool     `json:"enabled_flag"`          // 是否启用
	DefaultFlag        bool     `json:"default_flag"`          // 是否系统默认
	SortOrder          int      `json:"sort_order"`            // 排序号
	RemarkText         string   `json:"remark_text"`           // 备注
	RoleULIDList       []string `json:"role_ulid_list"`        // 已授权角色
	RoleNameList       []string `json:"role_name_list"`        // 已授权角色名称
	UserULIDList       []string `json:"user_ulid_list"`        // 用户直授 ULID
	UserLabelList      []string `json:"user_label_list"`       // 用户直授展示（登录名，供前端标签）
	LastTestStatusCode string   `json:"last_test_status_code"` // 最近测试状态
	LastTestMessage    string   `json:"last_test_message"`     // 最近测试结论
	LastTestAt         string   `json:"last_test_at"`          // 最近测试时间（RFC3339）
	UpdatedAt          string   `json:"updated_at"`            // 更新时间（RFC3339）
}

// ModelProviderTestDTO 供应商连通性测试结果。
type ModelProviderTestDTO struct {
	StatusCode   string `json:"status_code"`    // success/failed
	MessageText  string `json:"message_text"`   // 结论说明
	LatencyMs    int64  `json:"latency_ms"`     // 往返耗时（RFC3339）
	ReplyText    string `json:"reply_text"`     // 模型回显片段
	ProbeModelID string `json:"probe_model_id"` // 用于探测的模型 ULID
}

// ModelCatalogTestDTO 模型连通性测试结果。
type ModelCatalogTestDTO struct {
	StatusCode  string `json:"status_code"`  // success/failed
	MessageText string `json:"message_text"` // 结论说明
	LatencyMs   int64  `json:"latency_ms"`   // 往返耗时（RFC3339）
	ReplyText   string `json:"reply_text"`   // 模型回显片段
}

// ModelOptionVO 对话可用模型选项（AI 面板选择器消费，按角色授权过滤）。
type ModelOptionVO struct {
	ModelULID      string `json:"model_ulid"`       // 模型 ULID
	ModelName      string `json:"model_name"`       // 模型名
	ModelLabel     string `json:"model_label"`      // 展示名
	ProviderName   string `json:"provider_name"`    // 供应商名称
	ProtocolCode   string `json:"protocol_code"`    // 协议
	DefaultFlag    bool   `json:"default_flag"`     // 是否默认
	MaxTokens      int    `json:"max_tokens"`       // 输出上限
	ContextWindow  int    `json:"context_window"`   // 上下文窗口
	LastTestStatus string `json:"last_test_status"` // 最近测试状态
}

// ModelCapabilityVO 当前用户可用能力探测（前端据此提示 AI 是否可用）。
type ModelCapabilityVO struct {
	ChatReady           bool            `json:"chat_ready"`            // 对话链路是否可用
	DefaultModelULID    string          `json:"default_model_ulid"`    // 默认模型 ULID
	DefaultModelLabel   string          `json:"default_model_label"`   // 默认模型展示名
	DefaultModelName    string          `json:"default_model_name"`    // 默认模型名
	DefaultProviderName string          `json:"default_provider_name"` // 默认模型所属供应商
	OptionList          []ModelOptionVO `json:"option_list"`           // 可用模型选项
	HintText            string          `json:"hint_text"`             // 面向用户的提示文案
}

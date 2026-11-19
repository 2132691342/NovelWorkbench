package domain

// 模型协议编码常量（仅支持 OpenAI 兼容与 Anthropic 原生两种对话协议）。
const (
	ProtocolCodeOpenAI    = "openai"    // OpenAI 兼容（/v1/chat/completions）
	ProtocolCodeAnthropic = "anthropic" // Anthropic 原生（/v1/messages）
)

// 供应商连通性测试状态常量。
const (
	ProviderTestStatusUntested = "untested" // 未测试
	ProviderTestStatusSuccess  = "success"  // 连通
	ProviderTestStatusFailed   = "failed"   // 不通
)

// ModelProviderDO 模型供应商持久化对象（一个供应商 = 一套网关地址 + 凭据 + 对话模型名）。
type ModelProviderDO struct {
	BaseDO
	ProviderULID       string  `json:"provider_ulid" gorm:"column:provider_ulid;primaryKey;size:40"`      // 供应商 ULID（MPR_ 前缀）
	ProviderName       string  `json:"provider_name" gorm:"column:provider_name;size:64"`                 // 供应商名称（展示用）
	ProtocolCode       string  `json:"protocol_code" gorm:"column:protocol_code;size:16;index"`           // 协议：openai/anthropic
	BaseURL            string  `json:"base_url" gorm:"column:base_url;size:256"`                          // 网关地址（OpenAI 需含 /v1）
	APIKeyCipher       string  `json:"-" gorm:"column:api_key_cipher;size:512"`                           // API Key 密文（AES-256-GCM）
	ChatModelName      string  `json:"chat_model_name" gorm:"column:chat_model_name;size:128"`            // 对话模型名
	Temperature        float64 `json:"temperature" gorm:"column:temperature"`                             // 采样温度
	MaxTokens          int     `json:"max_tokens" gorm:"column:max_tokens"`                               // 单次输出上限
	EnabledFlag        bool    `json:"enabled_flag" gorm:"column:enabled_flag;default:true"`              // 是否启用
	DefaultFlag        bool    `json:"default_flag" gorm:"column:default_flag;default:false;index"`       // 是否默认（全局唯一，由服务层保证）
	SortOrder          int     `json:"sort_order" gorm:"column:sort_order"`                               // 排序号
	RemarkText         string  `json:"remark_text" gorm:"column:remark_text;size:256"`                    // 备注
	LastTestStatusCode string  `json:"last_test_status_code" gorm:"column:last_test_status_code;size:16"` // 最近测试状态：untested/success/failed
	LastTestMessage    string  `json:"last_test_message" gorm:"column:last_test_message;size:512"`        // 最近测试结论
	LastTestAtMs       int64   `json:"last_test_at_ms" gorm:"column:last_test_at_ms"`                     // 最近测试时间（毫秒）
}

// TableName 指定表名。
func (ModelProviderDO) TableName() string { return "model_providers" }

// ModelProviderREQ 供应商创建/更新请求（provider_ulid 为空表示创建）。
type ModelProviderREQ struct {
	ProviderULID  string   `json:"provider_ulid" form:"provider_ulid" binding:"omitempty,max=40"`                // 供应商 ULID，空为创建
	ProviderName  string   `json:"provider_name" form:"provider_name" binding:"required,min=1,max=64"`           // 供应商名称
	ProtocolCode  string   `json:"protocol_code" form:"protocol_code" binding:"required,oneof=openai anthropic"` // 协议
	BaseURL       string   `json:"base_url" form:"base_url" binding:"required,url,max=256"`                      // 网关地址
	APIKeyText    string   `json:"api_key_text" form:"api_key_text" binding:"omitempty,max=256"`                 // API Key 明文（空=保持原值）
	ChatModelName string   `json:"chat_model_name" form:"chat_model_name" binding:"required,max=128"`            // 对话模型名
	Temperature   *float64 `json:"temperature" form:"temperature" binding:"omitempty,min=0,max=2"`               // 采样温度（nil 用默认）
	MaxTokens     int      `json:"max_tokens" form:"max_tokens" binding:"omitempty,min=0,max=128000"`            // 输出上限
	EnabledFlag   *bool    `json:"enabled_flag" form:"enabled_flag"`                                             // 启用状态（nil 保持）
	DefaultFlag   *bool    `json:"default_flag" form:"default_flag"`                                             // 默认标记（nil 保持）
	SortOrder     int      `json:"sort_order" form:"sort_order"`                                                 // 排序号
	RemarkText    string   `json:"remark_text" form:"remark_text" binding:"omitempty,max=256"`                   // 备注
}

// ModelProviderDeleteREQ 删除供应商请求。
type ModelProviderDeleteREQ struct {
	ProviderULID string `json:"provider_ulid" form:"provider_ulid" binding:"required,max=40"` // 供应商 ULID
}

// ModelProviderTestREQ 供应商连通性测试请求。
type ModelProviderTestREQ struct {
	ProviderULID string `json:"provider_ulid" form:"provider_ulid" binding:"required,max=40"` // 供应商 ULID
}

// ModelProviderVO 供应商视图对象（密钥仅回显掩码，不回传密文或明文）。
type ModelProviderVO struct {
	ProviderULID       string  `json:"provider_ulid"`         // 供应商 ULID
	ProviderName       string  `json:"provider_name"`         // 供应商名称
	ProtocolCode       string  `json:"protocol_code"`         // 协议
	BaseURL            string  `json:"base_url"`              // 网关地址
	APIKeyMasked       string  `json:"api_key_masked"`        // 密钥掩码（如 sk-1***4f）
	APIKeyConfigured   bool    `json:"api_key_configured"`    // 是否已配置密钥
	ChatModelName      string  `json:"chat_model_name"`       // 对话模型名
	Temperature        float64 `json:"temperature"`           // 采样温度
	MaxTokens          int     `json:"max_tokens"`            // 输出上限
	EnabledFlag        bool    `json:"enabled_flag"`          // 是否启用
	DefaultFlag        bool    `json:"default_flag"`          // 是否默认
	SortOrder          int     `json:"sort_order"`            // 排序号
	RemarkText         string  `json:"remark_text"`           // 备注
	LastTestStatusCode string  `json:"last_test_status_code"` // 最近测试状态
	LastTestMessage    string  `json:"last_test_message"`     // 最近测试结论
	LastTestAtMs       int64   `json:"last_test_at_ms"`       // 最近测试时间（毫秒）
	UpdatedAtMs        int64   `json:"updated_at_ms"`         // 更新时间（毫秒）
}

// ModelProviderTestDTO 连通性测试结果（仅验证对话通道）。
type ModelProviderTestDTO struct {
	StatusCode  string `json:"status_code"`  // success/failed
	MessageText string `json:"message_text"` // 结论说明
	LatencyMs   int64  `json:"latency_ms"`   // 往返耗时（毫秒）
	ReplyText   string `json:"reply_text"`   // 模型回显片段（成功时可能为空）
}

// ModelCapabilityVO 当前可用能力探测（前端据此提示 AI 对话是否可用）。
type ModelCapabilityVO struct {
	ChatReady            bool   `json:"chat_ready"`              // 对话链路是否可用
	DefaultProviderULID  string `json:"default_provider_ulid"`   // 默认供应商 ULID
	DefaultProviderName  string `json:"default_provider_name"`   // 默认供应商名称
	DefaultChatModelName string `json:"default_chat_model_name"` // 默认供应商对话模型
	HintText             string `json:"hint_text"`               // 面向用户的提示文案
}

// 本文件定义斜杠命令（技能资产）的持久化对象、请求与视图对象。

package domain

// SlashCommandDO 斜杠命令/技能持久化对象（/续写 等，本质是带占位符的提示词资产）。
type SlashCommandDO struct {
	BaseDO
	CommandULID     string `json:"command_ulid" gorm:"column:command_ulid;primaryKey;size:40"` // 命令 ULID（SKL_ 前缀）
	CommandCode     string `json:"command_code" gorm:"size:32;uniqueIndex"`                    // 命令编码（chat 中 /code 触发）
	CommandName     string `json:"command_name" gorm:"size:64"`                                // 命令名（展示用）
	CategoryCode    string `json:"category_code" gorm:"size:16"`                               // 分类：write/revise/outline/setting/other
	DescriptionText string `json:"description_text" gorm:"size:256"`                           // 一句话说明
	PromptTemplate  string `json:"prompt_template" gorm:"type:text"`                           // 提示词模板（{{占位符}}）
	EnabledFlag     bool   `json:"enabled_flag" gorm:"default:true"`                           // 是否启用
	BuiltinFlag     bool   `json:"builtin_flag" gorm:"default:true"`                           // 是否内置（内置不可删）
	SortOrder       int    `json:"sort_order"`                                                 // 排序号
}

// TableName 指定表名。
func (SlashCommandDO) TableName() string { return "slash_commands" }

// SlashCommandREQ 创建/更新斜杠命令请求。
type SlashCommandREQ struct {
	CommandULID     string `json:"command_ulid" binding:"omitempty"`                                           // 命令 ULID，更新时必填
	CommandCode     string `json:"command_code" binding:"required,max=32"`                                     // 命令编码
	CommandName     string `json:"command_name" binding:"required,max=64"`                                     // 命令名
	CategoryCode    string `json:"category_code" binding:"omitempty,oneof=write revise outline setting other"` // 分类
	DescriptionText string `json:"description_text" binding:"omitempty,max=256"`                               // 说明
	PromptTemplate  string `json:"prompt_template" binding:"required,max=8000"`                                // 提示词模板
	EnabledFlag     *bool  `json:"enabled_flag"`                                                               // 是否启用（nil 保持原值）
	SortOrder       int    `json:"sort_order"`                                                                 // 排序号
}

// SlashCommandVO 斜杠命令视图对象（前端 / 自动补全面板消费）。
type SlashCommandVO struct {
	CommandULID     string `json:"command_ulid"`     // 命令 ULID
	CommandCode     string `json:"command_code"`     // 命令编码
	CommandName     string `json:"command_name"`     // 命令名
	CategoryCode    string `json:"category_code"`    // 分类
	DescriptionText string `json:"description_text"` // 说明
	PromptTemplate  string `json:"prompt_template"`  // 提示词模板
	EnabledFlag     bool   `json:"enabled_flag"`     // 是否启用
	BuiltinFlag     bool   `json:"builtin_flag"`     // 是否内置
	SortOrder       int    `json:"sort_order"`       // 排序号
}

// SlashListREQ 斜杠命令列表查询（GET query）。
type SlashListREQ struct {
	EnabledOnly bool `json:"enabled_only" form:"enabled_only"` // 仅返回启用命令
}

// SlashDeleteREQ 删除斜杠命令请求（内置不可删）。
type SlashDeleteREQ struct {
	CommandULID string `json:"command_ulid" form:"command_ulid" binding:"required"` // 命令 ULID
}

// SlashParseREQ 解析正文斜杠命令请求。
type SlashParseREQ struct {
	ContentText string `json:"content_text" form:"content_text" binding:"required,max=20000"` // 待解析正文
}

// SlashParseDTO 斜杠命令解析结果（chat 入口消费）。
type SlashParseDTO struct {
	SkillCode      string            `json:"skill_code"`      // 命中命令编码（空为自由对话）
	RemainderText  string            `json:"remainder_text"`  // 去掉命令后的用户正文
	PlaceholderMap map[string]string `json:"placeholder_map"` // 模板占位符补充参数
}

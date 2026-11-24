// 本文件定义提示词（Prompt）资产的持久化对象、请求与视图对象。
// 一份提示词分两种：身份提示词（system）整段会话常驻，指令提示词（command）由 /编码 单次触发。
// 二者都以系统级消息注入模型，用户输入始终是用户级消息。

package domain

// 提示词作用域编码常量。
const (
	PromptScopeGlobal  = "global"  // 全局提示词（所有项目可用）
	PromptScopeProject = "project" // 项目提示词（仅所属项目可用，同编码覆盖全局）
)

// 提示词种类编码常量。
const (
	PromptKindSystem  = "system"  // 身份与底线
	PromptKindCommand = "command" // 单次任务指令
)

// 提示词分类编码常量。
const (
	PromptCategoryWrite   = "write"   // 写作
	PromptCategoryRevise  = "revise"  // 修订
	PromptCategoryOutline = "outline" // 规划
	PromptCategorySetting = "setting" // 设定
	PromptCategoryReview  = "review"  // 审稿
	PromptCategoryOther   = "other"   // 其他
)

// reservedActionCodeList 写作台斜杠动作编码（前端 composerCommands.ts 同一份清单）。
// 动作在客户端就地执行、不经过模型，因此不能被提示词占用，否则 /help 之类的输入
// 到底是「执行动作」还是「注入提示词」将取决于实现顺序，作者无法预期。
var reservedActionCodeList = []string{
	"help", "clear", "history", "new", "attach", "copy", "check", "focus", "snapshot",
}

// IsReservedActionCode 判断编码是否为保留的斜杠动作编码。
func IsReservedActionCode(promptCode string) bool {
	for _, actionCode := range reservedActionCodeList {
		if actionCode == promptCode {
			return true
		}
	}
	return false
}

// PromptDO 提示词持久化对象（带作用域的提示词资产，全局与项目级共存）。
type PromptDO struct {
	BaseDO
	PromptULID      string `json:"prompt_ulid" gorm:"column:prompt_ulid;primaryKey;size:40"`         // 提示词 ULID（PRM_ 前缀）
	PromptCode      string `json:"prompt_code" gorm:"column:prompt_code;size:32;index"`              // 提示词编码（对话中以 /code 触发）
	PromptName      string `json:"prompt_name" gorm:"column:prompt_name;size:64"`                    // 提示词名称
	PromptKindCode  string `json:"prompt_kind_code" gorm:"column:prompt_kind_code;size:16;index"`    // 种类：system/command
	ScopeCode       string `json:"scope_code" gorm:"column:scope_code;size:16;index;default:global"` // 作用域：global/project
	ProjectULID     string `json:"project_ulid" gorm:"column:project_ulid;size:40;index"`            // 所属项目（全局提示词为空）
	DomainCode      string `json:"domain_code" gorm:"column:domain_code;size:32;index"`              // 流程域编码（内置资产才有）
	CategoryCode    string `json:"category_code" gorm:"column:category_code;size:16"`                // 分类：write/revise/outline/setting/review/other
	DescriptionText string `json:"description_text" gorm:"column:description_text;size:256"`         // 一句话说明
	PromptText      string `json:"prompt_text" gorm:"column:prompt_text;type:text"`                  // 提示词正文（可含 {{占位符}}）
	EnabledFlag     bool   `json:"enabled_flag" gorm:"column:enabled_flag;default:true"`             // 是否启用
	BuiltinFlag     bool   `json:"builtin_flag" gorm:"column:builtin_flag;default:false"`            // 是否内置（内置不可删）
	AssetHashText   string `json:"asset_hash_text" gorm:"column:asset_hash_text;size:64"`            // 代码资产指纹（播种/升级时写入）
	CustomizedFlag  bool   `json:"customized_flag" gorm:"column:customized_flag;default:false"`      // 作者是否改过出厂内容（改过则升级不覆盖）
	SortOrder       int    `json:"sort_order" gorm:"column:sort_order"`                              // 排序号
}

// TableName 指定表名。
func (PromptDO) TableName() string { return "prompts" }

// PromptREQ 创建/更新提示词请求（prompt_ulid 为空表示创建）。
type PromptREQ struct {
	PromptULID      string `json:"prompt_ulid" binding:"omitempty,max=40"`                                            // 提示词 ULID
	PromptCode      string `json:"prompt_code" binding:"required,min=2,max=32"`                                       // 提示词编码
	PromptName      string `json:"prompt_name" binding:"required,max=64"`                                             // 提示词名称
	PromptKindCode  string `json:"prompt_kind_code" binding:"omitempty,oneof=system command"`                         // 种类
	ScopeCode       string `json:"scope_code" binding:"omitempty,oneof=global project"`                               // 作用域
	ProjectULID     string `json:"project_ulid" binding:"omitempty,max=40"`                                           // 项目 ULID（项目提示词必填）
	CategoryCode    string `json:"category_code" binding:"omitempty,oneof=write revise outline setting review other"` // 分类
	DescriptionText string `json:"description_text" binding:"omitempty,max=256"`                                      // 说明
	PromptText      string `json:"prompt_text" binding:"required,min=10,max=12000"`                                   // 提示词正文
	EnabledFlag     *bool  `json:"enabled_flag"`                                                                      // 是否启用（nil 保持原值）
	SortOrder       int    `json:"sort_order" binding:"omitempty,min=0,max=9999"`                                     // 排序号
}

// PromptDeleteREQ 删除提示词请求（内置不可删）。
type PromptDeleteREQ struct {
	PromptULID string `json:"prompt_ulid" binding:"required,max=40"` // 提示词 ULID
}

// PromptListREQ 提示词列表查询请求。
type PromptListREQ struct {
	ScopeCode      string `json:"scope_code" form:"scope_code" binding:"omitempty,max=16"`             // 作用域过滤
	ProjectULID    string `json:"project_ulid" form:"project_ulid" binding:"omitempty,max=40"`         // 项目过滤（项目内提示词与全局提示词合并返回）
	PromptKindCode string `json:"prompt_kind_code" form:"prompt_kind_code" binding:"omitempty,max=16"` // 种类过滤
	CategoryCode   string `json:"category_code" form:"category_code" binding:"omitempty,max=16"`       // 分类过滤
	KeywordText    string `json:"keyword_text" form:"keyword_text" binding:"omitempty,max=64"`         // 名称/编码模糊
	EnabledOnly    bool   `json:"enabled_only" form:"enabled_only"`                                    // 仅启用
}

// PromptVO 提示词视图对象（管理端与对话自动补全共用）。
type PromptVO struct {
	PromptULID      string `json:"prompt_ulid"`      // 提示词 ULID
	PromptCode      string `json:"prompt_code"`      // 提示词编码
	PromptName      string `json:"prompt_name"`      // 提示词名称
	PromptKindCode  string `json:"prompt_kind_code"` // 种类
	ScopeCode       string `json:"scope_code"`       // 作用域
	ProjectULID     string `json:"project_ulid"`     // 所属项目
	DomainCode      string `json:"domain_code"`      // 流程域编码
	DomainName      string `json:"domain_name"`      // 流程域名称
	CategoryCode    string `json:"category_code"`    // 分类
	DescriptionText string `json:"description_text"` // 说明
	PromptText      string `json:"prompt_text"`      // 提示词正文
	EnabledFlag     bool   `json:"enabled_flag"`     // 是否启用
	BuiltinFlag     bool   `json:"builtin_flag"`     // 是否内置
	CustomizedFlag  bool   `json:"customized_flag"`  // 作者是否改过出厂内容
	OverriddenFlag  bool   `json:"overridden_flag"`  // 是否被同编码项目提示词覆盖
	SortOrder       int    `json:"sort_order"`       // 排序号
	CreatedAt       string `json:"created_at"`       // 创建时间（RFC3339）
	UpdatedAt       string `json:"updated_at"`       // 更新时间（RFC3339）
}

// PromptResolveREQ 解析提示词编码请求（对话前置：把用户输入解析为提示词）。
type PromptResolveREQ struct {
	ProjectULID string `json:"project_ulid" binding:"required,max=40"`    // 项目 ULID
	ContentText string `json:"content_text" binding:"required,max=20000"` // 待解析正文
	PromptCode  string `json:"prompt_code" binding:"omitempty,max=32"`    // 显式提示词编码（优先）
}

// PromptResolveDTO 提示词解析结果（对话入口消费）。
type PromptResolveDTO struct {
	PromptULID    string `json:"prompt_ulid"`    // 命中提示词 ULID（空为自由对话）
	PromptCode    string `json:"prompt_code"`    // 命中提示词编码
	PromptName    string `json:"prompt_name"`    // 命中提示词名称
	PromptText    string `json:"prompt_text"`    // 提示词正文（保留占位符）
	RemainderText string `json:"remainder_text"` // 去掉斜杠命令后的用户正文
}

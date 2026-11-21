// 本文件定义技能（Skill）资产、技能变量与技能参考资料的持久化对象、请求与视图对象。

package domain

// 技能作用域编码常量。
const (
	SkillScopeGlobal  = "global"  // 全局技能（所有项目可用）
	SkillScopeProject = "project" // 项目技能（仅所属项目可用，同编码覆盖全局）
)

// 技能分类编码常量。
const (
	SkillCategoryWrite   = "write"   // 写作
	SkillCategoryRevise  = "revise"  // 修订
	SkillCategoryOutline = "outline" // 规划
	SkillCategorySetting = "setting" // 设定
	SkillCategoryReview  = "review"  // 审稿
	SkillCategoryOther   = "other"   // 其他
)

// SkillVariableDO 技能变量声明（模板占位符的元数据，随技能一起持久化）。
type SkillVariableDO struct {
	VariableName    string `json:"variable_name"`    // 变量名（模板中写作 {{variable_name}}）
	VariableLabel   string `json:"variable_label"`   // 展示名
	DefaultValue    string `json:"default_value"`    // 缺省值（调用未传时使用）
	RequiredFlag    bool   `json:"required_flag"`    // 是否必填
	DescriptionText string `json:"description_text"` // 填写提示
}

// SkillDO 技能持久化对象（带作用域的提示词资产，全局与项目级共存）。
type SkillDO struct {
	BaseDO
	SkillULID        string `json:"skill_ulid" gorm:"column:skill_ulid;primaryKey;size:40"`           // 技能 ULID（SKL_ 前缀）
	SkillCode        string `json:"skill_code" gorm:"column:skill_code;size:32;index"`                // 技能编码（对话中以 /code 触发）
	SkillName        string `json:"skill_name" gorm:"column:skill_name;size:64"`                      // 技能名称
	ScopeCode        string `json:"scope_code" gorm:"column:scope_code;size:16;index;default:global"` // 作用域：global/project
	ProjectULID      string `json:"project_ulid" gorm:"column:project_ulid;size:40;index"`            // 所属项目（全局技能为空）
	CategoryCode     string `json:"category_code" gorm:"column:category_code;size:16"`                // 分类：write/revise/outline/setting/review/other
	DescriptionText  string `json:"description_text" gorm:"column:description_text;size:256"`         // 一句话说明
	PromptTemplate   string `json:"prompt_template" gorm:"column:prompt_template;type:text"`          // 提示词模板（{{占位符}}）
	VariableSpecJSON string `json:"variable_spec_json" gorm:"column:variable_spec_json;type:text"`    // 变量声明 JSON 数组
	EnabledFlag      bool   `json:"enabled_flag" gorm:"column:enabled_flag;default:true"`             // 是否启用
	BuiltinFlag      bool   `json:"builtin_flag" gorm:"column:builtin_flag;default:false"`            // 是否内置（内置不可删）
	AssetHashText    string `json:"asset_hash_text" gorm:"column:asset_hash_text;size:64"`            // 代码资产指纹（播种/升级时写入，用于判断是否有新版本）
	CustomizedFlag   bool   `json:"customized_flag" gorm:"column:customized_flag;default:false"`      // 作者是否改过出厂内容（改过则代码升级不覆盖）
	SortOrder        int    `json:"sort_order" gorm:"column:sort_order"`                              // 排序号
}

// TableName 指定表名。
func (SkillDO) TableName() string { return "skills" }

// SkillReferenceDO 技能参考资料（技能被命中时按关键词挑片段注入上下文）。
type SkillReferenceDO struct {
	BaseDO
	RefULID     string `json:"ref_ulid" gorm:"column:ref_ulid;primaryKey;size:40"` // 参考资料 ULID（SKR_ 前缀）
	SkillULID   string `json:"skill_ulid" gorm:"column:skill_ulid;size:40;index"`  // 所属技能
	RefName     string `json:"ref_name" gorm:"column:ref_name;size:64"`            // 资料名（如「文风样例」）
	RefText     string `json:"ref_text" gorm:"column:ref_text;type:text"`          // 资料正文
	KeywordList string `json:"keyword_list" gorm:"column:keyword_list;size:256"`   // 触发关键词（逗号分隔，空表示总是注入）
	SortOrder   int    `json:"sort_order" gorm:"column:sort_order"`                // 排序号
}

// TableName 指定表名。
func (SkillReferenceDO) TableName() string { return "skill_references" }

// SkillREQ 创建/更新技能请求（skill_ulid 为空表示创建）。
type SkillREQ struct {
	SkillULID       string            `json:"skill_ulid" binding:"omitempty,max=40"`                                             // 技能 ULID
	SkillCode       string            `json:"skill_code" binding:"required,min=2,max=32"`                                        // 技能编码
	SkillName       string            `json:"skill_name" binding:"required,max=64"`                                              // 技能名称
	ScopeCode       string            `json:"scope_code" binding:"omitempty,oneof=global project"`                               // 作用域
	ProjectULID     string            `json:"project_ulid" binding:"omitempty,max=40"`                                           // 项目 ULID（项目技能必填）
	CategoryCode    string            `json:"category_code" binding:"omitempty,oneof=write revise outline setting review other"` // 分类
	DescriptionText string            `json:"description_text" binding:"omitempty,max=256"`                                      // 说明
	PromptTemplate  string            `json:"prompt_template" binding:"required,max=8000"`                                       // 提示词模板
	VariableList    []SkillVariableDO `json:"variable_list"`                                                                     // 变量声明
	EnabledFlag     *bool             `json:"enabled_flag"`                                                                      // 是否启用（nil 保持原值）
	SortOrder       int               `json:"sort_order" binding:"omitempty,min=0,max=9999"`                                     // 排序号
}

// SkillDeleteREQ 删除技能请求（内置不可删）。
type SkillDeleteREQ struct {
	SkillULID string `json:"skill_ulid" binding:"required,max=40"` // 技能 ULID
}

// SkillListREQ 技能列表查询请求（管理端）。
type SkillListREQ struct {
	ScopeCode    string `json:"scope_code" form:"scope_code" binding:"omitempty,max=16"`       // 作用域过滤
	ProjectULID  string `json:"project_ulid" form:"project_ulid" binding:"omitempty,max=40"`   // 项目过滤（项目内技能与全局技能合并返回）
	CategoryCode string `json:"category_code" form:"category_code" binding:"omitempty,max=16"` // 分类过滤
	KeywordText  string `json:"keyword_text" form:"keyword_text" binding:"omitempty,max=64"`   // 名称/编码模糊
	EnabledOnly  bool   `json:"enabled_only" form:"enabled_only"`                              // 仅启用
}

// SkillVO 技能视图对象（管理端与对话自动补全共用）。
type SkillVO struct {
	SkillULID       string            `json:"skill_ulid"`       // 技能 ULID
	SkillCode       string            `json:"skill_code"`       // 技能编码
	SkillName       string            `json:"skill_name"`       // 技能名称
	ScopeCode       string            `json:"scope_code"`       // 作用域
	ProjectULID     string            `json:"project_ulid"`     // 所属项目
	CategoryCode    string            `json:"category_code"`    // 分类
	DescriptionText string            `json:"description_text"` // 说明
	PromptTemplate  string            `json:"prompt_template"`  // 提示词模板
	VariableList    []SkillVariableDO `json:"variable_list"`    // 变量声明
	RefCount        int               `json:"ref_count"`        // 参考资料条数
	EnabledFlag     bool              `json:"enabled_flag"`     // 是否启用
	BuiltinFlag     bool              `json:"builtin_flag"`     // 是否内置
	CustomizedFlag  bool              `json:"customized_flag"`  // 作者是否改过出厂内容（改过则代码升级不覆盖）
	OverriddenFlag  bool              `json:"overridden_flag"`  // 是否被同编码项目技能覆盖
	DomainCode      string            `json:"domain_code"`      // 所属流程域编码（内置技能才有）
	DomainName      string            `json:"domain_name"`      // 所属流程域名称（内置技能才有）
	SortOrder       int               `json:"sort_order"`       // 排序号
	CreatedAt       string            `json:"created_at"`       // 创建时间
	UpdatedAt       string            `json:"updated_at"`       // 更新时间
}

// SkillReferenceREQ 技能参考资料保存请求（ref_ulid 为空表示新建）。
type SkillReferenceREQ struct {
	RefULID     string `json:"ref_ulid" binding:"omitempty,max=40"`           // 参考资料 ULID
	SkillULID   string `json:"skill_ulid" binding:"required,max=40"`          // 所属技能
	RefName     string `json:"ref_name" binding:"required,max=64"`            // 资料名
	RefText     string `json:"ref_text" binding:"required,max=20000"`         // 资料正文
	KeywordList string `json:"keyword_list" binding:"omitempty,max=256"`      // 触发关键词（逗号分隔）
	SortOrder   int    `json:"sort_order" binding:"omitempty,min=0,max=9999"` // 排序号
}

// SkillReferenceDeleteREQ 删除技能参考资料请求。
type SkillReferenceDeleteREQ struct {
	RefULID string `json:"ref_ulid" binding:"required,max=40"` // 参考资料 ULID
}

// SkillReferenceVO 技能参考资料视图对象。
type SkillReferenceVO struct {
	RefULID     string `json:"ref_ulid"`     // 参考资料 ULID
	SkillULID   string `json:"skill_ulid"`   // 所属技能
	RefName     string `json:"ref_name"`     // 资料名
	RefText     string `json:"ref_text"`     // 资料正文
	KeywordList string `json:"keyword_list"` // 触发关键词
	SortOrder   int    `json:"sort_order"`   // 排序号
	CreatedAt   string `json:"created_at"`   // 创建时间
}

// SkillListBySkillREQ 查询某技能的参考资料请求。
type SkillListBySkillREQ struct {
	SkillULID string `json:"skill_ulid" form:"skill_ulid" binding:"required,max=40"` // 技能 ULID
}

// SkillResolveREQ 解析技能编码请求（对话前置：把用户输入解析为技能 + 变量）。
type SkillResolveREQ struct {
	ProjectULID   string            `json:"project_ulid" binding:"required,max=40"`    // 项目 ULID
	ContentText   string            `json:"content_text" binding:"required,max=20000"` // 待解析正文
	SkillCode     string            `json:"skill_code" binding:"omitempty,max=32"`     // 显式技能编码（优先）
	VariableValue map[string]string `json:"variable_value"`                            // 变量取值
}

// SkillResolveDTO 技能解析结果（对话入口消费）。
type SkillResolveDTO struct {
	SkillULID     string            `json:"skill_ulid"`     // 命中技能 ULID（空为自由对话）
	SkillCode     string            `json:"skill_code"`     // 命中技能编码
	SkillName     string            `json:"skill_name"`     // 命中技能名称
	RemainderText string            `json:"remainder_text"` // 去掉斜杠命令后的用户正文
	VariableMap   map[string]string `json:"variable_map"`   // 渲染后的变量取值
	MissingList   []string          `json:"missing_list"`   // 缺失的必填变量
	RefTextList   []string          `json:"ref_text_list"`  // 命中的参考资料正文
}

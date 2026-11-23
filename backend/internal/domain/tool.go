// 本文件定义 AI 工具（Tool）资产的持久化对象、请求与视图对象。
// 工具是对话期可被显式调用或按关键词自动触发的能力单元：内置工具由服务端函数实现，
// 结果作为一条上下文块注入，模型据此作答，避免让模型自己"心算"事实。

package domain

// 工具类型编码常量。
const (
	ToolTypeBuiltin = "builtin" // 内置工具（服务端函数实现）
)

// ToolDO 工具持久化对象。
type ToolDO struct {
	BaseDO
	ToolULID        string `json:"tool_ulid" gorm:"column:tool_ulid;primaryKey;size:40"`        // 工具 ULID（TOL_ 前缀）
	ToolCode        string `json:"tool_code" gorm:"column:tool_code;size:32;uniqueIndex"`       // 工具编码（对话中以 #code 触发）
	ToolName        string `json:"tool_name" gorm:"column:tool_name;size:64"`                   // 工具名称
	ToolTypeCode    string `json:"tool_type_code" gorm:"column:tool_type_code;size:16;index"`   // 类型：builtin
	DescriptionText string `json:"description_text" gorm:"column:description_text;size:256"`    // 一句话说明（模型可见）
	TriggerKeywords string `json:"trigger_keywords" gorm:"column:trigger_keywords;size:256"`    // 自动触发关键词（逗号分隔）
	ParamSchemaJSON string `json:"param_schema_json" gorm:"column:param_schema_json;type:text"` // 参数声明 JSON 数组
	EnabledFlag     bool   `json:"enabled_flag" gorm:"column:enabled_flag;default:true"`        // 是否启用
	BuiltinFlag     bool   `json:"builtin_flag" gorm:"column:builtin_flag;default:false"`       // 是否内置（内置不可删）
	AssetHashText   string `json:"asset_hash_text" gorm:"column:asset_hash_text;size:64"`       // 出厂指纹（代码升级时判断是否被改过）
	CustomizedFlag  bool   `json:"customized_flag" gorm:"column:customized_flag;default:false"` // 是否被改过（改过则代码升级不覆盖）
	SortOrder       int    `json:"sort_order" gorm:"column:sort_order"`                         // 排序号
}

// TableName 指定表名。
func (ToolDO) TableName() string { return "tools" }

// ToolParamDO 工具参数声明（随工具持久化，前端表单与调用校验共用）。
type ToolParamDO struct {
	ParamName       string `json:"param_name"`       // 参数名
	ParamLabel      string `json:"param_label"`      // 展示名
	RequiredFlag    bool   `json:"required_flag"`    // 是否必填
	DefaultValue    string `json:"default_value"`    // 缺省值
	DescriptionText string `json:"description_text"` // 填写提示
}

// ToolREQ 创建/更新工具请求（tool_ulid 为空表示创建）。
type ToolREQ struct {
	ToolULID        string        `json:"tool_ulid" binding:"omitempty,max=40"`          // 工具 ULID
	ToolCode        string        `json:"tool_code" binding:"required,min=2,max=32"`     // 工具编码
	ToolName        string        `json:"tool_name" binding:"required,max=64"`           // 工具名称
	DescriptionText string        `json:"description_text" binding:"omitempty,max=256"`  // 说明
	TriggerKeywords string        `json:"trigger_keywords" binding:"omitempty,max=256"`  // 触发关键词
	ParamList       []ToolParamDO `json:"param_list"`                                    // 参数声明
	EnabledFlag     *bool         `json:"enabled_flag"`                                  // 是否启用（nil 保持）
	SortOrder       int           `json:"sort_order" binding:"omitempty,min=0,max=9999"` // 排序号
}

// ToolDeleteREQ 删除工具请求（内置不可删）。
type ToolDeleteREQ struct {
	ToolULID string `json:"tool_ulid" binding:"required,max=40"` // 工具 ULID
}

// ToolListREQ 工具列表查询请求。
type ToolListREQ struct {
	KeywordText string `json:"keyword_text" form:"keyword_text" binding:"omitempty,max=64"` // 名称/编码模糊
	EnabledOnly bool   `json:"enabled_only" form:"enabled_only"`                            // 仅启用
}

// ToolVO 工具视图对象。
type ToolVO struct {
	ToolULID        string        `json:"tool_ulid"`        // 工具 ULID
	ToolCode        string        `json:"tool_code"`        // 工具编码
	ToolName        string        `json:"tool_name"`        // 工具名称
	ToolTypeCode    string        `json:"tool_type_code"`   // 类型
	DescriptionText string        `json:"description_text"` // 说明
	TriggerKeywords string        `json:"trigger_keywords"` // 触发关键词
	ParamList       []ToolParamDO `json:"param_list"`       // 参数声明
	EnabledFlag     bool          `json:"enabled_flag"`     // 是否启用
	BuiltinFlag     bool          `json:"builtin_flag"`     // 是否内置
	CustomizedFlag  bool          `json:"customized_flag"`  // 是否被改过
	SortOrder       int           `json:"sort_order"`       // 排序号
	CreatedAt       string        `json:"created_at"`       // 创建时间
	UpdatedAt       string        `json:"updated_at"`       // 更新时间
}

// ToolInvokeREQ 工具试运行请求（对话期由服务端自动组装，管理端可手动验证）。
type ToolInvokeREQ struct {
	ProjectULID string            `json:"project_ulid" binding:"omitempty,max=40"`     // 项目 ULID（读取章节等上下文）
	ToolCode    string            `json:"tool_code" binding:"required,max=32"`         // 工具编码
	ChapterULID string            `json:"chapter_ulid" binding:"omitempty,max=40"`     // 章节 ULID（缺省用传入正文）
	ContentText string            `json:"content_text" binding:"omitempty,max=200000"` // 待检正文（优先于章节）
	ParamValue  map[string]string `json:"param_value"`                                 // 参数取值
}

// ToolInvokeDTO 工具运行结果（结果文本注入上下文，指标供前端展示）。
type ToolInvokeDTO struct {
	ToolCode     string            `json:"tool_code"`     // 工具编码
	ToolName     string            `json:"tool_name"`     // 工具名称
	ResultText   string            `json:"result_text"`   // 结果正文（注入模型上下文）
	MetricMap    map[string]string `json:"metric_map"`    // 关键指标（前端展示）
	LatencyMs    int64             `json:"latency_ms"`    // 执行耗时
	ErrorMessage string            `json:"error_message"` // 失败原因（成功为空）
}

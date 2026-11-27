// 本文件定义异步任务、应用配置与每日字数统计的持久化对象和视图对象。

package domain

// JobDO 异步任务持久化对象（RabbitMQ 消费的落库凭据）。
type JobDO struct {
	BaseDO
	JobULID       string `json:"job_ulid" gorm:"column:job_ulid;primaryKey;size:40"` // 任务 ULID（JOB_ 前缀）
	JobTypeCode   string `json:"job_type_code" gorm:"size:32;index"`                 // 类型：doc_index/doc_reindex
	PayloadJSON   string `json:"payload_json" gorm:"type:text"`                      // 任务载荷（KnowledgeIndexJobDTO 序列化）
	JobStatusCode string `json:"job_status_code" gorm:"size:16;index"`               // 状态：queued/running/success/failed
	RetryCount    int    `json:"retry_count"`                                        // 已重试次数
	ErrorMessage  string `json:"error_message" gorm:"size:512"`                      // 失败原因
}

// TableName 指定表名。
func (JobDO) TableName() string { return "jobs" }

// 异步任务类型与状态编码常量。
const (
	JobTypeDocIndex   = "doc_index"   // 知识库文档首次索引
	JobTypeDocReindex = "doc_reindex" // 知识库文档重建索引

	JobStatusQueued  = "queued"  // 已入队
	JobStatusRunning = "running" // 执行中
	JobStatusSuccess = "success" // 成功
	JobStatusFailed  = "failed"  // 失败
)

// JobListREQ 异步任务分页查询请求（管理端）。
type JobListREQ struct {
	PageREQ
	JobTypeCode   string `json:"job_type_code" binding:"omitempty,max=32"`   // 类型过滤
	JobStatusCode string `json:"job_status_code" binding:"omitempty,max=16"` // 状态过滤
	KeywordText   string `json:"keyword_text" binding:"omitempty,max=64"`    // 任务 ULID/文档 ULID 模糊
}

// JobRetryREQ 重试异步任务请求（仅失败任务可重试）。
type JobRetryREQ struct {
	JobULID string `json:"job_ulid" binding:"required,max=40"` // 任务 ULID
}

// JobStatsVO 任务状态概览（任务中心顶部卡片）。
type JobStatsVO struct {
	QueuedCount  int64 `json:"queued_count"`  // 排队中
	RunningCount int64 `json:"running_count"` // 执行中
	SuccessCount int64 `json:"success_count"` // 成功
	FailedCount  int64 `json:"failed_count"`  // 失败
}

// JobVO 异步任务视图对象（管理端任务中心）。
type JobVO struct {
	JobULID       string `json:"job_ulid"`        // 任务 ULID
	JobTypeCode   string `json:"job_type_code"`   // 类型
	JobStatusCode string `json:"job_status_code"` // 状态
	DocULID       string `json:"doc_ulid"`        // 关联文档 ULID（解析自载荷）
	DocName       string `json:"doc_name"`        // 关联文档名
	RetryCount    int    `json:"retry_count"`     // 已重试次数
	ErrorMessage  string `json:"error_message"`   // 失败原因
	CreatedAt     string `json:"created_at"`      // 创建时间（RFC3339）
	UpdatedAt     string `json:"updated_at"`      // 更新时间（RFC3339）
}

// AppSettingDO 应用配置持久化对象（key-value，运行时可改）。
type AppSettingDO struct {
	BaseDO
	SettingULID  string `json:"setting_ulid" gorm:"column:setting_ulid;primaryKey;size:40"` // 配置 ULID（SET_ 前缀）
	SettingKey   string `json:"setting_key" gorm:"size:64;uniqueIndex"`                     // 配置键
	SettingValue string `json:"setting_value" gorm:"size:512"`                              // 配置值
	SettingDesc  string `json:"setting_desc" gorm:"size:256"`                               // 说明
}

// TableName 指定表名。
func (AppSettingDO) TableName() string { return "app_settings" }

// WordCountStatDO 每日字数统计持久化对象（ECharts 趋势图数据源）。
type WordCountStatDO struct {
	BaseDO
	WordStatULID string `json:"word_stat_ulid" gorm:"column:word_stat_ulid;primaryKey;size:40"` // 统计 ULID（WCS_ 前缀）
	ProjectULID  string `json:"project_ulid" gorm:"column:project_ulid;size:40;index"`          // 所属项目
	StatDate     string `json:"stat_date" gorm:"size:10;index"`                                 // 统计日期（YYYY-MM-DD）
	WordDelta    int64  `json:"word_delta"`                                                     // 当日人工净增字数
	AIWordDelta  int64  `json:"ai_word_delta"`                                                  // 当日 AI 生成字数
}

// TableName 指定表名。
func (WordCountStatDO) TableName() string { return "word_count_stats" }

// SettingUpdateREQ 更新应用配置请求（setting_key 须在白名单内，setting_value 为正整数）。
type SettingUpdateREQ struct {
	SettingKey   string `json:"setting_key" form:"setting_key" binding:"required,max=64"`      // 配置键
	SettingValue string `json:"setting_value" form:"setting_value" binding:"required,max=512"` // 配置值
}

// StatsOverviewREQ 数据总览查询（GET query，project_ulid 空为全局）。
type StatsOverviewREQ struct {
	ProjectULID string `json:"project_ulid" form:"project_ulid" binding:"omitempty"` // 项目 ULID，空为全局统计
}

// StatsOverviewVO 数据总览（工作台首页）。
type StatsOverviewVO struct {
	ProjectCount   int64        `json:"project_count"`    // 项目总数
	TotalWordCount int64        `json:"total_word_count"` // 累计字数
	TodayWordDelta int64        `json:"today_word_delta"` // 今日新增字数
	AIWordTotal    int64        `json:"ai_word_total"`    // AI 生成累计字数
	TrendList      []WordStatVO `json:"trend_list"`       // 近 30 天趋势
}

// WordStatVO 单日字数视图。
type WordStatVO struct {
	StatDate    string `json:"stat_date"`     // 日期（YYYY-MM-DD）
	WordDelta   int64  `json:"word_delta"`    // 人工净增
	AIWordDelta int64  `json:"ai_word_delta"` // AI 生成
}

// 配置值类型编码：前端据此选择数字输入、开关或文本框。
const (
	SettingValueTypeInt  = "int"  // 整数（带区间）
	SettingValueTypeBool = "bool" // 布尔（开关）
	SettingValueTypeText = "text" // 纯文本
)

// SettingVO 配置视图对象。
// 除键值与说明外，额外下发默认值、值类型与合法区间，供设置页按类型渲染控件并就地校验，
// 避免前端为每个配置键硬编码一份规则（键增删时前端无需改代码）。
type SettingVO struct {
	SettingKey         string `json:"setting_key"`          // 配置键
	SettingValue       string `json:"setting_value"`        // 当前值
	SettingDesc        string `json:"setting_desc"`         // 说明
	SettingValueType   string `json:"setting_value_type"`   // 值类型：int/bool/text（前端据此选控件）
	SettingDefaultText string `json:"setting_default_text"` // 默认值（白名单外的历史键为空）
	SettingValueMin    int    `json:"setting_value_min"`    // 合法最小值（仅 int 有效，0 表示无下限声明）
	SettingValueMax    int    `json:"setting_value_max"`    // 合法最大值（仅 int 有效，0 表示无上限声明）
}

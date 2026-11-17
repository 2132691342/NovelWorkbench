package domain

// JobDO 异步任务持久化对象（RabbitMQ 消费的落库凭据）。
type JobDO struct {
	BaseDO
	JobULID       string `json:"job_ulid" gorm:"column:job_ulid;primaryKey;size:40"` // 任务 ULID（JOB_ 前缀）
	JobTypeCode   string `json:"job_type_code" gorm:"size:32;index"` // 类型：doc_index/doc_reindex
	PayloadJSON   string `json:"payload_json" gorm:"type:text"`      // 任务载荷（KnowledgeIndexJobDTO 序列化）
	JobStatusCode string `json:"job_status_code" gorm:"size:16;index"` // 状态：queued/running/success/failed
	RetryCount    int    `json:"retry_count"`                        // 已重试次数
	ErrorMessage  string `json:"error_message" gorm:"size:512"`      // 失败原因
}

// TableName 指定表名。
func (JobDO) TableName() string { return "jobs" }

// AppSettingDO 应用配置持久化对象（key-value，运行时可改）。
type AppSettingDO struct {
	BaseDO
	SettingULID  string `json:"setting_ulid" gorm:"column:setting_ulid;primaryKey;size:40"` // 配置 ULID（SET_ 前缀）
	SettingKey   string `json:"setting_key" gorm:"size:64;uniqueIndex"` // 配置键
	SettingValue string `json:"setting_value" gorm:"size:512"`         // 配置值
	SettingDesc  string `json:"setting_desc" gorm:"size:256"`          // 说明
}

// TableName 指定表名。
func (AppSettingDO) TableName() string { return "app_settings" }

// WordCountStatDO 每日字数统计持久化对象（ECharts 趋势图数据源）。
type WordCountStatDO struct {
	BaseDO
	WordStatULID  string `json:"word_stat_ulid" gorm:"column:word_stat_ulid;primaryKey;size:40"` // 统计 ULID（WCS_ 前缀）
	ProjectULID   string `json:"project_ulid" gorm:"column:project_ulid;size:40;index"`       // 所属项目
	StatDate      string `json:"stat_date" gorm:"size:10;index"`          // 统计日期（YYYY-MM-DD）
	WordDelta     int64  `json:"word_delta"`                               // 当日人工净增字数
	AIWordDelta   int64  `json:"ai_word_delta"`                            // 当日 AI 生成字数
}

// TableName 指定表名。
func (WordCountStatDO) TableName() string { return "word_count_stats" }

// SettingUpdateREQ 更新应用配置请求（setting_key 须在白名单内，setting_value 为正整数）。
type SettingUpdateREQ struct {
	SettingKey   string `json:"setting_key" form:"setting_key" binding:"required,max=64"`     // 配置键
	SettingValue string `json:"setting_value" form:"setting_value" binding:"required,max=512"` // 配置值
}

// StatsOverviewREQ 数据总览查询（GET query，project_ulid 空为全局）。
type StatsOverviewREQ struct {
	ProjectULID string `json:"project_ulid" form:"project_ulid" binding:"omitempty"` // 项目 ULID，空为全局统计
}

// StatsOverviewVO 数据总览（工作台首页）。
type StatsOverviewVO struct {
	ProjectCount   int64            `json:"project_count"`   // 项目总数
	TotalWordCount int64            `json:"total_word_count"` // 累计字数
	TodayWordDelta int64            `json:"today_word_delta"` // 今日新增字数
	AIWordTotal    int64            `json:"ai_word_total"`    // AI 生成累计字数
	TrendList      []WordStatVO     `json:"trend_list"`       // 近 30 天趋势
}

// WordStatVO 单日字数视图。
type WordStatVO struct {
	StatDate     string `json:"stat_date"`     // 日期（YYYY-MM-DD）
	WordDelta    int64  `json:"word_delta"`    // 人工净增
	AIWordDelta  int64  `json:"ai_word_delta"` // AI 生成
}

// SettingVO 配置视图对象。
type SettingVO struct {
	SettingKey   string `json:"setting_key"`   // 配置键
	SettingValue string `json:"setting_value"` // 配置值
	SettingDesc  string `json:"setting_desc"`  // 说明
}

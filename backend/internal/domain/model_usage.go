package domain

// ModelCallTypeChat 对话生成调用类型（用量统计维度）。
const ModelCallTypeChat = "chat"

// ModelUsageStatDO 模型用量按日聚合持久化对象（供应商 × 模型 × 调用类型 × 日期 唯一）。
type ModelUsageStatDO struct {
	BaseDO
	UsageStatULID    string `json:"usage_stat_ulid" gorm:"column:usage_stat_ulid;primaryKey;size:40"`           // 统计 ULID（MUS_ 前缀）
	StatDate         string `json:"stat_date" gorm:"column:stat_date;size:10;uniqueIndex:uk_mus_dim"`           // 统计日期（YYYY-MM-DD）
	ProviderULID     string `json:"provider_ulid" gorm:"column:provider_ulid;size:40;uniqueIndex:uk_mus_dim"`   // 供应商 ULID
	ProviderName     string `json:"provider_name" gorm:"column:provider_name;size:64"`                          // 供应商名称快照
	ModelName        string `json:"model_name" gorm:"column:model_name;size:128;uniqueIndex:uk_mus_dim"`        // 模型名
	CallTypeCode     string `json:"call_type_code" gorm:"column:call_type_code;size:16;uniqueIndex:uk_mus_dim"` // 调用类型：chat
	CallCount        int64  `json:"call_count" gorm:"column:call_count"`                                        // 调用次数
	SuccessCount     int64  `json:"success_count" gorm:"column:success_count"`                                  // 成功次数
	FailCount        int64  `json:"fail_count" gorm:"column:fail_count"`                                        // 失败次数
	PromptTokens     int64  `json:"prompt_tokens" gorm:"column:prompt_tokens"`                                  // 输入 token 合计
	CompletionTokens int64  `json:"completion_tokens" gorm:"column:completion_tokens"`                          // 输出 token 合计
	TotalTokens      int64  `json:"total_tokens" gorm:"column:total_tokens"`                                    // 总 token 合计
	LatencyMsSum     int64  `json:"latency_ms_sum" gorm:"column:latency_ms_sum"`                                // 耗时合计（毫秒，算均值用）
}

// TableName 指定表名。
func (ModelUsageStatDO) TableName() string { return "model_usage_stats" }

// ModelUsageDailyVO 用量趋势点（按日聚合）。
type ModelUsageDailyVO struct {
	StatDate         string `json:"stat_date"`         // 日期
	CallCount        int64  `json:"call_count"`        // 调用次数
	FailCount        int64  `json:"fail_count"`        // 失败次数
	TotalTokens      int64  `json:"total_tokens"`      // token 合计
	CompletionTokens int64  `json:"completion_tokens"` // 输出 token
	AvgLatencyMs     int64  `json:"avg_latency_ms"`    // 平均耗时（毫秒）
}

// ModelUsageGroupVO 用量分布项（按供应商或模型聚合）。
type ModelUsageGroupVO struct {
	GroupName        string  `json:"group_name"`         // 分组名（供应商名或模型名）
	CallCount        int64   `json:"call_count"`         // 调用次数
	FailCount        int64   `json:"fail_count"`         // 失败次数
	TotalTokens      int64   `json:"total_tokens"`       // token 合计
	AvgLatencyMs     int64   `json:"avg_latency_ms"`     // 平均耗时（毫秒）
	SuccessRateValue float64 `json:"success_rate_value"` // 成功率（0-1）
}

// ModelUsageOverviewVO 用量总览（管理端「用量分析」页）。
type ModelUsageOverviewVO struct {
	CallCountTotal   int64               `json:"call_count_total"`   // 区间调用总次数
	FailCountTotal   int64               `json:"fail_count_total"`   // 区间失败总次数
	TotalTokensTotal int64               `json:"total_tokens_total"` // 区间 token 总量
	AvgLatencyMs     int64               `json:"avg_latency_ms"`     // 区间平均耗时（毫秒）
	SuccessRateValue float64             `json:"success_rate_value"` // 区间成功率（0-1）
	DailyList        []ModelUsageDailyVO `json:"daily_list"`         // 按日趋势
	ProviderList     []ModelUsageGroupVO `json:"provider_list"`      // 按供应商分布
	ModelList        []ModelUsageGroupVO `json:"model_list"`         // 按模型分布
}

// ModelUsageQueryREQ 用量查询请求（GET query，区间默认近 14 天）。
type ModelUsageQueryREQ struct {
	FromDate string `json:"from_date" form:"from_date" binding:"omitempty"` // 起始日期（YYYY-MM-DD）
	ToDate   string `json:"to_date" form:"to_date" binding:"omitempty"`     // 结束日期（YYYY-MM-DD）
}

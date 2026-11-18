package domain

// 依赖探测状态常量（系统运行状态页）。
const (
	HealthStatusUp       = "up"       // 可用
	HealthStatusDown     = "down"     // 不可用
	HealthStatusDegraded = "degraded" // 可用但降级（未配置/部分能力缺失）
)

// DependencyHealthVO 单个外部依赖的健康快照。
type DependencyHealthVO struct {
	DependencyCode string `json:"dependency_code"` // 依赖编码：postgres/redis/rabbitmq/minio/manticore/model
	DependencyName string `json:"dependency_name"` // 依赖名称（展示用）
	StatusCode     string `json:"status_code"`     // up/down/degraded
	MessageText    string `json:"message_text"`    // 结论说明
	LatencyMs      int64  `json:"latency_ms"`      // 探测耗时（毫秒）
}

// SystemStatusVO 系统运行状态聚合（管理端「运行状态」页）。
type SystemStatusVO struct {
	ServerStartedAtMs int64                `json:"server_started_at_ms"` // 进程启动时间（毫秒）
	ServerUptimeSec   int64                `json:"server_uptime_sec"`    // 已运行秒数
	GoVersionText     string               `json:"go_version_text"`      // 运行时版本
	BuildVersionText  string               `json:"build_version_text"`   // 构建版本
	DependencyList    []DependencyHealthVO `json:"dependency_list"`      // 依赖健康列表
	ChatReady         bool                 `json:"chat_ready"`           // 对话链路是否可用
	EmbeddingReady    bool                 `json:"embedding_ready"`      // 向量链路是否可用
	HintText          string               `json:"hint_text"`            // 面向运维的提示
}

package ai

import "context"

// 模型调用类型常量（与 domain.ModelCallType* 对齐）。
const (
	CallTypeChat      = "chat"      // 对话生成
	CallTypeEmbedding = "embedding" // 向量化
)

// 向量通道探测结论常量。
const (
	EmbeddingStatusReady         = "ready"          // 可用
	EmbeddingStatusNotConfigured = "not_configured" // 未配置
	EmbeddingStatusFailed        = "failed"         // 不可用
)

// UsageRecord 一次模型调用的用量记录（由 service 落库聚合）。
type UsageRecord struct {
	ProviderULID     string // 供应商 ULID
	ProviderName     string // 供应商名称
	ModelName        string // 模型名
	CallTypeCode     string // chat/embedding
	SuccessFlag      bool   // 是否成功
	PromptTokens     int    // 输入 token
	CompletionTokens int    // 输出 token
	LatencyMs        int64  // 调用耗时（毫秒）
}

// UsageRecorder 用量记录接口（service.ModelUsageService 实现，避免 ai 依赖存储层）。
type UsageRecorder interface {
	// RecordUsage 异步安全地累加一条用量记录。
	RecordUsage(ctx context.Context, record UsageRecord)
}

// 本文件定义模型调用的用量上报契约：调用记录结构与上报接口。

package ai

import "context"

// CallTypeChat 对话生成调用类型（与 domain.ModelCallTypeChat 对齐）。
const CallTypeChat = "chat"

// UsageRecord 一次模型调用的用量记录（由 service 落库聚合）。
type UsageRecord struct {
	ProviderULID     string // 供应商 ULID
	ProviderName     string // 供应商名称
	ModelName        string // 模型名
	CallTypeCode     string // 调用类型：chat
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

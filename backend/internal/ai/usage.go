// 本文件定义模型调用的用量上报契约：调用记录结构、上报接口与流式用量汇总。

package ai

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

// CallTypeChat 对话生成调用类型（与 domain.ModelCallTypeChat 对齐）。
const CallTypeChat = "chat"

// UsageRecord 一次模型调用的用量记录（由 service 落库聚合）。
// 一次模型调用只产生一条记录：调用次数与 token 必须落在同一条上，
// 否则会出现「有调用次数、token 全为 0」的伪统计。
type UsageRecord struct {
	ProviderULID     string // 供应商 ULID
	ProviderName     string // 供应商名称
	ModelName        string // 模型名
	CallTypeCode     string // 调用类型：chat
	SuccessFlag      bool   // 是否成功
	UsageKnownFlag   bool   // 是否取得真实 token 用量（false 时 token 记 0，另计未知用量）
	PromptTokens     int    // 输入 token
	CompletionTokens int    // 输出 token
	LatencyMs        int64  // 调用耗时（毫秒）
}

// UsageRecorder 用量记录接口（service.ModelUsageService 实现，避免 ai 依赖存储层）。
type UsageRecorder interface {
	// RecordUsage 异步安全地累加一条用量记录。
	RecordUsage(ctx context.Context, record UsageRecord)
}

// StreamUsage 汇总流式分片里的真实 token 用量（网关通常在末包返回，早到的分片也可能带输入量）。
type StreamUsage struct {
	promptTokens     int  // 输入 token（取各分片最大值，末包为最终值）
	completionTokens int  // 输出 token（取各分片最大值，末包为最终值）
	found            bool // 是否收到过用量（false 表示网关全程未返回）
}

// Add 吸收一个流式分片的用量信息（无用量分片自动忽略）。
func (u *StreamUsage) Add(chunk *schema.Message) {
	if chunk == nil || chunk.ResponseMeta == nil || chunk.ResponseMeta.Usage == nil {
		return
	}
	usage := chunk.ResponseMeta.Usage
	u.found = true
	if usage.PromptTokens > u.promptTokens {
		u.promptTokens = usage.PromptTokens
	}
	if usage.CompletionTokens > u.completionTokens {
		u.completionTokens = usage.CompletionTokens
	}
}

// TokenUsage 返回汇总后的用量；网关全程未返回用量时返回 nil（调用方按用量未知上报）。
func (u *StreamUsage) TokenUsage() *schema.TokenUsage {
	if !u.found {
		return nil
	}
	return &schema.TokenUsage{
		PromptTokens:     u.promptTokens,
		CompletionTokens: u.completionTokens,
		TotalTokens:      u.promptTokens + u.completionTokens,
	}
}

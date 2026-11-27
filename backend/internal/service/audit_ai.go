// 本文件实现 AI 调用的审计留痕：一次模型调用一条 ai_call span，记录模型名、token 与耗时。
// 只记提示词字符数，不落 prompt 全文（避免把作品正文与密钥写进审计）。

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
)

// aiCallMeta AI 调用审计的静态上下文（耗时、token 与成败在调用结束后回填）。
type aiCallMeta struct {
	ActionCode     string // 动作码：ai/chat、ai/workflow/draft 等
	ActionName     string // 中文动作名（如「AI 出稿」）
	ParentSpanCode string // 父 span 动作码（挂在哪一步之下）
	TraceID        string // 链路 ID（空则取 ctx 中的请求链路）
	UserULID       string // 触发用户
	TargetType     string // 目标类型（workflow_run 等）
	TargetULID     string // 目标 ULID
	ModelName      string // 模型名
	PromptChars    int    // 提示词字符数（不落原文）
}

// recordAICall 落一条 ai_call 审计：失败只告警，绝不影响主流程。
func recordAICall(ctx context.Context, recorder pkg.AuditRecorder, meta aiCallMeta,
	latency time.Duration, tokenCount int, callErr error) {
	if recorder == nil {
		return
	}
	levelCode, httpStatus := domain.AuditLevelInfo, 200
	respCode, respMessage := domain.RespCodeSuccess, "模型调用成功"
	if callErr != nil {
		levelCode, httpStatus = domain.AuditLevelError, 500
		respCode, respMessage = domain.RespCodeAIProviderError, pkg.TruncateText(callErr.Error(), 512)
	}
	traceID := meta.TraceID
	if traceID == "" {
		traceID = pkg.RequestIDFromContext(ctx)
	}
	userULID := meta.UserULID
	if userULID == "" {
		userULID = pkg.ActorFromContext(ctx)
	}
	do := &domain.AuditLogDO{
		LogLevelCode:   levelCode,
		SpanTypeCode:   domain.AuditSpanTypeAICall,
		TraceID:        traceID,
		ParentSpanCode: meta.ParentSpanCode,
		UserULID:       userULID,
		ActionCode:     meta.ActionCode,
		ActionName:     meta.ActionName,
		ModelName:      meta.ModelName,
		TokenCount:     tokenCount,
		RequestPath:    "ai://" + meta.ActionCode,
		RequestMethod:  "INTERNAL",
		RequestBody:    aiCallRequestBody(meta),
		RespBody:       aiCallRespBody(tokenCount, latency, callErr == nil),
		TargetType:     meta.TargetType,
		TargetULID:     meta.TargetULID,
		RespCode:       respCode,
		RespMessage:    respMessage,
		HTTPStatus:     httpStatus,
		LatencyMs:      latency.Milliseconds(),
	}
	if err := recorder.Record(ctx, do); err != nil {
		pkg.SLogger().Warnw("AI 调用审计写入失败", "action_code", meta.ActionCode, "error", err)
	}
}

// aiCallRequestBody 组装请求摘要（模型名与提示词字符数，不含正文）。
func aiCallRequestBody(meta aiCallMeta) string {
	raw, err := json.Marshal(map[string]any{
		"model_name":   meta.ModelName,
		"prompt_chars": meta.PromptChars,
	})
	if err != nil {
		return "{}"
	}
	return string(raw)
}

// aiCallRespBody 组装响应摘要（token、耗时与成败，不含模型输出原文）。
func aiCallRespBody(tokenCount int, latency time.Duration, success bool) string {
	return fmt.Sprintf(`{"token_count":%d,"latency_ms":%d,"success":%t}`,
		tokenCount, latency.Milliseconds(), success)
}

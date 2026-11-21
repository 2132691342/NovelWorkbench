// 本文件实现对话流式生成的消费与转发：thinking/chunk 事件、心跳、空闲看门狗与用量补记。

package service

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cloudwego/eino/schema"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
)

// streamChat 消费模型流并转发 thinking/chunk 事件（15s 无输出发心跳，客户端断开即中止）。
// completed=false 表示客户端断开或生成失败（生成失败已发 error 事件）。
func (s *ChatService) streamChat(ctx context.Context, events *chatEventWriter,
	messages []*schema.Message, providerSpec ai.ProviderSpec, promptRunes int) (string, string, bool) {
	startedAt := time.Now()
	stream, err := s.aiClient.Stream(ctx, providerSpec, messages)
	if err != nil {
		pkg.SLogger().Errorw("模型流式生成启动失败", "error", err)
		_ = events.emit(domain.StreamEventError, "", "AI 生成启动失败："+err.Error(), nil)
		return "", "", false
	}
	defer stream.Close()
	type streamItem struct {
		chunk *schema.Message
		err   error
	}
	items := make(chan streamItem, 1)
	go func() {
		defer close(items)
		for {
			chunk, err := stream.Recv()
			select {
			case items <- streamItem{chunk: chunk, err: err}:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	var reply strings.Builder
	var reasoning strings.Builder
	var lastChunk *schema.Message
	lastActivityAt := startedAt
	pingTimer := time.NewTimer(chatPingInterval)
	defer pingTimer.Stop()
	finish := func(completed bool) (string, string, bool) {
		// 流结束补记 token 用量（首包已按成功调用计次）
		usage := &schema.TokenUsage{
			PromptTokens:     promptRunes,
			CompletionTokens: utf8.RuneCountInString(reply.String()),
		}
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
		if lastChunk != nil && lastChunk.ResponseMeta != nil && lastChunk.ResponseMeta.Usage != nil {
			usage = lastChunk.ResponseMeta.Usage
		}
		s.aiClient.RecordStreamUsage(providerSpec, usage, time.Since(startedAt))
		if !completed {
			return "", "", false
		}
		return reply.String(), reasoning.String(), true
	}
	for {
		select {
		case <-ctx.Done():
			return finish(false) // 客户端断开：中止生成并停止落库
		case item, ok := <-items:
			if !ok {
				return finish(false)
			}
			if item.err != nil {
				if errors.Is(item.err, io.EOF) {
					return finish(true)
				}
				pkg.SLogger().Errorw("模型流式生成失败", "error", item.err)
				_ = events.emit(domain.StreamEventError, "", "AI 生成失败，请稍后重试", nil)
				return finish(false)
			}
			if item.chunk == nil {
				continue
			}
			lastChunk = item.chunk
			lastActivityAt = time.Now()
			if item.chunk.ReasoningContent != "" {
				reasoning.WriteString(item.chunk.ReasoningContent)
				if err := events.emit(domain.StreamEventThinking, item.chunk.ReasoningContent, "", nil); err != nil {
					pkg.SLogger().Warnw("SSE 思考事件写入失败，中止生成", "error", err)
					return finish(false)
				}
				resetTimer(pingTimer)
			}
			if item.chunk.Content == "" {
				continue
			}
			reply.WriteString(item.chunk.Content)
			if err := events.emit(domain.StreamEventChunk, item.chunk.Content, "", nil); err != nil {
				pkg.SLogger().Warnw("SSE 事件写入失败，中止生成", "error", err)
				return finish(false)
			}
			resetTimer(pingTimer)
		case <-pingTimer.C:
			if err := events.writer.Ping(); err != nil {
				return finish(false)
			}
			if time.Since(lastActivityAt) > chatStreamIdleTimeout {
				pkg.SLogger().Errorw("模型流式生成长时间无分片，判定网关已断",
					"session_ulid", events.sessionULID, "idle_seconds", int(time.Since(lastActivityAt).Seconds()))
				_ = events.emit(domain.StreamEventError, "", "模型长时间无响应，请重试或换用响应更快的模型", nil)
				return finish(false)
			}
			pingTimer.Reset(chatPingInterval)
		}
	}
}

// resetTimer 重置定时器（先停再排空再重置，避免已触发信号串扰）。
func resetTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(chatPingInterval)
}

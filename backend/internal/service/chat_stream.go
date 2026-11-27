// 本文件实现对话流式生成的消费与转发：thinking/chunk 事件、工具调用往返、心跳与空闲看门狗。
// 工具由模型自行决定是否调用：服务端只下发声明，模型回 tool_calls 时才执行，
// 执行结果以 tool 消息回填后继续生成，直到模型给出正文或达到往返上限。

package service

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
)

// streamChat 消费模型流并转发 thinking/chunk 事件（15s 无输出发心跳，客户端断开即中止）。
// completed=false 表示客户端断开或生成失败（生成失败已发 error 事件）。
func (s *ChatService) streamChat(ctx context.Context, events *chatEventWriter,
	messages []*schema.Message, providerSpec ai.ProviderSpec) (string, string, bool) {
	var reply strings.Builder
	var reasoning strings.Builder
	toolCodeList := make([]string, 0, 2)
	lastRoundKey := ""
	// 工具往返：模型调用工具 → 执行 → 回填结果 → 继续生成。上限防止模型反复调用同一工具打转。
	for round := 0; round < chatToolRoundLimit; round++ {
		outcome := s.streamOnce(ctx, events, messages, providerSpec)
		if !outcome.ok {
			return "", "", false
		}
		reply.WriteString(outcome.contentText)
		reasoning.WriteString(outcome.reasoningText)
		if len(outcome.toolCallList) == 0 {
			events.toolCodeList = toolCodeList
			return reply.String(), reasoning.String(), true
		}
		// 同一批工具调用再次出现说明模型在原地打转：去掉工具后逼它用已有事实作答
		roundKey := toolCallSignature(outcome.toolCallList)
		if roundKey == lastRoundKey {
			events.toolInfoList = nil
		}
		lastRoundKey = roundKey
		assistantTurn, resultList, codeList, err := s.executeToolCalls(ctx, events, outcome, providerSpec)
		if err != nil {
			return "", "", false
		}
		toolCodeList = append(toolCodeList, codeList...)
		messages = append(messages, assistantTurn)
		for i := range resultList {
			messages = append(messages, resultList[i])
		}
	}
	// 往返超限：仍把已有正文交给前端，并提示停止继续调用工具
	events.toolCodeList = toolCodeList
	if reply.Len() == 0 {
		_ = events.emit(domain.StreamEventError, "", "工具调用次数过多，已中止本轮生成", nil)
		return "", "", false
	}
	return reply.String(), reasoning.String(), true
}

// streamOutcome 单轮流式结果：正文、思考、待执行的工具调用与是否正常结束。
type streamOutcome struct {
	contentText   string            // 本轮正文（可能与工具调用并存）
	reasoningText string            // 本轮思考过程
	toolCallList  []schema.ToolCall // 模型请求调用的工具
	ok            bool              // 是否正常结束（断开或失败为 false）
}

// streamOnce 跑一轮流式生成，把增量事件转发给前端并累积完整消息。
// 用量按「一次模型调用一条记录」在流结束时上报：成功带真实 token，失败带已收集到的 token。
func (s *ChatService) streamOnce(ctx context.Context, events *chatEventWriter,
	messages []*schema.Message, providerSpec ai.ProviderSpec) (outcome streamOutcome) {
	startedAt := time.Now()
	firstChunkAt := time.Time{}
	var usage ai.StreamUsage
	defer func() {
		latency := time.Since(startedAt)
		if !firstChunkAt.IsZero() {
			latency = time.Since(firstChunkAt) // 真实生成耗时：首包 → 流结束
		}
		if outcome.ok {
			s.aiClient.RecordStreamUsage(providerSpec, usage.TokenUsage(), latency)
			return
		}
		s.aiClient.RecordStreamFailure(providerSpec, usage.TokenUsage(), latency)
	}()
	stream, err := s.aiClient.StreamWithTools(ctx, providerSpec, messages, events.toolInfoList)
	if err != nil {
		pkg.SLogger().Errorw("模型流式生成启动失败", "error", err)
		_ = events.emit(domain.StreamEventError, "", "AI 生成启动失败："+err.Error(), nil)
		return streamOutcome{}
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
	toolCallChunkList := make([]schema.ToolCall, 0, 2)
	lastActivityAt := startedAt
	pingTimer := time.NewTimer(chatPingInterval)
	defer pingTimer.Stop()
	for {
		select {
		case <-ctx.Done():
			// 客户端断开：中止生成；模型已被调用，用量仍按一次失败调用上报
			return streamOutcome{contentText: reply.String()}
		case item, ok := <-items:
			if !ok {
				return streamOutcome{contentText: reply.String(), reasoningText: reasoning.String()}
			}
			if item.err != nil {
				if errors.Is(item.err, io.EOF) {
					toolCalls, mergeErr := mergeToolCallChunks(toolCallChunkList)
					if mergeErr != nil {
						pkg.SLogger().Errorw("工具调用分片合并失败", "error", mergeErr)
						_ = events.emit(domain.StreamEventError, "", "工具调用解析失败，请重试", nil)
						return streamOutcome{contentText: reply.String()}
					}
					return streamOutcome{
						contentText: reply.String(), reasoningText: reasoning.String(),
						toolCallList: toolCalls, ok: true,
					}
				}
				pkg.SLogger().Errorw("模型流式生成失败", "error", item.err)
				_ = events.emit(domain.StreamEventError, "", "AI 生成失败，请稍后重试", nil)
				return streamOutcome{contentText: reply.String()}
			}
			if item.chunk == nil {
				continue
			}
			if firstChunkAt.IsZero() {
				firstChunkAt = time.Now()
			}
			usage.Add(item.chunk)
			lastActivityAt = time.Now()
			if item.chunk.ReasoningContent != "" {
				reasoning.WriteString(item.chunk.ReasoningContent)
				if err := events.emit(domain.StreamEventThinking, item.chunk.ReasoningContent, "", nil); err != nil {
					pkg.SLogger().Warnw("SSE 思考事件写入失败，中止生成", "error", err)
					return streamOutcome{contentText: reply.String()}
				}
				resetTimer(pingTimer)
			}
			if len(item.chunk.ToolCalls) > 0 {
				toolCallChunkList = append(toolCallChunkList, item.chunk.ToolCalls...)
				resetTimer(pingTimer)
			}
			if item.chunk.Content == "" {
				continue
			}
			reply.WriteString(item.chunk.Content)
			if err := events.emit(domain.StreamEventChunk, item.chunk.Content, "", nil); err != nil {
				pkg.SLogger().Warnw("SSE 事件写入失败，中止生成", "error", err)
				return streamOutcome{contentText: reply.String()}
			}
			resetTimer(pingTimer)
		case <-pingTimer.C:
			if err := events.writer.Ping(); err != nil {
				return streamOutcome{contentText: reply.String()}
			}
			if time.Since(lastActivityAt) > chatStreamIdleTimeout {
				pkg.SLogger().Errorw("模型流式生成长时间无分片，判定网关已断",
					"session_ulid", events.sessionULID, "idle_seconds", int(time.Since(lastActivityAt).Seconds()))
				_ = events.emit(domain.StreamEventError, "", "模型长时间无响应，请重试或换用响应更快的模型", nil)
				return streamOutcome{contentText: reply.String()}
			}
			pingTimer.Reset(chatPingInterval)
		}
	}
}

// toolCallSignature 生成一批工具调用的稳定签名（同一签名重复出现即判定模型在原地打转）。
func toolCallSignature(callList []schema.ToolCall) string {
	parts := make([]string, 0, len(callList))
	for i := range callList {
		parts = append(parts, callList[i].Function.Name+"\x1f"+callList[i].Function.Arguments)
	}
	return strings.Join(parts, "\x1e")
}

// mergeToolCallChunks 合并流式分片里的工具调用（按 index 归并名称与参数片段，保持调用顺序）。
func mergeToolCallChunks(chunkList []schema.ToolCall) ([]schema.ToolCall, error) {
	if len(chunkList) == 0 {
		return nil, nil
	}
	if len(chunkList) == 1 && chunkList[0].Index == nil {
		return chunkList, nil
	}
	orderList := make([]int, 0, 4)
	groupMap := make(map[int][]schema.ToolCall, 4)
	for i := range chunkList {
		chunk := chunkList[i]
		index := 0
		if chunk.Index != nil {
			index = *chunk.Index
		}
		if _, ok := groupMap[index]; !ok {
			orderList = append(orderList, index)
		}
		groupMap[index] = append(groupMap[index], chunk)
	}
	merged := make([]schema.ToolCall, 0, len(orderList))
	for _, index := range orderList {
		group := groupMap[index]
		call := schema.ToolCall{}
		if group[0].Index != nil {
			call.Index = group[0].Index
		}
		var args strings.Builder
		for i := range group {
			if group[i].ID != "" {
				call.ID = group[i].ID
			}
			if group[i].Type != "" {
				call.Type = group[i].Type
			}
			if group[i].Function.Name != "" {
				call.Function.Name = group[i].Function.Name
			}
			args.WriteString(group[i].Function.Arguments)
		}
		call.Function.Arguments = args.String()
		merged = append(merged, call)
	}
	return merged, nil
}

// executeToolCalls 执行模型请求的工具：返回回填用的 assistant 消息、tool 结果消息与执行的编码。
// 单个工具失败不中止对话：把失败原因写进 tool 消息，让模型自己决定如何说明。
// 同一轮里重复调用「相同工具 + 相同参数」只执行一次，其余复用首次结果，避免把往返轮数打满。
func (s *ChatService) executeToolCalls(ctx context.Context, events *chatEventWriter, outcome streamOutcome,
	providerSpec ai.ProviderSpec) (*schema.Message, []*schema.Message, []string, error) {
	assistantTurn := &schema.Message{
		Role:      schema.Assistant,
		Content:   outcome.contentText,
		ToolCalls: outcome.toolCallList,
	}
	resultList := make([]*schema.Message, 0, len(outcome.toolCallList))
	codeList := make([]string, 0, len(outcome.toolCallList))
	seenMap := make(map[string]string, len(outcome.toolCallList))
	for i := range outcome.toolCallList {
		call := outcome.toolCallList[i]
		toolCode := call.Function.Name
		codeList = append(codeList, toolCode)
		dedupeKey := toolCode + "\x1f" + call.Function.Arguments
		if cached, ok := seenMap[dedupeKey]; ok {
			resultList = append(resultList, schema.ToolMessage(cached, call.ID, schema.WithToolName(toolCode)))
			continue
		}
		resultText, err := s.invokeModelToolCall(ctx, events, toolCode, call.Function.Arguments)
		if err != nil {
			pkg.SLogger().Warnw("模型选择的工具执行失败", "tool_code", toolCode, "error", err)
			resultText = "工具执行失败：" + err.Error()
		}
		seenMap[dedupeKey] = resultText
		resultList = append(resultList, schema.ToolMessage(resultText, call.ID, schema.WithToolName(toolCode)))
	}
	return assistantTurn, resultList, codeList, nil
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

// Package ai 封装模型网关调用（eino + OpenAI 兼容 / Anthropic 原生组件）、提示词装配与技能资产。
package ai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/cloudwego/eino-ext/components/embedding/openai"
	einoClaude "github.com/cloudwego/eino-ext/components/model/claude"
	einoOpenAI "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// 协议编码常量（与 domain.ProtocolCode* 保持一致，避免 ai 反向依赖领域枚举语义）。
const (
	ProtocolOpenAI    = "openai"    // OpenAI 兼容协议
	ProtocolAnthropic = "anthropic" // Anthropic 原生协议
)

// 模型调用默认参数（供应商未给出时兜底）。
const (
	defaultMaxTokens   = 2048 // 单次输出上限
	defaultTemperature = 0.7  // 采样温度
	defaultTimeout     = 120 * time.Second
)

// ProviderSpec 一次调用所需的供应商能力快照（由 service 解密密钥后传入，ai 不接触存储）。
type ProviderSpec struct {
	ProviderULID       string        // 供应商 ULID（用量统计维度）
	ProviderName       string        // 供应商名称（用量统计维度）
	ProtocolCode       string        // 协议：openai/anthropic
	BaseURL            string        // 网关地址
	APIKey             string        // 明文密钥（仅内存）
	ChatModelName      string        // 对话模型名
	EmbeddingModelName string        // 向量模型名（空=不支持向量）
	EmbeddingDim       int           // 向量维度
	Temperature        float64       // 采样温度
	MaxTokens          int           // 输出上限
	Timeout            time.Duration // 单次调用超时
}

// SupportsEmbedding 判断该供应商是否可用于向量化（仅 OpenAI 兼容协议提供 embeddings）。
func (s ProviderSpec) SupportsEmbedding() bool {
	return s.EmbeddingModelName != "" && s.ProtocolCode == ProtocolOpenAI
}

// cacheKey 生成模型实例缓存键（含密钥摘要，密钥变更自动失效）。
func (s ProviderSpec) cacheKey() string {
	digest := sha256.Sum256([]byte(s.APIKey))
	return fmt.Sprintf("%s|%s|%s|%s|%s|%.2f|%d", s.ProviderULID, s.ProtocolCode, s.BaseURL,
		s.ChatModelName, hex.EncodeToString(digest[:8]), s.Temperature, s.MaxTokens)
}

// providerModels 单个供应商的模型实例缓存。
type providerModels struct {
	chat  model.BaseChatModel // 对话模型（nil 表示构造失败，需重建）
	embed embedding.Embedder  // 向量模型
}

// Client 模型网关：按供应商规格构造/复用 eino 组件，并统一上报用量。
type Client struct {
	mu     sync.RWMutex
	cache  map[string]*providerModels
	usage  UsageRecorder
	logger func(format string, args ...any)
}

// NewClient 构造模型网关客户端（模型实例按供应商规格惰性构造并缓存）。
func NewClient() *Client {
	return &Client{cache: make(map[string]*providerModels)}
}

// SetUsageRecorder 注入用量记录器（可为 nil，nil 时不上报）。
func (c *Client) SetUsageRecorder(recorder UsageRecorder) {
	c.usage = recorder
}

// chatModelOf 取得（或构造）供应商的对话模型。
func (c *Client) chatModelOf(spec ProviderSpec) (model.BaseChatModel, error) {
	key := spec.cacheKey()
	c.mu.RLock()
	if cached, ok := c.cache[key]; ok && cached.chat != nil {
		c.mu.RUnlock()
		return cached.chat, nil
	}
	c.mu.RUnlock()
	built, err := c.buildChatModel(spec)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	entry := c.cache[key]
	if entry == nil {
		entry = &providerModels{}
		c.cache[key] = entry
	}
	entry.chat = built
	c.mu.Unlock()
	return built, nil
}

// buildChatModel 按协议构造 eino 对话模型。
func (c *Client) buildChatModel(spec ProviderSpec) (model.BaseChatModel, error) {
	timeout := spec.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	maxTokens := spec.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}
	temperature := spec.Temperature
	if temperature <= 0 {
		temperature = defaultTemperature
	}
	switch spec.ProtocolCode {
	case ProtocolAnthropic:
		baseURL := spec.BaseURL
		built, err := einoClaude.NewChatModel(context.Background(), &einoClaude.Config{
			APIKey:         spec.APIKey,
			BaseURL:        &baseURL,
			Model:          spec.ChatModelName,
			MaxTokens:      maxTokens,
			Temperature:    float32Ptr(float32(temperature)),
			RequestTimeout: timeout,
		})
		if err != nil {
			return nil, fmt.Errorf("init anthropic chat model: %w", err)
		}
		return built, nil
	case ProtocolOpenAI:
		built, err := einoOpenAI.NewChatModel(context.Background(), &einoOpenAI.ChatModelConfig{
			BaseURL:     spec.BaseURL,
			APIKey:      spec.APIKey,
			Model:       spec.ChatModelName,
			Timeout:     timeout,
			MaxTokens:   &maxTokens,
			Temperature: float32Ptr(float32(temperature)),
		})
		if err != nil {
			return nil, fmt.Errorf("init openai chat model: %w", err)
		}
		return built, nil
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", spec.ProtocolCode)
	}
}

// embedderOf 取得（或构造）供应商的向量模型。
func (c *Client) embedderOf(spec ProviderSpec) (embedding.Embedder, error) {
	if !spec.SupportsEmbedding() {
		return nil, errors.New("embedding model is not configured for this provider")
	}
	key := spec.cacheKey()
	c.mu.RLock()
	if cached, ok := c.cache[key]; ok && cached.embed != nil {
		c.mu.RUnlock()
		return cached.embed, nil
	}
	c.mu.RUnlock()
	timeout := spec.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	built, err := openai.NewEmbedder(context.Background(), &openai.EmbeddingConfig{
		BaseURL: spec.BaseURL,
		APIKey:  spec.APIKey,
		Model:   spec.EmbeddingModelName,
		Timeout: timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("init embedding model: %w", err)
	}
	c.mu.Lock()
	entry := c.cache[key]
	if entry == nil {
		entry = &providerModels{}
		c.cache[key] = entry
	}
	entry.embed = built
	c.mu.Unlock()
	return built, nil
}

// Generate 非流式对话（用量与耗时自动上报）。
func (c *Client) Generate(ctx context.Context, spec ProviderSpec, messages []*schema.Message) (*schema.Message, error) {
	startedAt := time.Now()
	chatModel, err := c.chatModelOf(spec)
	if err != nil {
		c.record(spec, CallTypeChat, false, nil, time.Since(startedAt))
		return nil, err
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout(spec))
	defer cancel()
	reply, err := chatModel.Generate(callCtx, messages)
	c.record(spec, CallTypeChat, err == nil, usageOf(reply), time.Since(startedAt))
	if err != nil {
		return nil, fmt.Errorf("chat generate: %w", err)
	}
	return reply, nil
}

// Stream 流式对话（首个分片到达即返回，用量在流结束时由调用方补记）。
func (c *Client) Stream(ctx context.Context, spec ProviderSpec, messages []*schema.Message) (*schema.StreamReader[*schema.Message], error) {
	startedAt := time.Now()
	chatModel, err := c.chatModelOf(spec)
	if err != nil {
		c.record(spec, CallTypeChat, false, nil, time.Since(startedAt))
		return nil, err
	}
	stream, err := chatModel.Stream(ctx, messages)
	if err != nil {
		c.record(spec, CallTypeChat, false, nil, time.Since(startedAt))
		return nil, fmt.Errorf("chat stream: %w", err)
	}
	c.record(spec, CallTypeChat, true, nil, time.Since(startedAt))
	return stream, nil
}

// RecordStreamUsage 流式生成结束后补记 token 用量（流式首包已记一次成功调用）。
func (c *Client) RecordStreamUsage(spec ProviderSpec, usage *schema.TokenUsage, latency time.Duration) {
	if c.usage == nil || usage == nil {
		return
	}
	c.usage.RecordUsage(context.Background(), UsageRecord{
		ProviderULID:     spec.ProviderULID,
		ProviderName:     spec.ProviderName,
		ModelName:        spec.ChatModelName,
		CallTypeCode:     CallTypeChat,
		SuccessFlag:      true,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		LatencyMs:        latency.Milliseconds(),
	})
}

// Embed 批量文本向量化（用量自动上报）。
func (c *Client) Embed(ctx context.Context, spec ProviderSpec, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	startedAt := time.Now()
	embedder, err := c.embedderOf(spec)
	if err != nil {
		c.record(spec, CallTypeEmbedding, false, nil, time.Since(startedAt))
		return nil, err
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout(spec))
	defer cancel()
	vectors64, err := embedder.EmbedStrings(callCtx, texts)
	c.record(spec, CallTypeEmbedding, err == nil, nil, time.Since(startedAt))
	if err != nil {
		return nil, fmt.Errorf("embed texts: %w", err)
	}
	vectors32 := make([][]float32, len(vectors64))
	for i, vector := range vectors64 {
		row := make([]float32, len(vector))
		for j, value := range vector {
			row[j] = float32(value)
		}
		vectors32[i] = row
	}
	return vectors32, nil
}

// ProbeResult 供应商连通性测试结果。
type ProbeResult struct {
	ReplyText       string // 模型回显片段
	EmbeddingStatus string // ready/not_configured/failed
	EmbeddingNote   string // 向量通道结论说明
}

// Probe 连通性测试：先跑一次极小对话，再按需验证向量通道。
func (c *Client) Probe(ctx context.Context, spec ProviderSpec) (ProbeResult, error) {
	result := ProbeResult{EmbeddingStatus: EmbeddingStatusNotConfigured}
	probeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	reply, err := c.Generate(probeCtx, spec, []*schema.Message{
		{Role: schema.System, Content: "你是连通性探针，只回复两个字。"},
		{Role: schema.User, Content: "回复：通"},
	})
	if err != nil {
		return result, err
	}
	result.ReplyText = truncateRunes(reply.Content, 40)
	if !spec.SupportsEmbedding() {
		result.EmbeddingNote = "未配置向量模型或协议不支持 embeddings"
		return result, nil
	}
	vectors, err := c.Embed(probeCtx, spec, []string{"向量通道探针"})
	if err != nil {
		result.EmbeddingStatus = EmbeddingStatusFailed
		result.EmbeddingNote = "向量通道失败：" + err.Error()
		return result, nil
	}
	if len(vectors) == 0 || len(vectors[0]) == 0 {
		result.EmbeddingStatus = EmbeddingStatusFailed
		result.EmbeddingNote = "向量通道返回空结果"
		return result, nil
	}
	result.EmbeddingStatus = EmbeddingStatusReady
	result.EmbeddingNote = fmt.Sprintf("向量通道正常，维度 %d", len(vectors[0]))
	return result, nil
}

// record 上报一次调用（token 用量可为空，如流式首包与向量化）。
func (c *Client) record(spec ProviderSpec, callType string, success bool, usage *schema.TokenUsage, latency time.Duration) {
	if c.usage == nil {
		return
	}
	record := UsageRecord{
		ProviderULID: spec.ProviderULID,
		ProviderName: spec.ProviderName,
		ModelName:    modelNameOf(spec, callType),
		CallTypeCode: callType,
		SuccessFlag:  success,
		LatencyMs:    latency.Milliseconds(),
	}
	if usage != nil {
		record.PromptTokens = usage.PromptTokens
		record.CompletionTokens = usage.CompletionTokens
	}
	c.usage.RecordUsage(context.Background(), record)
}

// callTimeout 返回单次调用超时。
func callTimeout(spec ProviderSpec) time.Duration {
	if spec.Timeout > 0 {
		return spec.Timeout
	}
	return defaultTimeout
}

// modelNameOf 返回用量统计中的模型名（向量化取向量模型）。
func modelNameOf(spec ProviderSpec, callType string) string {
	if callType == CallTypeEmbedding {
		return spec.EmbeddingModelName
	}
	return spec.ChatModelName
}

// usageOf 提取非流式响应的 token 用量。
func usageOf(message *schema.Message) *schema.TokenUsage {
	if message == nil || message.ResponseMeta == nil {
		return nil
	}
	return message.ResponseMeta.Usage
}

// float32Ptr 构造 float32 指针（组件配置使用指针表达"可缺省"）。
func float32Ptr(value float32) *float32 {
	return &value
}

// truncateRunes 截断字符串到指定 rune 数（越界原样返回）。
func truncateRunes(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit])
}

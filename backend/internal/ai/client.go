// Package ai 封装模型网关调用（eino + OpenAI 兼容 / Anthropic 原生组件）、提示词装配与技能资产。
package ai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	einoClaude "github.com/cloudwego/eino-ext/components/model/claude"
	einoOpenAI "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"strings"
	"sync"
	"time"
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
	ProviderULID  string        // 供应商 ULID（用量统计维度）
	ProviderName  string        // 供应商名称（用量统计维度）
	ProtocolCode  string        // 协议：openai/anthropic
	BaseURL       string        // 网关地址
	APIKey        string        // 明文密钥（仅内存）
	ChatModelName string        // 对话模型名
	Temperature   float64       // 采样温度
	MaxTokens     int           // 输出上限
	Timeout       time.Duration // 单次调用超时
}

// cacheKey 生成模型实例缓存键（含密钥摘要，密钥变更自动失效）。
func (s ProviderSpec) cacheKey() string {
	digest := sha256.Sum256([]byte(s.APIKey))
	return fmt.Sprintf("%s|%s|%s|%s|%s|%.2f|%d", s.ProviderULID, s.ProtocolCode, s.BaseURL,
		s.ChatModelName, hex.EncodeToString(digest[:8]), s.Temperature, s.MaxTokens)
}

// providerModels 单个供应商的模型实例缓存。
type providerModels struct {
	chat model.BaseChatModel // 对话模型（nil 表示构造失败，需重建）
}

// Client 模型网关：按供应商规格构造/复用 eino 对话组件，并统一上报用量。
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

// ProbeResult 供应商对话连通性测试结果。
type ProbeResult struct {
	ReplyText string // 模型回显片段
}

// Probe 连通性测试：跑一次极小对话验证网关可用。
func (c *Client) Probe(ctx context.Context, spec ProviderSpec) (ProbeResult, error) {
	probeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	reply, err := c.Generate(probeCtx, spec, []*schema.Message{
		{Role: schema.System, Content: "你是连通性探针，只回复两个字。"},
		{Role: schema.User, Content: "回复：通"},
	})
	if err != nil {
		return ProbeResult{}, err
	}
	return ProbeResult{ReplyText: truncateRunes(reply.Content, 40)}, nil
}

// RerankCandidate 供对话模型重排的候选切片摘要（ID + 短文本）。
type RerankCandidate struct {
	ID   string // 切片 ULID
	Text string // 截断后的切片文本
}

// Rerank 用对话模型按查询相关性对候选 ID 重排，返回按得分降序的 ID 与说明。
// 严格解析 JSON 对象 {"ids":["CHK_x",...]}；任何异常均 fail-open 返回原顺序。
func (c *Client) Rerank(ctx context.Context, spec ProviderSpec, query string, candidates []RerankCandidate) ([]string, string) {
	original := make([]string, len(candidates))
	lines := make([]string, 0, len(candidates))
	for i, candidate := range candidates {
		original[i] = candidate.ID
		lines = append(lines, fmt.Sprintf("%d. ID=%s | %s", i+1, candidate.ID, candidate.Text))
	}
	prompt := "你是检索重排器。按与查询的相关性，仅输出严格 JSON：{\"ids\":[\"候选ID\",...]}，按最相关到最不相关排列，只使用给定ID，不要输出其他文字。\n查询：" +
		query + "\n候选：\n" + strings.Join(lines, "\n")
	reply, err := c.Generate(ctx, spec, []*schema.Message{
		{Role: schema.System, Content: "你只输出一个 JSON 对象，不输出解释或 Markdown。"},
		{Role: schema.User, Content: prompt},
	})
	if err != nil {
		return original, "rerun unavailable: " + err.Error()
	}
	ids, parseErr := parseRerankIDs(reply.Content, original)
	if parseErr != nil {
		return original, "rerun fallback: " + parseErr.Error()
	}
	return ids, "reranked by chat model"
}

// parseRerankIDs 从模型回复中严格解析 {"ids":[...]} 并校验 ID 属于候选集。
func parseRerankIDs(content string, allowed []string) ([]string, error) {
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "```") {
		return nil, fmt.Errorf("markdown fenced output")
	}
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("no JSON object")
	}
	var payload struct {
		IDs []string `json:"ids"`
	}
	if err := json.Unmarshal([]byte(trimmed[start:end+1]), &payload); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	valid := make(map[string]bool, len(allowed))
	for _, id := range allowed {
		valid[id] = true
	}
	seen := make(map[string]bool, len(payload.IDs))
	ordered := make([]string, 0, len(payload.IDs))
	for _, id := range payload.IDs {
		if !valid[id] || seen[id] {
			return nil, fmt.Errorf("invalid or duplicate id %q", id)
		}
		seen[id] = true
		ordered = append(ordered, id)
	}
	for _, id := range allowed {
		if !seen[id] {
			ordered = append(ordered, id)
		}
	}
	return ordered, nil
}

// ParseRerankIDs 解析并校验对话模型返回的重排 ID 列表，供 service 测试复用。
func ParseRerankIDs(content string, allowed []string) ([]string, error) {
	return parseRerankIDs(content, allowed)
}

// record 上报一次调用（token 用量可为空，如流式首包）。
func (c *Client) record(spec ProviderSpec, callType string, success bool, usage *schema.TokenUsage, latency time.Duration) {
	if c.usage == nil {
		return
	}
	record := UsageRecord{
		ProviderULID: spec.ProviderULID,
		ProviderName: spec.ProviderName,
		ModelName:    spec.ChatModelName,
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

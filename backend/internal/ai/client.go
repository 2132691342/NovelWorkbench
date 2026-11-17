// Package ai 封装模型网关调用（eino + OpenAI 兼容组件）与提示词上下文装配。
package ai

import (
	"context"
	"fmt"
	"time"

	einoEmbedding "github.com/cloudwego/eino-ext/components/embedding/openai"
	einoOpenAI "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
)

// Client 模型网关客户端：对话走 eino ChatModel，向量走 eino Embedder。
type Client struct {
	chatModel *einoOpenAI.ChatModel // 对话模型
	embedder  *einoEmbedding.Embedder // 向量模型
	timeout   time.Duration          // 单次调用超时
}

// NewClient 按配置构造对话与向量客户端。
func NewClient(baseURL string, apiKey string, chatModel string, embeddingModel string, timeout time.Duration) (*Client, error) {
	chatModelClient, err := einoOpenAI.NewChatModel(context.Background(), &einoOpenAI.ChatModelConfig{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   chatModel,
		Timeout: timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("init chat model: %w", err)
	}
	embedderClient, err := einoEmbedding.NewEmbedder(context.Background(), &einoEmbedding.EmbeddingConfig{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   embeddingModel,
		Timeout: timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("init embedding model: %w", err)
	}
	return &Client{chatModel: chatModelClient, embedder: embedderClient, timeout: timeout}, nil
}

// Chat 非流式对话：messages[0] 为 system 提示，其余按 user/assistant 顺序排列。
func (c *Client) Chat(ctx context.Context, messages []*schema.Message) (*schema.Message, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	reply, err := c.chatModel.Generate(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("chat generate: %w", err)
	}
	return reply, nil
}

// ChatStream 流式对话：返回增量消息流，调用方负责 Close。
func (c *Client) ChatStream(ctx context.Context, messages []*schema.Message) (*schema.StreamReader[*schema.Message], error) {
	stream, err := c.chatModel.Stream(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("chat stream: %w", err)
	}
	return stream, nil
}

// Embed 批量文本转向量（浮点 32 便于 pgvector 存取）。
func (c *Client) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	vectors64, err := c.embedder.EmbedStrings(ctx, texts)
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

package infra

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ManticoreClient Manticore Search 的 HTTP SQL 客户端（POST /sql?mode=raw）。
type ManticoreClient struct {
	baseURL    string       // HTTP JSON API 地址
	httpClient *http.Client // 复用的 HTTP 客户端（5s 超时）
}

// NewManticore 构造 Manticore 客户端（纯构造，连通性由 Ping 检查）。
func NewManticore(cfg *ManticoreConfig) *ManticoreClient {
	return &ManticoreClient{
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// Ping 探活 Manticore（执行 SELECT 1）。
func (m *ManticoreClient) Ping(ctx context.Context) error {
	if _, err := m.Exec(ctx, "SELECT 1"); err != nil {
		return fmt.Errorf("ping manticore: %w", err)
	}
	return nil
}

// Exec 以 raw 模式执行 SQL 语句，返回响应体。
func (m *ManticoreClient) Exec(ctx context.Context, sql string) ([]byte, error) {
	form := url.Values{"query": {sql}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/sql?mode=raw", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build manticore request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("exec manticore sql: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read manticore response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("manticore status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

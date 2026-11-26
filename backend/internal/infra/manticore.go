// 本文件实现 Manticore 全文检索的 HTTP SQL 客户端（连接构造与语句执行）。

package infra

import (
	"context"
	"encoding/json"
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

// ManticoreResultSet 一次查询的结果集（raw 模式返回数组，多语句时逐条对应）。
type ManticoreResultSet struct {
	Rows  []map[string]any `json:"data"`  // 数据行
	Total int              `json:"total"` // 返回行数
	Error string           `json:"error"` // 语句级错误
}

// Query 执行查询语句并解析为结构化行；语句级错误与传输错误都返回 error。
func (m *ManticoreClient) Query(ctx context.Context, sql string) ([]map[string]any, error) {
	body, err := m.Exec(ctx, sql)
	if err != nil {
		return nil, err
	}
	return ParseManticoreRows(body)
}

// ExecWrite 执行写入类语句（INSERT/UPDATE/DELETE/DDL），只关心是否成功。
func (m *ManticoreClient) ExecWrite(ctx context.Context, sql string) error {
	body, err := m.Exec(ctx, sql)
	if err != nil {
		return err
	}
	_, err = ParseManticoreRows(body)
	return err
}

// ParseManticoreRows 解析 raw 模式响应：数组即多结果集，语句级 error 视为失败。
func ParseManticoreRows(body []byte) ([]map[string]any, error) {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return nil, nil
	}
	var resultSets []ManticoreResultSet
	if err := json.Unmarshal([]byte(trimmed), &resultSets); err != nil {
		// 写入类语句成功时可能返回单个对象而非数组，这里容忍并校验 error 字段。
		var single ManticoreResultSet
		if singleErr := json.Unmarshal([]byte(trimmed), &single); singleErr != nil {
			return nil, fmt.Errorf("parse manticore response: %w", err)
		}
		if single.Error != "" {
			return nil, fmt.Errorf("manticore error: %s", single.Error)
		}
		return single.Rows, nil
	}
	rows := make([]map[string]any, 0, 16)
	for i := range resultSets {
		if resultSets[i].Error != "" {
			return nil, fmt.Errorf("manticore error: %s", resultSets[i].Error)
		}
		rows = append(rows, resultSets[i].Rows...)
	}
	return rows, nil
}

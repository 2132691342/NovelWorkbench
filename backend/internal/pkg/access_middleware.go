// 本文件实现权限校验与审计落库两个 HTTP 中间件。
// 审计中间件记录写操作的完整证据链：请求体（脱敏）、响应码与说明、HTTP 状态、级别、耗时与链路 ID。

package pkg

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// PermChecker 权限判定接口（使用方定义，service.RoleService 实现）。
type PermChecker interface {
	// HasPerm 判断用户是否拥有指定权限码。
	HasPerm(ctx context.Context, userULID string, permCode string) (bool, error)
}

// RequirePermMiddleware 校验当前用户拥有指定权限码，缺失返回 ERR_FORBIDDEN。
func RequirePermMiddleware(checker PermChecker, permCode string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userULID := c.GetString(ContextKeyUserULID)
		if userULID == "" {
			c.Abort()
			Fail(c, domain.RespCodeUnauthorized, "未登录")
			return
		}
		ok, err := checker.HasPerm(c.Request.Context(), userULID, permCode)
		if err != nil {
			Logger().Error("perm_check_failed",
				zap.String("request_id", c.GetString(ContextKeyRequestID)),
				zap.String("user_ulid", userULID),
				zap.Error(err),
			)
			c.Abort()
			Fail(c, domain.RespCodeInternalError, "权限校验失败")
			return
		}
		if !ok {
			c.Abort()
			Fail(c, domain.RespCodeForbidden, "无权限执行该操作")
			return
		}
		c.Next()
	}
}

// AuditRecorder 审计落库接口（使用方定义，service.AuditService 实现）。
type AuditRecorder interface {
	// Record 落库一条审计记录。
	Record(ctx context.Context, do *domain.AuditLogDO) error
}

// auditTargetKeys 审计目标提取白名单（body 键 → 目标类型）。
var auditTargetKeys = map[string]string{
	"user_ulid":          "user",
	"role_ulid":          "role",
	"menu_ulid":          "menu",
	"project_ulid":       "project",
	"chapter_ulid":       "chapter",
	"doc_ulid":           "document",
	"prompt_ulid":        "prompt",
	"tool_ulid":          "tool",
	"model_ulid":         "model",
	"provider_ulid":      "provider",
	"system_prompt_code": "prompt",
	"session_ulid":       "chat_session",
	"notification_ulid":  "notification",
}

// auditSecretKeys 需要脱敏的请求体字段（凭据类，落库前替换为掩码）。
var auditSecretKeys = map[string]bool{
	"password_text":     true,
	"new_password_text": true,
	"old_password_text": true,
	"api_key_text":      true,
	"access_token":      true,
}

// auditBodyLimitBytes 审计 body 采样上限（超出只记动作码）。
const auditBodyLimitBytes = 32 * 1024

// auditRespLimitBytes 审计响应采样上限（超出标注截断，只影响详情展示）。
const auditRespLimitBytes = 32 * 1024

// AuditMiddleware 记录变更类请求的审计日志（POST 且不在跳过名单内，无论成败）。
func AuditMiddleware(recorder AuditRecorder, skipPaths []string) gin.HandlerFunc {
	skipSet := make(map[string]bool, len(skipPaths))
	for _, path := range skipPaths {
		skipSet[path] = true
	}
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost || skipSet[c.Request.URL.Path] {
			c.Next()
			return
		}
		startedAt := time.Now()
		requestBody := readAuditBody(c)
		recorderWriter := &auditResponseWriter{ResponseWriter: c.Writer, buffer: &bytes.Buffer{}}
		c.Writer = recorderWriter
		c.Next()
		c.Writer = recorderWriter.ResponseWriter

		rawResp := recorderWriter.buffer.Bytes()
		respCode, respMessage := parseAuditResp(rawResp)
		status := recorderWriter.Status()
		actionCode := strings.TrimPrefix(c.Request.URL.Path, "/api/v1/")
		do := &domain.AuditLogDO{
			LogLevelCode:   AuditLevelOf(status),
			SpanTypeCode:   domain.AuditSpanTypeHTTP, // 入口请求：链路时间线的根 span
			TraceID:        c.GetString(ContextKeyRequestID),
			ParentSpanCode: "", // 顶级 span 没有父动作
			UserULID:       c.GetString(ContextKeyUserULID),
			ActionCode:     actionCode,
			ActionName:     AuditActionLabel(actionCode),
			RequestPath:    c.Request.URL.Path,
			RequestMethod:  c.Request.Method,
			RequestBody:    requestBody,
			TargetType:     "",
			TargetULID:     "",
			RespCode:       respCode,
			RespMessage:    TruncateText(respMessage, 512),
			RespBody:       sanitizeAuditResp(rawResp, recorderWriter.truncated),
			HTTPStatus:     status,
			LatencyMs:      time.Since(startedAt).Milliseconds(),
			IPAddr:         c.ClientIP(),
			UserAgentText:  TruncateText(c.Request.UserAgent(), 512),
		}
		do.TargetType, do.TargetULID = extractAuditTarget(c, requestBody)
		if err := recorder.Record(c.Request.Context(), do); err != nil {
			// 审计失败不阻断业务，仅边界层记录
			Logger().Warn("audit_record_failed",
				zap.String("request_id", c.GetString(ContextKeyRequestID)),
				zap.Error(err),
			)
		}
	}
}

// auditResponseWriter 捕获响应体片段（上限 32KB，超出置截断标记）。
type auditResponseWriter struct {
	gin.ResponseWriter
	buffer    *bytes.Buffer // 采样缓冲
	truncated bool          // 是否因超出上限被截断
}

// Write 先落业务缓冲再透传，捕获失败不影响响应。
func (w *auditResponseWriter) Write(data []byte) (int, error) {
	if remain := auditRespLimitBytes - w.buffer.Len(); remain > 0 {
		if len(data) > remain {
			w.buffer.Write(data[:remain])
			w.truncated = true
		} else {
			w.buffer.Write(data)
		}
	} else if len(data) > 0 {
		w.truncated = true
	}
	return w.ResponseWriter.Write(data)
}

// AuditLevelOf 由 HTTP 状态推导审计级别（中间件与历史行补全共用）。
func AuditLevelOf(status int) string {
	switch {
	case status >= 500:
		return domain.AuditLevelError
	case status >= 400:
		return domain.AuditLevelWarn
	default:
		return domain.AuditLevelInfo
	}
}

// readAndRestoreBody 读取请求体并还原，供审计与幂等中间件共用（两者可同时挂载）。
func readAndRestoreBody(c *gin.Context, limitBytes int64) ([]byte, error) {
	if c.Request.Body == nil {
		return nil, nil
	}
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, limitBytes))
	c.Request.Body = io.NopCloser(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	return raw, nil
}

// readAuditBody 读取并还原请求体：上传类请求不采样，凭据字段脱敏后落库。
func readAuditBody(c *gin.Context) string {
	if c.Request.Body == nil || c.Request.ContentLength <= 0 {
		return ""
	}
	if strings.HasPrefix(c.Request.Header.Get("Content-Type"), "multipart/") {
		return "[multipart 上传，未采样]"
	}
	if c.Request.ContentLength > auditBodyLimitBytes {
		return "[请求体超过 32KB，未采样]"
	}
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, auditBodyLimitBytes))
	c.Request.Body = io.NopCloser(bytes.NewReader(raw)) // 还原 body 供 handler 绑定
	if err != nil {
		return ""
	}
	return sanitizeAuditBody(raw)
}

// sanitizeAuditBody 把凭据字段替换为掩码，其余字段压缩为单行 JSON。
func sanitizeAuditBody(raw []byte) string {
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		return TruncateText(string(raw), 2000)
	}
	for key := range fields {
		if auditSecretKeys[key] {
			fields[key] = "***"
			continue
		}
		// 嵌套结构同样按字段名脱敏一层
		if nested, ok := fields[key].(map[string]any); ok {
			for nestedKey := range nested {
				if auditSecretKeys[nestedKey] {
					nested[nestedKey] = "***"
				}
			}
		}
	}
	sanitized, err := json.Marshal(fields)
	if err != nil {
		return ""
	}
	return TruncateText(string(sanitized), 4000)
}

// sanitizeAuditResp 把响应体裁剪为可读的落库文本：包络 data 保留原文，非 JSON 与流式响应给占位说明。
func sanitizeAuditResp(raw []byte, truncated bool) string {
	if len(raw) == 0 {
		return ""
	}
	if !json.Valid(raw) {
		if truncated {
			return "[响应超过 32KB 或为流式输出，未完整采样]"
		}
		return TruncateText(string(raw), 2000)
	}
	text := TruncateText(string(raw), auditBodyLimitBytes)
	if truncated {
		text = text + "…[响应超过 32KB，已截断]"
	}
	return text
}

// parseAuditResp 从响应体解析业务码与说明（非 JSON 包络时留空）。
func parseAuditResp(raw []byte) (string, string) {
	if len(raw) == 0 {
		return "", ""
	}
	var envelope struct {
		RespCode    string `json:"resp_code"`
		RespMessage string `json:"resp_message"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return "", ""
	}
	return envelope.RespCode, envelope.RespMessage
}

// extractAuditTarget 从请求体提取目标 ULID（白名单键，取第一个命中项）。
func extractAuditTarget(c *gin.Context, requestBody string) (string, string) {
	if requestBody == "" || strings.HasPrefix(requestBody, "[") {
		return "", ""
	}
	var fields map[string]any
	if err := json.Unmarshal([]byte(requestBody), &fields); err != nil {
		return "", ""
	}
	for key, targetType := range auditTargetKeys {
		if value, ok := fields[key].(string); ok && value != "" && value != "***" {
			return targetType, value
		}
	}
	return "", ""
}

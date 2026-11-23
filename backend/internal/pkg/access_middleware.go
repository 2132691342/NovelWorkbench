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
	"user_ulid":         "user",
	"role_ulid":         "role",
	"menu_ulid":         "menu",
	"project_ulid":      "project",
	"chapter_ulid":      "chapter",
	"doc_ulid":          "document",
	"skill_ulid":        "skill",
	"tool_ulid":         "tool",
	"model_ulid":        "model",
	"provider_ulid":     "provider",
	"profile_ulid":      "prompt_profile",
	"session_ulid":      "chat_session",
	"notification_ulid": "notification",
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

// auditRespLimitBytes 审计响应采样上限（只需解析包络与说明）。
const auditRespLimitBytes = 8 * 1024

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

		respCode, respMessage := parseAuditResp(recorderWriter.buffer.Bytes())
		status := recorderWriter.Status()
		do := &domain.AuditLogDO{
			LogLevelCode:  AuditLevelOf(status),
			TraceID:       c.GetString(ContextKeyRequestID),
			UserULID:      c.GetString(ContextKeyUserULID),
			ActionCode:    strings.TrimPrefix(c.Request.URL.Path, "/api/v1/"),
			RequestPath:   c.Request.URL.Path,
			RequestMethod: c.Request.Method,
			RequestBody:   requestBody,
			TargetType:    "",
			TargetULID:    "",
			RespCode:      respCode,
			RespMessage:   TruncateText(respMessage, 512),
			HTTPStatus:    status,
			LatencyMs:     time.Since(startedAt).Milliseconds(),
			IPAddr:        c.ClientIP(),
			UserAgentText: TruncateText(c.Request.UserAgent(), 512),
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

// auditResponseWriter 捕获响应体片段（只留前 8KB，够解析包络即可）。
type auditResponseWriter struct {
	gin.ResponseWriter
	buffer *bytes.Buffer
}

// Write 先落业务缓冲再透传，捕获失败不影响响应。
func (w *auditResponseWriter) Write(data []byte) (int, error) {
	if remain := auditRespLimitBytes - w.buffer.Len(); remain > 0 {
		if len(data) > remain {
			w.buffer.Write(data[:remain])
		} else {
			w.buffer.Write(data)
		}
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
		// 嵌套结构（技能变量等）同样按字段名脱敏一层
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

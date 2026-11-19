// 本文件实现权限校验与审计落库两个 HTTP 中间件。

package pkg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

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
	"user_ulid":    "user",
	"role_ulid":    "role",
	"project_ulid": "project",
	"chapter_ulid": "chapter",
	"doc_ulid":     "document",
}

// auditBodyLimitBytes 审计 body 采样上限（超出只记动作码）。
const auditBodyLimitBytes = 64 * 1024

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
		targetType, targetULID := extractAuditTarget(c)
		c.Next()
		do := &domain.AuditLogDO{
			UserULID:   c.GetString(ContextKeyUserULID),
			ActionCode: strings.TrimPrefix(c.Request.URL.Path, "/api/v1/"),
			TargetType: targetType,
			TargetULID: targetULID,
			DetailText: fmt.Sprintf("status=%d", c.Writer.Status()),
			IPAddr:     c.ClientIP(),
		}
		if err := recorder.Record(c.Request.Context(), do); err != nil {
			// 审计失败不阻断业务，仅边界层记录
			Logger().Warn("audit_record_failed",
				zap.String("request_id", c.GetString(ContextKeyRequestID)),
				zap.Error(err),
			)
		}
	}
}

// extractAuditTarget 从 admin 接口 body 提取目标 ULID（受限白名单，其他路径不读 body）。
func extractAuditTarget(c *gin.Context) (string, string) {
	if !strings.HasPrefix(c.Request.URL.Path, "/api/v1/admin/") {
		return "", ""
	}
	if c.Request.Body == nil || c.Request.ContentLength <= 0 || c.Request.ContentLength > auditBodyLimitBytes {
		return "", ""
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, auditBodyLimitBytes))
	c.Request.Body = io.NopCloser(bytes.NewReader(body)) // 还原 body 供 handler 绑定
	if err != nil {
		return "", ""
	}
	var fields map[string]any
	if err := json.Unmarshal(body, &fields); err != nil {
		return "", ""
	}
	for key, targetType := range auditTargetKeys {
		if value, ok := fields[key].(string); ok && value != "" {
			return targetType, value
		}
	}
	return "", ""
}

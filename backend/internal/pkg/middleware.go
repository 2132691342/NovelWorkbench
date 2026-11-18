package pkg

import (
	"context"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// gin 上下文键常量。
const (
	ContextKeyRequestID = "request_id" // 请求链路 ID
	ContextKeyUserULID  = "user_ulid"  // 当前登录用户 ULID
)

// RequestIDMiddleware 为每条请求分配链路 ID（透传 X-Request-ID）。
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = NewULID("REQ_")
		}
		c.Set(ContextKeyRequestID, requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// LoggerMiddleware 记录请求日志（出口层统一记录，含耗时）。
func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		Logger().Info("http_request",
			zap.String("request_id", c.GetString(ContextKeyRequestID)),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Int64("latency_ms", time.Since(start).Milliseconds()),
		)
	}
}

// RecoveryMiddleware panic 恢复，统一输出内部错误响应。
func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				Logger().Error("http_panic",
					zap.String("request_id", c.GetString(ContextKeyRequestID)),
					zap.Any("panic", recovered),
					zap.ByteString("stack", debug.Stack()),
				)
				c.Abort()
				Fail(c, domain.RespCodeInternalError, "服务内部错误")
			}
		}()
		c.Next()
	}
}

// MethodWhitelistMiddleware 业务接口仅允许 GET/POST。
func MethodWhitelistMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodPost {
			c.Abort()
			Fail(c, domain.RespCodeInvalidParam, "仅支持 GET/POST")
			return
		}
		c.Next()
	}
}

// AuthMiddleware 校验 Bearer 令牌并注入用户 ULID；skipPaths 为免鉴权白名单。
func AuthMiddleware(signer *JWTSigner, tokenStore TokenStore, skipPaths []string) gin.HandlerFunc {
	skipSet := make(map[string]bool, len(skipPaths))
	for _, path := range skipPaths {
		skipSet[path] = true
	}
	return func(c *gin.Context) {
		if skipSet[c.Request.URL.Path] {
			c.Next()
			return
		}
		tokenString := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if tokenString == "" {
			c.Abort()
			Fail(c, domain.RespCodeUnauthorized, "未登录")
			return
		}
		userULID, err := signer.ParseToken(tokenString)
		if err != nil {
			c.Abort()
			Fail(c, domain.RespCodeUnauthorized, "登录已失效")
			return
		}
		if tokenStore != nil && tokenStore.IsRevoked(c.Request.Context(), tokenString) {
			c.Abort()
			Fail(c, domain.RespCodeUnauthorized, "登录已失效")
			return
		}
		c.Set(ContextKeyUserULID, userULID)
		c.Next()
	}
}

// TokenStore 令牌吊销存储（登出时拉黑，Redis 实现）。
type TokenStore interface {
	// Revoke 吊销令牌（写入黑名单直至自然过期）。
	Revoke(ctx context.Context, tokenString string, ttl time.Duration) error
	// IsRevoked 判断令牌是否已被吊销。
	IsRevoked(ctx context.Context, tokenString string) bool
}

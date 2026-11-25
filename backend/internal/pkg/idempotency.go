// 本文件实现写请求幂等：同一次「点击」重复提交时只执行一次，重放首次响应。
// 客户端每次业务动作生成一个 Idempotency-Key；服务端按「用户 + 路径 + 键」缓存首响应，
// 键相同但请求体不同视为客户端错误，避免把两次不同动作误当成同一动作。

package pkg

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// IdempotencyHeader 幂等键请求头与回放标记响应头。
const (
	IdempotencyHeader       = "Idempotency-Key"   // 客户端生成的单次动作键
	IdempotencyReplayHeader = "Idempotent-Replay" // 响应头：true 表示本次为首响应的重放
)

// 幂等缓存状态。
const (
	idempotencyStateProcessing = "processing" // 首请求执行中
	idempotencyStateDone       = "done"       // 已有可重放的响应
)

// idempotencyBodyLimitBytes 重放缓存上限（超出不做缓存，按普通请求执行）。
const idempotencyBodyLimitBytes = 1 << 20

// IdempotencyRecord 首响应的落库形态（状态码 + 响应头 + 响应体）。
type IdempotencyRecord struct {
	StateCode  string            `json:"state_code"`  // processing/done
	BodyHash   string            `json:"body_hash"`   // 请求体指纹（防同键不同请求）
	HTTPStatus int               `json:"http_status"` // 首响应状态码
	HeaderMap  map[string]string `json:"header_map"`  // 首响应关键头
	BodyText   string            `json:"body_text"`   // 首响应体
}

// IdempotencyStore 幂等记录存取（Redis 实现；不可用时调用方降级放行）。
type IdempotencyStore interface {
	// Claim 抢占键：返回 (record, true) 表示键已存在。
	Claim(ctx context.Context, key string, record *IdempotencyRecord, ttlSeconds int) (*IdempotencyRecord, bool, error)
	// Save 覆盖写入已完成的首响应。
	Save(ctx context.Context, key string, record *IdempotencyRecord, ttlSeconds int) error
	// Release 释放键（首请求失败于框架层时允许客户端重试）。
	Release(ctx context.Context, key string) error
}

// idempotentActionVerbs 需要幂等保护的写动作末段（白名单）。
// 名单外的 POST 一律视为查询（后端为携带 body 而统一用 POST），缓存它们的响应会让列表 / 详情返回陈旧数据。
var idempotentActionVerbs = map[string]bool{
	"create": true, "save": true, "update": true, "delete": true,
	"restore": true, "upload": true, "reindex": true, "review": true,
	"invoke": true, "set_default": true, "grant": true, "test": true,
	"read": true, "read_all": true, "apply": true, "retry": true,
	"change": true, "reset_password": true,
}

// isIdempotentAction 判断请求路径是否为受保护的写动作。
func isIdempotentAction(requestPath string) bool {
	trimmed := strings.TrimSuffix(requestPath, "/")
	lastSegment := trimmed[strings.LastIndex(trimmed, "/")+1:]
	return idempotentActionVerbs[lastSegment]
}

// IdempotencyMiddleware 为携带幂等键的写请求提供「只执行一次」语义；store 为 nil 时直接放行。
// skipPaths 放行登录/登出：这两个动作本身就是「重放即重做」，缓存首响应会把旧令牌再发一次。
func IdempotencyMiddleware(store IdempotencyStore, ttlSeconds int, skipPaths []string) gin.HandlerFunc {
	skipSet := make(map[string]bool, len(skipPaths))
	for _, path := range skipPaths {
		skipSet[path] = true
	}
	return func(c *gin.Context) {
		idempotencyKey := strings.TrimSpace(c.GetHeader(IdempotencyHeader))
		if store == nil || idempotencyKey == "" || c.Request.Method != http.MethodPost ||
			skipSet[c.Request.URL.Path] || !isIdempotentAction(c.Request.URL.Path) {
			c.Next()
			return
		}
		userULID := c.GetString(ContextKeyUserULID)
		storageKey := idempotencyStorageKey(userULID, c.Request.URL.Path, idempotencyKey)
		bodyHash := idempotencyBodyHash(c)
		existing, hit, err := store.Claim(c.Request.Context(), storageKey, &IdempotencyRecord{
			StateCode: idempotencyStateProcessing,
			BodyHash:  bodyHash,
		}, ttlSeconds)
		if err != nil {
			Logger().Warn("idempotency_claim_failed", zapError(err))
			c.Next()
			return
		}
		if hit {
			replayIdempotent(c, existing, bodyHash)
			return
		}
		bufferedWriter := &idempotencyResponseWriter{ResponseWriter: c.Writer, buffer: &bytes.Buffer{}}
		c.Writer = bufferedWriter
		c.Next()
		c.Writer = bufferedWriter.ResponseWriter
		if bufferedWriter.Status() >= http.StatusMultipleChoices {
			// 只有 2xx 才缓存：业务校验失败时让客户端改正后重试，否则同键不同体反而会被拒
			_ = store.Release(context.WithoutCancel(c.Request.Context()), storageKey)
			return
		}
		record := &IdempotencyRecord{
			StateCode:  idempotencyStateDone,
			BodyHash:   bodyHash,
			HTTPStatus: bufferedWriter.Status(),
			HeaderMap:  map[string]string{"Content-Type": c.Writer.Header().Get("Content-Type")},
			BodyText:   bufferedWriter.buffer.String(),
		}
		if err := store.Save(context.WithoutCancel(c.Request.Context()), storageKey, record, ttlSeconds); err != nil {
			Logger().Warn("idempotency_save_failed", zapError(err))
		}
	}
}

// replayIdempotent 重放首响应；请求体指纹不一致时明确拒绝，不猜测客户端意图。
func replayIdempotent(c *gin.Context, record *IdempotencyRecord, bodyHash string) {
	if record.BodyHash != bodyHash {
		c.Abort()
		Fail(c, "ERR_INVALID_PARAM", "同一 Idempotency-Key 不能用于不同的请求内容")
		return
	}
	if record.StateCode != idempotencyStateDone {
		c.Abort()
		c.Header("Retry-After", "1")
		Fail(c, "ERR_CONFLICT", "相同请求正在处理中，请稍候")
		return
	}
	c.Header(IdempotencyReplayHeader, "true")
	contentType := record.HeaderMap["Content-Type"]
	if contentType == "" {
		contentType = "application/json; charset=utf-8"
	}
	c.Data(record.HTTPStatus, contentType, []byte(record.BodyText))
	c.Abort()
}

// idempotencyStorageKey 生成 Redis 键：按用户与路径隔离，键本身哈希后固定长度。
func idempotencyStorageKey(userULID, requestPath, idempotencyKey string) string {
	sum := sha256.Sum256([]byte(userULID + "\x00" + requestPath + "\x00" + idempotencyKey))
	return "nw:idempotency:" + hex.EncodeToString(sum[:])
}

// idempotencyBodyHash 读取请求体指纹并还原 body（供后续 handler 正常绑定）。
func idempotencyBodyHash(c *gin.Context) string {
	if c.Request.Body == nil || c.Request.ContentLength <= 0 {
		return ""
	}
	raw, err := readAndRestoreBody(c, idempotencyBodyLimitBytes)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// idempotencyResponseWriter 完整缓存响应体（上限 1MB，超出即放弃缓存）。
type idempotencyResponseWriter struct {
	gin.ResponseWriter
	buffer   *bytes.Buffer // 响应缓冲
	overflow bool          // 是否超出缓存上限
}

// Write 先写缓冲再透传；超限后停止缓冲，只透传。
func (w *idempotencyResponseWriter) Write(data []byte) (int, error) {
	if !w.overflow {
		if w.buffer.Len()+len(data) > idempotencyBodyLimitBytes {
			w.overflow = true
		} else {
			w.buffer.Write(data)
		}
	}
	return w.ResponseWriter.Write(data)
}

// zapError 包装 error 为 zap 字段（日志只用一个出口）。
func zapError(err error) zap.Field {
	return zap.Error(err)
}

// MarshalIdempotencyRecord 供仓储层序列化记录。
func MarshalIdempotencyRecord(record *IdempotencyRecord) (string, error) {
	raw, err := json.Marshal(record)
	return string(raw), err
}

// UnmarshalIdempotencyRecord 供仓储层反序列化记录。
func UnmarshalIdempotencyRecord(text string) (*IdempotencyRecord, error) {
	record := &IdempotencyRecord{}
	if err := json.Unmarshal([]byte(text), record); err != nil {
		return nil, err
	}
	return record, nil
}

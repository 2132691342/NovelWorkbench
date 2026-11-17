package repo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
)

// 令牌黑名单键前缀。
const tokenBlacklistPrefix = "nw:token:blacklist:"

// TokenRepo 令牌吊销黑名单（Redis 实现 pkg.TokenStore）。
type TokenRepo struct {
	rdb *redis.Client
}

// NewTokenRepo 构造令牌吊销仓储。
func NewTokenRepo(rdb *redis.Client) *TokenRepo {
	return &TokenRepo{rdb: rdb}
}

// 编译期断言 TokenRepo 实现 pkg.TokenStore。
var _ pkg.TokenStore = (*TokenRepo)(nil)

// Revoke 吊销令牌：写入黑名单键（值 "1"），TTL 到期后自然清除。
func (r *TokenRepo) Revoke(ctx context.Context, tokenString string, ttl time.Duration) error {
	if err := r.rdb.Set(ctx, tokenBlacklistKey(tokenString), "1", ttl).Err(); err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}
	return nil
}

// IsRevoked 判断令牌是否已被吊销（Redis 异常时视为未吊销，放行由上层日志兜底）。
func (r *TokenRepo) IsRevoked(ctx context.Context, tokenString string) bool {
	count, err := r.rdb.Exists(ctx, tokenBlacklistKey(tokenString)).Result()
	return err == nil && count > 0
}

// tokenBlacklistKey 计算黑名单键：nw:token:blacklist:<sha256hex(token)>。
func tokenBlacklistKey(tokenString string) string {
	sum := sha256.Sum256([]byte(tokenString))
	return tokenBlacklistPrefix + hex.EncodeToString(sum[:])
}

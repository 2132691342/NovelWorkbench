// 本文件实现幂等记录的 Redis 存取（pkg.IdempotencyStore 实现）。

package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
)

// IdempotencyRepo 幂等记录仓储：Claim 用 SET NX 抢占，完成态用 SET XX 覆盖。
type IdempotencyRepo struct {
	rdb *redis.Client // Redis 客户端
}

// NewIdempotencyRepo 构造幂等记录仓储；rdb 为空时返回 nil（中间件按降级放行处理）。
func NewIdempotencyRepo(rdb *redis.Client) *IdempotencyRepo {
	if rdb == nil {
		return nil
	}
	return &IdempotencyRepo{rdb: rdb}
}

// 编译期断言 IdempotencyRepo 实现 pkg.IdempotencyStore。
var _ pkg.IdempotencyStore = (*IdempotencyRepo)(nil)

// Claim 抢占幂等键：键已存在时返回其当前记录与 true。
func (r *IdempotencyRepo) Claim(ctx context.Context, key string, record *pkg.IdempotencyRecord,
	ttlSeconds int) (*pkg.IdempotencyRecord, bool, error) {
	payload, err := pkg.MarshalIdempotencyRecord(record)
	if err != nil {
		return nil, false, fmt.Errorf("marshal idempotency record: %w", err)
	}
	ttl := time.Duration(ttlSeconds) * time.Second
	claimed, err := r.rdb.SetNX(ctx, key, payload, ttl).Result()
	if err != nil {
		return nil, false, fmt.Errorf("claim idempotency key: %w", err)
	}
	if claimed {
		return nil, false, nil
	}
	current, err := r.rdb.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			// 抢占与读取之间恰好过期：按未命中处理，让本次请求继续执行
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read idempotency key: %w", err)
	}
	existing, err := pkg.UnmarshalIdempotencyRecord(current)
	if err != nil {
		return nil, false, fmt.Errorf("unmarshal idempotency record: %w", err)
	}
	return existing, true, nil
}

// Save 覆盖写入已完成的首响应（键不存在时不重建，避免孤儿记录）。
func (r *IdempotencyRepo) Save(ctx context.Context, key string, record *pkg.IdempotencyRecord,
	ttlSeconds int) error {
	payload, err := pkg.MarshalIdempotencyRecord(record)
	if err != nil {
		return fmt.Errorf("marshal idempotency record: %w", err)
	}
	if err := r.rdb.SetXX(ctx, key, payload, time.Duration(ttlSeconds)*time.Second).Err(); err != nil {
		return fmt.Errorf("save idempotency record: %w", err)
	}
	return nil
}

// Release 释放幂等键（首请求在框架层失败时允许重试）。
func (r *IdempotencyRepo) Release(ctx context.Context, key string) error {
	if err := r.rdb.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("release idempotency key: %w", err)
	}
	return nil
}

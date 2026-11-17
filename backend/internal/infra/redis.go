package infra

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewRedis 构建 Redis 客户端并探活；Ping 失败仍返回可用客户端，由调用方降级处理。
func NewRedis(cfg *RedisConfig) (*redis.Client, error) {
	if cfg == nil || cfg.Addr == "" {
		return nil, fmt.Errorf("redis addr is required")
	}
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return client, fmt.Errorf("ping redis: %w", err)
	}
	return client, nil
}

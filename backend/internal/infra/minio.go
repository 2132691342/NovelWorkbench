package infra

import (
	"context"
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// NewMinIO 构建 MinIO 客户端并确保桶存在（不存在则创建）；连通性失败仍返回可用客户端，由调用方降级处理。
func NewMinIO(cfg *MinIOConfig) (*minio.Client, error) {
	if cfg == nil || cfg.Endpoint == "" {
		return nil, fmt.Errorf("minio endpoint is required")
	}
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return client, fmt.Errorf("check minio bucket %s: %w", cfg.Bucket, err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
			return client, fmt.Errorf("create minio bucket %s: %w", cfg.Bucket, err)
		}
	}
	return client, nil
}

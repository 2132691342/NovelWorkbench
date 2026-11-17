package repo

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
)

// ObjectRepo MinIO 对象存储仓储（知识库文档原文存取）。
type ObjectRepo struct {
	client *minio.Client
	bucket string
}

// NewObjectRepo 构造对象存储仓储。
func NewObjectRepo(client *minio.Client, bucket string) *ObjectRepo {
	return &ObjectRepo{client: client, bucket: bucket}
}

// PutObject 上传对象（对象键即文档 ULID）。
func (r *ObjectRepo) PutObject(ctx context.Context, objectKey string, reader io.Reader, size int64, contentType string) error {
	if r == nil || r.client == nil {
		return fmt.Errorf("put object %s: minio unavailable", objectKey)
	}
	if _, err := r.client.PutObject(ctx, r.bucket, objectKey, reader, size, minio.PutObjectOptions{ContentType: contentType}); err != nil {
		return fmt.Errorf("put object %s: %w", objectKey, err)
	}
	return nil
}

// GetObject 读取对象内容流（调用方负责 Close）。
func (r *ObjectRepo) GetObject(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	if r == nil || r.client == nil {
		return nil, fmt.Errorf("get object %s: minio unavailable", objectKey)
	}
	obj, err := r.client.GetObject(ctx, r.bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("get object %s: %w", objectKey, err)
	}
	return obj, nil
}

// RemoveObject 删除对象（不存在视为成功）。
func (r *ObjectRepo) RemoveObject(ctx context.Context, objectKey string) error {
	if r == nil || r.client == nil {
		return fmt.Errorf("remove object %s: minio unavailable", objectKey)
	}
	if err := r.client.RemoveObject(ctx, r.bucket, objectKey, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("remove object %s: %w", objectKey, err)
	}
	return nil
}

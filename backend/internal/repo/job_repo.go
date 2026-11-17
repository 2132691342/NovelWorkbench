package repo

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// JobRepo 异步任务留痕仓储（RabbitMQ 消费的落库凭据）。
type JobRepo struct {
	db *gorm.DB
}

// NewJobRepo 构造任务仓储。
func NewJobRepo(db *gorm.DB) *JobRepo {
	return &JobRepo{db: db}
}

// Create 插入任务记录（队列投递前置 queued）。
func (r *JobRepo) Create(ctx context.Context, do *domain.JobDO) error {
	if err := r.db.WithContext(ctx).Create(do).Error; err != nil {
		return fmt.Errorf("create job: %w", err)
	}
	return nil
}

// Update 按列名增量更新任务（map 更新防零值丢失）。
func (r *JobRepo) Update(ctx context.Context, jobULID string, fields map[string]any) error {
	result := r.db.WithContext(ctx).Model(&domain.JobDO{}).
		Where("job_ulid = ?", jobULID).Updates(fields)
	if result.Error != nil {
		return fmt.Errorf("update job: %w", result.Error)
	}
	return nil
}

// GetPendingByDoc 按文档 ULID 查询最近一条待处理任务（payload 载荷含 doc_ulid），不存在时返回 (nil, nil)。
func (r *JobRepo) GetPendingByDoc(ctx context.Context, docULID string) (*domain.JobDO, error) {
	var do domain.JobDO
	err := r.db.WithContext(ctx).Model(&domain.JobDO{}).
		Where("payload_json LIKE ? AND job_status_code IN ?", "%\"doc_ulid\":\""+docULID+"\"%", []string{"queued", "running"}).
		Order("created_at DESC").First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get pending job: %w", err)
	}
	return &do, nil
}

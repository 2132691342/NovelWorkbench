// 本文件实现异步任务留痕的持久化仓储。

package repo

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// JobRepo 异步任务留痕仓储（RabbitMQ 消费的落库凭据）。
type JobRepo struct {
	db *gorm.DB // 事务入口
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

// GetByULID 按任务 ULID 查询，不存在时返回 (nil, nil)。
func (r *JobRepo) GetByULID(ctx context.Context, jobULID string) (*domain.JobDO, error) {
	var do domain.JobDO
	err := r.db.WithContext(ctx).Where("job_ulid = ?", jobULID).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get job by ulid: %w", err)
	}
	return &do, nil
}

// ListPage 分页查询任务（管理端任务中心，按时间倒序）。
func (r *JobRepo) ListPage(ctx context.Context, req *domain.JobListREQ) ([]domain.JobDO, int64, error) {
	query := r.db.WithContext(ctx).Model(&domain.JobDO{})
	if req.JobTypeCode != "" {
		query = query.Where("job_type_code = ?", req.JobTypeCode)
	}
	if req.JobStatusCode != "" {
		query = query.Where("job_status_code = ?", req.JobStatusCode)
	}
	if req.KeywordText != "" {
		like := "%" + req.KeywordText + "%"
		query = query.Where("job_ulid LIKE ? OR payload_json LIKE ?", like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count jobs: %w", err)
	}
	var dos []domain.JobDO
	if err := query.Order("created_at DESC").
		Offset((req.PageNum - 1) * req.PageSize).Limit(req.PageSize).
		Find(&dos).Error; err != nil {
		return nil, 0, fmt.Errorf("list jobs: %w", err)
	}
	return dos, total, nil
}

// CountByStatus 统计各状态任务数（仪表盘/任务中心概览）。
func (r *JobRepo) CountByStatus(ctx context.Context) (map[string]int64, error) {
	type statusCount struct {
		JobStatusCode string `gorm:"column:job_status_code"`
		Total         int64  `gorm:"column:total"`
	}
	var rows []statusCount
	if err := r.db.WithContext(ctx).Model(&domain.JobDO{}).
		Select("job_status_code, COUNT(*) AS total").
		Group("job_status_code").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("count jobs by status: %w", err)
	}
	result := make(map[string]int64, len(rows))
	for i := range rows {
		result[rows[i].JobStatusCode] = rows[i].Total
	}
	return result, nil
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

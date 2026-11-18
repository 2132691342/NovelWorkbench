package repo

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// AuditRepo 审计日志持久化仓储。
type AuditRepo struct {
	db *gorm.DB
}

// NewAuditRepo 构造审计日志仓储。
func NewAuditRepo(db *gorm.DB) *AuditRepo {
	return &AuditRepo{db: db}
}

// Create 插入审计日志记录。
func (r *AuditRepo) Create(ctx context.Context, do *domain.AuditLogDO) error {
	if err := r.db.WithContext(ctx).Create(do).Error; err != nil {
		return fmt.Errorf("create audit log: %w", err)
	}
	return nil
}

// ListPage 分页查询审计日志（按时间倒序），支持动作码模糊与用户过滤。
func (r *AuditRepo) ListPage(ctx context.Context, req *domain.AuditListREQ) ([]domain.AuditLogDO, int64, error) {
	query := r.db.WithContext(ctx).Model(&domain.AuditLogDO{})
	if req.ActionCode != "" {
		query = query.Where("action_code LIKE ?", "%"+req.ActionCode+"%")
	}
	if req.UserULID != "" {
		query = query.Where("user_ulid = ?", req.UserULID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}
	var dos []domain.AuditLogDO
	if err := query.Order("created_at DESC").
		Offset((req.PageNum - 1) * req.PageSize).Limit(req.PageSize).
		Find(&dos).Error; err != nil {
		return nil, 0, fmt.Errorf("list audit logs: %w", err)
	}
	return dos, total, nil
}

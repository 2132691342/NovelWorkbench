// 本文件实现审计日志的持久化仓储。

package repo

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// AuditRepo 审计日志持久化仓储。
type AuditRepo struct {
	db *gorm.DB // 事务入口
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

// GetByULID 按 ULID 查询审计详情，不存在时返回 (nil, nil)。
func (r *AuditRepo) GetByULID(ctx context.Context, auditULID string) (*domain.AuditLogDO, error) {
	var do domain.AuditLogDO
	err := r.db.WithContext(ctx).Where("audit_ulid = ?", auditULID).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get audit log: %w", err)
	}
	return &do, nil
}

// ListPage 分页查询审计日志（按时间倒序），支持级别/链路/目标/关键字/日期过滤。
func (r *AuditRepo) ListPage(ctx context.Context, req *domain.AuditListREQ) ([]domain.AuditLogDO, int64, error) {
	query := r.db.WithContext(ctx).Model(&domain.AuditLogDO{})
	if req.LogLevelCode != "" {
		query = query.Where("log_level_code = ?", req.LogLevelCode)
	}
	if req.TraceID != "" {
		query = query.Where("trace_id = ?", req.TraceID)
	}
	if req.TargetType != "" {
		query = query.Where("target_type = ?", req.TargetType)
	}
	if req.UserULID != "" {
		query = query.Where("user_ulid = ?", req.UserULID)
	}
	if req.ActionCode != "" {
		query = query.Where("action_code LIKE ?", "%"+req.ActionCode+"%")
	}
	if keyword := strings.TrimSpace(req.KeywordText); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("action_code ILIKE ? OR user_name ILIKE ? OR resp_message ILIKE ?", like, like, like)
	}
	if req.DateFrom != "" {
		query = query.Where("created_at >= ?", req.DateFrom+" 00:00:00")
	}
	if req.DateTo != "" {
		query = query.Where("created_at <= ?", req.DateTo+" 23:59:59")
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

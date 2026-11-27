// 本文件实现登录日志的持久化仓储。

package repo

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// LoginLogRepo 登录日志持久化仓储。
type LoginLogRepo struct {
	db *gorm.DB // 事务入口
}

// NewLoginLogRepo 构造登录日志仓储。
func NewLoginLogRepo(db *gorm.DB) *LoginLogRepo {
	return &LoginLogRepo{db: db}
}

// Create 插入登录日志。
func (r *LoginLogRepo) Create(ctx context.Context, do *domain.LoginLogDO) error {
	if err := r.db.WithContext(ctx).Create(do).Error; err != nil {
		return fmt.Errorf("create login log: %w", err)
	}
	return nil
}

// ListPage 分页查询登录日志（按时间倒序，支持账号/IP、结果与日期过滤）。
func (r *LoginLogRepo) ListPage(ctx context.Context, req *domain.LoginLogListREQ) ([]domain.LoginLogDO, int64, error) {
	query := r.db.WithContext(ctx).Model(&domain.LoginLogDO{})
	if req.KeywordText != "" {
		like := "%" + req.KeywordText + "%"
		query = query.Where("login_account_text LIKE ? OR ip_addr LIKE ?", like, like)
	}
	if req.LoginResultCode != "" {
		query = query.Where("login_result_code = ?", req.LoginResultCode)
	}
	if req.DateFrom != "" {
		query = query.Where("created_at >= ?", req.DateFrom+" 00:00:00")
	}
	if req.DateTo != "" {
		query = query.Where("created_at <= ?", req.DateTo+" 23:59:59")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count login logs: %w", err)
	}
	var dos []domain.LoginLogDO
	if err := query.Order("created_at DESC").
		Offset((req.PageNum - 1) * req.PageSize).Limit(req.PageSize).
		Find(&dos).Error; err != nil {
		return nil, 0, fmt.Errorf("list login logs: %w", err)
	}
	return dos, total, nil
}

// CountAll 统计登录日志总条数（留存摘要展示）。
func (r *LoginLogRepo) CountAll(ctx context.Context) (int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&domain.LoginLogDO{}).Count(&total).Error; err != nil {
		return 0, fmt.Errorf("count login logs: %w", err)
	}
	return total, nil
}

// DeleteBefore 删除指定时间之前的登录日志（与审计共用同一留存窗口）。
func (r *LoginLogRepo) DeleteBefore(ctx context.Context, cutoff time.Time) (int, error) {
	result := r.db.WithContext(ctx).Where("created_at < ?", cutoff).Delete(&domain.LoginLogDO{})
	if result.Error != nil {
		return 0, fmt.Errorf("purge login logs: %w", result.Error)
	}
	return int(result.RowsAffected), nil
}

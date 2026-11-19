// 本文件实现登录日志的持久化仓储。

package repo

import (
	"context"
	"fmt"

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

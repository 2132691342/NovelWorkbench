// 本文件实现用户账号的持久化仓储。

package repo

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// UserRepo 用户账号持久化仓储（SQL 只出现在本层，跨仓储事务由 service 通过 repo.WithTx 协调）。
type UserRepo struct {
	db *gorm.DB // 事务入口
}

// NewUserRepo 构造用户仓储。
func NewUserRepo(db *gorm.DB) *UserRepo {
	return &UserRepo{db: db}
}

// Create 插入用户记录。
func (r *UserRepo) Create(ctx context.Context, do *domain.UserDO) error {
	if err := r.db.WithContext(ctx).Create(do).Error; err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// GetByULID 按 ULID 查询用户，不存在时返回 (nil, nil)。
func (r *UserRepo) GetByULID(ctx context.Context, userULID string) (*domain.UserDO, error) {
	var do domain.UserDO
	err := r.db.WithContext(ctx).Where("user_ulid = ?", userULID).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get user by ulid: %w", err)
	}
	return &do, nil
}

// GetByAccount 按登录名或邮箱查询用户，不存在时返回 (nil, nil)。
func (r *UserRepo) GetByAccount(ctx context.Context, accountText string) (*domain.UserDO, error) {
	var do domain.UserDO
	err := r.db.WithContext(ctx).
		Where("user_name = ? OR user_email = ?", accountText, accountText).
		First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get user by account: %w", err)
	}
	return &do, nil
}

// GetByUserName 按登录名查询用户，不存在时返回 (nil, nil)。
func (r *UserRepo) GetByUserName(ctx context.Context, userName string) (*domain.UserDO, error) {
	var do domain.UserDO
	err := r.db.WithContext(ctx).Where("user_name = ?", userName).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get user by name: %w", err)
	}
	return &do, nil
}

// GetByEmail 按邮箱查询用户，不存在时返回 (nil, nil)。
func (r *UserRepo) GetByEmail(ctx context.Context, userEmail string) (*domain.UserDO, error) {
	var do domain.UserDO
	err := r.db.WithContext(ctx).Where("user_email = ?", userEmail).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &do, nil
}

// ListPage 分页查询用户（登录名/邮箱模糊 + 状态过滤），返回列表与总条数。
func (r *UserRepo) ListPage(ctx context.Context, req *domain.UserAdminListREQ) ([]domain.UserDO, int64, error) {
	query := r.db.WithContext(ctx).Model(&domain.UserDO{})
	if req.KeywordText != "" {
		like := "%" + req.KeywordText + "%"
		query = query.Where("user_name LIKE ? OR user_email LIKE ?", like, like)
	}
	if req.UserStatusCode != "" {
		query = query.Where("user_status_code = ?", req.UserStatusCode)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}
	var dos []domain.UserDO
	if err := query.Order("created_at DESC").
		Offset((req.PageNum - 1) * req.PageSize).Limit(req.PageSize).
		Find(&dos).Error; err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	return dos, total, nil
}

// Count 统计用户总数（启动期引导判断用）。
func (r *UserRepo) Count(ctx context.Context) (int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&domain.UserDO{}).Count(&total).Error; err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return total, nil
}

// Update 更新用户可变字段（map 更新避免零值跳过）。
func (r *UserRepo) Update(ctx context.Context, userULID string, updates map[string]any) error {
	if err := r.db.WithContext(ctx).Model(&domain.UserDO{}).
		Where("user_ulid = ?", userULID).Updates(updates).Error; err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}

// Delete 按 ULID 删除用户。
func (r *UserRepo) Delete(ctx context.Context, userULID string) error {
	if err := r.db.WithContext(ctx).Where("user_ulid = ?", userULID).Delete(&domain.UserDO{}).Error; err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

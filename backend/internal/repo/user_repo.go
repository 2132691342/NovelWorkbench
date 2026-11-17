package repo

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// UserRepo 用户账号持久化仓储（唯一允许出现 gorm 类型的层）。
type UserRepo struct {
	db *gorm.DB
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

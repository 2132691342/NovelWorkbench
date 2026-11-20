// 本文件实现系统提示词档案的持久化仓储。

package repo

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// PromptProfileRepo 系统提示词档案持久化仓储。
type PromptProfileRepo struct {
	db *gorm.DB // 事务入口
}

// NewPromptProfileRepo 构造档案仓储。
func NewPromptProfileRepo(db *gorm.DB) *PromptProfileRepo {
	return &PromptProfileRepo{db: db}
}

// Create 插入档案记录。
func (r *PromptProfileRepo) Create(ctx context.Context, do *domain.PromptProfileDO) error {
	if err := r.db.WithContext(ctx).Create(do).Error; err != nil {
		return fmt.Errorf("create prompt profile: %w", err)
	}
	return nil
}

// Update 按列名增量更新档案（map 更新防零值丢失）。
func (r *PromptProfileRepo) Update(ctx context.Context, promptProfileULID string, fields map[string]any) error {
	result := r.db.WithContext(ctx).Model(&domain.PromptProfileDO{}).
		Where("prompt_profile_ulid = ?", promptProfileULID).Updates(fields)
	if result.Error != nil {
		return fmt.Errorf("update prompt profile: %w", result.Error)
	}
	return nil
}

// Delete 删除档案记录。
func (r *PromptProfileRepo) Delete(ctx context.Context, promptProfileULID string) error {
	if err := r.db.WithContext(ctx).
		Where("prompt_profile_ulid = ?", promptProfileULID).Delete(&domain.PromptProfileDO{}).Error; err != nil {
		return fmt.Errorf("delete prompt profile: %w", err)
	}
	return nil
}

// GetByULID 按 ULID 查询档案，不存在时返回 (nil, nil)。
func (r *PromptProfileRepo) GetByULID(ctx context.Context, promptProfileULID string) (*domain.PromptProfileDO, error) {
	var do domain.PromptProfileDO
	err := r.db.WithContext(ctx).Where("prompt_profile_ulid = ?", promptProfileULID).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get prompt profile: %w", err)
	}
	return &do, nil
}

// GetByCode 按 (作用域, 项目, 编码) 查询档案，不存在时返回 (nil, nil)。
func (r *PromptProfileRepo) GetByCode(ctx context.Context, scopeCode, projectULID, promptProfileCode string) (*domain.PromptProfileDO, error) {
	var do domain.PromptProfileDO
	err := r.db.WithContext(ctx).
		Where("scope_code = ? AND project_ulid = ? AND prompt_profile_code = ?", scopeCode, projectULID, promptProfileCode).
		First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get prompt profile by code: %w", err)
	}
	return &do, nil
}

// List 按条件查询档案：项目不为空时返回「该项目档案 + 全局档案」。
func (r *PromptProfileRepo) List(ctx context.Context, req *domain.PromptProfileListREQ) ([]domain.PromptProfileDO, error) {
	query := r.db.WithContext(ctx).Model(&domain.PromptProfileDO{})
	if req.ProjectULID != "" {
		query = query.Where("(scope_code = ? AND project_ulid = ?) OR scope_code = ?",
			domain.PromptProfileScopeProject, req.ProjectULID, domain.PromptProfileScopeGlobal)
	} else {
		query = query.Where("scope_code = ?", domain.PromptProfileScopeGlobal)
	}
	if req.EnabledOnly {
		query = query.Where("enabled_flag = ?", true)
	}
	var dos []domain.PromptProfileDO
	if err := query.Order("scope_code DESC, sort_order ASC, created_at ASC").Find(&dos).Error; err != nil {
		return nil, fmt.Errorf("list prompt profiles: %w", err)
	}
	return dos, nil
}

// GetDefaultGlobal 取启用的全局档案中排序最前的一条（无则返回 nil）。
func (r *PromptProfileRepo) GetDefaultGlobal(ctx context.Context) (*domain.PromptProfileDO, error) {
	var do domain.PromptProfileDO
	err := r.db.WithContext(ctx).
		Where("scope_code = ? AND enabled_flag = ?", domain.PromptProfileScopeGlobal, true).
		Order("sort_order ASC, created_at ASC").First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get default prompt profile: %w", err)
	}
	return &do, nil
}

// CountByCode 统计同作用域同项目的编码占用数（唯一性校验用）。
func (r *PromptProfileRepo) CountByCode(ctx context.Context, scopeCode, projectULID, promptProfileCode, excludeULID string) (int64, error) {
	query := r.db.WithContext(ctx).Model(&domain.PromptProfileDO{}).
		Where("scope_code = ? AND project_ulid = ? AND prompt_profile_code = ?", scopeCode, projectULID, promptProfileCode)
	if strings.TrimSpace(excludeULID) != "" {
		query = query.Where("prompt_profile_ulid <> ?", excludeULID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count prompt profile by code: %w", err)
	}
	return count, nil
}

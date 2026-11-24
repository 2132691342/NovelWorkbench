// 本文件实现提示词仓储：按作用域/项目/种类/分类查询，以及内置资产的指纹查询。

package repo

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// PromptRepo 提示词持久化仓储。
type PromptRepo struct {
	db *gorm.DB // 事务入口
}

// NewPromptRepo 构造提示词仓储。
func NewPromptRepo(db *gorm.DB) *PromptRepo {
	return &PromptRepo{db: db}
}

// Create 插入提示词记录。
func (r *PromptRepo) Create(ctx context.Context, do *domain.PromptDO) error {
	if err := r.db.WithContext(ctx).Create(do).Error; err != nil {
		return fmt.Errorf("create prompt: %w", err)
	}
	return nil
}

// Update 按列名增量更新提示词（map 更新防零值丢失）。
func (r *PromptRepo) Update(ctx context.Context, promptULID string, fields map[string]any) error {
	result := r.db.WithContext(ctx).Model(&domain.PromptDO{}).
		Where("prompt_ulid = ?", promptULID).Updates(fields)
	if result.Error != nil {
		return fmt.Errorf("update prompt: %w", result.Error)
	}
	return nil
}

// Delete 删除提示词记录。
func (r *PromptRepo) Delete(ctx context.Context, promptULID string) error {
	if err := r.db.WithContext(ctx).
		Where("prompt_ulid = ?", promptULID).Delete(&domain.PromptDO{}).Error; err != nil {
		return fmt.Errorf("delete prompt: %w", err)
	}
	return nil
}

// GetByULID 按 ULID 查询提示词，不存在时返回 (nil, nil)。
func (r *PromptRepo) GetByULID(ctx context.Context, promptULID string) (*domain.PromptDO, error) {
	var do domain.PromptDO
	err := r.db.WithContext(ctx).Where("prompt_ulid = ?", promptULID).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get prompt: %w", err)
	}
	return &do, nil
}

// GetByCode 按 (作用域, 项目, 编码) 查询提示词，不存在时返回 (nil, nil)。
func (r *PromptRepo) GetByCode(ctx context.Context, scopeCode, projectULID, promptCode string) (*domain.PromptDO, error) {
	var do domain.PromptDO
	err := r.db.WithContext(ctx).
		Where("scope_code = ? AND project_ulid = ? AND prompt_code = ?", scopeCode, projectULID, promptCode).
		First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get prompt by code: %w", err)
	}
	return &do, nil
}

// List 按条件查询提示词：项目不为空时返回「该项目提示词 + 全局提示词」。
func (r *PromptRepo) List(ctx context.Context, req *domain.PromptListREQ) ([]domain.PromptDO, error) {
	query := r.db.WithContext(ctx).Model(&domain.PromptDO{})
	switch {
	case req.ProjectULID != "":
		query = query.Where("(scope_code = ? AND project_ulid = ?) OR scope_code = ?",
			domain.PromptScopeProject, req.ProjectULID, domain.PromptScopeGlobal)
	case req.ScopeCode != "":
		query = query.Where("scope_code = ?", req.ScopeCode)
	}
	if req.PromptKindCode != "" {
		query = query.Where("prompt_kind_code = ?", req.PromptKindCode)
	}
	if req.CategoryCode != "" {
		query = query.Where("category_code = ?", req.CategoryCode)
	}
	if req.EnabledOnly {
		query = query.Where("enabled_flag = ?", true)
	}
	if keyword := strings.TrimSpace(req.KeywordText); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("prompt_name ILIKE ? OR prompt_code ILIKE ?", like, like)
	}
	var dos []domain.PromptDO
	if err := query.Order("prompt_kind_code DESC, sort_order ASC, created_at ASC").Find(&dos).Error; err != nil {
		return nil, fmt.Errorf("list prompts: %w", err)
	}
	return dos, nil
}

// ListBuiltin 查询全部内置提示词（播种清理校验用）。
func (r *PromptRepo) ListBuiltin(ctx context.Context) ([]domain.PromptDO, error) {
	var dos []domain.PromptDO
	if err := r.db.WithContext(ctx).
		Where("scope_code = ? AND builtin_flag = ?", domain.PromptScopeGlobal, true).
		Find(&dos).Error; err != nil {
		return nil, fmt.Errorf("list builtin prompts: %w", err)
	}
	return dos, nil
}

// GetDefaultSystem 取启用的全局身份提示词中排序最前的一条（无则返回 nil）。
func (r *PromptRepo) GetDefaultSystem(ctx context.Context) (*domain.PromptDO, error) {
	var do domain.PromptDO
	err := r.db.WithContext(ctx).
		Where("scope_code = ? AND prompt_kind_code = ? AND enabled_flag = ?",
			domain.PromptScopeGlobal, domain.PromptKindSystem, true).
		Order("sort_order ASC, created_at ASC").First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get default system prompt: %w", err)
	}
	return &do, nil
}

// CountByCode 统计同作用域同项目的编码占用数（唯一性校验用）。
func (r *PromptRepo) CountByCode(ctx context.Context, scopeCode, projectULID, promptCode, excludeULID string) (int64, error) {
	query := r.db.WithContext(ctx).Model(&domain.PromptDO{}).
		Where("scope_code = ? AND project_ulid = ? AND prompt_code = ?", scopeCode, projectULID, promptCode)
	if strings.TrimSpace(excludeULID) != "" {
		query = query.Where("prompt_ulid <> ?", excludeULID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count prompt by code: %w", err)
	}
	return count, nil
}

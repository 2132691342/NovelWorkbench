package repo

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// LoreRepo 世界观设定持久化仓储。
type LoreRepo struct {
	db *gorm.DB
}

// NewLoreRepo 构造设定仓储。
func NewLoreRepo(db *gorm.DB) *LoreRepo {
	return &LoreRepo{db: db}
}

// Create 插入设定记录。
func (r *LoreRepo) Create(ctx context.Context, do *domain.LoreDO) error {
	if err := r.db.WithContext(ctx).Create(do).Error; err != nil {
		return fmt.Errorf("create lore: %w", err)
	}
	return nil
}

// Update 按列名增量更新设定（map 更新防零值丢失）。
func (r *LoreRepo) Update(ctx context.Context, loreULID string, fields map[string]any) error {
	result := r.db.WithContext(ctx).Model(&domain.LoreDO{}).
		Where("lore_ulid = ?", loreULID).Updates(fields)
	if result.Error != nil {
		return fmt.Errorf("update lore: %w", result.Error)
	}
	return nil
}

// Delete 删除设定记录。
func (r *LoreRepo) Delete(ctx context.Context, loreULID string) error {
	if err := r.db.WithContext(ctx).
		Where("lore_ulid = ?", loreULID).Delete(&domain.LoreDO{}).Error; err != nil {
		return fmt.Errorf("delete lore: %w", err)
	}
	return nil
}

// DeleteByProject 删除项目下全部设定条目。
func (r *LoreRepo) DeleteByProject(ctx context.Context, projectULID string) error {
	if err := r.db.WithContext(ctx).
		Where("project_ulid = ?", projectULID).Delete(&domain.LoreDO{}).Error; err != nil {
		return fmt.Errorf("delete lores by project: %w", err)
	}
	return nil
}

// GetByULID 按 ULID 查询设定，不存在时返回 (nil, nil)。
func (r *LoreRepo) GetByULID(ctx context.Context, loreULID string) (*domain.LoreDO, error) {
	var do domain.LoreDO
	err := r.db.WithContext(ctx).Where("lore_ulid = ?", loreULID).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get lore: %w", err)
	}
	return &do, nil
}

// ListByProject 按项目查询设定（类型过滤 + 名称/内容模糊，排序号升序）。
func (r *LoreRepo) ListByProject(ctx context.Context, projectULID, loreTypeCode, keywordText string) ([]domain.LoreDO, error) {
	query := r.db.WithContext(ctx).Model(&domain.LoreDO{}).Where("project_ulid = ?", projectULID)
	if loreTypeCode != "" {
		query = query.Where("lore_type_code = ?", loreTypeCode)
	}
	if keywordText != "" {
		pattern := "%" + keywordText + "%"
		query = query.Where("lore_name ILIKE ? OR content_text ILIKE ?", pattern, pattern)
	}
	var dos []domain.LoreDO
	if err := query.Order("sort_order ASC, created_at ASC").Find(&dos).Error; err != nil {
		return nil, fmt.Errorf("list lores: %w", err)
	}
	return dos, nil
}

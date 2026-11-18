package repo

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// ModelProviderRepo 模型供应商持久化仓储。
type ModelProviderRepo struct {
	db *gorm.DB
}

// NewModelProviderRepo 构造模型供应商仓储。
func NewModelProviderRepo(db *gorm.DB) *ModelProviderRepo {
	return &ModelProviderRepo{db: db}
}

// Create 插入供应商记录。
func (r *ModelProviderRepo) Create(ctx context.Context, do *domain.ModelProviderDO) error {
	if err := r.db.WithContext(ctx).Create(do).Error; err != nil {
		return fmt.Errorf("create model provider: %w", err)
	}
	return nil
}

// Update 按列名增量更新供应商（map 更新防零值丢失）。
func (r *ModelProviderRepo) Update(ctx context.Context, providerULID string, fields map[string]any) error {
	result := r.db.WithContext(ctx).Model(&domain.ModelProviderDO{}).
		Where("provider_ulid = ?", providerULID).Updates(fields)
	if result.Error != nil {
		return fmt.Errorf("update model provider: %w", result.Error)
	}
	return nil
}

// Delete 删除供应商记录。
func (r *ModelProviderRepo) Delete(ctx context.Context, providerULID string) error {
	if err := r.db.WithContext(ctx).
		Where("provider_ulid = ?", providerULID).Delete(&domain.ModelProviderDO{}).Error; err != nil {
		return fmt.Errorf("delete model provider: %w", err)
	}
	return nil
}

// GetByULID 按 ULID 查询供应商，不存在时返回 (nil, nil)。
func (r *ModelProviderRepo) GetByULID(ctx context.Context, providerULID string) (*domain.ModelProviderDO, error) {
	var do domain.ModelProviderDO
	err := r.db.WithContext(ctx).Where("provider_ulid = ?", providerULID).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get model provider: %w", err)
	}
	return &do, nil
}

// List 查询全部供应商（默认优先、排序号升序）。
func (r *ModelProviderRepo) List(ctx context.Context) ([]domain.ModelProviderDO, error) {
	var dos []domain.ModelProviderDO
	if err := r.db.WithContext(ctx).
		Order("default_flag DESC, sort_order ASC, created_at ASC").Find(&dos).Error; err != nil {
		return nil, fmt.Errorf("list model providers: %w", err)
	}
	return dos, nil
}

// Count 统计供应商数量（启动播种判空用）。
func (r *ModelProviderRepo) Count(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&domain.ModelProviderDO{}).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count model providers: %w", err)
	}
	return count, nil
}

// ClearDefault 清除全部默认标记（保证默认唯一）。
func (r *ModelProviderRepo) ClearDefault(ctx context.Context) error {
	result := r.db.WithContext(ctx).Model(&domain.ModelProviderDO{}).
		Where("default_flag = ?", true).Update("default_flag", false)
	if result.Error != nil {
		return fmt.Errorf("clear default provider: %w", result.Error)
	}
	return nil
}

// FindDefault 查询默认供应商（启用中），不存在时返回 (nil, nil)。
func (r *ModelProviderRepo) FindDefault(ctx context.Context) (*domain.ModelProviderDO, error) {
	var do domain.ModelProviderDO
	err := r.db.WithContext(ctx).
		Where("enabled_flag = ? AND default_flag = ?", true, true).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("find default provider: %w", err)
	}
	return &do, nil
}

// FindEmbeddingProvider 查询首个可提供向量的启用供应商（默认优先）。
func (r *ModelProviderRepo) FindEmbeddingProvider(ctx context.Context) (*domain.ModelProviderDO, error) {
	var do domain.ModelProviderDO
	err := r.db.WithContext(ctx).
		Where("enabled_flag = ? AND embedding_model_name <> ''", true).
		Order("default_flag DESC, sort_order ASC, created_at ASC").First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("find embedding provider: %w", err)
	}
	return &do, nil
}

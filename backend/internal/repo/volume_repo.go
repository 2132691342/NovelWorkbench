// 本文件实现卷的持久化仓储。

package repo

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// VolumeRepo 卷持久化仓储。
type VolumeRepo struct {
	db *gorm.DB // 事务入口
}

// NewVolumeRepo 构造卷仓储。
func NewVolumeRepo(db *gorm.DB) *VolumeRepo {
	return &VolumeRepo{db: db}
}

// Create 插入卷记录。
func (r *VolumeRepo) Create(ctx context.Context, do *domain.VolumeDO) error {
	if err := r.db.WithContext(ctx).Create(do).Error; err != nil {
		return fmt.Errorf("create volume: %w", err)
	}
	return nil
}

// Update 按列名增量更新卷（map 更新防零值丢失）。
func (r *VolumeRepo) Update(ctx context.Context, volumeULID string, fields map[string]any) error {
	result := r.db.WithContext(ctx).Model(&domain.VolumeDO{}).
		Where("volume_ulid = ?", volumeULID).Updates(fields)
	if result.Error != nil {
		return fmt.Errorf("update volume: %w", result.Error)
	}
	return nil
}

// Delete 删除卷记录。
func (r *VolumeRepo) Delete(ctx context.Context, volumeULID string) error {
	if err := r.db.WithContext(ctx).
		Where("volume_ulid = ?", volumeULID).Delete(&domain.VolumeDO{}).Error; err != nil {
		return fmt.Errorf("delete volume: %w", err)
	}
	return nil
}

// GetByULID 按 ULID 查询卷，不存在时返回 (nil, nil)。
func (r *VolumeRepo) GetByULID(ctx context.Context, volumeULID string) (*domain.VolumeDO, error) {
	var do domain.VolumeDO
	err := r.db.WithContext(ctx).Where("volume_ulid = ?", volumeULID).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get volume: %w", err)
	}
	return &do, nil
}

// ListByProject 按项目查询卷列表（排序号升序）。
func (r *VolumeRepo) ListByProject(ctx context.Context, projectULID string) ([]domain.VolumeDO, error) {
	var dos []domain.VolumeDO
	err := r.db.WithContext(ctx).Where("project_ulid = ?", projectULID).
		Order("volume_order ASC").Find(&dos).Error
	if err != nil {
		return nil, fmt.Errorf("list volumes: %w", err)
	}
	return dos, nil
}

// MaxOrder 查询项目内最大卷排序号（无卷时返回 0）。
func (r *VolumeRepo) MaxOrder(ctx context.Context, projectULID string) (int, error) {
	var maxOrder int
	err := r.db.WithContext(ctx).Model(&domain.VolumeDO{}).
		Where("project_ulid = ?", projectULID).
		Select("COALESCE(MAX(volume_order), 0)").Scan(&maxOrder).Error
	if err != nil {
		return 0, fmt.Errorf("max volume order: %w", err)
	}
	return maxOrder, nil
}

// DeleteByProject 删除项目下全部卷。
func (r *VolumeRepo) DeleteByProject(ctx context.Context, projectULID string) error {
	if err := r.db.WithContext(ctx).
		Where("project_ulid = ?", projectULID).Delete(&domain.VolumeDO{}).Error; err != nil {
		return fmt.Errorf("delete volumes by project: %w", err)
	}
	return nil
}

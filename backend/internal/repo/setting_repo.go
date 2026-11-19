// 本文件实现应用配置（key-value）的持久化仓储。

package repo

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// SettingRepo 应用配置（key-value）持久化仓储。
type SettingRepo struct {
	db *gorm.DB // 事务入口
}

// NewSettingRepo 构造应用配置仓储。
func NewSettingRepo(db *gorm.DB) *SettingRepo {
	return &SettingRepo{db: db}
}

// Create 插入配置记录。
func (r *SettingRepo) Create(ctx context.Context, do *domain.AppSettingDO) error {
	if err := r.db.WithContext(ctx).Create(do).Error; err != nil {
		return fmt.Errorf("create setting: %w", err)
	}
	return nil
}

// GetByKey 按配置键查询，不存在时返回 (nil, nil)。
func (r *SettingRepo) GetByKey(ctx context.Context, settingKey string) (*domain.AppSettingDO, error) {
	var do domain.AppSettingDO
	err := r.db.WithContext(ctx).Where("setting_key = ?", settingKey).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get setting: %w", err)
	}
	return &do, nil
}

// List 查询全部配置（按配置键升序）。
func (r *SettingRepo) List(ctx context.Context) ([]domain.AppSettingDO, error) {
	var dos []domain.AppSettingDO
	if err := r.db.WithContext(ctx).Model(&domain.AppSettingDO{}).
		Order("setting_key ASC").Find(&dos).Error; err != nil {
		return nil, fmt.Errorf("list settings: %w", err)
	}
	return dos, nil
}

// UpdateValue 更新配置值。
func (r *SettingRepo) UpdateValue(ctx context.Context, settingKey, settingValue string) error {
	result := r.db.WithContext(ctx).Model(&domain.AppSettingDO{}).
		Where("setting_key = ?", settingKey).
		Updates(map[string]any{"setting_value": settingValue})
	if result.Error != nil {
		return fmt.Errorf("update setting: %w", result.Error)
	}
	return nil
}

// UpdateDescription 更新配置说明（内置配置文案升级时使用）。
func (r *SettingRepo) UpdateDescription(ctx context.Context, settingKey, settingDesc string) error {
	if err := r.db.WithContext(ctx).Model(&domain.AppSettingDO{}).
		Where("setting_key = ?", settingKey).Update("setting_desc", settingDesc).Error; err != nil {
		return fmt.Errorf("update setting description: %w", err)
	}
	return nil
}

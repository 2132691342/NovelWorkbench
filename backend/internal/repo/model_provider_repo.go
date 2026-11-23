// 本文件实现模型生态的持久化仓储：供应商、模型目录与模型角色授权。

package repo

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
)

// ModelProviderRepo 模型生态持久化仓储（供应商 + 模型目录 + 角色授权）。
type ModelProviderRepo struct {
	db *gorm.DB // 事务入口
}

// NewModelProviderRepo 构造模型生态仓储。
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

// Delete 删除供应商记录（调用方需先确认其下无模型）。
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

// List 查询全部供应商（排序号升序）。
func (r *ModelProviderRepo) List(ctx context.Context) ([]domain.ModelProviderDO, error) {
	var dos []domain.ModelProviderDO
	if err := r.db.WithContext(ctx).
		Order("sort_order ASC, created_at ASC").Find(&dos).Error; err != nil {
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

// CreateModel 插入模型目录记录。
func (r *ModelProviderRepo) CreateModel(ctx context.Context, do *domain.ModelCatalogDO) error {
	if err := r.db.WithContext(ctx).Create(do).Error; err != nil {
		return fmt.Errorf("create model catalog: %w", err)
	}
	return nil
}

// UpdateModel 按列名增量更新模型目录。
func (r *ModelProviderRepo) UpdateModel(ctx context.Context, modelULID string, fields map[string]any) error {
	result := r.db.WithContext(ctx).Model(&domain.ModelCatalogDO{}).
		Where("model_ulid = ?", modelULID).Updates(fields)
	if result.Error != nil {
		return fmt.Errorf("update model catalog: %w", result.Error)
	}
	return nil
}

// DeleteModel 删除模型目录记录（授权行由调用方在同一事务内清理）。
func (r *ModelProviderRepo) DeleteModel(ctx context.Context, modelULID string) error {
	if err := r.db.WithContext(ctx).
		Where("model_ulid = ?", modelULID).Delete(&domain.ModelCatalogDO{}).Error; err != nil {
		return fmt.Errorf("delete model catalog: %w", err)
	}
	return nil
}

// GetModelByULID 按 ULID 查询模型，不存在时返回 (nil, nil)。
func (r *ModelProviderRepo) GetModelByULID(ctx context.Context, modelULID string) (*domain.ModelCatalogDO, error) {
	var do domain.ModelCatalogDO
	err := r.db.WithContext(ctx).Where("model_ulid = ?", modelULID).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get model catalog: %w", err)
	}
	return &do, nil
}

// ListModels 查询模型目录（providerULID 为空查全部；默认模型优先）。
func (r *ModelProviderRepo) ListModels(ctx context.Context, providerULID string) ([]domain.ModelCatalogDO, error) {
	var dos []domain.ModelCatalogDO
	query := r.db.WithContext(ctx).Model(&domain.ModelCatalogDO{})
	if providerULID != "" {
		query = query.Where("provider_ulid = ?", providerULID)
	}
	if err := query.Order("default_flag DESC, sort_order ASC, created_at ASC").Find(&dos).Error; err != nil {
		return nil, fmt.Errorf("list model catalogs: %w", err)
	}
	return dos, nil
}

// LegacyProviderModelRow 旧版「供应商自带一个模型」的数据行（迁移读取用）。
type LegacyProviderModelRow struct {
	ProviderULID       string  `gorm:"column:provider_ulid"`         // 供应商 ULID
	ChatModelName      string  `gorm:"column:chat_model_name"`       // 旧版模型名
	Temperature        float64 `gorm:"column:temperature"`           // 旧版采样温度
	MaxTokens          int     `gorm:"column:max_tokens"`            // 旧版输出上限
	DefaultFlag        bool    `gorm:"column:default_flag"`          // 旧版默认标记
	LastTestStatusCode string  `gorm:"column:last_test_status_code"` // 旧版测试状态
}

// ListLegacyProviderModels 读取旧版供应商表里仍带模型名的行（列被新结构忽略后仍存在于库中）。
func (r *ModelProviderRepo) ListLegacyProviderModels(ctx context.Context) ([]LegacyProviderModelRow, error) {
	var rows []LegacyProviderModelRow
	err := r.db.WithContext(ctx).Raw(
		`SELECT provider_ulid, chat_model_name, temperature, max_tokens, default_flag, last_test_status_code
		 FROM model_providers WHERE chat_model_name IS NOT NULL AND chat_model_name <> ''`).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list legacy provider models: %w", err)
	}
	return rows, nil
}

// CountModels 统计模型目录总数（旧数据迁移判空用）。
func (r *ModelProviderRepo) CountModels(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&domain.ModelCatalogDO{}).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count model catalogs: %w", err)
	}
	return count, nil
}

// CountModelsByProvider 统计供应商下的模型数量。
func (r *ModelProviderRepo) CountModelsByProvider(ctx context.Context, providerULID string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&domain.ModelCatalogDO{}).
		Where("provider_ulid = ?", providerULID).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count model catalogs: %w", err)
	}
	return count, nil
}

// ClearDefaultModel 清除全部模型默认标记（保证默认唯一）。
func (r *ModelProviderRepo) ClearDefaultModel(ctx context.Context) error {
	result := r.db.WithContext(ctx).Model(&domain.ModelCatalogDO{}).
		Where("default_flag = ?", true).Update("default_flag", false)
	if result.Error != nil {
		return fmt.Errorf("clear default model: %w", result.Error)
	}
	return nil
}

// FindDefaultModel 查询系统默认模型（模型启用），不存在时返回 (nil, nil)。
func (r *ModelProviderRepo) FindDefaultModel(ctx context.Context) (*domain.ModelCatalogDO, error) {
	var do domain.ModelCatalogDO
	err := r.db.WithContext(ctx).
		Where("enabled_flag = ? AND default_flag = ?", true, true).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("find default model: %w", err)
	}
	return &do, nil
}

// ReplaceGrants 整体覆盖模型的角色授权（同一事务内先删后插）。
func (r *ModelProviderRepo) ReplaceGrants(ctx context.Context, modelULID string, roleULIDList []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("model_ulid = ?", modelULID).Delete(&domain.ModelRoleGrantDO{}).Error; err != nil {
			return fmt.Errorf("clear model grants: %w", err)
		}
		if len(roleULIDList) == 0 {
			return nil
		}
		grantList := make([]domain.ModelRoleGrantDO, 0, len(roleULIDList))
		for _, roleULID := range roleULIDList {
			grantList = append(grantList, domain.ModelRoleGrantDO{
				GrantULID: pkg.NewULID(domain.IDPrefixModelGrant),
				ModelULID: modelULID,
				RoleULID:  roleULID,
			})
		}
		if err := tx.Create(&grantList).Error; err != nil {
			return fmt.Errorf("create model grants: %w", err)
		}
		return nil
	})
}

// DeleteGrantsByModel 删除模型的全部授权（模型删除时级联）。
func (r *ModelProviderRepo) DeleteGrantsByModel(ctx context.Context, modelULID string) error {
	if err := r.db.WithContext(ctx).
		Where("model_ulid = ?", modelULID).Delete(&domain.ModelRoleGrantDO{}).Error; err != nil {
		return fmt.Errorf("delete model grants: %w", err)
	}
	return nil
}

// ListGrantsByModelULIDList 批量查询模型的授权角色（模型列表页一次查询）。
func (r *ModelProviderRepo) ListGrantsByModelULIDList(ctx context.Context, modelULIDList []string) (map[string][]string, error) {
	grantMap := make(map[string][]string, len(modelULIDList))
	if len(modelULIDList) == 0 {
		return grantMap, nil
	}
	var dos []domain.ModelRoleGrantDO
	if err := r.db.WithContext(ctx).
		Where("model_ulid IN ?", modelULIDList).Find(&dos).Error; err != nil {
		return nil, fmt.Errorf("list model grants: %w", err)
	}
	for _, do := range dos {
		grantMap[do.ModelULID] = append(grantMap[do.ModelULID], do.RoleULID)
	}
	return grantMap, nil
}

// ListGrantedModelULIDList 查询某角色集合可见的模型 ULID（授权并集）。
func (r *ModelProviderRepo) ListGrantedModelULIDList(ctx context.Context, roleULIDList []string) ([]string, error) {
	if len(roleULIDList) == 0 {
		return nil, nil
	}
	var modelULIDList []string
	if err := r.db.WithContext(ctx).Model(&domain.ModelRoleGrantDO{}).
		Where("role_ulid IN ?", roleULIDList).
		Distinct().Pluck("model_ulid", &modelULIDList).Error; err != nil {
		return nil, fmt.Errorf("list granted models: %w", err)
	}
	return modelULIDList, nil
}

// ReplaceUserGrants 整体覆盖模型的用户直授（同一事务内先删后插）。
func (r *ModelProviderRepo) ReplaceUserGrants(ctx context.Context, modelULID string, userULIDList []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("model_ulid = ?", modelULID).Delete(&domain.ModelUserGrantDO{}).Error; err != nil {
			return fmt.Errorf("clear model user grants: %w", err)
		}
		if len(userULIDList) == 0 {
			return nil
		}
		grantList := make([]domain.ModelUserGrantDO, 0, len(userULIDList))
		for _, userULID := range userULIDList {
			grantList = append(grantList, domain.ModelUserGrantDO{
				GrantULID: pkg.NewULID(domain.IDPrefixModelUserGrant),
				ModelULID: modelULID,
				UserULID:  userULID,
			})
		}
		if err := tx.Create(&grantList).Error; err != nil {
			return fmt.Errorf("create model user grants: %w", err)
		}
		return nil
	})
}

// DeleteUserGrantsByUser 删除某用户的全部模型直授（删用户时级联）。
func (r *ModelProviderRepo) DeleteUserGrantsByUser(ctx context.Context, userULID string) error {
	if err := r.db.WithContext(ctx).
		Where("user_ulid = ?", userULID).Delete(&domain.ModelUserGrantDO{}).Error; err != nil {
		return fmt.Errorf("delete model user grants: %w", err)
	}
	return nil
}

// DeleteRoleGrantsByRole 删除某角色关联的全部模型授权（删角色时级联）。
func (r *ModelProviderRepo) DeleteRoleGrantsByRole(ctx context.Context, roleULID string) error {
	if err := r.db.WithContext(ctx).
		Where("role_ulid = ?", roleULID).Delete(&domain.ModelRoleGrantDO{}).Error; err != nil {
		return fmt.Errorf("delete model role grants: %w", err)
	}
	return nil
}

// ListUserGrantsByModelULIDList 批量查询模型的用户直授（模型列表一次查询）。
func (r *ModelProviderRepo) ListUserGrantsByModelULIDList(ctx context.Context, modelULIDList []string) (map[string][]string, error) {
	grantMap := make(map[string][]string, len(modelULIDList))
	if len(modelULIDList) == 0 {
		return grantMap, nil
	}
	var dos []domain.ModelUserGrantDO
	if err := r.db.WithContext(ctx).
		Where("model_ulid IN ?", modelULIDList).Find(&dos).Error; err != nil {
		return nil, fmt.Errorf("list model user grants: %w", err)
	}
	for _, do := range dos {
		grantMap[do.ModelULID] = append(grantMap[do.ModelULID], do.UserULID)
	}
	return grantMap, nil
}

// ListUserGrantedModelULIDList 查询某用户直授可见的模型 ULID。
func (r *ModelProviderRepo) ListUserGrantedModelULIDList(ctx context.Context, userULID string) ([]string, error) {
	if userULID == "" {
		return nil, nil
	}
	var modelULIDList []string
	if err := r.db.WithContext(ctx).Model(&domain.ModelUserGrantDO{}).
		Where("user_ulid = ?", userULID).
		Pluck("model_ulid", &modelULIDList).Error; err != nil {
		return nil, fmt.Errorf("list user granted models: %w", err)
	}
	return modelULIDList, nil
}

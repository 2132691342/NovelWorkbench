// 本文件实现模型生态去重所需的持久化操作：供应商合并、授权迁移与用量引用查询。
// 与 model_provider_repo.go 同一仓储类型，拆文件只为单文件行数可控。

package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
)

// ListUsageKeys 读取用量统计里出现过的 (供应商, 模型名) 组合（去重时判断模型是否被历史引用）。
func (r *ModelProviderRepo) ListUsageKeys(ctx context.Context) (map[string]map[string]bool, error) {
	var rows []struct {
		ProviderULID string `gorm:"column:provider_ulid"` // 供应商 ULID
		ModelName    string `gorm:"column:model_name"`    // 模型名快照
	}
	if err := r.db.WithContext(ctx).Model(&domain.ModelUsageStatDO{}).
		Select("provider_ulid, model_name").Group("provider_ulid, model_name").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list model usage keys: %w", err)
	}
	keyMap := make(map[string]map[string]bool, len(rows))
	for _, row := range rows {
		if keyMap[row.ProviderULID] == nil {
			keyMap[row.ProviderULID] = make(map[string]bool, 2)
		}
		keyMap[row.ProviderULID][row.ModelName] = true
	}
	return keyMap, nil
}

// ReassignRoleGrants 把指定角色的授权行从旧模型迁到保留模型（键已存在则跳过）。
func (r *ModelProviderRepo) ReassignRoleGrants(ctx context.Context, fromModelULID, toModelULID string,
	roleULIDList []string) (int, error) {
	if len(roleULIDList) == 0 {
		return 0, nil
	}
	existing, err := r.existingRoleGrants(ctx, toModelULID)
	if err != nil {
		return 0, err
	}
	createList := make([]domain.ModelRoleGrantDO, 0, len(roleULIDList))
	for _, roleULID := range roleULIDList {
		if existing[roleULID] {
			continue
		}
		existing[roleULID] = true
		createList = append(createList, domain.ModelRoleGrantDO{
			GrantULID: pkg.NewULID(domain.IDPrefixModelGrant),
			ModelULID: toModelULID,
			RoleULID:  roleULID,
		})
	}
	if len(createList) == 0 {
		return 0, nil
	}
	if err := r.db.WithContext(ctx).Create(&createList).Error; err != nil {
		return 0, fmt.Errorf("reassign model role grants: %w", err)
	}
	return len(createList), nil
}

// ReassignUserGrants 把指定用户的直授行从旧模型迁到保留模型（键已存在则跳过）。
func (r *ModelProviderRepo) ReassignUserGrants(ctx context.Context, fromModelULID, toModelULID string,
	userULIDList []string) (int, error) {
	if len(userULIDList) == 0 {
		return 0, nil
	}
	existing, err := r.existingUserGrants(ctx, toModelULID)
	if err != nil {
		return 0, err
	}
	createList := make([]domain.ModelUserGrantDO, 0, len(userULIDList))
	for _, userULID := range userULIDList {
		if existing[userULID] {
			continue
		}
		existing[userULID] = true
		createList = append(createList, domain.ModelUserGrantDO{
			GrantULID: pkg.NewULID(domain.IDPrefixModelUserGrant),
			ModelULID: toModelULID,
			UserULID:  userULID,
		})
	}
	if len(createList) == 0 {
		return 0, nil
	}
	if err := r.db.WithContext(ctx).Create(&createList).Error; err != nil {
		return 0, fmt.Errorf("reassign model user grants: %w", err)
	}
	return len(createList), nil
}

// existingRoleGrants 读取模型已持有的角色授权键集合。
func (r *ModelProviderRepo) existingRoleGrants(ctx context.Context, modelULID string) (map[string]bool, error) {
	var roleULIDList []string
	if err := r.db.WithContext(ctx).Model(&domain.ModelRoleGrantDO{}).
		Where("model_ulid = ?", modelULID).Pluck("role_ulid", &roleULIDList).Error; err != nil {
		return nil, fmt.Errorf("list model role grants: %w", err)
	}
	keySet := make(map[string]bool, len(roleULIDList))
	for _, roleULID := range roleULIDList {
		keySet[roleULID] = true
	}
	return keySet, nil
}

// existingUserGrants 读取模型已持有的用户直授键集合。
func (r *ModelProviderRepo) existingUserGrants(ctx context.Context, modelULID string) (map[string]bool, error) {
	var userULIDList []string
	if err := r.db.WithContext(ctx).Model(&domain.ModelUserGrantDO{}).
		Where("model_ulid = ?", modelULID).Pluck("user_ulid", &userULIDList).Error; err != nil {
		return nil, fmt.Errorf("list model user grants: %w", err)
	}
	keySet := make(map[string]bool, len(userULIDList))
	for _, userULID := range userULIDList {
		keySet[userULID] = true
	}
	return keySet, nil
}

// DeleteUserGrantsByModel 删除模型的全部用户直授（模型删除时级联）。
func (r *ModelProviderRepo) DeleteUserGrantsByModel(ctx context.Context, modelULID string) error {
	if err := r.db.WithContext(ctx).
		Where("model_ulid = ?", modelULID).Delete(&domain.ModelUserGrantDO{}).Error; err != nil {
		return fmt.Errorf("delete model user grants: %w", err)
	}
	return nil
}

// DedupeProviderRow 去重所需的供应商投影行（只读老库一定存在的列，缺新列时也能跑）。
type DedupeProviderRow struct {
	ProviderULID string    `gorm:"column:provider_ulid"` // 供应商 ULID
	ProviderName string    `gorm:"column:provider_name"` // 供应商名称
	CreatedBy    string    `gorm:"column:created_by"`    // 创建人（空=系统写入）
	UpdatedAt    time.Time `gorm:"column:updated_at"`    // 更新时间
}

// DedupeCatalogRow 去重所需的模型目录投影行（同上，避开可能尚未补齐的新列）。
type DedupeCatalogRow struct {
	ModelULID    string    `gorm:"column:model_ulid"`    // 模型 ULID
	ProviderULID string    `gorm:"column:provider_ulid"` // 所属供应商
	ModelName    string    `gorm:"column:model_name"`    // 模型名
	DefaultFlag  bool      `gorm:"column:default_flag"`  // 默认标记
	CreatedBy    string    `gorm:"column:created_by"`    // 创建人（空=系统写入）
	UpdatedAt    time.Time `gorm:"column:updated_at"`    // 更新时间
}

// ListDedupeProviders 读取去重所需的供应商投影（按创建时间升序，便于稳定选择保留行）。
func (r *ModelProviderRepo) ListDedupeProviders(ctx context.Context) ([]DedupeProviderRow, error) {
	var rows []DedupeProviderRow
	if err := r.db.WithContext(ctx).Model(&domain.ModelProviderDO{}).
		Select("provider_ulid, provider_name, created_by, updated_at").
		Order("created_at ASC").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list dedupe providers: %w", err)
	}
	return rows, nil
}

// ListDedupeCatalogs 读取去重所需的模型目录投影。
func (r *ModelProviderRepo) ListDedupeCatalogs(ctx context.Context) ([]DedupeCatalogRow, error) {
	var rows []DedupeCatalogRow
	if err := r.db.WithContext(ctx).Model(&domain.ModelCatalogDO{}).
		Select("model_ulid, provider_ulid, model_name, default_flag, created_by, updated_at").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list dedupe catalogs: %w", err)
	}
	return rows, nil
}

// ListProviderULIDsWithLegacyModel 读取仍带旧版 chat_model_name 的供应商 ULID（空供应商清理时须豁免）。
func (r *ModelProviderRepo) ListProviderULIDsWithLegacyModel(ctx context.Context) (map[string]bool, error) {
	keySet := make(map[string]bool, 8)
	hasColumn, err := r.HasLegacyModelColumn(ctx)
	if err != nil {
		return nil, err
	}
	if !hasColumn {
		return keySet, nil
	}
	var rows []struct {
		ProviderULID string `gorm:"column:provider_ulid"` // 供应商 ULID
	}
	if err := r.db.WithContext(ctx).Raw(
		`SELECT provider_ulid FROM model_providers WHERE chat_model_name IS NOT NULL AND chat_model_name <> ''`).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list providers with legacy model: %w", err)
	}
	for _, row := range rows {
		keySet[row.ProviderULID] = true
	}
	return keySet, nil
}

// ReassignCatalog 把单条模型改挂到另一供应商（唯一索引已存在时逐行改挂，避免整批冲突）。
func (r *ModelProviderRepo) ReassignCatalog(ctx context.Context, modelULID, toProviderULID string) error {
	result := r.db.WithContext(ctx).Model(&domain.ModelCatalogDO{}).
		Where("model_ulid = ?", modelULID).Update("provider_ulid", toProviderULID)
	if result.Error != nil {
		return fmt.Errorf("reassign model catalog: %w", result.Error)
	}
	return nil
}

// ClearLegacyModelName 清空旧版 chat_model_name（迁移完成的标记，避免目录被清空后旧值被再次灌回）。
func (r *ModelProviderRepo) ClearLegacyModelName(ctx context.Context, providerULID string) error {
	hasColumn, err := r.HasLegacyModelColumn(ctx)
	if err != nil {
		return err
	}
	if !hasColumn {
		return nil
	}
	if err := r.db.WithContext(ctx).Exec(
		`UPDATE model_providers SET chat_model_name = '' WHERE provider_ulid = ?`, providerULID).Error; err != nil {
		return fmt.Errorf("clear legacy chat model name: %w", err)
	}
	return nil
}

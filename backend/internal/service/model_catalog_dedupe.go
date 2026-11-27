// 本文件实现模型生态的启动期数据修复：同名供应商合并、重复模型目录去重、默认模型收敛与空供应商清理。
// 唯一索引要求存量数据先干净（有重复行时 AutoMigrate 建索引会整体失败），因此本流程必须可重复执行。

package service

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// CatalogDedupeResult 模型生态去重结果（启动日志与排查用）。
type CatalogDedupeResult struct {
	DeletedCatalogCount  int // 删除的重复模型目录行数
	DeletedProviderCount int // 删除的重复/空供应商行数
	MigratedGrantCount   int // 迁移到保留行的授权行数
}

// catalogDedupeCandidate 去重候选：目录行投影 + 引用标记（是否被授权或用量统计引用）。
type catalogDedupeCandidate struct {
	Row      repo.DedupeCatalogRow // 目录行投影（只含判重必需列）
	HasGrant bool                  // 存在角色授权或用户直授
	HasUsage bool                  // 存在用量统计行
}

// dedupeTables 去重涉及的附表可用性（老库可能还没有这些表，缺失即跳过对应步骤）。
type dedupeTables struct {
	roleGrant bool // model_role_grants 是否存在
	userGrant bool // model_user_grants 是否存在
	usage     bool // model_usage_stats 是否存在
}

// DedupeModelCatalogs 修复模型生态存量脏数据（单事务、幂等）。
// 顺序：合并同名供应商 → 按 (供应商, 模型名) 去重 → 收敛默认标记 → 删除空供应商。
func DedupeModelCatalogs(ctx context.Context, db *gorm.DB) (CatalogDedupeResult, error) {
	var result CatalogDedupeResult
	// 两张主表必须都在：缺模型目录表时无法判断供应商是否被引用，贸然合并会丢旧版 chat_model_name。
	if db == nil || !db.Migrator().HasTable(&domain.ModelProviderDO{}) ||
		!db.Migrator().HasTable(&domain.ModelCatalogDO{}) {
		return result, nil // 首次启动表还没建，交给 AutoMigrate 建表后再清理
	}
	tables := dedupeTables{
		roleGrant: db.Migrator().HasTable(&domain.ModelRoleGrantDO{}),
		userGrant: db.Migrator().HasTable(&domain.ModelUserGrantDO{}),
		usage:     db.Migrator().HasTable(&domain.ModelUsageStatDO{}),
	}
	err := repo.WithTx(db, func(tx *gorm.DB) error {
		return dedupeInTx(ctx, repo.NewModelProviderRepo(tx), tables, &result)
	})
	if err != nil {
		return result, fmt.Errorf("dedupe model catalogs: %w", err)
	}
	if result.DeletedCatalogCount > 0 || result.DeletedProviderCount > 0 || result.MigratedGrantCount > 0 {
		pkg.SLogger().Infow("模型生态重复数据已清理",
			"deleted_catalogs", result.DeletedCatalogCount,
			"deleted_providers", result.DeletedProviderCount,
			"migrated_grants", result.MigratedGrantCount)
	}
	return result, nil
}

// dedupeInTx 在事务内按固定顺序执行去重步骤（步骤间共享同一份事务句柄）。
func dedupeInTx(ctx context.Context, providerRepo *repo.ModelProviderRepo, tables dedupeTables,
	result *CatalogDedupeResult) error {
	usageModelKeys := make(map[string]map[string]bool, 8)
	if tables.usage {
		keys, err := providerRepo.ListUsageKeys(ctx)
		if err != nil {
			return err
		}
		usageModelKeys = keys
	}
	providerList, err := providerRepo.ListDedupeProviders(ctx)
	if err != nil {
		return err
	}
	catalogList, err := providerRepo.ListDedupeCatalogs(ctx)
	if err != nil {
		return err
	}
	// 先只算合并计划：目录去重按「保留供应商」分组，才能把重复供应商下的同名模型并到一起。
	plan := planProviderMerge(providerList, catalogList, usageModelKeys)
	if err := dedupeCatalogs(ctx, providerRepo, catalogList, plan, usageModelKeys, tables, result); err != nil {
		return err
	}
	catalogList, err = providerRepo.ListDedupeCatalogs(ctx) // 目录去重后重读：改挂与删除都需要最新视图
	if err != nil {
		return err
	}
	if err := applyProviderMerge(ctx, providerRepo, plan, catalogList, result); err != nil {
		return err
	}
	if err := convergeDefaultModels(ctx, providerRepo, usageModelKeys, tables); err != nil {
		return err
	}
	return deleteEmptyProviders(ctx, providerRepo, usageModelKeys, result)
}

// applyProviderMerge 把重复供应商下残留的模型改挂到保留供应商，然后删除重复供应商行。
func applyProviderMerge(ctx context.Context, providerRepo *repo.ModelProviderRepo, plan providerMergePlan,
	catalogList []repo.DedupeCatalogRow, result *CatalogDedupeResult) error {
	if len(plan.duplicates) == 0 {
		return nil
	}
	for i := range catalogList {
		row := catalogList[i]
		keeperULID, ok := plan.canonical[row.ProviderULID]
		if !ok {
			continue
		}
		if err := providerRepo.ReassignCatalog(ctx, row.ModelULID, keeperULID); err != nil {
			return err
		}
	}
	for _, dup := range plan.duplicates {
		if err := providerRepo.Delete(ctx, dup.ProviderULID); err != nil {
			return err
		}
		result.DeletedProviderCount++
		pkg.SLogger().Infow("同名模型供应商已合并", "provider_name", plan.keeperName[plan.canonical[dup.ProviderULID]],
			"kept_provider_ulid", plan.canonical[dup.ProviderULID], "merged_provider_ulid", dup.ProviderULID)
	}
	return nil
}

// providerMergePlan 同名供应商合并计划（先算出保留关系，目录去重后再真正改挂与删除）。
type providerMergePlan struct {
	canonical  map[string]string        // 重复供应商 ULID → 保留供应商 ULID
	duplicates []repo.DedupeProviderRow // 待删除的重复供应商行
	keeperName map[string]string        // 保留供应商 ULID → 名称（日志用）
}

// planProviderMerge 计算同名供应商的保留关系（有模型/用量引用 → 手工创建 → 更新更晚 → ULID 升序）。
func planProviderMerge(providerList []repo.DedupeProviderRow, catalogList []repo.DedupeCatalogRow,
	usageModelKeys map[string]map[string]bool) providerMergePlan {
	plan := providerMergePlan{canonical: make(map[string]string, 4), keeperName: make(map[string]string, 4)}
	modelCount := make(map[string]int, len(providerList))
	for i := range catalogList {
		modelCount[catalogList[i].ProviderULID]++
	}
	groupIndex := make(map[string][]int, len(providerList))
	for i := range providerList {
		// 空名也参与分组：provider_name 唯一索引对空串同样生效，不合并就建不起索引。
		key := strings.TrimSpace(providerList[i].ProviderName)
		groupIndex[key] = append(groupIndex[key], i)
	}
	for _, indexList := range groupIndex {
		if len(indexList) < 2 {
			continue
		}
		keeperIdx := indexList[0]
		for _, index := range indexList[1:] {
			if providerPreferred(providerList[index], providerList[keeperIdx], modelCount, usageModelKeys) {
				keeperIdx = index
			}
		}
		keeper := providerList[keeperIdx]
		plan.keeperName[keeper.ProviderULID] = keeper.ProviderName
		for _, index := range indexList {
			if index == keeperIdx {
				continue
			}
			plan.canonical[providerList[index].ProviderULID] = keeper.ProviderULID
			plan.duplicates = append(plan.duplicates, providerList[index])
		}
	}
	return plan
}

// canonicalProvider 返回供应商的保留 ULID（无重复时返回自身）。
func (p providerMergePlan) canonicalProvider(providerULID string) string {
	if keeper, ok := p.canonical[providerULID]; ok {
		return keeper
	}
	return providerULID
}

// providerPreferred 判断供应商 a 是否应优先保留（有模型/用量引用 → 手工创建 → 更新更晚 → ULID 升序）。
func providerPreferred(a, b repo.DedupeProviderRow, modelCount map[string]int,
	usageModelKeys map[string]map[string]bool) bool {
	aRef := modelCount[a.ProviderULID] > 0 || len(usageModelKeys[a.ProviderULID]) > 0
	bRef := modelCount[b.ProviderULID] > 0 || len(usageModelKeys[b.ProviderULID]) > 0
	if aRef != bRef {
		return aRef
	}
	aManual := strings.TrimSpace(a.CreatedBy) != ""
	bManual := strings.TrimSpace(b.CreatedBy) != ""
	if aManual != bManual {
		return aManual
	}
	if !a.UpdatedAt.Equal(b.UpdatedAt) {
		return a.UpdatedAt.After(b.UpdatedAt)
	}
	return a.ProviderULID < b.ProviderULID
}

// dedupeCatalogs 按 (供应商, 模型名) 去重：保留优先级最高的行，其余行的授权迁到保留行后删除。
func dedupeCatalogs(ctx context.Context, providerRepo *repo.ModelProviderRepo, catalogList []repo.DedupeCatalogRow,
	plan providerMergePlan, usageModelKeys map[string]map[string]bool, tables dedupeTables,
	result *CatalogDedupeResult) error {
	if len(catalogList) == 0 {
		return nil
	}
	modelULIDList := make([]string, 0, len(catalogList))
	for i := range catalogList {
		modelULIDList = append(modelULIDList, catalogList[i].ModelULID)
	}
	roleGrantMap := make(map[string][]string, len(catalogList))
	userGrantMap := make(map[string][]string, len(catalogList))
	if tables.roleGrant {
		loaded, err := providerRepo.ListGrantsByModelULIDList(ctx, modelULIDList)
		if err != nil {
			return err
		}
		roleGrantMap = loaded
	}
	if tables.userGrant {
		loaded, err := providerRepo.ListUserGrantsByModelULIDList(ctx, modelULIDList)
		if err != nil {
			return err
		}
		userGrantMap = loaded
	}
	candidates := make([]catalogDedupeCandidate, len(catalogList))
	groupIndex := make(map[string][]int, len(catalogList))
	for i := range catalogList {
		row := catalogList[i]
		candidates[i] = catalogDedupeCandidate{
			Row:      row,
			HasGrant: len(roleGrantMap[row.ModelULID]) > 0 || len(userGrantMap[row.ModelULID]) > 0,
			HasUsage: usageModelKeys[row.ProviderULID][row.ModelName],
		}
		// 按保留供应商分组：同名供应商下的同名模型会在此并成一组。
		key := catalogGroupKey(plan.canonicalProvider(row.ProviderULID), row.ModelName)
		groupIndex[key] = append(groupIndex[key], i)
	}
	for _, indexList := range groupIndex {
		if len(indexList) < 2 {
			continue
		}
		group := make([]catalogDedupeCandidate, 0, len(indexList))
		for _, index := range indexList {
			group = append(group, candidates[index])
		}
		keeper := group[pickKeeper(group)]
		keeperDefault := keeper.Row.DefaultFlag
		keeperRoleKeys := append([]string{}, roleGrantMap[keeper.Row.ModelULID]...)
		keeperUserKeys := append([]string{}, userGrantMap[keeper.Row.ModelULID]...)
		for _, candidate := range group {
			if candidate.Row.ModelULID == keeper.Row.ModelULID {
				continue
			}
			roleMove := grantsToMove(keeperRoleKeys, roleGrantMap[candidate.Row.ModelULID])
			userMove := grantsToMove(keeperUserKeys, userGrantMap[candidate.Row.ModelULID])
			if err := moveGrants(ctx, providerRepo, candidate.Row.ModelULID, keeper.Row.ModelULID,
				roleMove, userMove, tables); err != nil {
				return err
			}
			result.MigratedGrantCount += len(roleMove) + len(userMove)
			keeperRoleKeys = append(keeperRoleKeys, roleMove...)
			keeperUserKeys = append(keeperUserKeys, userMove...)
			keeperDefault = keeperDefault || candidate.Row.DefaultFlag
			if err := providerRepo.DeleteModel(ctx, candidate.Row.ModelULID); err != nil {
				return err
			}
			result.DeletedCatalogCount++
			pkg.SLogger().Infow("重复模型目录已合并", "provider_ulid", keeper.Row.ProviderULID,
				"model_name", keeper.Row.ModelName, "kept_model_ulid", keeper.Row.ModelULID,
				"deleted_model_ulid", candidate.Row.ModelULID)
		}
		if keeperDefault && !keeper.Row.DefaultFlag {
			if err := providerRepo.UpdateModel(ctx, keeper.Row.ModelULID, map[string]any{"default_flag": true}); err != nil {
				return err
			}
		}
	}
	return nil
}

// moveGrants 把被删行的授权迁到保留行，再清理被删行的残留授权（用量统计不改写）。
func moveGrants(ctx context.Context, providerRepo *repo.ModelProviderRepo, fromModelULID, toModelULID string,
	roleMove, userMove []string, tables dedupeTables) error {
	if tables.roleGrant {
		if _, err := providerRepo.ReassignRoleGrants(ctx, fromModelULID, toModelULID, roleMove); err != nil {
			return err
		}
		if err := providerRepo.DeleteGrantsByModel(ctx, fromModelULID); err != nil {
			return err
		}
	}
	if tables.userGrant {
		if _, err := providerRepo.ReassignUserGrants(ctx, fromModelULID, toModelULID, userMove); err != nil {
			return err
		}
		if err := providerRepo.DeleteUserGrantsByModel(ctx, fromModelULID); err != nil {
			return err
		}
	}
	return nil
}

// convergeDefaultModels 收敛默认标记：全表只留优先级最高的一行为真。
func convergeDefaultModels(ctx context.Context, providerRepo *repo.ModelProviderRepo,
	usageModelKeys map[string]map[string]bool, tables dedupeTables) error {
	catalogList, err := providerRepo.ListDedupeCatalogs(ctx)
	if err != nil {
		return err
	}
	defaultList := make([]repo.DedupeCatalogRow, 0, 1)
	for i := range catalogList {
		if catalogList[i].DefaultFlag {
			defaultList = append(defaultList, catalogList[i])
		}
	}
	if len(defaultList) <= 1 {
		return nil
	}
	ulidList := make([]string, 0, len(defaultList))
	for i := range defaultList {
		ulidList = append(ulidList, defaultList[i].ModelULID)
	}
	roleGrantMap := make(map[string][]string, len(ulidList))
	userGrantMap := make(map[string][]string, len(ulidList))
	if tables.roleGrant {
		if roleGrantMap, err = providerRepo.ListGrantsByModelULIDList(ctx, ulidList); err != nil {
			return err
		}
	}
	if tables.userGrant {
		if userGrantMap, err = providerRepo.ListUserGrantsByModelULIDList(ctx, ulidList); err != nil {
			return err
		}
	}
	candidates := make([]catalogDedupeCandidate, 0, len(defaultList))
	for i := range defaultList {
		row := defaultList[i]
		candidates = append(candidates, catalogDedupeCandidate{
			Row:      row,
			HasGrant: len(roleGrantMap[row.ModelULID]) > 0 || len(userGrantMap[row.ModelULID]) > 0,
			HasUsage: usageModelKeys[row.ProviderULID][row.ModelName],
		})
	}
	for _, modelULID := range defaultsToClear(candidates) {
		if err := providerRepo.UpdateModel(ctx, modelULID, map[string]any{"default_flag": false}); err != nil {
			return err
		}
	}
	return nil
}

// deleteEmptyProviders 删除播种/迁移遗留的空供应商（无模型且无用量引用）。
// 只清理 created_by 为空的行：管理员刚建好还没挂模型的供应商必须保留，重启不能把它删掉。
// 仍带旧版 chat_model_name 的供应商也保留：它的模型还没被 MigrateLegacyModels 迁出来。
func deleteEmptyProviders(ctx context.Context, providerRepo *repo.ModelProviderRepo,
	usageModelKeys map[string]map[string]bool, result *CatalogDedupeResult) error {
	providerList, err := providerRepo.ListDedupeProviders(ctx)
	if err != nil {
		return err
	}
	catalogList, err := providerRepo.ListDedupeCatalogs(ctx)
	if err != nil {
		return err
	}
	legacySet, err := providerRepo.ListProviderULIDsWithLegacyModel(ctx)
	if err != nil {
		return err
	}
	modelCount := make(map[string]int, len(providerList))
	for i := range catalogList {
		modelCount[catalogList[i].ProviderULID]++
	}
	for i := range providerList {
		row := providerList[i]
		if strings.TrimSpace(row.CreatedBy) != "" {
			continue // 手工创建的供应商不自动删除
		}
		if legacySet[row.ProviderULID] {
			continue // 旧版模型待迁移，删了会丢模型名
		}
		if modelCount[row.ProviderULID] > 0 || len(usageModelKeys[row.ProviderULID]) > 0 {
			continue
		}
		if err := providerRepo.Delete(ctx, row.ProviderULID); err != nil {
			return err
		}
		result.DeletedProviderCount++
		pkg.SLogger().Infow("空模型供应商已清理", "provider_ulid", row.ProviderULID, "provider_name", row.ProviderName)
	}
	return nil
}

// pickKeeper 选出应保留的候选行下标（空列表返回 -1）。
// 规则：有 grants/usage 引用优先 → created_by 非空（手工创建）优先 → updated_at 更晚优先 → ULID 升序兜底。
func pickKeeper(rows []catalogDedupeCandidate) int {
	if len(rows) == 0 {
		return -1
	}
	best := 0
	for i := 1; i < len(rows); i++ {
		if keeperPreferred(rows[i], rows[best]) {
			best = i
		}
	}
	return best
}

// keeperPreferred 判断候选 a 是否应优先于 b 保留（保持选择规则唯一实现）。
func keeperPreferred(a, b catalogDedupeCandidate) bool {
	aRef := a.HasGrant || a.HasUsage
	bRef := b.HasGrant || b.HasUsage
	if aRef != bRef {
		return aRef
	}
	aManual := strings.TrimSpace(a.Row.CreatedBy) != ""
	bManual := strings.TrimSpace(b.Row.CreatedBy) != ""
	if aManual != bManual {
		return aManual
	}
	if !a.Row.UpdatedAt.Equal(b.Row.UpdatedAt) {
		return a.Row.UpdatedAt.After(b.Row.UpdatedAt)
	}
	return a.Row.ModelULID < b.Row.ModelULID
}

// grantsToMove 计算需从被删行迁到保留行的授权键（去掉保留行已持有的键，保持原顺序）。
func grantsToMove(keeperKeys, removedKeys []string) []string {
	exists := make(map[string]bool, len(keeperKeys)+len(removedKeys))
	for _, key := range keeperKeys {
		exists[key] = true
	}
	moved := make([]string, 0, len(removedKeys))
	for _, key := range removedKeys {
		if key == "" || exists[key] {
			continue
		}
		exists[key] = true
		moved = append(moved, key)
	}
	return moved
}

// defaultsToClear 收敛默认标记：多行同时为真时保留优先级最高的一行，返回其余需清除默认标记的 ULID。
func defaultsToClear(rows []catalogDedupeCandidate) []string {
	if len(rows) <= 1 {
		return nil
	}
	keeper := pickKeeper(rows)
	clearList := make([]string, 0, len(rows)-1)
	for i := range rows {
		if i == keeper {
			continue
		}
		clearList = append(clearList, rows[i].Row.ModelULID)
	}
	return clearList
}

// catalogGroupKey 模型目录去重分组键（供应商 + 模型名，与唯一索引维度一致）。
func catalogGroupKey(providerULID, modelName string) string {
	return providerULID + "\x00" + modelName
}

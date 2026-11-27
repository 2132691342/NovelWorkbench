// 模型生态去重的完整链路回归：分组键 → 保留行选择 → 授权迁移 → 默认标记收敛 → 供应商合并计划。
// 规则直接决定「删哪一行」：删错会丢授权或丢历史默认模型，
// 保留行选择错会让唯一索引 (provider_ulid, model_name) 建不起来导致整个服务起不来。

package service

import (
	"testing"
	"time"

	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// dedupeRow 构造去重候选行（时间与标记由参数控制，便于逐条验证优先级）。
func dedupeRow(modelULID, createdBy string, updatedAt time.Time, hasGrant, hasUsage, defaultFlag bool) catalogDedupeCandidate {
	return catalogDedupeCandidate{
		Row: repo.DedupeCatalogRow{
			ModelULID:    modelULID,
			ModelName:    "mcode/MiniMax-M3.1-Flash",
			ProviderULID: "MPR_1",
			DefaultFlag:  defaultFlag,
			CreatedBy:    createdBy,
			UpdatedAt:    updatedAt,
		},
		HasGrant: hasGrant,
		HasUsage: hasUsage,
	}
}

// TestModelCatalogDedupeChain 覆盖模型目录去重的完整判定链。
func TestModelCatalogDedupeChain(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// 环节1：分组键必须把供应商与模型名同时纳入（与唯一索引维度一致）。
	if catalogGroupKey("MPR_1", "m1") == catalogGroupKey("MPR_2", "m1") {
		t.Fatal("不同供应商的同名模型不应分到同一组")
	}
	if catalogGroupKey("MPR_1", "m1") == catalogGroupKey("MPR_1", "m2") {
		t.Fatal("同供应商的不同模型不应分到同一组")
	}

	// 环节2：保留行优先级。有 grants/usage 的行即使更新更晚、是手工创建也要保留；
	// 无引用时手工创建优先，其次更新更晚优先，最后 ULID 升序兜底保证结果确定。
	referenced := []catalogDedupeCandidate{
		dedupeRow("MDL_MANUAL_NEW", "USR_ADMIN", base.Add(48*time.Hour), false, false, true),
		dedupeRow("MDL_SEED_OLD", "", base, true, false, false),
	}
	if got := referenced[pickKeeper(referenced)].Row.ModelULID; got != "MDL_SEED_OLD" {
		t.Fatalf("被授权引用的行应保留，实际保留 %s", got)
	}
	referenced[0].HasUsage = true
	referenced[1].HasGrant = false
	if got := referenced[pickKeeper(referenced)].Row.ModelULID; got != "MDL_MANUAL_NEW" {
		t.Fatalf("被用量引用的行应保留，实际保留 %s", got)
	}
	manual := []catalogDedupeCandidate{
		dedupeRow("MDL_SEED", "", base.Add(72*time.Hour), false, false, false),
		dedupeRow("MDL_MANUAL", "USR_ADMIN", base, false, false, false),
	}
	if got := manual[pickKeeper(manual)].Row.ModelULID; got != "MDL_MANUAL" {
		t.Fatalf("手工创建的行应优先保留，实际保留 %s", got)
	}
	manual[0].Row.CreatedBy = "USR_OTHER"
	manual[0].Row.UpdatedAt = base.Add(72 * time.Hour)
	manual[1].Row.UpdatedAt = base.Add(96 * time.Hour)
	if got := manual[pickKeeper(manual)].Row.ModelULID; got != "MDL_MANUAL" {
		t.Fatalf("updated_at 更晚的行应保留，实际保留 %s", got)
	}
	manual[0].Row.UpdatedAt = manual[1].Row.UpdatedAt
	if got := manual[pickKeeper(manual)].Row.ModelULID; got != "MDL_MANUAL" {
		t.Fatalf("时间相同时应按 ULID 升序保留，实际保留 %s", got)
	}
	if pickKeeper(nil) != -1 {
		t.Fatal("空候选应返回 -1")
	}

	// 环节3：授权迁移。只迁保留行没有的键，重复键跳过，空键忽略，保持原顺序。
	moved := grantsToMove([]string{"ROL_ADMIN"}, []string{"ROL_ADMIN", "ROL_EDITOR", "", "ROL_EDITOR", "ROL_VIEWER"})
	if len(moved) != 2 || moved[0] != "ROL_EDITOR" || moved[1] != "ROL_VIEWER" {
		t.Fatalf("应只迁移保留行没有的授权键，实际 %v", moved)
	}
	if moved := grantsToMove(nil, []string{"ROL_ADMIN"}); len(moved) != 1 || moved[0] != "ROL_ADMIN" {
		t.Fatalf("保留行无授权时应全部迁移，实际 %v", moved)
	}
	if moved := grantsToMove([]string{"ROL_ADMIN"}, nil); len(moved) != 0 {
		t.Fatalf("被删行无授权时不应迁移，实际 %v", moved)
	}

	// 环节4：默认标记收敛。多行为真时只保留优先级最高的一行。
	defaultRows := []catalogDedupeCandidate{
		dedupeRow("MDL_A", "", base, false, false, true),
		dedupeRow("MDL_B", "USR_ADMIN", base, false, false, true),
		dedupeRow("MDL_C", "", base.Add(time.Hour), false, false, true),
	}
	clearList := defaultsToClear(defaultRows)
	if len(clearList) != 2 {
		t.Fatalf("三行默认应清除两行，实际 %v", clearList)
	}
	for _, modelULID := range clearList {
		if modelULID == "MDL_B" {
			t.Fatal("手工创建且被引用的默认行不应被清除")
		}
	}
	if clearList := defaultsToClear(defaultRows[:1]); clearList != nil {
		t.Fatalf("单行默认无需收敛，实际 %v", clearList)
	}

	// 环节5：供应商合并优先级。有模型引用优先 → 手工创建优先 → 更新更晚优先。
	seedProvider := repo.DedupeProviderRow{ProviderULID: "MPR_SEED", ProviderName: "默认模型网关"}
	seedProvider.UpdatedAt = base.Add(48 * time.Hour)
	manualProvider := repo.DedupeProviderRow{ProviderULID: "MPR_MANUAL", ProviderName: "默认模型网关"}
	manualProvider.CreatedBy = "USR_ADMIN"
	manualProvider.UpdatedAt = base
	if !providerPreferred(manualProvider, seedProvider, map[string]int{"MPR_MANUAL": 1}, nil) {
		t.Fatal("有模型挂载的供应商应优先保留")
	}
	if providerPreferred(seedProvider, manualProvider, nil, nil) {
		t.Fatal("无引用时手工创建的供应商应优先保留")
	}
	if !providerPreferred(seedProvider, manualProvider, nil, map[string]map[string]bool{"MPR_SEED": {"m": true}}) {
		t.Fatal("有用量引用的供应商应优先保留")
	}
	laterProvider := manualProvider
	laterProvider.ProviderULID = "MPR_MANUAL2"
	laterProvider.UpdatedAt = base.Add(time.Hour)
	if !providerPreferred(laterProvider, manualProvider, nil, nil) {
		t.Fatal("updated_at 更晚的供应商应优先保留")
	}

	// 环节6：同名供应商合并计划。保留行选谁、哪些行进入待删清单、规范映射是否正确。
	providers := []repo.DedupeProviderRow{
		{ProviderULID: "MPR_SEED", ProviderName: "默认模型网关", UpdatedAt: base.Add(72 * time.Hour)},
		{ProviderULID: "MPR_MANUAL", ProviderName: "默认模型网关", CreatedBy: "USR_ADMIN", UpdatedAt: base},
		{ProviderULID: "MPR_OTHER", ProviderName: "另一网关", UpdatedAt: base},
	}
	catalogs := []repo.DedupeCatalogRow{
		{ModelULID: "MDL_SEED", ProviderULID: "MPR_SEED", ModelName: "mcode/MiniMax-M3.1-Flash", DefaultFlag: true},
		{ModelULID: "MDL_MANUAL", ProviderULID: "MPR_MANUAL", ModelName: "mcode/MiniMax-M3.1-Flash"},
	}
	plan := planProviderMerge(providers, catalogs, nil)
	if plan.canonical["MPR_SEED"] != "MPR_MANUAL" {
		t.Fatalf("手工创建的供应商应作为保留行，实际 %v", plan.canonical)
	}
	if len(plan.duplicates) != 1 || plan.duplicates[0].ProviderULID != "MPR_SEED" {
		t.Fatalf("应只有种子供应商进入待删清单，实际 %v", plan.duplicates)
	}
	if plan.canonicalProvider("MPR_SEED") != "MPR_MANUAL" || plan.canonicalProvider("MPR_OTHER") != "MPR_OTHER" {
		t.Fatalf("供应商规范映射错误：%v", plan.canonical)
	}
	if plan.keeperName["MPR_MANUAL"] != "默认模型网关" {
		t.Fatalf("保留供应商名称应可查，实际 %v", plan.keeperName)
	}

	// 环节7：跨供应商同名模型必须并进同一组，否则唯一索引仍会冲突。
	groupIndex := make(map[string][]int, len(catalogs))
	for i := range catalogs {
		key := catalogGroupKey(plan.canonicalProvider(catalogs[i].ProviderULID), catalogs[i].ModelName)
		groupIndex[key] = append(groupIndex[key], i)
	}
	if len(groupIndex) != 1 {
		t.Fatalf("跨供应商的同名模型应并成一组，实际 %v", groupIndex)
	}
	group := []catalogDedupeCandidate{
		{Row: catalogs[0], HasGrant: false, HasUsage: false},
		{Row: catalogs[1], HasGrant: false, HasUsage: false},
	}
	keeper := group[pickKeeper(group)]
	if keeper.Row.ModelULID != "MDL_MANUAL" {
		t.Fatalf("手工创建的行应保留，实际保留 %s", keeper.Row.ModelULID)
	}
	// 种子行有默认标记、手工行有 created_by：按规则手工行胜出，默认标记需继承。
	if !keeper.Row.DefaultFlag && !group[0].Row.DefaultFlag {
		t.Fatal("默认标记应能从被删行继承到保留行")
	}
	if clearList := defaultsToClear(group); len(clearList) != 1 || clearList[0] != "MDL_SEED" {
		t.Fatalf("多行默认为真时应只清除被删行，实际 %v", clearList)
	}

	// 环节8：空名供应商也要合并。provider_name 唯一索引对空串同样生效。
	emptyPlan := planProviderMerge([]repo.DedupeProviderRow{
		{ProviderULID: "MPR_A", ProviderName: ""},
		{ProviderULID: "MPR_B", ProviderName: "  "},
	}, nil, nil)
	if len(emptyPlan.duplicates) != 1 {
		t.Fatalf("两条空名供应商应合并成一条，实际待删 %v", emptyPlan.duplicates)
	}
	if emptyPlan.canonicalProvider("MPR_B") == "MPR_B" {
		t.Fatalf("空名重复供应商应映射到保留行，实际 %v", emptyPlan.canonical)
	}
}

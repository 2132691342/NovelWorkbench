// 访问控制的链路回归：权限码规范化 → 角色脏数据修复 → 权限码下线清理 → 菜单种子可注册性
// → 导航装配 → 权限目录 → 用户列表按角色过滤。
// 这些环节共同保证「权限可勾选、可保存、可回读、可下架、菜单可导航、用户可按角色筛」这条闭环，
// 任一环断裂都会表现为「点得进却用不了」或「面板被锁死」，因此集中在本文件按链路组织。

package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// sqlCapture 收集 gorm 打印的 SQL（DryRun 下每条语句各一行）。
type sqlCapture struct {
	mu    sync.Mutex
	lines []string
}

// Printf 记录一条 SQL。
func (c *sqlCapture) Printf(format string, args ...any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lines = append(c.lines, fmt.Sprintf(format, args...))
}

// joined 返回全部 SQL 文本。
func (c *sqlCapture) joined() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return strings.Join(c.lines, "\n")
}

// newDryRunUserAdminService 构造 DryRun 模式下的用户管理服务（连接串指向不可达端口，不会真的建连）。
func newDryRunUserAdminService(t *testing.T) (*UserAdminService, *sqlCapture) {
	t.Helper()
	capture := &sqlCapture{}
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=127.0.0.1 port=1 user=x dbname=x"}), &gorm.Config{
		DryRun:               true,
		DisableAutomaticPing: true,
		Logger:               logger.New(capture, logger.Config{LogLevel: logger.Info}),
	})
	if err != nil {
		t.Fatalf("打开 DryRun 数据库失败: %v", err)
	}
	return NewUserAdminService(repo.NewUserRepo(db), repo.NewRoleRepo(db), nil, nil), capture
}

// knownPermSet 构造权限目录夹具（菜单是权限码唯一登记处，这里直接给集合）。
func knownPermSet(codes ...string) map[string]bool {
	known := make(map[string]bool, len(codes))
	for _, code := range codes {
		known[code] = true
	}
	return known
}

// menuDO 构造测试用菜单行。
func menuDO(ulid, parentULID, code, name, menuType, permCode string) domain.MenuDO {
	return domain.MenuDO{
		MenuULID: ulid, ParentULID: parentULID, MenuCode: code, MenuName: name,
		MenuTypeCode: menuType, ScopeCode: domain.MenuScopeGlobal,
		PermCode: permCode, VisibleFlag: true, EnabledFlag: true,
	}
}

// testMenus 构造固定的菜单夹具：一组两层级菜单、隐藏页与停用页。
func testMenus() []domain.MenuDO {
	menus := []domain.MenuDO{
		menuDO("MNU_1", "", "group_system", "系统管理", domain.MenuTypeGroup, ""),
		menuDO("MNU_2", "MNU_1", "admin_users", "用户管理", domain.MenuTypePage, "system:user:manage"),
		menuDO("MNU_3", "MNU_1", "admin_user_add", "新增用户", domain.MenuTypeAction, "system:user:add"),
		menuDO("MNU_4", "", "skills", "技能管理", domain.MenuTypePage, "skill:manage"),
		menuDO("MNU_5", "", "hidden_page", "隐藏页", domain.MenuTypePage, ""),
		menuDO("MNU_6", "", "disabled_page", "停用页", domain.MenuTypePage, ""),
	}
	menus[4].VisibleFlag = false
	menus[5].EnabledFlag = false
	return menus
}

// TestAccessControlChain 覆盖「权限可勾选、可保存、可回读、可下架、菜单可导航」这条闭环。
// 环节1~3 是角色权限的变更链路（编辑 → 回读 → 下架），环节4~7 是菜单与导航的下发链路；
// 两者串在同一次管理操作里，任一环断裂都会表现为「点得进却用不了」或「面板被锁死」。
func TestAccessControlChain(t *testing.T) {
	known := knownPermSet("prompt:manage", "tool:manage", "system:audit:read")

	// 环节1：权限码规范化。去重保序、空白剔除、非管理员禁用通配、未登记编码报错。
	normalized, err := normalizePermCodeList("editor",
		[]string{"tool:manage", "prompt:manage", "tool:manage", " "}, known)
	if err != nil {
		t.Fatalf("合法权限码应通过校验: %v", err)
	}
	if len(normalized) != 2 || normalized[0] != "tool:manage" || normalized[1] != "prompt:manage" {
		t.Fatalf("去重保序失败: %v", normalized)
	}
	if _, err := normalizePermCodeList("editor", []string{domain.PermAdminAll}, known); err == nil {
		t.Fatal("非管理员角色带通配 * 应被拒绝")
	}
	if _, err := normalizePermCodeList(domain.RoleCodeAdmin, []string{domain.PermAdminAll}, known); err != nil {
		t.Fatalf("管理员角色应允许通配 *: %v", err)
	}
	if _, err := normalizePermCodeList("editor", []string{"ghost:perm"}, known); err == nil {
		t.Fatal("未登记权限码应被拒绝")
	}

	// 环节2：历史脏数据修复。普通角色误带 "*" 会让前端锁死权限面板，修复后必须只剩合法权限码。
	dirty := &domain.RoleDO{
		RoleCode:      domain.RoleCodeUser,
		PermCodesJSON: `["prompt:manage","*","ghost:perm","tool:manage"]`,
	}
	if repaired := marshalPermCodes(repairedPermCodeList(dirty, known)); repaired != `["prompt:manage","tool:manage"]` {
		t.Fatalf("普通角色脏权限应被清洗，实际 %s", repaired)
	}
	adminDirty := &domain.RoleDO{RoleCode: domain.RoleCodeAdmin, PermCodesJSON: `["prompt:manage"]`}
	if repaired := marshalPermCodes(repairedPermCodeList(adminDirty, known)); repaired != `["*"]` {
		t.Fatalf("管理员角色应固定为通配，实际 %s", repaired)
	}
	clean := &domain.RoleDO{RoleCode: domain.RoleCodeAdmin, PermCodesJSON: `["*"]`}
	if repaired := marshalPermCodes(repairedPermCodeList(clean, known)); repaired != `["*"]` {
		t.Fatalf("已合规权限不应被改写，实际 %s", repaired)
	}

	// 环节3：权限码下线。删菜单后角色里的悬挂权限码必须被剔除，管理员通配不受影响。
	normalRole := &domain.RoleDO{
		RoleCode:      "editor",
		PermCodesJSON: `["prompt:manage","system:audit:read"]`,
	}
	if pruned := marshalPermCodes(prunedPermCodeList(normalRole, "system:audit:read")); pruned != `["prompt:manage"]` {
		t.Fatalf("下线权限码应从普通角色剔除，实际 %s", pruned)
	}
	adminRole := &domain.RoleDO{RoleCode: domain.RoleCodeAdmin, PermCodesJSON: `["*"]`}
	if pruned := marshalPermCodes(prunedPermCodeList(adminRole, domain.PermAdminAll)); pruned != `["*"]` {
		t.Fatalf("通配权限码本身不允许被剔除，实际 %s", pruned)
	}

	// 环节4：种子可注册性。菜单即路由，种子里的组件路径必须在前端有对应页面文件，
	// 否则点击落到 404；编码与路由路径必须唯一，否则前端生成重复路由名。
	srcRoot := filepath.Join("..", "..", "..", "frontend", "src")
	hasFrontend := false
	if _, err := os.Stat(srcRoot); err == nil {
		hasFrontend = true
	}
	seedCodeSet := map[string]bool{}
	seedRouteSet := map[string]bool{}
	profileRegistered := false
	for _, seed := range menuSeedList {
		if seedCodeSet[seed.MenuCode] {
			t.Fatalf("菜单编码重复：%s", seed.MenuCode)
		}
		seedCodeSet[seed.MenuCode] = true
		if seed.RoutePath != "" {
			if seedRouteSet[seed.RoutePath] {
				t.Fatalf("菜单路由重复：%s", seed.RoutePath)
			}
			seedRouteSet[seed.RoutePath] = true
		}
		if hasFrontend && seed.ComponentPath != "" {
			basePath := filepath.Join(srcRoot, filepath.FromSlash(seed.ComponentPath))
			if !pageFileExists(basePath) {
				t.Fatalf("菜单 %s 的组件路径 %s 在前端不存在", seed.MenuCode, seed.ComponentPath)
			}
		}
		// 顶栏「个人中心」直接跳 /profile，必须登记为隐藏页，否则点下去 404。
		if seed.MenuCode == "profile" {
			profileRegistered = seed.RoutePath == "/profile" && seed.HiddenFlag
		}
	}
	if !profileRegistered {
		t.Fatal("缺少个人中心隐藏菜单登记")
	}

	// 环节5：可下发过滤。动作节点与未授权权限码不下发；隐藏页保留（visible_flag=false），
	// 让前端仍能注册路由，旧链接与深链不因入口合并而失效。
	visible := filterVisibleMenus(testMenus(), []string{"system:user:manage"})
	if len(visible) != 4 {
		t.Fatalf("可下发菜单数 = %d, want 4", len(visible))
	}
	hiddenCount := 0
	for _, do := range visible {
		if !do.VisibleFlag {
			hiddenCount++
		}
	}
	if hiddenCount != 1 {
		t.Fatalf("隐藏页应保留且带隐藏标记, got %d", hiddenCount)
	}
	if got := filterVisibleMenus(testMenus(), []string{domain.PermAdminAll}); len(got) != 5 {
		t.Fatalf("通配权限下可下发菜单数 = %d, want 5", len(got))
	}

	// 环节6：菜单树装配。父子按 ULID 关联，动作节点不进树，权限码与隐藏标记随节点保留。
	tree := buildMenuTree(filterVisibleMenus(testMenus(), []string{domain.PermAdminAll}), "")
	if len(tree) != 4 {
		t.Fatalf("根节点数 = %d, want 4", len(tree))
	}
	var hiddenNode, group *domain.MenuTreeNodeVO
	for i := range tree {
		switch tree[i].MenuCode {
		case "hidden_page":
			hiddenNode = &tree[i]
		case "group_system":
			group = &tree[i]
		}
	}
	if hiddenNode == nil || hiddenNode.VisibleFlag {
		t.Fatalf("隐藏页节点标记丢失: %+v", hiddenNode)
	}
	if group == nil || len(group.ChildList) != 1 ||
		group.ChildList[0].MenuCode != "admin_users" || group.ChildList[0].PermCode != "system:user:manage" {
		t.Fatalf("分组下级装配错误: %+v", group)
	}

	// 环节7：权限目录。通配独立成组、动作与菜单权限说明区分、同权限码只登记一次。
	catalog := buildPermCatalog(testMenus())
	if len(catalog) != 3 || catalog[0].GroupCode != "all" || catalog[0].GroupName != "全部权限" {
		t.Fatalf("权限分组数或通配分组错误: %+v", catalog)
	}
	groupMap := make(map[string]domain.PermGroupVO, len(catalog))
	for _, catalogGroup := range catalog {
		groupMap[catalogGroup.GroupCode] = catalogGroup
	}
	systemGroup, ok := groupMap["system"]
	if !ok || systemGroup.GroupName != "系统管理" || len(systemGroup.PermList) != 2 {
		t.Fatalf("system 分组错误: %+v", systemGroup)
	}
	permDesc := make(map[string]string, len(systemGroup.PermList))
	for _, item := range systemGroup.PermList {
		permDesc[item.PermCode] = item.PermDesc
	}
	if permDesc["system:user:add"] != "动作权限：新增用户" ||
		permDesc["system:user:manage"] != "菜单权限：系统管理 / 用户管理" {
		t.Fatalf("权限说明错误: %v", permDesc)
	}
	dup := append(testMenus(), menuDO("MNU_7", "", "admin_users_copy", "用户管理副本", domain.MenuTypePage, "system:user:manage"))
	for _, catalogGroup := range buildPermCatalog(dup) {
		permSeen := map[string]bool{}
		for _, item := range catalogGroup.PermList {
			if permSeen[item.PermCode] {
				t.Fatalf("权限码重复登记：%s", item.PermCode)
			}
			permSeen[item.PermCode] = true
		}
	}
}

// pageFileExists 判定页面文件（.vue 或 .tsx）是否存在。
func pageFileExists(basePath string) bool {
	for _, ext := range []string{".vue", ".tsx"} {
		if info, err := os.Stat(basePath + ext); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

// TestUserAdminListRoleFilter 覆盖用户列表按角色过滤的查询分流：
// role_code 非空走 JOIN 路径、为空走原路径、未知编码返回空页不报错。
// 用 gorm DryRun 捕获 SQL，不依赖真实数据库。
func TestUserAdminListRoleFilter(t *testing.T) {
	// 环节1：带角色编码时走 user_roles/roles 关联查询。
	svc, capture := newDryRunUserAdminService(t)
	page, err := svc.ListPage(context.Background(), &domain.UserAdminListREQ{RoleCode: "editor"})
	if err != nil {
		t.Fatalf("按角色过滤不应报错: %v", err)
	}
	if page.Total != 0 || len(page.Items) != 0 {
		t.Fatalf("DryRun 下应返回空页，实际 total=%d items=%d", page.Total, len(page.Items))
	}
	sqlText := capture.joined()
	if !strings.Contains(sqlText, "user_roles") || !strings.Contains(sqlText, "roles.role_code") {
		t.Fatalf("按角色过滤应走关联查询，实际 SQL: %s", sqlText)
	}

	// 环节2：未知角色编码只影响结果集，不应变成错误。
	unknownPage, err := svc.ListPage(context.Background(), &domain.UserAdminListREQ{RoleCode: "no_such_role"})
	if err != nil {
		t.Fatalf("未知角色编码应返回空页而非报错: %v", err)
	}
	if unknownPage.Total != 0 {
		t.Fatalf("未知角色编码应返回 total=0，实际 %d", unknownPage.Total)
	}

	// 环节3：不带角色编码时不得引入角色关联（否则同一用户多角色会重复出行）。
	plainSvc, plainCapture := newDryRunUserAdminService(t)
	if _, err := plainSvc.ListPage(context.Background(), &domain.UserAdminListREQ{}); err != nil {
		t.Fatalf("不带角色过滤不应报错: %v", err)
	}
	if strings.Contains(plainCapture.joined(), "user_roles") {
		t.Fatalf("无角色过滤时不应 JOIN 角色表，实际 SQL: %s", plainCapture.joined())
	}
}

// 菜单与权限链路的场景回归：导航过滤 → 菜单树装配 → 权限目录汇总。
// 三者是同一次「后端驱动导航」请求的上下游，因此合并为一条用例。

package service

import (
	"testing"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

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

// TestMenuAndPermissionChain 覆盖一次导航装配请求的完整链路：
// 可见性过滤 → 菜单树装配 → 权限目录汇总与去重。
func TestMenuAndPermissionChain(t *testing.T) {
	// 环节1：可见性过滤。动作节点、隐藏节点与未授权权限码都不进导航树；通配权限放行全部。
	visible := filterVisibleMenus(testMenus(), []string{"system:user:manage"})
	if len(visible) != 3 {
		t.Fatalf("可见菜单数 = %d, want 3", len(visible))
	}
	for _, do := range visible {
		if do.MenuTypeCode == domain.MenuTypeAction || !do.VisibleFlag {
			t.Fatalf("动作/隐藏节点不应进树: %s", do.MenuCode)
		}
		if do.PermCode != "" && do.PermCode != "system:user:manage" {
			t.Fatalf("未授权权限码不应进树: %s", do.MenuCode)
		}
	}
	if got := filterVisibleMenus(testMenus(), []string{domain.PermAdminAll}); len(got) != 4 {
		t.Fatalf("通配权限下可见菜单数 = %d, want 4", len(got))
	}

	// 环节2：菜单树装配。父子按 ULID 关联，动作节点不进树，权限码随节点保留。
	tree := buildMenuTree(filterVisibleMenus(testMenus(), []string{domain.PermAdminAll}), "")
	if len(tree) != 3 {
		t.Fatalf("根节点数 = %d, want 3", len(tree))
	}
	var group *domain.MenuTreeNodeVO
	for i := range tree {
		if tree[i].MenuCode == "group_system" {
			group = &tree[i]
		}
	}
	if group == nil {
		t.Fatal("未装配出系统管理分组")
	}
	if len(group.ChildList) != 1 || group.ChildList[0].MenuCode != "admin_users" {
		t.Fatalf("分组下级装配错误: %+v", group.ChildList)
	}
	if group.ChildList[0].PermCode != "system:user:manage" {
		t.Fatalf("节点权限码丢失: %+v", group.ChildList[0])
	}

	// 环节3：权限目录。通配独立成组、按首段分组、权限说明区分菜单与动作、重复权限码只登记一次。
	catalog := buildPermCatalog(testMenus())
	if len(catalog) != 3 {
		t.Fatalf("权限分组数 = %d, want 3（all/skill/system）", len(catalog))
	}
	if catalog[0].GroupCode != "all" || catalog[0].GroupName != "全部权限" {
		t.Fatalf("通配权限应独立成组且排首位: %+v", catalog[0])
	}
	if len(catalog[0].PermList) != 1 || catalog[0].PermList[0].PermCode != domain.PermAdminAll {
		t.Fatalf("通配权限组内容错误: %+v", catalog[0].PermList)
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
	if permDesc["system:user:add"] != "动作权限：新增用户" {
		t.Fatalf("动作权限说明错误: %v", permDesc)
	}
	if permDesc["system:user:manage"] != "菜单权限：系统管理 / 用户管理" {
		t.Fatalf("菜单权限说明错误: %v", permDesc)
	}

	dup := append(testMenus(), menuDO("MNU_7", "", "admin_users_copy", "用户管理副本", domain.MenuTypePage, "system:user:manage"))
	for _, catalogGroup := range buildPermCatalog(dup) {
		count := 0
		for _, item := range catalogGroup.PermList {
			if item.PermCode == "system:user:manage" {
				count++
			}
		}
		if count > 1 {
			t.Fatalf("权限码重复登记于目录 %d 次", count)
		}
	}
}

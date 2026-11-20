// 本文件覆盖菜单与权限链路的场景回归：导航过滤、菜单树装配、权限目录汇总与去重。

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

// TestFilterVisibleMenus 覆盖导航过滤：动作节点、隐藏节点、未授权权限码不进树，通配权限放行全部。
func TestFilterVisibleMenus(t *testing.T) {
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
}

// TestBuildMenuTree 覆盖菜单树装配：父子按 ULID 关联，动作节点不进树，权限码随节点保留。
func TestBuildMenuTree(t *testing.T) {
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
}

// TestBuildPermCatalog 覆盖权限目录：动作节点登记权限、通配独立分组、按首段分组、
// 重复权限码只登记一次，权限说明含菜单与动作两种格式。
func TestBuildPermCatalog(t *testing.T) {
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
	for _, group := range catalog {
		groupMap[group.GroupCode] = group
	}
	systemGroup, ok := groupMap["system"]
	if !ok || systemGroup.GroupName != "系统管理" || len(systemGroup.PermList) != 2 {
		t.Fatalf("system 分组错误: %+v", systemGroup)
	}
	permCodes := make(map[string]string, len(systemGroup.PermList))
	for _, item := range systemGroup.PermList {
		permCodes[item.PermCode] = item.PermDesc
	}
	if permCodes["system:user:add"] != "动作权限：新增用户" {
		t.Fatalf("动作权限说明错误: %v", permCodes)
	}
	if permCodes["system:user:manage"] != "菜单权限：系统管理 / 用户管理" {
		t.Fatalf("菜单权限说明错误: %v", permCodes)
	}

	dup := append(testMenus(), menuDO("MNU_7", "", "admin_users_copy", "用户管理副本", domain.MenuTypePage, "system:user:manage"))
	catalog = buildPermCatalog(dup)
	for _, group := range catalog {
		count := 0
		for _, item := range group.PermList {
			if item.PermCode == "system:user:manage" {
				count++
			}
		}
		if count > 1 {
			t.Fatalf("权限码重复登记于目录 %d 次", count)
		}
	}
}

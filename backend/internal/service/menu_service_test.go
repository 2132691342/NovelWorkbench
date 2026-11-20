// 本文件覆盖菜单与权限目录链路：导航可见性过滤 → 菜单树装配 → 权限目录汇总与分组。

package service

import (
	"testing"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// menuDO 构造测试用菜单行。
func menuDO(ulid, parentULID, code, name, menuType, permCode string) domain.MenuDO {
	return domain.MenuDO{
		MenuULID:     ulid,
		ParentULID:   parentULID,
		MenuCode:     code,
		MenuName:     name,
		MenuTypeCode: menuType,
		ScopeCode:    domain.MenuScopeGlobal,
		PermCode:     permCode,
		VisibleFlag:  true,
		EnabledFlag:  true,
	}
}

// TestMenuAndPermissionChain 覆盖菜单与权限目录链路。
func TestMenuAndPermissionChain(t *testing.T) {
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

	// ---- 环节一：导航可见性过滤（动作节点、隐藏节点与未授权权限码都不进树） ----
	visible := filterVisibleMenus(menus, []string{"system:user:manage"})
	if len(visible) != 3 {
		t.Fatalf("可见菜单数 = %d, want 3", len(visible))
	}
	for _, do := range visible {
		if do.MenuTypeCode == domain.MenuTypeAction {
			t.Fatalf("动作节点不应进导航树: %s", do.MenuCode)
		}
		if !do.VisibleFlag {
			t.Fatalf("隐藏菜单不应进导航树: %s", do.MenuCode)
		}
		if do.PermCode != "" && do.PermCode != "system:user:manage" {
			t.Fatalf("未授权权限码的菜单不应进导航树: %s", do.MenuCode)
		}
	}

	// 通配权限码放行全部带权限的菜单。
	visible = filterVisibleMenus(menus, []string{domain.PermAdminAll})
	if len(visible) != 4 {
		t.Fatalf("通配权限下可见菜单数 = %d, want 4", len(visible))
	}

	// ---- 环节二：菜单树装配（父子按 ULID 关联，孤儿节点不进树） ----
	visible = filterVisibleMenus(menus, []string{domain.PermAdminAll})
	tree := buildMenuTree(visible, "")
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

	// ---- 环节三：权限目录汇总（动作节点登记为权限，通配单独成组，重复权限码只登记一次） ----
	catalog := buildPermCatalog(menus)
	if len(catalog) != 3 {
		t.Fatalf("权限分组数 = %d, want 3（all/skill/system）", len(catalog))
	}
	if catalog[0].GroupCode != "all" {
		t.Fatalf("首个分组应为通配权限组，实际 %q", catalog[0].GroupCode)
	}
	if len(catalog[0].PermList) != 1 || catalog[0].PermList[0].PermCode != domain.PermAdminAll {
		t.Fatalf("通配权限组内容错误: %+v", catalog[0].PermList)
	}

	groupMap := make(map[string]domain.PermGroupVO, len(catalog))
	for _, group := range catalog {
		groupMap[group.GroupCode] = group
	}
	systemGroup, ok := groupMap["system"]
	if !ok {
		t.Fatal("缺少 system 权限分组")
	}
	if len(systemGroup.PermList) != 2 {
		t.Fatalf("system 分组权限数 = %d, want 2", len(systemGroup.PermList))
	}
	permCodes := map[string]string{}
	for _, item := range systemGroup.PermList {
		permCodes[item.PermCode] = item.PermDesc
	}
	if _, ok := permCodes["system:user:manage"]; !ok {
		t.Fatalf("system 分组缺少菜单权限: %+v", systemGroup.PermList)
	}
	actionDesc, ok := permCodes["system:user:add"]
	if !ok {
		t.Fatalf("system 分组缺少动作权限: %+v", systemGroup.PermList)
	}
	if actionDesc != "动作权限：新增用户" {
		t.Fatalf("动作权限说明 = %q", actionDesc)
	}
	menuDesc := permCodes["system:user:manage"]
	if menuDesc != "菜单权限：系统管理 / 用户管理" {
		t.Fatalf("菜单权限说明 = %q", menuDesc)
	}

	// ---- 环节四：重复登记的权限码只出现在目录一次 ----
	dup := append([]domain.MenuDO{}, menus...)
	dup = append(dup, menuDO("MNU_7", "", "admin_users_copy", "用户管理副本", domain.MenuTypePage, "system:user:manage"))
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

	// ---- 环节五：权限分组编码与展示名 ----
	if permGroupCode(domain.PermAdminAll) != "all" {
		t.Fatalf("通配权限分组 = %q", permGroupCode(domain.PermAdminAll))
	}
	if permGroupCode("system:user:manage") != "system" {
		t.Fatalf("系统权限分组 = %q", permGroupCode("system:user:manage"))
	}
	if permGroupCode("skill:manage") != "skill" {
		t.Fatalf("技能权限分组 = %q", permGroupCode("skill:manage"))
	}
	if permGroupCode("no_colon") != "other" {
		t.Fatalf("无分隔符权限分组 = %q", permGroupCode("no_colon"))
	}
	if permGroupLabel("all") != "全部权限" || permGroupLabel("system") != "系统管理" ||
		permGroupLabel("skill") != "技能" || permGroupLabel("project") != "项目" ||
		permGroupLabel("unknown") != "其他" {
		t.Fatal("权限分组展示名不匹配")
	}
}

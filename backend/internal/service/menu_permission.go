// 本文件实现权限目录：由菜单行汇总权限码，按首段分组返回给角色授权界面。

package service

import (
	"context"
	"sort"
	"strings"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
)

// PermissionCatalog 权限目录：按权限码首段分组，返回全部已登记权限。
func (s *MenuService) PermissionCatalog(ctx context.Context) ([]domain.PermGroupVO, error) {
	dos, err := s.menuRepo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	return buildPermCatalog(dos), nil
}

// buildPermCatalog 由菜单行汇总权限目录（纯函数）：
// 通配权限码单独登记；同一权限码重复登记只展示一次；分组按编码升序返回。
func buildPermCatalog(dos []domain.MenuDO) []domain.PermGroupVO {
	groupMap := make(map[string]*domain.PermGroupVO, 8)
	groupKeys := make([]string, 0, 8)
	seenPerm := make(map[string]bool, 16)
	selfName := make(map[string]string, len(dos))
	for i := range dos {
		selfName[dos[i].MenuULID] = dos[i].MenuName
	}
	appendPerm := func(permCode, permName, permDesc, menuCode, menuName string) {
		if seenPerm[permCode] {
			return // 同一权限码可能被历史数据重复登记，目录只展示一次
		}
		seenPerm[permCode] = true
		groupCode := permGroupCode(permCode)
		group, ok := groupMap[groupCode]
		if !ok {
			group = &domain.PermGroupVO{GroupCode: groupCode, GroupName: permGroupLabel(groupCode), PermList: []domain.PermItemVO{}}
			groupMap[groupCode] = group
			groupKeys = append(groupKeys, groupCode)
		}
		group.PermList = append(group.PermList, domain.PermItemVO{
			PermCode: permCode, PermName: permName, PermDesc: permDesc,
			MenuCode: menuCode, MenuName: menuName, BuiltinFlag: true,
		})
	}
	// 通配权限单独登记，便于角色页展示"全部权限"。
	appendPerm(domain.PermAdminAll, "全部权限", "超级管理员，拥有全部菜单与动作权限", "", "全部")
	for i := range dos {
		if dos[i].PermCode == "" || dos[i].PermCode == domain.PermAdminAll {
			continue
		}
		appendPerm(dos[i].PermCode, dos[i].MenuName, menuDescOf(&dos[i], selfName), dos[i].MenuCode, dos[i].MenuName)
	}
	sort.Slice(groupKeys, func(i, j int) bool { return groupKeys[i] < groupKeys[j] })
	result := make([]domain.PermGroupVO, 0, len(groupKeys))
	for _, key := range groupKeys {
		result = append(result, *groupMap[key])
	}
	return result
}

// menuDescOf 生成权限条目说明（动作节点标注为动作）。
func menuDescOf(do *domain.MenuDO, selfName map[string]string) string {
	if do.MenuTypeCode == domain.MenuTypeAction {
		return "动作权限：" + do.MenuName
	}
	if do.ParentULID != "" {
		if parentName, ok := selfName[do.ParentULID]; ok {
			return "菜单权限：" + parentName + " / " + do.MenuName
		}
	}
	return "菜单权限：" + do.MenuName
}

// permGroupCode 取权限码首段作为分组编码（system/prompt/tool/project）。
func permGroupCode(permCode string) string {
	if permCode == domain.PermAdminAll {
		return "all"
	}
	if index := strings.Index(permCode, ":"); index > 0 {
		return permCode[:index]
	}
	return "other"
}

// permGroupLabel 权限分组编码 → 展示名。
func permGroupLabel(groupCode string) string {
	switch groupCode {
	case "all":
		return "全部权限"
	case "system":
		return "系统管理"
	case "prompt":
		return "提示词"
	case "project":
		return "项目"
	default:
		return "其他"
	}
}

// toMenuVO 组装菜单视图对象。
func toMenuVO(do *domain.MenuDO, parentName string) domain.MenuVO {
	return domain.MenuVO{
		MenuULID:      do.MenuULID,
		ParentULID:    do.ParentULID,
		ParentName:    parentName,
		MenuCode:      do.MenuCode,
		MenuName:      do.MenuName,
		MenuTypeCode:  do.MenuTypeCode,
		ScopeCode:     do.ScopeCode,
		RoutePath:     do.RoutePath,
		ComponentPath: do.ComponentPath,
		IconCode:      do.IconCode,
		PermCode:      do.PermCode,
		VisibleFlag:   do.VisibleFlag,
		EnabledFlag:   do.EnabledFlag,
		SortOrder:     do.SortOrder,
		RemarkText:    do.RemarkText,
		CreatedAt:     pkg.FormatTime(do.CreatedAt),
		UpdatedAt:     pkg.FormatTime(do.UpdatedAt),
	}
}

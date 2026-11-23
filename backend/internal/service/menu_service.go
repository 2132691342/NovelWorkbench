// 本文件实现菜单业务：内置菜单播种、导航树装配与菜单 CRUD（权限目录见 menu_permission.go）。

package service

import (
	"context"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// menuSeed 内置菜单定义（编码 → 层级、路由、组件、图标、权限码；启动期按编码查缺补插）。
type menuSeed struct {
	MenuCode      string // 菜单编码
	ParentCode    string // 上级菜单编码
	MenuName      string // 菜单名称
	MenuTypeCode  string // 类型：group/page/action
	ScopeCode     string // 作用域：global/project
	RoutePath     string // 路由路径
	ComponentPath string // 前端组件路径（views/Xxx/index）
	IconCode      string // 图标编码
	PermCode      string // 权限码
	SortOrder     int    // 排序号
	HiddenFlag    bool   // 是否默认隐藏（被合并入口取代的旧页面，仍保留路由与权限码）
}

// menuSeedList 内置菜单清单（导航与权限登记的唯一来源）。
var menuSeedList = []menuSeed{
	{MenuCode: "dashboard", MenuName: "工作台", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeGlobal, RoutePath: "/dashboard", ComponentPath: "views/Dashboard/index", IconCode: "dashboard", SortOrder: 10},

	{MenuCode: "group_project", MenuName: "创作", MenuTypeCode: domain.MenuTypeGroup, ScopeCode: domain.MenuScopeGlobal, IconCode: "book", SortOrder: 20},
	{MenuCode: "workspace", ParentCode: "group_project", MenuName: "写作台", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeProject, RoutePath: "/workspace/:project_ulid", ComponentPath: "views/Workspace/index", IconCode: "edit", SortOrder: 10},
	{MenuCode: "outline", ParentCode: "group_project", MenuName: "创作规划", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeProject, RoutePath: "/outline/:project_ulid", ComponentPath: "views/Outline/index", IconCode: "tree", SortOrder: 20},
	{MenuCode: "continuity", ParentCode: "group_project", MenuName: "连续性看板", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeProject, RoutePath: "/continuity/:project_ulid", ComponentPath: "views/Continuity/index", IconCode: "shield", SortOrder: 30},
	{MenuCode: "material", ParentCode: "group_project", MenuName: "素材库", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeProject, RoutePath: "/material/:project_ulid", ComponentPath: "views/Material/index", IconCode: "library", SortOrder: 40},
	{MenuCode: "character", ParentCode: "group_project", MenuName: "角色管理", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeProject, RoutePath: "/character/:project_ulid", ComponentPath: "views/Character/index", IconCode: "user", SortOrder: 41, HiddenFlag: true},
	{MenuCode: "lore", ParentCode: "group_project", MenuName: "世界观设定", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeProject, RoutePath: "/lore/:project_ulid", ComponentPath: "views/Lore/index", IconCode: "globe", SortOrder: 42, HiddenFlag: true},
	{MenuCode: "knowledge", ParentCode: "group_project", MenuName: "知识库", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeProject, RoutePath: "/knowledge/:project_ulid", ComponentPath: "views/Knowledge/index", IconCode: "database", SortOrder: 43, HiddenFlag: true},
	{MenuCode: "notes", ParentCode: "group_project", MenuName: "素材笔记", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeProject, RoutePath: "/notes/:project_ulid", ComponentPath: "views/Notes/index", IconCode: "note", SortOrder: 44, HiddenFlag: true},

	{MenuCode: "group_platform", MenuName: "AI 资产", MenuTypeCode: domain.MenuTypeGroup, ScopeCode: domain.MenuScopeGlobal, IconCode: "layers", SortOrder: 30},
	{MenuCode: "models", ParentCode: "group_platform", MenuName: "模型供应商", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeGlobal, RoutePath: "/models", ComponentPath: "views/Models/index", IconCode: "model", PermCode: domain.PermModelManage, SortOrder: 10},
	{MenuCode: "skills", ParentCode: "group_platform", MenuName: "技能管理", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeGlobal, RoutePath: "/skills", ComponentPath: "views/Skills/index", IconCode: "wand", PermCode: domain.PermSkillManage, SortOrder: 20},
	{MenuCode: "tools", ParentCode: "group_platform", MenuName: "工具管理", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeGlobal, RoutePath: "/tools", ComponentPath: "views/Tools/index", IconCode: "tool", PermCode: domain.PermToolManage, SortOrder: 25},
	{MenuCode: "prompts", ParentCode: "group_platform", MenuName: "提示词档案", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeGlobal, RoutePath: "/prompts", ComponentPath: "views/Prompts/index", IconCode: "sparkles", PermCode: domain.PermPromptManage, SortOrder: 30},
	{MenuCode: "notifications", ParentCode: "group_platform", MenuName: "通知中心", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeGlobal, RoutePath: "/notifications", ComponentPath: "views/Notifications/index", IconCode: "bell", SortOrder: 40},

	{MenuCode: "group_system", MenuName: "系统", MenuTypeCode: domain.MenuTypeGroup, ScopeCode: domain.MenuScopeGlobal, IconCode: "settings", SortOrder: 40},
	{MenuCode: "admin_users", ParentCode: "group_system", MenuName: "用户管理", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeGlobal, RoutePath: "/admin/users", ComponentPath: "views/Admin/Users", IconCode: "users", PermCode: domain.PermUserManage, SortOrder: 10},
	{MenuCode: "admin_roles", ParentCode: "group_system", MenuName: "角色权限", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeGlobal, RoutePath: "/admin/roles", ComponentPath: "views/Admin/Roles", IconCode: "key", PermCode: domain.PermRoleManage, SortOrder: 20},
	{MenuCode: "admin_menus", ParentCode: "group_system", MenuName: "菜单管理", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeGlobal, RoutePath: "/admin/menus", ComponentPath: "views/Admin/Menus", IconCode: "menu", PermCode: domain.PermMenuManage, SortOrder: 30},
	{MenuCode: "admin_perms", ParentCode: "group_system", MenuName: "权限目录", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeGlobal, RoutePath: "/admin/permissions", ComponentPath: "views/Admin/Permissions", IconCode: "lock", PermCode: domain.PermPermissionRead, SortOrder: 40},
	{MenuCode: "admin_audit", ParentCode: "group_system", MenuName: "审计日志", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeGlobal, RoutePath: "/admin/audit", ComponentPath: "views/Admin/Audit", IconCode: "history", PermCode: domain.PermAuditRead, SortOrder: 50},
	{MenuCode: "admin_login_log", ParentCode: "group_system", MenuName: "登录日志", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeGlobal, RoutePath: "/admin/login-logs", ComponentPath: "views/Admin/LoginLogs", IconCode: "lock", PermCode: domain.PermLoginLogRead, SortOrder: 55},
	{MenuCode: "admin_usage", ParentCode: "group_system", MenuName: "用量分析", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeGlobal, RoutePath: "/admin/usage", ComponentPath: "views/Admin/Usage", IconCode: "chart", PermCode: domain.PermUsageRead, SortOrder: 60},
	{MenuCode: "admin_jobs", ParentCode: "group_system", MenuName: "任务中心", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeGlobal, RoutePath: "/admin/jobs", ComponentPath: "views/Admin/Jobs", IconCode: "pulse", PermCode: domain.PermJobManage, SortOrder: 65},
	{MenuCode: "admin_status", ParentCode: "group_system", MenuName: "运行状态", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeGlobal, RoutePath: "/admin/status", ComponentPath: "views/Admin/Status", IconCode: "pulse", PermCode: domain.PermSystemRead, SortOrder: 70},
	{MenuCode: "settings", MenuName: "系统设置", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeGlobal, RoutePath: "/settings", ComponentPath: "views/Settings/index", IconCode: "sliders", PermCode: domain.PermSettingManage, SortOrder: 50},

	// 动作权限（不进入导航，仅登记权限码，由角色勾选后控制接口）
	{MenuCode: "action_model_write", MenuName: "维护模型供应商", MenuTypeCode: domain.MenuTypeAction, ScopeCode: domain.MenuScopeGlobal, PermCode: domain.PermModelManage, SortOrder: 910},
}

// MenuService 菜单与权限目录业务。
type MenuService struct {
	menuRepo *repo.MenuRepo // 菜单仓储
	roleSvc  *RoleService   // 角色服务（权限集合解析）
}

// NewMenuService 构造菜单服务。
func NewMenuService(menuRepo *repo.MenuRepo, roleSvc *RoleService) *MenuService {
	return &MenuService{menuRepo: menuRepo, roleSvc: roleSvc}
}

// SeedDefaults 播种内置菜单（幂等：按 menu_code 查缺补插；已存在则同步内置字段，保证升级后路由与权限不漂移）。
func (s *MenuService) SeedDefaults(ctx context.Context) error {
	ulidByCode := make(map[string]string, len(menuSeedList))
	for _, item := range menuSeedList {
		existing, err := s.menuRepo.GetByCode(ctx, item.MenuCode)
		if err != nil {
			return err
		}
		if existing != nil {
			ulidByCode[item.MenuCode] = existing.MenuULID
			if err := s.syncSeed(ctx, existing, item); err != nil {
				return err
			}
			continue
		}
		parentULID := ""
		if item.ParentCode != "" {
			resolved, ok := ulidByCode[item.ParentCode]
			if !ok {
				return pkg.NewBizError(domain.RespCodeInternalError, "菜单播种顺序错误，缺少上级: "+item.ParentCode)
			}
			parentULID = resolved
		}
		do := &domain.MenuDO{
			MenuULID:      pkg.NewULID(domain.IDPrefixMenu),
			ParentULID:    parentULID,
			MenuCode:      item.MenuCode,
			MenuName:      item.MenuName,
			MenuTypeCode:  item.MenuTypeCode,
			ScopeCode:     item.ScopeCode,
			RoutePath:     item.RoutePath,
			ComponentPath: item.ComponentPath,
			IconCode:      item.IconCode,
			PermCode:      item.PermCode,
			VisibleFlag:   !item.HiddenFlag,
			EnabledFlag:   true,
			SortOrder:     item.SortOrder,
		}
		if item.ScopeCode == "" {
			do.ScopeCode = domain.MenuScopeGlobal
		}
		if err := s.menuRepo.Create(ctx, do); err != nil {
			return err
		}
		ulidByCode[item.MenuCode] = do.MenuULID
	}
	return nil
}

// syncSeed 用内置定义校正已存在的菜单行（只改与代码不一致的字段，保留作者调整的展示开关与备注）。
func (s *MenuService) syncSeed(ctx context.Context, existing *domain.MenuDO, item menuSeed) error {
	fields := map[string]any{}
	if existing.MenuName != item.MenuName {
		fields["menu_name"] = item.MenuName
	}
	if existing.MenuTypeCode != item.MenuTypeCode {
		fields["menu_type_code"] = item.MenuTypeCode
	}
	if existing.ScopeCode != item.ScopeCode && item.ScopeCode != "" {
		fields["scope_code"] = item.ScopeCode
	}
	if existing.RoutePath != item.RoutePath {
		fields["route_path"] = item.RoutePath
	}
	if existing.ComponentPath != item.ComponentPath {
		fields["component_path"] = item.ComponentPath
	}
	if existing.IconCode != item.IconCode {
		fields["icon_code"] = item.IconCode
	}
	if existing.PermCode != item.PermCode {
		fields["perm_code"] = item.PermCode
	}
	if existing.SortOrder != item.SortOrder {
		fields["sort_order"] = item.SortOrder
	}
	// 入口已被合并的内置页强制隐藏：这是产品决策而非作者偏好，必须随代码生效；
	// 其余菜单仍尊重作者在管理端调整的展示开关。
	if item.HiddenFlag && existing.VisibleFlag {
		fields["visible_flag"] = false
	}
	if len(fields) == 0 {
		return nil
	}
	return s.menuRepo.Update(ctx, existing.MenuULID, fields)
}

// Nav 装配当前用户的导航树与权限码集合（禁用/隐藏菜单与动作节点不进树）。
func (s *MenuService) Nav(ctx context.Context, userULID string) (domain.MenuNavDTO, error) {
	permCodes, err := s.roleSvc.PermCodesOfUser(ctx, userULID)
	if err != nil {
		return domain.MenuNavDTO{}, err
	}
	dos, err := s.menuRepo.ListEnabled(ctx)
	if err != nil {
		return domain.MenuNavDTO{}, err
	}
	tree := buildMenuTree(filterVisibleMenus(dos, permCodes), "")
	hitPerms := permCodes
	if hitPerms == nil {
		hitPerms = []string{}
	}
	return domain.MenuNavDTO{MenuTreeList: tree, PermCodeList: hitPerms}, nil
}

// filterVisibleMenus 过滤出可下发菜单（纯函数）：
// 动作节点不下发；隐藏节点保留（visible_flag=false）以便前端注册路由，深链与旧链接仍可用；
// 带权限码的菜单需命中用户权限，通配权限码（*）放行全部。
func filterVisibleMenus(dos []domain.MenuDO, permCodes []string) []domain.MenuDO {
	allowAll := containsString(permCodes, domain.PermAdminAll)
	visible := make([]domain.MenuDO, 0, len(dos))
	for i := range dos {
		if dos[i].MenuTypeCode == domain.MenuTypeAction {
			continue
		}
		if dos[i].PermCode != "" && !allowAll && !containsString(permCodes, dos[i].PermCode) {
			continue
		}
		visible = append(visible, dos[i])
	}
	return visible
}

// buildMenuTree 由平铺菜单构建树（父子按 ULID 关联，同级保持排序号顺序）。
func buildMenuTree(dos []domain.MenuDO, parentULID string) []domain.MenuTreeNodeVO {
	nodes := make([]domain.MenuTreeNodeVO, 0, len(dos))
	for i := range dos {
		if dos[i].ParentULID != parentULID {
			continue
		}
		node := domain.MenuTreeNodeVO{
			MenuCode:      dos[i].MenuCode,
			MenuName:      dos[i].MenuName,
			MenuTypeCode:  dos[i].MenuTypeCode,
			ScopeCode:     dos[i].ScopeCode,
			RoutePath:     dos[i].RoutePath,
			ComponentPath: dos[i].ComponentPath,
			IconCode:      dos[i].IconCode,
			PermCode:      dos[i].PermCode,
			VisibleFlag:   dos[i].VisibleFlag,
			ChildList:     buildMenuTree(dos, dos[i].MenuULID),
		}
		nodes = append(nodes, node)
	}
	return nodes
}

// List 查询菜单平铺列表（管理端，含禁用/隐藏/动作节点）。
func (s *MenuService) List(ctx context.Context, req *domain.MenuListREQ) ([]domain.MenuVO, error) {
	dos, err := s.menuRepo.List(ctx, req)
	if err != nil {
		return nil, err
	}
	nameByULID := make(map[string]string, len(dos))
	for i := range dos {
		nameByULID[dos[i].MenuULID] = dos[i].MenuName
	}
	vos := make([]domain.MenuVO, 0, len(dos))
	for i := range dos {
		vos = append(vos, toMenuVO(&dos[i], nameByULID[dos[i].ParentULID]))
	}
	return vos, nil
}

// Detail 查询菜单详情（ID 置于路径末尾）。
func (s *MenuService) Detail(ctx context.Context, menuULID string) (domain.MenuVO, error) {
	do, err := s.menuRepo.GetByULID(ctx, menuULID)
	if err != nil {
		return domain.MenuVO{}, err
	}
	if do == nil {
		return domain.MenuVO{}, pkg.NewBizError(domain.RespCodeNotFound, "菜单不存在")
	}
	parentName := ""
	if do.ParentULID != "" {
		parent, parentErr := s.menuRepo.GetByULID(ctx, do.ParentULID)
		if parentErr != nil {
			return domain.MenuVO{}, parentErr
		}
		if parent != nil {
			parentName = parent.MenuName
		}
	}
	return toMenuVO(do, parentName), nil
}

// Save 创建或更新菜单（编码唯一、权限码唯一、上级必须存在且不成环）。
func (s *MenuService) Save(ctx context.Context, req *domain.MenuREQ) (domain.MenuVO, error) {
	scopeCode := req.ScopeCode
	if scopeCode == "" {
		scopeCode = domain.MenuScopeGlobal
	}
	if req.MenuTypeCode == domain.MenuTypeGroup && req.RoutePath != "" {
		return domain.MenuVO{}, pkg.NewBizError(domain.RespCodeInvalidParam, "分组不能配置路由")
	}
	if err := s.validateParent(ctx, req.MenuULID, req.ParentULID); err != nil {
		return domain.MenuVO{}, err
	}
	existingByCode, err := s.menuRepo.GetByCode(ctx, req.MenuCode)
	if err != nil {
		return domain.MenuVO{}, err
	}
	if existingByCode != nil && existingByCode.MenuULID != req.MenuULID {
		return domain.MenuVO{}, pkg.NewBizError(domain.RespCodeConflict, "菜单编码已存在")
	}
	if req.PermCode != "" {
		usedCount, countErr := s.menuRepo.CountByPermCode(ctx, req.PermCode, req.MenuULID)
		if countErr != nil {
			return domain.MenuVO{}, countErr
		}
		if usedCount > 0 {
			return domain.MenuVO{}, pkg.NewBizError(domain.RespCodeConflict, "权限码已被其他菜单登记: "+req.PermCode)
		}
	}
	visibleFlag, enabledFlag := true, true
	if req.VisibleFlag != nil {
		visibleFlag = *req.VisibleFlag
	}
	if req.EnabledFlag != nil {
		enabledFlag = *req.EnabledFlag
	}
	if req.MenuULID == "" {
		do := &domain.MenuDO{
			MenuULID:      pkg.NewULID(domain.IDPrefixMenu),
			ParentULID:    req.ParentULID,
			MenuCode:      req.MenuCode,
			MenuName:      req.MenuName,
			MenuTypeCode:  req.MenuTypeCode,
			ScopeCode:     scopeCode,
			RoutePath:     req.RoutePath,
			ComponentPath: req.ComponentPath,
			IconCode:      req.IconCode,
			PermCode:      req.PermCode,
			VisibleFlag:   visibleFlag,
			EnabledFlag:   enabledFlag,
			SortOrder:     req.SortOrder,
			RemarkText:    req.RemarkText,
		}
		if err := s.menuRepo.Create(ctx, do); err != nil {
			return domain.MenuVO{}, err
		}
		return toMenuVO(do, ""), nil
	}
	current, err := s.menuRepo.GetByULID(ctx, req.MenuULID)
	if err != nil {
		return domain.MenuVO{}, err
	}
	if current == nil {
		return domain.MenuVO{}, pkg.NewBizError(domain.RespCodeNotFound, "菜单不存在")
	}
	fields := map[string]any{
		"parent_ulid":    req.ParentULID,
		"menu_code":      req.MenuCode,
		"menu_name":      req.MenuName,
		"menu_type_code": req.MenuTypeCode,
		"scope_code":     scopeCode,
		"route_path":     req.RoutePath,
		"component_path": req.ComponentPath,
		"icon_code":      req.IconCode,
		"perm_code":      req.PermCode,
		"visible_flag":   visibleFlag,
		"enabled_flag":   enabledFlag,
		"sort_order":     req.SortOrder,
		"remark_text":    req.RemarkText,
	}
	if err := s.menuRepo.Update(ctx, req.MenuULID, fields); err != nil {
		return domain.MenuVO{}, err
	}
	current.ParentULID = req.ParentULID
	current.MenuCode = req.MenuCode
	current.MenuName = req.MenuName
	current.MenuTypeCode = req.MenuTypeCode
	current.ScopeCode = scopeCode
	current.RoutePath = req.RoutePath
	current.ComponentPath = req.ComponentPath
	current.IconCode = req.IconCode
	current.PermCode = req.PermCode
	current.VisibleFlag = visibleFlag
	current.EnabledFlag = enabledFlag
	current.SortOrder = req.SortOrder
	current.RemarkText = req.RemarkText
	return toMenuVO(current, ""), nil
}

// Delete 删除菜单（存在下级时拒绝，避免悬空节点）。
func (s *MenuService) Delete(ctx context.Context, req *domain.MenuDeleteREQ) error {
	current, err := s.menuRepo.GetByULID(ctx, req.MenuULID)
	if err != nil {
		return err
	}
	if current == nil {
		return pkg.NewBizError(domain.RespCodeNotFound, "菜单不存在")
	}
	children, err := s.menuRepo.CountChildren(ctx, req.MenuULID)
	if err != nil {
		return err
	}
	if children > 0 {
		return pkg.NewBizError(domain.RespCodeConflict, "请先删除下级菜单")
	}
	return s.menuRepo.Delete(ctx, req.MenuULID)
}

// validateParent 校验上级菜单：不能指向自己或自己的下级（成环检测）。
func (s *MenuService) validateParent(ctx context.Context, menuULID, parentULID string) error {
	if parentULID == "" {
		return nil
	}
	if parentULID == menuULID {
		return pkg.NewBizError(domain.RespCodeInvalidParam, "上级菜单不能是自己")
	}
	cursor := parentULID
	for depth := 0; cursor != ""; depth++ {
		if depth > 32 {
			return pkg.NewBizError(domain.RespCodeInvalidParam, "菜单层级过深或存在环")
		}
		if cursor == menuULID {
			return pkg.NewBizError(domain.RespCodeInvalidParam, "上级菜单不能是自己的下级")
		}
		parent, err := s.menuRepo.GetByULID(ctx, cursor)
		if err != nil {
			return err
		}
		if parent == nil {
			return pkg.NewBizError(domain.RespCodeInvalidParam, "上级菜单不存在")
		}
		cursor = parent.ParentULID
	}
	return nil
}

// containsString 判断字符串是否在切片中。
func containsString(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}

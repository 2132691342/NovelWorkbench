// 本文件实现菜单与权限目录业务：内置菜单播种、导航树装配、菜单 CRUD 与权限目录。

package service

import (
	"context"
	"sort"
	"strings"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// menuSeed 内置菜单定义（编码 → 层级、路由、图标、权限码；启动期按编码查缺补插）。
type menuSeed struct {
	MenuCode     string // 菜单编码
	ParentCode   string // 上级菜单编码
	MenuName     string // 菜单名称
	MenuTypeCode string // 类型：group/page/action
	ScopeCode    string // 作用域：global/project
	RoutePath    string // 路由路径
	IconCode     string // 图标编码
	PermCode     string // 权限码
	SortOrder    int    // 排序号
}

// menuSeedList 内置菜单清单（导航与权限登记的唯一来源）。
var menuSeedList = []menuSeed{
	{MenuCode: "dashboard", MenuName: "仪表盘", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeGlobal, RoutePath: "/", IconCode: "dashboard", SortOrder: 10},

	{MenuCode: "group_project", MenuName: "创作", MenuTypeCode: domain.MenuTypeGroup, ScopeCode: domain.MenuScopeGlobal, IconCode: "book", SortOrder: 20},
	{MenuCode: "workspace", ParentCode: "group_project", MenuName: "写作台", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeProject, RoutePath: "/workspace/:project_ulid", IconCode: "edit", SortOrder: 10},
	{MenuCode: "outline", ParentCode: "group_project", MenuName: "创作规划", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeProject, RoutePath: "/outline/:project_ulid", IconCode: "tree", SortOrder: 20},
	{MenuCode: "continuity", ParentCode: "group_project", MenuName: "连续性看板", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeProject, RoutePath: "/continuity/:project_ulid", IconCode: "shield", SortOrder: 30},
	{MenuCode: "character", ParentCode: "group_project", MenuName: "角色管理", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeProject, RoutePath: "/character/:project_ulid", IconCode: "user", SortOrder: 40},
	{MenuCode: "lore", ParentCode: "group_project", MenuName: "世界观设定", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeProject, RoutePath: "/lore/:project_ulid", IconCode: "globe", SortOrder: 50},
	{MenuCode: "knowledge", ParentCode: "group_project", MenuName: "知识库", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeProject, RoutePath: "/knowledge/:project_ulid", IconCode: "database", SortOrder: 60},
	{MenuCode: "notes", ParentCode: "group_project", MenuName: "素材笔记", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeProject, RoutePath: "/notes/:project_ulid", IconCode: "note", SortOrder: 70},

	{MenuCode: "group_platform", MenuName: "平台", MenuTypeCode: domain.MenuTypeGroup, ScopeCode: domain.MenuScopeGlobal, IconCode: "layers", SortOrder: 30},
	{MenuCode: "models", ParentCode: "group_platform", MenuName: "模型供应商", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeGlobal, RoutePath: "/models", IconCode: "model", SortOrder: 10},
	{MenuCode: "skills", ParentCode: "group_platform", MenuName: "技能管理", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeGlobal, RoutePath: "/skills", IconCode: "wand", PermCode: domain.PermSkillManage, SortOrder: 20},
	{MenuCode: "prompts", ParentCode: "group_platform", MenuName: "提示词档案", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeGlobal, RoutePath: "/prompts", IconCode: "sparkles", PermCode: domain.PermPromptManage, SortOrder: 25},
	{MenuCode: "notifications", ParentCode: "group_platform", MenuName: "通知中心", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeGlobal, RoutePath: "/notifications", IconCode: "bell", SortOrder: 30},

	{MenuCode: "group_system", MenuName: "系统", MenuTypeCode: domain.MenuTypeGroup, ScopeCode: domain.MenuScopeGlobal, IconCode: "settings", SortOrder: 40},
	{MenuCode: "admin_users", ParentCode: "group_system", MenuName: "用户管理", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeGlobal, RoutePath: "/admin/users", IconCode: "users", PermCode: domain.PermUserManage, SortOrder: 10},
	{MenuCode: "admin_roles", ParentCode: "group_system", MenuName: "角色权限", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeGlobal, RoutePath: "/admin/roles", IconCode: "key", PermCode: domain.PermRoleManage, SortOrder: 20},
	{MenuCode: "admin_menus", ParentCode: "group_system", MenuName: "菜单管理", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeGlobal, RoutePath: "/admin/menus", IconCode: "menu", PermCode: domain.PermMenuManage, SortOrder: 30},
	{MenuCode: "admin_perms", ParentCode: "group_system", MenuName: "权限目录", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeGlobal, RoutePath: "/admin/permissions", IconCode: "lock", PermCode: domain.PermPermissionRead, SortOrder: 40},
	{MenuCode: "admin_audit", ParentCode: "group_system", MenuName: "审计日志", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeGlobal, RoutePath: "/admin/audit", IconCode: "history", PermCode: domain.PermAuditRead, SortOrder: 50},
	{MenuCode: "admin_login_log", ParentCode: "group_system", MenuName: "登录日志", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeGlobal, RoutePath: "/admin/login-logs", IconCode: "lock", PermCode: domain.PermLoginLogRead, SortOrder: 55},
	{MenuCode: "admin_usage", ParentCode: "group_system", MenuName: "用量分析", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeGlobal, RoutePath: "/admin/usage", IconCode: "chart", PermCode: domain.PermUsageRead, SortOrder: 60},
	{MenuCode: "admin_jobs", ParentCode: "group_system", MenuName: "任务中心", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeGlobal, RoutePath: "/admin/jobs", IconCode: "pulse", PermCode: domain.PermJobManage, SortOrder: 65},
	{MenuCode: "admin_status", ParentCode: "group_system", MenuName: "运行状态", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeGlobal, RoutePath: "/admin/status", IconCode: "pulse", PermCode: domain.PermSystemRead, SortOrder: 70},
	{MenuCode: "settings", MenuName: "系统设置", MenuTypeCode: domain.MenuTypePage, ScopeCode: domain.MenuScopeGlobal, RoutePath: "/settings", IconCode: "sliders", PermCode: domain.PermSettingManage, SortOrder: 50},

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

// SeedDefaults 播种内置菜单（幂等：按 menu_code 查缺补插，父级编码在建子级前解析）。
func (s *MenuService) SeedDefaults(ctx context.Context) error {
	ulidByCode := make(map[string]string, len(menuSeedList))
	for _, item := range menuSeedList {
		existing, err := s.menuRepo.GetByCode(ctx, item.MenuCode)
		if err != nil {
			return err
		}
		if existing != nil {
			ulidByCode[item.MenuCode] = existing.MenuULID
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
			MenuULID:     pkg.NewULID(domain.IDPrefixMenu),
			ParentULID:   parentULID,
			MenuCode:     item.MenuCode,
			MenuName:     item.MenuName,
			MenuTypeCode: item.MenuTypeCode,
			ScopeCode:    item.ScopeCode,
			RoutePath:    item.RoutePath,
			IconCode:     item.IconCode,
			PermCode:     item.PermCode,
			VisibleFlag:  true,
			EnabledFlag:  true,
			SortOrder:    item.SortOrder,
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

// filterVisibleMenus 过滤出导航可见菜单（纯函数）：
// 动作节点与隐藏节点不进树；带权限码的菜单需命中用户权限，通配权限码（*）放行全部。
func filterVisibleMenus(dos []domain.MenuDO, permCodes []string) []domain.MenuDO {
	allowAll := containsString(permCodes, domain.PermAdminAll)
	visible := make([]domain.MenuDO, 0, len(dos))
	for i := range dos {
		if dos[i].MenuTypeCode == domain.MenuTypeAction || !dos[i].VisibleFlag {
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
			MenuCode:     dos[i].MenuCode,
			MenuName:     dos[i].MenuName,
			MenuTypeCode: dos[i].MenuTypeCode,
			ScopeCode:    dos[i].ScopeCode,
			RoutePath:    dos[i].RoutePath,
			IconCode:     dos[i].IconCode,
			PermCode:     dos[i].PermCode,
			ChildList:    buildMenuTree(dos, dos[i].MenuULID),
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
			MenuULID:     pkg.NewULID(domain.IDPrefixMenu),
			ParentULID:   req.ParentULID,
			MenuCode:     req.MenuCode,
			MenuName:     req.MenuName,
			MenuTypeCode: req.MenuTypeCode,
			ScopeCode:    scopeCode,
			RoutePath:    req.RoutePath,
			IconCode:     req.IconCode,
			PermCode:     req.PermCode,
			VisibleFlag:  visibleFlag,
			EnabledFlag:  enabledFlag,
			SortOrder:    req.SortOrder,
			RemarkText:   req.RemarkText,
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

// permGroupCode 取权限码首段作为分组编码（system/skill/project）。
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
	case "skill":
		return "技能"
	case "project":
		return "项目"
	default:
		return "其他"
	}
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

// toMenuVO 组装菜单视图对象。
func toMenuVO(do *domain.MenuDO, parentName string) domain.MenuVO {
	return domain.MenuVO{
		MenuULID:     do.MenuULID,
		ParentULID:   do.ParentULID,
		ParentName:   parentName,
		MenuCode:     do.MenuCode,
		MenuName:     do.MenuName,
		MenuTypeCode: do.MenuTypeCode,
		ScopeCode:    do.ScopeCode,
		RoutePath:    do.RoutePath,
		IconCode:     do.IconCode,
		PermCode:     do.PermCode,
		VisibleFlag:  do.VisibleFlag,
		EnabledFlag:  do.EnabledFlag,
		SortOrder:    do.SortOrder,
		RemarkText:   do.RemarkText,
		CreatedAtMs:  do.CreatedAt.UnixMilli(),
		UpdatedAtMs:  do.UpdatedAt.UnixMilli(),
	}
}

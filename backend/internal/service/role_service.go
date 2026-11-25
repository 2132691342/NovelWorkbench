// 本文件实现角色业务：内置角色播种、分页查询、保存与删除。

package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// roleDefaultList 启动播种的内置角色（编码/名称/权限的唯一来源）。
var roleDefaultList = []domain.RoleDO{
	{RoleCode: domain.RoleCodeAdmin, RoleName: "管理员", BuiltinFlag: true},
	{RoleCode: domain.RoleCodeUser, RoleName: "普通用户", BuiltinFlag: true},
}

// RoleService 角色与权限业务：内置角色播种、角色 CRUD、用户授权、权限判定。
type RoleService struct {
	roleRepo     *repo.RoleRepo          // 角色仓储
	userRepo     *repo.UserRepo          // 用户仓储
	menuRepo     *repo.MenuRepo          // 菜单仓储（权限码登记处，授权时校验）
	providerRepo *repo.ModelProviderRepo // 模型生态仓储（删角色时清理模型授权）
}

// NewRoleService 构造角色服务。
func NewRoleService(roleRepo *repo.RoleRepo, userRepo *repo.UserRepo, menuRepo *repo.MenuRepo,
	providerRepo *repo.ModelProviderRepo) *RoleService {
	return &RoleService{roleRepo: roleRepo, userRepo: userRepo, menuRepo: menuRepo, providerRepo: providerRepo}
}

// SeedDefaults 播种内置角色（幂等：按 role_code 查缺补插），并修复历史脏权限。
// 修复只做「减法」：剔除未登记权限码与非管理员角色上的通配 "*"，管理员手工授予的合法权限保留。
func (s *RoleService) SeedDefaults(ctx context.Context) error {
	knownCodes, err := s.knownPermCodes(ctx)
	if err != nil {
		return err
	}
	registryReady := len(knownCodes) > 0 // 权限目录为空（首次启动，菜单尚未播种）时不做减法
	for _, item := range roleDefaultList {
		existing, err := s.roleRepo.GetByCode(ctx, item.RoleCode)
		if err != nil {
			return err
		}
		if existing == nil {
			permCodes := []string(nil)
			if item.RoleCode == domain.RoleCodeAdmin {
				permCodes = []string{domain.PermAdminAll}
			}
			do := &domain.RoleDO{
				RoleULID:      pkg.NewULID(domain.IDPrefixRole),
				RoleCode:      item.RoleCode,
				RoleName:      item.RoleName,
				PermCodesJSON: marshalPermCodes(permCodes),
				BuiltinFlag:   true,
			}
			if err := s.roleRepo.Create(ctx, do); err != nil {
				return err
			}
			continue
		}
		if !registryReady {
			continue
		}
		if err := s.repairPermCodes(ctx, existing, knownCodes); err != nil {
			return err
		}
	}
	return nil
}

// builtinRolePermCodes 管理员角色的权限是代码资产（固定 "*"）；其余内置角色权限由管理员在界面维护。
func builtinRolePermCodes(roleCode string) []string {
	if roleCode == domain.RoleCodeAdmin {
		return []string{domain.PermAdminAll}
	}
	return nil
}

// repairPermCodes 修正内置角色的历史脏权限：
// 管理员固定为 "*"；非管理员剔除通配 "*"（曾导致权限面板被锁死、无法再取消）与未登记的权限码。
func (s *RoleService) repairPermCodes(ctx context.Context, current *domain.RoleDO, knownCodes map[string]bool) error {
	kept := repairedPermCodeList(current, knownCodes)
	actual := unmarshalPermCodes(current.PermCodesJSON)
	if sameStringSlice(kept, actual) {
		return nil
	}
	pkg.SLogger().Warnw("内置角色权限码修复", "role_code", current.RoleCode,
		"before_count", len(actual), "after_count", len(kept))
	do := &domain.RoleDO{
		RoleULID:      current.RoleULID,
		RoleCode:      current.RoleCode,
		RoleName:      current.RoleName,
		RemarkText:    current.RemarkText,
		PermCodesJSON: marshalPermCodes(kept),
	}
	return s.roleRepo.Update(ctx, do)
}

// repairedPermCodeList 内置角色权限修复的纯函数实现。
func repairedPermCodeList(current *domain.RoleDO, knownCodes map[string]bool) []string {
	expected := builtinRolePermCodes(current.RoleCode)
	if expected != nil {
		return expected
	}
	actual := unmarshalPermCodes(current.PermCodesJSON)
	kept := make([]string, 0, len(actual))
	for _, permCode := range actual {
		if permCode == domain.PermAdminAll {
			continue
		}
		if knownCodes[permCode] {
			kept = append(kept, permCode)
		}
	}
	return kept
}

// sameStringSlice 判断两个权限码切片是否完全一致（顺序敏感，权限码本就有稳定顺序）。
func sameStringSlice(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

// ListPage 分页查询角色。
func (s *RoleService) ListPage(ctx context.Context, req *domain.RoleListREQ) (domain.PageDTO[domain.RoleVO], error) {
	req.Normalize()
	dos, total, err := s.roleRepo.ListPage(ctx, req.PageNum, req.PageSize)
	if err != nil {
		return domain.PageDTO[domain.RoleVO]{}, err
	}
	vos := make([]domain.RoleVO, 0, len(dos))
	for i := range dos {
		vos = append(vos, toRoleVO(&dos[i]))
	}
	return domain.NewPageDTO(total, req.PageREQ, vos), nil
}

// Save 创建或更新角色（role_ulid 为空创建；角色编码唯一；权限码必须在权限目录内登记）。
func (s *RoleService) Save(ctx context.Context, req *domain.RoleSaveREQ) (domain.RoleVO, error) {
	existing, err := s.roleRepo.GetByCode(ctx, req.RoleCode)
	if err != nil {
		return domain.RoleVO{}, err
	}
	var current *domain.RoleDO
	if req.RoleULID != "" {
		current, err = s.roleRepo.GetByULID(ctx, req.RoleULID)
		if err != nil {
			return domain.RoleVO{}, err
		}
		if current == nil {
			return domain.RoleVO{}, pkg.NewBizError(domain.RespCodeNotFound, "角色不存在")
		}
		if current.BuiltinFlag && current.RoleCode != req.RoleCode {
			return domain.RoleVO{}, pkg.NewBizError(domain.RespCodeBizFailed, "内置角色不允许修改编码")
		}
	}
	// 未提交 perm_codes 表示「不修改权限」，避免只想改名称的请求把权限清空
	requestedPermCodes := req.PermCodes
	if requestedPermCodes == nil {
		requestedPermCodes = []string{}
		if current != nil {
			requestedPermCodes = unmarshalPermCodes(current.PermCodesJSON)
		}
	}
	// 管理员角色的通配权限是系统不变量：任何保存都不会把它改没
	if req.RoleCode == domain.RoleCodeAdmin {
		requestedPermCodes = []string{domain.PermAdminAll}
	}
	permCodes, err := s.normalizePermCodes(ctx, req.RoleCode, requestedPermCodes)
	if err != nil {
		return domain.RoleVO{}, err
	}
	permCodesJSON := marshalPermCodes(permCodes)
	if req.RoleULID == "" {
		if existing != nil {
			return domain.RoleVO{}, pkg.NewBizError(domain.RespCodeConflict, "角色编码已存在")
		}
		do := &domain.RoleDO{
			RoleULID:      pkg.NewULID(domain.IDPrefixRole),
			RoleCode:      req.RoleCode,
			RoleName:      req.RoleName,
			RemarkText:    req.RemarkText,
			PermCodesJSON: permCodesJSON,
		}
		if err := s.roleRepo.Create(ctx, do); err != nil {
			return domain.RoleVO{}, err
		}
		created, err := s.roleRepo.GetByULID(ctx, do.RoleULID)
		if err != nil {
			return domain.RoleVO{}, err
		}
		return toRoleVO(created), nil
	}
	if existing != nil && existing.RoleULID != req.RoleULID {
		return domain.RoleVO{}, pkg.NewBizError(domain.RespCodeConflict, "角色编码已存在")
	}
	do := &domain.RoleDO{
		RoleULID:      req.RoleULID,
		RoleCode:      req.RoleCode,
		RoleName:      req.RoleName,
		RemarkText:    req.RemarkText,
		PermCodesJSON: permCodesJSON,
	}
	if err := s.roleRepo.Update(ctx, do); err != nil {
		return domain.RoleVO{}, err
	}
	updated, err := s.roleRepo.GetByULID(ctx, req.RoleULID)
	if err != nil {
		return domain.RoleVO{}, err
	}
	return toRoleVO(updated), nil
}

// normalizePermCodes 校验并规范化权限码：去重保序、剔除空值，
// 通配符 "*" 仅管理员可用，其余权限码必须已在菜单中登记（否则前端勾不上、接口也不认）。
func (s *RoleService) normalizePermCodes(ctx context.Context, roleCode string, permCodes []string) ([]string, error) {
	if len(permCodes) == 0 {
		return []string{}, nil
	}
	knownCodes, err := s.knownPermCodes(ctx)
	if err != nil {
		return nil, err
	}
	return normalizePermCodeList(roleCode, permCodes, knownCodes)
}

// normalizePermCodeList 权限码规范化的纯函数实现（测试与业务共用同一份规则）。
func normalizePermCodeList(roleCode string, permCodes []string, knownCodes map[string]bool) ([]string, error) {
	seen := make(map[string]bool, len(permCodes))
	result := make([]string, 0, len(permCodes))
	for _, permCode := range permCodes {
		permCode = strings.TrimSpace(permCode)
		if permCode == "" || seen[permCode] {
			continue
		}
		if permCode == domain.PermAdminAll {
			if roleCode != domain.RoleCodeAdmin {
				return nil, pkg.NewBizError(domain.RespCodeInvalidParam, `通配权限 "*" 仅管理员角色可用`)
			}
			seen[permCode] = true
			result = append(result, permCode)
			continue
		}
		if !knownCodes[permCode] {
			return nil, pkg.NewBizError(domain.RespCodeInvalidParam, "权限码未在权限目录登记: "+permCode)
		}
		seen[permCode] = true
		result = append(result, permCode)
	}
	return result, nil
}

// knownPermCodes 读取当前已登记的权限码集合（菜单是权限码的唯一登记处）。
func (s *RoleService) knownPermCodes(ctx context.Context) (map[string]bool, error) {
	dos, err := s.menuRepo.ListAll(ctx)
	if err != nil {
		return nil, pkg.WrapBizError(domain.RespCodeStorageError, "权限目录读取失败", err)
	}
	known := make(map[string]bool, len(dos))
	for i := range dos {
		if dos[i].PermCode != "" && dos[i].EnabledFlag {
			known[dos[i].PermCode] = true
		}
	}
	return known, nil
}

// Delete 删除角色（内置角色或仍有用户挂载时拒绝）。
func (s *RoleService) Delete(ctx context.Context, req *domain.RoleDeleteREQ) error {
	current, err := s.roleRepo.GetByULID(ctx, req.RoleULID)
	if err != nil {
		return err
	}
	if current == nil {
		return nil // 幂等：目标已不存在，视为删除成功
	}
	if current.BuiltinFlag {
		return pkg.NewBizError(domain.RespCodeBizFailed, "内置角色不允许删除")
	}
	userCount, err := s.roleRepo.CountUsersByRoleULID(ctx, req.RoleULID)
	if err != nil {
		return err
	}
	if userCount > 0 {
		return pkg.NewBizError(domain.RespCodeConflict, "角色下仍有用户，请先移除挂载")
	}
	if s.providerRepo != nil {
		if err := s.providerRepo.DeleteRoleGrantsByRole(ctx, req.RoleULID); err != nil {
			return pkg.WrapBizError(domain.RespCodeStorageError, "模型角色授权清理失败", err)
		}
	}
	return s.roleRepo.Delete(ctx, req.RoleULID)
}

// PrunePermCode 从所有角色中剔除已下线的权限码（删菜单/停用菜单后调用，避免权限目录与角色授权脱节）。
// 管理员角色的通配 "*" 不动，其余角色按编码精确剔除。
func (s *RoleService) PrunePermCode(ctx context.Context, permCode string) error {
	if permCode == "" || permCode == domain.PermAdminAll {
		return nil
	}
	roles, err := s.roleRepo.ListAll(ctx)
	if err != nil {
		return err
	}
	for i := range roles {
		kept := prunedPermCodeList(&roles[i], permCode)
		if len(kept) == len(unmarshalPermCodes(roles[i].PermCodesJSON)) {
			continue
		}
		do := &domain.RoleDO{
			RoleULID:      roles[i].RoleULID,
			RoleCode:      roles[i].RoleCode,
			RoleName:      roles[i].RoleName,
			RemarkText:    roles[i].RemarkText,
			PermCodesJSON: marshalPermCodes(kept),
		}
		if err := s.roleRepo.Update(ctx, do); err != nil {
			return err
		}
	}
	return nil
}

// prunedPermCodeList 剔除指定权限码后的角色权限列表（纯函数）。
// 通配权限码不可被剔除：它是管理员角色的系统不变量，剔除会让系统失去超级管理员。
func prunedPermCodeList(role *domain.RoleDO, permCode string) []string {
	permCodes := unmarshalPermCodes(role.PermCodesJSON)
	if permCode == domain.PermAdminAll {
		return permCodes
	}
	kept := make([]string, 0, len(permCodes))
	for _, code := range permCodes {
		if code == permCode {
			continue
		}
		kept = append(kept, code)
	}
	return kept
}

// HasPerm 判断用户是否拥有指定权限码（"*" 通配全部；pkg.PermChecker 实现）。
func (s *RoleService) HasPerm(ctx context.Context, userULID string, permCode string) (bool, error) {
	roles, err := s.roleRepo.ListRolesByUserULID(ctx, userULID)
	if err != nil {
		return false, err
	}
	for i := range roles {
		for _, code := range unmarshalPermCodes(roles[i].PermCodesJSON) {
			if code == domain.PermAdminAll || code == permCode {
				return true, nil
			}
		}
	}
	return false, nil
}

// RoleCodesOfUser 查询用户的角色编码列表。
func (s *RoleService) RoleCodesOfUser(ctx context.Context, userULID string) ([]string, error) {
	return s.roleRepo.ListRoleCodesByUserULID(ctx, userULID)
}

// PermCodesOfUser 汇总用户的权限码集合（多角色取并集、去重保序；含 "*" 通配）。
func (s *RoleService) PermCodesOfUser(ctx context.Context, userULID string) ([]string, error) {
	roles, err := s.roleRepo.ListRolesByUserULID(ctx, userULID)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, 16)
	permCodes := make([]string, 0, 16)
	for i := range roles {
		for _, code := range unmarshalPermCodes(roles[i].PermCodesJSON) {
			if seen[code] {
				continue
			}
			seen[code] = true
			permCodes = append(permCodes, code)
		}
	}
	return permCodes, nil
}

// toRoleVO 组装角色视图对象。
func toRoleVO(do *domain.RoleDO) domain.RoleVO {
	return domain.RoleVO{
		RoleULID:    do.RoleULID,
		RoleCode:    do.RoleCode,
		RoleName:    do.RoleName,
		RemarkText:  do.RemarkText,
		PermCodes:   unmarshalPermCodes(do.PermCodesJSON),
		BuiltinFlag: do.BuiltinFlag,
		CreatedAt:   pkg.FormatTime(do.CreatedAt),
		UpdatedAt:   pkg.FormatTime(do.UpdatedAt),
	}
}

// marshalPermCodes 序列化权限码列表（空列表存 "[]"，保证列非 NULL）。
func marshalPermCodes(permCodes []string) string {
	if len(permCodes) == 0 {
		return "[]"
	}
	data, err := json.Marshal(permCodes)
	if err != nil {
		return "[]" // []string 序列化不会失败，防御兜底
	}
	return string(data)
}

// unmarshalPermCodes 反序列化权限码列表（损坏数据按空列表处理）。
func unmarshalPermCodes(permCodesJSON string) []string {
	permCodes := make([]string, 0, 4)
	if err := json.Unmarshal([]byte(permCodesJSON), &permCodes); err != nil {
		return []string{}
	}
	return permCodes
}

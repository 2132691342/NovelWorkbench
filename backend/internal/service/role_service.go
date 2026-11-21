// 本文件实现角色业务：内置角色播种、分页查询、保存与删除。

package service

import (
	"context"
	"encoding/json"

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
	roleRepo *repo.RoleRepo // 角色仓储
	userRepo *repo.UserRepo // 用户仓储
}

// NewRoleService 构造角色服务。
func NewRoleService(roleRepo *repo.RoleRepo, userRepo *repo.UserRepo) *RoleService {
	return &RoleService{roleRepo: roleRepo, userRepo: userRepo}
}

// SeedDefaults 播种内置角色（幂等：按 role_code 查缺补插；admin 补齐全部权限）。
func (s *RoleService) SeedDefaults(ctx context.Context) error {
	for _, item := range roleDefaultList {
		existing, err := s.roleRepo.GetByCode(ctx, item.RoleCode)
		if err != nil {
			return err
		}
		if existing != nil {
			continue
		}
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
	}
	return nil
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

// Save 创建或更新角色（role_ulid 为空创建；角色编码唯一）。
func (s *RoleService) Save(ctx context.Context, req *domain.RoleSaveREQ) (domain.RoleVO, error) {
	existing, err := s.roleRepo.GetByCode(ctx, req.RoleCode)
	if err != nil {
		return domain.RoleVO{}, err
	}
	permCodesJSON := marshalPermCodes(req.PermCodes)
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
		return toRoleVO(do), nil
	}
	current, err := s.roleRepo.GetByULID(ctx, req.RoleULID)
	if err != nil {
		return domain.RoleVO{}, err
	}
	if current == nil {
		return domain.RoleVO{}, pkg.NewBizError(domain.RespCodeNotFound, "角色不存在")
	}
	if current.BuiltinFlag && current.RoleCode != req.RoleCode {
		return domain.RoleVO{}, pkg.NewBizError(domain.RespCodeBizFailed, "内置角色不允许修改编码")
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
	do.BuiltinFlag = current.BuiltinFlag
	return toRoleVO(do), nil
}

// Delete 删除角色（内置角色或仍有用户挂载时拒绝）。
func (s *RoleService) Delete(ctx context.Context, req *domain.RoleDeleteREQ) error {
	current, err := s.roleRepo.GetByULID(ctx, req.RoleULID)
	if err != nil {
		return err
	}
	if current == nil {
		return pkg.NewBizError(domain.RespCodeNotFound, "角色不存在")
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
	return s.roleRepo.Delete(ctx, req.RoleULID)
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

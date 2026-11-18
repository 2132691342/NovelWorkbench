package service

import (
	"context"

	"golang.org/x/crypto/bcrypt"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// UserAdminService 用户管理业务：列表、创建、更新、删除、重置密码、角色挂载、启动期管理员引导。
type UserAdminService struct {
	userRepo *repo.UserRepo
	roleRepo *repo.RoleRepo
}

// NewUserAdminService 构造用户管理服务。
func NewUserAdminService(userRepo *repo.UserRepo, roleRepo *repo.RoleRepo) *UserAdminService {
	return &UserAdminService{userRepo: userRepo, roleRepo: roleRepo}
}

// ListPage 分页查询用户（含角色编码）。
func (s *UserAdminService) ListPage(ctx context.Context, req *domain.UserAdminListREQ) (domain.PageDTO[domain.UserAdminVO], error) {
	req.Normalize()
	dos, total, err := s.userRepo.ListPage(ctx, req)
	if err != nil {
		return domain.PageDTO[domain.UserAdminVO]{}, err
	}
	roleCodeMap, err := s.roleCodeMap(ctx, dos)
	if err != nil {
		return domain.PageDTO[domain.UserAdminVO]{}, err
	}
	vos := make([]domain.UserAdminVO, 0, len(dos))
	for i := range dos {
		vos = append(vos, toUserAdminVO(&dos[i], roleCodeMap[dos[i].UserULID]))
	}
	return domain.NewPageDTO(total, req.PageREQ, vos), nil
}

// Create 创建用户（登录名/邮箱唯一，角色缺省挂载普通用户）。
func (s *UserAdminService) Create(ctx context.Context, req *domain.UserAdminCreateREQ) (domain.UserAdminVO, error) {
	if existing, err := s.userRepo.GetByUserName(ctx, req.UserName); err != nil {
		return domain.UserAdminVO{}, err
	} else if existing != nil {
		return domain.UserAdminVO{}, pkg.NewBizError(domain.RespCodeConflict, "用户名已被占用")
	}
	if existing, err := s.userRepo.GetByEmail(ctx, req.UserEmail); err != nil {
		return domain.UserAdminVO{}, err
	} else if existing != nil {
		return domain.UserAdminVO{}, pkg.NewBizError(domain.RespCodeConflict, "邮箱已被占用")
	}
	hashBytes, err := bcrypt.GenerateFromPassword([]byte(req.PasswordText), bcrypt.DefaultCost)
	if err != nil {
		return domain.UserAdminVO{}, pkg.WrapBizError(domain.RespCodeInternalError, "加密密码失败", err)
	}
	roleCodes := req.RoleCodes
	if len(roleCodes) == 0 {
		roleCodes = []string{domain.RoleCodeUser}
	}
	roleULIDs, err := s.roleULIDsByCodes(ctx, roleCodes)
	if err != nil {
		return domain.UserAdminVO{}, err
	}
	do := &domain.UserDO{
		UserULID:       pkg.NewULID(domain.IDPrefixUser),
		UserName:       req.UserName,
		UserEmail:      req.UserEmail,
		PasswordHash:   string(hashBytes),
		RemarkText:     req.RemarkText,
		UserStatusCode: domain.UserStatusActive,
	}
	if err := s.userRepo.Create(ctx, do); err != nil {
		return domain.UserAdminVO{}, err
	}
	if err := s.roleRepo.ReplaceUserRoles(ctx, do.UserULID, roleULIDs); err != nil {
		return domain.UserAdminVO{}, err
	}
	return toUserAdminVO(do, roleCodes), nil
}

// Update 更新用户资料/状态/角色（operatorULID 用于禁止操作自己）。
func (s *UserAdminService) Update(ctx context.Context, req *domain.UserAdminUpdateREQ, operatorULID string) (domain.UserAdminVO, error) {
	do, err := s.mustGetUser(ctx, req.UserULID)
	if err != nil {
		return domain.UserAdminVO{}, err
	}
	if req.UserStatusCode == domain.UserStatusDisabled && req.UserULID == operatorULID {
		return domain.UserAdminVO{}, pkg.NewBizError(domain.RespCodeBizFailed, "不能禁用自己")
	}
	updates := map[string]any{}
	if req.UserEmail != "" {
		if existing, err := s.userRepo.GetByEmail(ctx, req.UserEmail); err != nil {
			return domain.UserAdminVO{}, err
		} else if existing != nil && existing.UserULID != req.UserULID {
			return domain.UserAdminVO{}, pkg.NewBizError(domain.RespCodeConflict, "邮箱已被占用")
		}
		updates["user_email"] = req.UserEmail
	}
	if req.AvatarURL != "" {
		updates["avatar_url"] = req.AvatarURL
	}
	if req.RemarkText != "" {
		updates["remark_text"] = req.RemarkText
	}
	if req.UserStatusCode != "" {
		updates["user_status_code"] = req.UserStatusCode
	}
	if len(updates) > 0 {
		if err := s.userRepo.Update(ctx, req.UserULID, updates); err != nil {
			return domain.UserAdminVO{}, err
		}
	}
	roleCodes := req.RoleCodes
	if roleCodes != nil {
		roleULIDs, err := s.roleULIDsByCodes(ctx, roleCodes)
		if err != nil {
			return domain.UserAdminVO{}, err
		}
		if err := s.roleRepo.ReplaceUserRoles(ctx, req.UserULID, roleULIDs); err != nil {
			return domain.UserAdminVO{}, err
		}
	} else if roleCodes, err = s.roleRepo.ListRoleCodesByUserULID(ctx, req.UserULID); err != nil {
		return domain.UserAdminVO{}, err
	}
	do.UserEmail = pickUpdateStr(updates, "user_email", do.UserEmail)
	do.AvatarURL = pickUpdateStr(updates, "avatar_url", do.AvatarURL)
	do.RemarkText = pickUpdateStr(updates, "remark_text", do.RemarkText)
	do.UserStatusCode = pickUpdateStr(updates, "user_status_code", do.UserStatusCode)
	return toUserAdminVO(do, roleCodes), nil
}

// Delete 删除用户（禁止删除自己；级联清理角色挂载）。
func (s *UserAdminService) Delete(ctx context.Context, req *domain.UserAdminDeleteREQ, operatorULID string) error {
	if req.UserULID == operatorULID {
		return pkg.NewBizError(domain.RespCodeBizFailed, "不能删除自己")
	}
	if _, err := s.mustGetUser(ctx, req.UserULID); err != nil {
		return err
	}
	if err := s.roleRepo.DeleteUserRoles(ctx, req.UserULID); err != nil {
		return err
	}
	return s.userRepo.Delete(ctx, req.UserULID)
}

// ResetPassword 重置用户密码（bcrypt 哈希后覆盖）。
func (s *UserAdminService) ResetPassword(ctx context.Context, req *domain.UserResetPasswordREQ) error {
	if _, err := s.mustGetUser(ctx, req.UserULID); err != nil {
		return err
	}
	hashBytes, err := bcrypt.GenerateFromPassword([]byte(req.NewPasswordText), bcrypt.DefaultCost)
	if err != nil {
		return pkg.WrapBizError(domain.RespCodeInternalError, "加密密码失败", err)
	}
	return s.userRepo.Update(ctx, req.UserULID, map[string]any{"password_hash": string(hashBytes)})
}

// BootstrapAdmin 启动期引导管理员（仅当用户表为空时创建并挂载 admin 角色）。
func (s *UserAdminService) BootstrapAdmin(ctx context.Context, userName string, userEmail string, passwordText string) error {
	total, err := s.userRepo.Count(ctx)
	if err != nil {
		return err
	}
	if total > 0 {
		return nil
	}
	hashBytes, err := bcrypt.GenerateFromPassword([]byte(passwordText), bcrypt.DefaultCost)
	if err != nil {
		return pkg.WrapBizError(domain.RespCodeInternalError, "加密密码失败", err)
	}
	adminRole, err := s.roleRepo.GetByCode(ctx, domain.RoleCodeAdmin)
	if err != nil {
		return err
	}
	if adminRole == nil {
		return pkg.NewBizError(domain.RespCodeInternalError, "admin 角色未播种")
	}
	do := &domain.UserDO{
		UserULID:       pkg.NewULID(domain.IDPrefixUser),
		UserName:       userName,
		UserEmail:      userEmail,
		PasswordHash:   string(hashBytes),
		UserStatusCode: domain.UserStatusActive,
		RemarkText:     "启动期引导创建的管理员",
	}
	if err := s.userRepo.Create(ctx, do); err != nil {
		return err
	}
	return s.roleRepo.ReplaceUserRoles(ctx, do.UserULID, []string{adminRole.RoleULID})
}

// mustGetUser 查询用户，不存在时返回业务错误。
func (s *UserAdminService) mustGetUser(ctx context.Context, userULID string) (*domain.UserDO, error) {
	do, err := s.userRepo.GetByULID(ctx, userULID)
	if err != nil {
		return nil, err
	}
	if do == nil {
		return nil, pkg.NewBizError(domain.RespCodeNotFound, "用户不存在")
	}
	return do, nil
}

// roleULIDsByCodes 按角色编码解析角色 ULID（未知编码报参数错误）。
func (s *UserAdminService) roleULIDsByCodes(ctx context.Context, roleCodes []string) ([]string, error) {
	allRoles, err := s.roleRepo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	codeMap := make(map[string]string, len(allRoles))
	for i := range allRoles {
		codeMap[allRoles[i].RoleCode] = allRoles[i].RoleULID
	}
	roleULIDs := make([]string, 0, len(roleCodes))
	for _, roleCode := range roleCodes {
		roleULID, ok := codeMap[roleCode]
		if !ok {
			return nil, pkg.NewBizError(domain.RespCodeInvalidParam, "角色编码不存在: "+roleCode)
		}
		roleULIDs = append(roleULIDs, roleULID)
	}
	return roleULIDs, nil
}

// roleCodeMap 批量组装 用户ULID → 角色编码列表（两查防 N+1）。
func (s *UserAdminService) roleCodeMap(ctx context.Context, dos []domain.UserDO) (map[string][]string, error) {
	result := make(map[string][]string, len(dos))
	if len(dos) == 0 {
		return result, nil
	}
	userULIDs := make([]string, 0, len(dos))
	for i := range dos {
		userULIDs = append(userULIDs, dos[i].UserULID)
		result[dos[i].UserULID] = []string{}
	}
	links, err := s.roleRepo.ListLinksByUserULIDs(ctx, userULIDs)
	if err != nil {
		return nil, err
	}
	allRoles, err := s.roleRepo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	roleCodeByULID := make(map[string]string, len(allRoles))
	for i := range allRoles {
		roleCodeByULID[allRoles[i].RoleULID] = allRoles[i].RoleCode
	}
	for _, link := range links {
		if roleCode, ok := roleCodeByULID[link.RoleULID]; ok {
			result[link.UserULID] = append(result[link.UserULID], roleCode)
		}
	}
	return result, nil
}

// toUserAdminVO 组装用户管理视图对象。
func toUserAdminVO(do *domain.UserDO, roleCodes []string) domain.UserAdminVO {
	vo := domain.UserAdminVO{
		UserVO: domain.UserVO{
			UserULID:       do.UserULID,
			UserName:       do.UserName,
			UserEmail:      do.UserEmail,
			AvatarURL:      do.AvatarURL,
			UserStatusCode: do.UserStatusCode,
			RoleCodes:      roleCodes,
			CreatedAtMs:    do.CreatedAt.UnixMilli(),
		},
		RemarkText:  do.RemarkText,
		UpdatedAtMs: do.UpdatedAt.UnixMilli(),
	}
	if vo.UserStatusCode == "" {
		vo.UserStatusCode = domain.UserStatusActive // 历史数据兜底
	}
	return vo
}

// pickUpdateStr 从更新 map 取值，未更新时保留原值。
func pickUpdateStr(updates map[string]any, key string, fallback string) string {
	if value, ok := updates[key].(string); ok {
		return value
	}
	return fallback
}

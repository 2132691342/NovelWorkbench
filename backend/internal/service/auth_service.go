// 本文件实现认证业务：登录、当前用户、个人资料、修改密码与登出吊销。

package service

import (
	"context"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// AuthService 认证业务：登录、用户信息、个人资料、修改密码与登出吊销。
type AuthService struct {
	userRepo        *repo.UserRepo       // 用户仓储
	roleRepo        *repo.RoleRepo       // 角色仓储
	signer          *pkg.JWTSigner       // 令牌签发器
	tokenStore      pkg.TokenStore       // 令牌吊销存储
	loginLogSvc     *LoginLogService     // 登录日志
	notificationSvc *NotificationService // 站内通知
}

// NewAuthService 构造认证服务（登录日志与通知均可空，便于测试与降级）。
func NewAuthService(userRepo *repo.UserRepo, roleRepo *repo.RoleRepo, signer *pkg.JWTSigner, tokenStore pkg.TokenStore,
	loginLogSvc *LoginLogService, notificationSvc *NotificationService) *AuthService {
	return &AuthService{
		userRepo:        userRepo,
		roleRepo:        roleRepo,
		signer:          signer,
		tokenStore:      tokenStore,
		loginLogSvc:     loginLogSvc,
		notificationSvc: notificationSvc,
	}
}

// Login 登录（账号匹配登录名或邮箱），成功签发 JWT，并记录成功/失败日志。
func (s *AuthService) Login(ctx context.Context, req *domain.UserLoginREQ, meta domain.LoginMetaDTO) (domain.UserLoginDTO, error) {
	do, err := s.userRepo.GetByAccount(ctx, req.AccountText)
	if err != nil {
		return domain.UserLoginDTO{}, err
	}
	if do == nil {
		s.recordLogin(ctx, req.AccountText, "", domain.LoginResultFailed, "账号不存在", meta)
		return domain.UserLoginDTO{}, pkg.NewBizError(domain.RespCodeUnauthorized, "账号或密码错误")
	}
	if do.UserStatusCode == domain.UserStatusDisabled {
		s.recordLogin(ctx, req.AccountText, do.UserULID, domain.LoginResultDisabled, "账号已被禁用", meta)
		return domain.UserLoginDTO{}, pkg.NewBizError(domain.RespCodeForbidden, "账号已被禁用")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(do.PasswordHash), []byte(req.PasswordText)); err != nil {
		s.recordLogin(ctx, req.AccountText, do.UserULID, domain.LoginResultFailed, "密码错误", meta)
		return domain.UserLoginDTO{}, pkg.NewBizError(domain.RespCodeUnauthorized, "账号或密码错误")
	}
	token, expiresInSec, err := s.signer.SignToken(do.UserULID, do.PwdEpoch)
	if err != nil {
		return domain.UserLoginDTO{}, err
	}
	userVO, err := s.withRoleCodes(ctx, toUserVO(do))
	if err != nil {
		return domain.UserLoginDTO{}, err
	}
	s.recordLogin(ctx, req.AccountText, do.UserULID, domain.LoginResultSuccess, "", meta)
	return domain.UserLoginDTO{
		AccessToken:  token,
		ExpiresInSec: expiresInSec,
		UserVO:       userVO,
	}, nil
}

// Profile 查询当前登录用户信息。
func (s *AuthService) Profile(ctx context.Context, userULID string) (domain.UserVO, error) {
	do, err := s.mustGetUser(ctx, userULID)
	if err != nil {
		return domain.UserVO{}, err
	}
	return s.withRoleCodes(ctx, toUserVO(do))
}

// ProfileDetail 查询个人中心资料（含备注、更新时间、角色名与权限码概览）。
func (s *AuthService) ProfileDetail(ctx context.Context, userULID string) (domain.UserProfileVO, error) {
	do, err := s.mustGetUser(ctx, userULID)
	if err != nil {
		return domain.UserProfileVO{}, err
	}
	return s.buildProfileVO(ctx, do)
}

// UpdateProfile 更新本人资料（邮箱查重；头像与备注允许清空）。
func (s *AuthService) UpdateProfile(ctx context.Context, userULID string, req *domain.UserProfileUpdateREQ) (domain.UserProfileVO, error) {
	do, err := s.mustGetUser(ctx, userULID)
	if err != nil {
		return domain.UserProfileVO{}, err
	}
	updates := map[string]any{
		"avatar_url":  req.AvatarURL,
		"remark_text": req.RemarkText,
	}
	if req.UserEmail != "" && req.UserEmail != do.UserEmail {
		existing, err := s.userRepo.GetByEmail(ctx, req.UserEmail)
		if err != nil {
			return domain.UserProfileVO{}, err
		}
		if existing != nil && existing.UserULID != userULID {
			return domain.UserProfileVO{}, pkg.NewBizError(domain.RespCodeConflict, "邮箱已被占用")
		}
		updates["user_email"] = req.UserEmail
		do.UserEmail = req.UserEmail
	}
	if err := s.userRepo.Update(ctx, userULID, updates); err != nil {
		return domain.UserProfileVO{}, err
	}
	do.AvatarURL = req.AvatarURL
	do.RemarkText = req.RemarkText
	return s.buildProfileVO(ctx, do)
}

// buildProfileVO 组装个人中心资料（角色名 + 权限并集一并下发，前端无需再拉两次接口）。
func (s *AuthService) buildProfileVO(ctx context.Context, do *domain.UserDO) (domain.UserProfileVO, error) {
	roleCodes, err := s.roleRepo.ListRoleCodesByUserULID(ctx, do.UserULID)
	if err != nil {
		return domain.UserProfileVO{}, err
	}
	roles, err := s.roleRepo.ListRolesByUserULID(ctx, do.UserULID)
	if err != nil {
		return domain.UserProfileVO{}, err
	}
	roleNameList := make([]string, 0, len(roles))
	permCodes := make([]string, 0, 16)
	seenPerm := make(map[string]bool, 16)
	for i := range roles {
		roleNameList = append(roleNameList, roles[i].RoleName)
		for _, permCode := range unmarshalPermCodes(roles[i].PermCodesJSON) {
			if seenPerm[permCode] {
				continue
			}
			seenPerm[permCode] = true
			permCodes = append(permCodes, permCode)
		}
	}
	vo := toUserProfileVO(do, roleCodes)
	vo.RoleNameList = roleNameList
	vo.PermCodes = permCodes
	return vo, nil
}

// ChangePassword 校验旧密码后更新本人密码，返回新令牌（旧令牌因密码版本递增立即失效）。
func (s *AuthService) ChangePassword(ctx context.Context, userULID string, req *domain.UserPasswordChangeREQ) (domain.UserPasswordChangeDTO, error) {
	do, err := s.mustGetUser(ctx, userULID)
	if err != nil {
		return domain.UserPasswordChangeDTO{}, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(do.PasswordHash), []byte(req.OldPasswordText)); err != nil {
		return domain.UserPasswordChangeDTO{}, pkg.NewBizError(domain.RespCodeUnauthorized, "当前密码不正确")
	}
	if err := validatePasswordStrength(req.NewPasswordText); err != nil {
		return domain.UserPasswordChangeDTO{}, err
	}
	if req.NewPasswordText == req.OldPasswordText {
		return domain.UserPasswordChangeDTO{}, pkg.NewBizError(domain.RespCodeInvalidParam, "新密码不能与当前密码相同")
	}
	hashBytes, err := bcrypt.GenerateFromPassword([]byte(req.NewPasswordText), bcrypt.DefaultCost)
	if err != nil {
		return domain.UserPasswordChangeDTO{}, pkg.WrapBizError(domain.RespCodeInternalError, "加密密码失败", err)
	}
	// 改密即让旧令牌失效：密码版本递增，令牌内的版本号随之落后
	if err := s.userRepo.Update(ctx, userULID, map[string]any{
		"password_hash": string(hashBytes),
		"pwd_epoch":     gorm.Expr("pwd_epoch + 1"),
	}); err != nil {
		return domain.UserPasswordChangeDTO{}, err
	}
	updated, err := s.mustGetUser(ctx, userULID)
	if err != nil {
		return domain.UserPasswordChangeDTO{}, err
	}
	token, expiresInSec, err := s.signer.SignToken(updated.UserULID, updated.PwdEpoch)
	if err != nil {
		return domain.UserPasswordChangeDTO{}, err
	}
	s.notify(ctx, domain.NotificationCreateDTO{
		UserULID:              userULID,
		NotificationTypeCode:  domain.NotificationTypeAccount,
		NotificationTitleText: "登录密码已修改",
		NotificationBodyText:  "你的账号密码刚刚被修改；如非本人操作，请立即联系管理员。",
		LinkURL:               "/profile",
		RefTypeCode:           "user",
		RefULID:               userULID,
	})
	return domain.UserPasswordChangeDTO{AccessToken: token, ExpiresInSec: expiresInSec}, nil
}

// Logout 吊销访问令牌（写入黑名单，TTL 为令牌签发有效期）。
func (s *AuthService) Logout(ctx context.Context, tokenString string) error {
	claims := &pkg.TokenClaims{}
	if _, _, err := (&jwt.Parser{}).ParseUnverified(tokenString, claims); err != nil {
		return pkg.NewBizError(domain.RespCodeUnauthorized, "登录已失效")
	}
	if claims.ExpiresAt == nil || claims.IssuedAt == nil {
		return pkg.NewBizError(domain.RespCodeUnauthorized, "登录已失效")
	}
	ttl := claims.ExpiresAt.Time.Sub(claims.IssuedAt.Time)
	if ttl <= 0 {
		return pkg.NewBizError(domain.RespCodeUnauthorized, "登录已失效")
	}
	if err := s.tokenStore.Revoke(ctx, tokenString, ttl); err != nil {
		return pkg.WrapBizError(domain.RespCodeStorageError, "吊销令牌失败", err)
	}
	return nil
}

// mustGetUser 查询用户，不存在时返回业务错误。
func (s *AuthService) mustGetUser(ctx context.Context, userULID string) (*domain.UserDO, error) {
	do, err := s.userRepo.GetByULID(ctx, userULID)
	if err != nil {
		return nil, err
	}
	if do == nil {
		return nil, pkg.NewBizError(domain.RespCodeNotFound, "用户不存在")
	}
	return do, nil
}

// recordLogin 记录登录结果（服务未装配日志时静默跳过）。
func (s *AuthService) recordLogin(ctx context.Context, accountText string, userULID string, resultCode string, reasonText string, meta domain.LoginMetaDTO) {
	if s.loginLogSvc == nil {
		return
	}
	s.loginLogSvc.Record(ctx, accountText, userULID, resultCode, reasonText, meta)
}

// notify 发送站内通知（失败只告警，不影响主链路）。
func (s *AuthService) notify(ctx context.Context, createDTO domain.NotificationCreateDTO) {
	if s.notificationSvc == nil {
		return
	}
	if err := s.notificationSvc.Notify(ctx, createDTO); err != nil {
		pkg.SLogger().Warnw("站内通知写入失败", "user_ulid", createDTO.UserULID, "error", err)
	}
}

// withRoleCodes 为用户视图补角色编码列表。
func (s *AuthService) withRoleCodes(ctx context.Context, vo domain.UserVO) (domain.UserVO, error) {
	roleCodes, err := s.roleRepo.ListRoleCodesByUserULID(ctx, vo.UserULID)
	if err != nil {
		return domain.UserVO{}, err
	}
	vo.RoleCodes = roleCodes
	return vo, nil
}

// toUserVO 组装用户视图对象。
func toUserVO(do *domain.UserDO) domain.UserVO {
	statusCode := do.UserStatusCode
	if statusCode == "" {
		statusCode = domain.UserStatusActive // 历史数据兜底
	}
	return domain.UserVO{
		UserULID:       do.UserULID,
		UserName:       do.UserName,
		UserEmail:      do.UserEmail,
		AvatarURL:      do.AvatarURL,
		UserStatusCode: statusCode,
		RoleCodes:      []string{},
		CreatedAt:      pkg.FormatTime(do.CreatedAt),
	}
}

// toUserProfileVO 组装个人中心资料视图。
func toUserProfileVO(do *domain.UserDO, roleCodes []string) domain.UserProfileVO {
	statusCode := do.UserStatusCode
	if statusCode == "" {
		statusCode = domain.UserStatusActive
	}
	if roleCodes == nil {
		roleCodes = []string{}
	}
	return domain.UserProfileVO{
		UserULID:       do.UserULID,
		UserName:       do.UserName,
		UserEmail:      do.UserEmail,
		AvatarURL:      do.AvatarURL,
		UserStatusCode: statusCode,
		RoleCodes:      roleCodes,
		RemarkText:     do.RemarkText,
		CreatedAt:      pkg.FormatTime(do.CreatedAt),
		UpdatedAt:      pkg.FormatTime(do.UpdatedAt),
	}
}

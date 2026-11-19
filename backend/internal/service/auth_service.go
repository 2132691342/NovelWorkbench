// 本文件实现认证业务：注册、登录、当前用户与登出吊销。

package service

import (
	"context"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// AuthService 认证业务：注册、登录、用户信息、登出吊销。
type AuthService struct {
	userRepo   *repo.UserRepo // 用户仓储
	roleRepo   *repo.RoleRepo // 角色仓储
	signer     *pkg.JWTSigner // 令牌签发器
	tokenStore pkg.TokenStore // 令牌吊销存储
}

// NewAuthService 构造认证服务。
func NewAuthService(userRepo *repo.UserRepo, roleRepo *repo.RoleRepo, signer *pkg.JWTSigner, tokenStore pkg.TokenStore) *AuthService {
	return &AuthService{userRepo: userRepo, roleRepo: roleRepo, signer: signer, tokenStore: tokenStore}
}

// Register 注册用户（登录名/邮箱唯一，bcrypt 哈希密码）。
func (s *AuthService) Register(ctx context.Context, req *domain.UserRegisterREQ) (domain.UserVO, error) {
	existing, err := s.userRepo.GetByUserName(ctx, req.UserName)
	if err != nil {
		return domain.UserVO{}, err
	}
	if existing != nil {
		return domain.UserVO{}, pkg.NewBizError(domain.RespCodeConflict, "用户名已被注册")
	}
	existing, err = s.userRepo.GetByEmail(ctx, req.UserEmail)
	if err != nil {
		return domain.UserVO{}, err
	}
	if existing != nil {
		return domain.UserVO{}, pkg.NewBizError(domain.RespCodeConflict, "邮箱已被注册")
	}
	hashBytes, err := bcrypt.GenerateFromPassword([]byte(req.PasswordText), bcrypt.DefaultCost)
	if err != nil {
		return domain.UserVO{}, pkg.WrapBizError(domain.RespCodeInternalError, "加密密码失败", err)
	}
	do := &domain.UserDO{
		UserULID:     pkg.NewULID(domain.IDPrefixUser),
		UserName:     req.UserName,
		UserEmail:    req.UserEmail,
		PasswordHash: string(hashBytes),
	}
	if err := s.userRepo.Create(ctx, do); err != nil {
		return domain.UserVO{}, err
	}
	return toUserVO(do), nil
}

// Login 登录（账号匹配登录名或邮箱），成功签发 JWT。
func (s *AuthService) Login(ctx context.Context, req *domain.UserLoginREQ) (domain.UserLoginDTO, error) {
	do, err := s.userRepo.GetByAccount(ctx, req.AccountText)
	if err != nil {
		return domain.UserLoginDTO{}, err
	}
	if do == nil {
		return domain.UserLoginDTO{}, pkg.NewBizError(domain.RespCodeUnauthorized, "账号或密码错误")
	}
	if do.UserStatusCode == domain.UserStatusDisabled {
		return domain.UserLoginDTO{}, pkg.NewBizError(domain.RespCodeForbidden, "账号已被禁用")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(do.PasswordHash), []byte(req.PasswordText)); err != nil {
		return domain.UserLoginDTO{}, pkg.NewBizError(domain.RespCodeUnauthorized, "账号或密码错误")
	}
	token, expiresInSec, err := s.signer.SignToken(do.UserULID)
	if err != nil {
		return domain.UserLoginDTO{}, err
	}
	userVO, err := s.withRoleCodes(ctx, toUserVO(do))
	if err != nil {
		return domain.UserLoginDTO{}, err
	}
	return domain.UserLoginDTO{
		AccessToken:  token,
		ExpiresInSec: expiresInSec,
		UserVO:       userVO,
	}, nil
}

// Profile 查询当前登录用户信息。
func (s *AuthService) Profile(ctx context.Context, userULID string) (domain.UserVO, error) {
	do, err := s.userRepo.GetByULID(ctx, userULID)
	if err != nil {
		return domain.UserVO{}, err
	}
	if do == nil {
		return domain.UserVO{}, pkg.NewBizError(domain.RespCodeNotFound, "用户不存在")
	}
	return s.withRoleCodes(ctx, toUserVO(do))
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
		CreatedAtMs:    do.CreatedAt.UnixMilli(),
	}
}

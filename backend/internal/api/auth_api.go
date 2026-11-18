package api

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/service"
)

// respondErr 统一错误出口：记录日志并输出业务失败响应。
func respondErr(c *gin.Context, op string, err error) {
	pkg.SLogger().Errorw("请求处理失败", "op", op, "request_id", c.GetString(pkg.ContextKeyRequestID), "error", err)
	pkg.FailErr(c, err)
}

// bindErr 将绑定/校验错误转换为参数错误业务码。
func bindErr(err error) error {
	return pkg.NewBizError(domain.RespCodeInvalidParam, "参数校验失败: "+err.Error())
}

// AuthAPI 认证模块 HTTP 处理器。
type AuthAPI struct {
	svc *service.AuthService
}

// NewAuthAPI 构造认证处理器。
func NewAuthAPI(s *service.AuthService) *AuthAPI {
	return &AuthAPI{svc: s}
}

// Register 处理用户注册（POST /auth/register）。
func (a *AuthAPI) Register(c *gin.Context) {
	var req domain.UserRegisterREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "auth.register", bindErr(err))
		return
	}
	vo, err := a.svc.Register(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "auth.register", err)
		return
	}
	pkg.Ok(c, vo)
}

// Login 处理用户登录（POST /auth/login）。
func (a *AuthAPI) Login(c *gin.Context) {
	var req domain.UserLoginREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "auth.login", bindErr(err))
		return
	}
	dto, err := a.svc.Login(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "auth.login", err)
		return
	}
	pkg.Ok(c, dto)
}

// Profile 返回当前登录用户信息（GET /auth/profile）。
func (a *AuthAPI) Profile(c *gin.Context) {
	vo, err := a.svc.Profile(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID))
	if err != nil {
		respondErr(c, "auth.profile", err)
		return
	}
	pkg.Ok(c, vo)
}

// Logout 吊销当前访问令牌（POST /auth/logout）。
func (a *AuthAPI) Logout(c *gin.Context) {
	tokenString := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	if tokenString == "" {
		respondErr(c, "auth.logout", pkg.NewBizError(domain.RespCodeUnauthorized, "未登录"))
		return
	}
	if err := a.svc.Logout(c.Request.Context(), tokenString); err != nil {
		respondErr(c, "auth.logout", err)
		return
	}
	pkg.Ok(c, nil)
}

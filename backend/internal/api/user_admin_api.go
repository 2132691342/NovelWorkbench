package api

import (
	"github.com/gin-gonic/gin"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/service"
)

// UserAdminAPI 用户管理模块 HTTP 处理器。
type UserAdminAPI struct {
	svc *service.UserAdminService
}

// NewUserAdminAPI 构造用户管理处理器。
func NewUserAdminAPI(s *service.UserAdminService) *UserAdminAPI {
	return &UserAdminAPI{svc: s}
}

// List 分页查询用户（POST /admin/user/list）。
func (a *UserAdminAPI) List(c *gin.Context) {
	var req domain.UserAdminListREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "admin.user.list", bindErr(err))
		return
	}
	page, err := a.svc.ListPage(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "admin.user.list", err)
		return
	}
	pkg.Ok(c, page)
}

// Create 创建用户（POST /admin/user/create）。
func (a *UserAdminAPI) Create(c *gin.Context) {
	var req domain.UserAdminCreateREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "admin.user.create", bindErr(err))
		return
	}
	vo, err := a.svc.Create(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "admin.user.create", err)
		return
	}
	pkg.Ok(c, vo)
}

// Update 更新用户（POST /admin/user/update）。
func (a *UserAdminAPI) Update(c *gin.Context) {
	var req domain.UserAdminUpdateREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "admin.user.update", bindErr(err))
		return
	}
	vo, err := a.svc.Update(c.Request.Context(), &req, c.GetString(pkg.ContextKeyUserULID))
	if err != nil {
		respondErr(c, "admin.user.update", err)
		return
	}
	pkg.Ok(c, vo)
}

// Delete 删除用户（POST /admin/user/delete）。
func (a *UserAdminAPI) Delete(c *gin.Context) {
	var req domain.UserAdminDeleteREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "admin.user.delete", bindErr(err))
		return
	}
	if err := a.svc.Delete(c.Request.Context(), &req, c.GetString(pkg.ContextKeyUserULID)); err != nil {
		respondErr(c, "admin.user.delete", err)
		return
	}
	pkg.Ok(c, nil)
}

// ResetPassword 重置用户密码（POST /admin/user/reset_password）。
func (a *UserAdminAPI) ResetPassword(c *gin.Context) {
	var req domain.UserResetPasswordREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "admin.user.reset_password", bindErr(err))
		return
	}
	if err := a.svc.ResetPassword(c.Request.Context(), &req); err != nil {
		respondErr(c, "admin.user.reset_password", err)
		return
	}
	pkg.Ok(c, nil)
}

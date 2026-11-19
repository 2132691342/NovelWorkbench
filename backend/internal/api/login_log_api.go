// 本文件实现登录日志模块的 HTTP 处理器（管理端分页查询）。

package api

import (
	"github.com/gin-gonic/gin"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/service"
)

// LoginLogAPI 登录日志模块 HTTP 处理器。
type LoginLogAPI struct {
	svc *service.LoginLogService
}

// NewLoginLogAPI 构造登录日志处理器。
func NewLoginLogAPI(s *service.LoginLogService) *LoginLogAPI {
	return &LoginLogAPI{svc: s}
}

// List 分页查询登录日志（POST /admin/login_log/list）。
func (a *LoginLogAPI) List(c *gin.Context) {
	var req domain.LoginLogListREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "admin.login_log.list", bindErr(err))
		return
	}
	page, err := a.svc.ListPage(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "admin.login_log.list", err)
		return
	}
	pkg.Ok(c, page)
}

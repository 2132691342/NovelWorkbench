// 本文件实现菜单与权限目录模块的 HTTP 处理器：导航树、菜单 CRUD 与权限目录。

package api

import (
	"github.com/gin-gonic/gin"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/service"
)

// MenuAPI 菜单与权限目录模块 HTTP 处理器。
type MenuAPI struct {
	svc *service.MenuService
}

// NewMenuAPI 构造菜单处理器。
func NewMenuAPI(s *service.MenuService) *MenuAPI {
	return &MenuAPI{svc: s}
}

// Nav 返回当前用户的导航树与权限码集合（GET /menu/nav）。
func (a *MenuAPI) Nav(c *gin.Context) {
	dto, err := a.svc.Nav(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID))
	if err != nil {
		respondErr(c, "menu.nav", err)
		return
	}
	pkg.Ok(c, dto)
}

// List 查询菜单平铺列表（GET /menu/list，管理端）。
func (a *MenuAPI) List(c *gin.Context) {
	var req domain.MenuListREQ
	if err := c.ShouldBindQuery(&req); err != nil {
		respondErr(c, "menu.list", bindErr(err))
		return
	}
	vos, err := a.svc.List(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "menu.list", err)
		return
	}
	pkg.Ok(c, vos)
}

// Detail 查询菜单详情（GET /menu/detail/{MenuULID}）。
func (a *MenuAPI) Detail(c *gin.Context) {
	vo, err := a.svc.Detail(c.Request.Context(), c.Param("MenuULID"))
	if err != nil {
		respondErr(c, "menu.detail", err)
		return
	}
	pkg.Ok(c, vo)
}

// Save 创建或更新菜单（POST /menu/save）。
func (a *MenuAPI) Save(c *gin.Context) {
	var req domain.MenuREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "menu.save", bindErr(err))
		return
	}
	vo, err := a.svc.Save(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "menu.save", err)
		return
	}
	pkg.Ok(c, vo)
}

// Delete 删除菜单（POST /menu/delete）。
func (a *MenuAPI) Delete(c *gin.Context) {
	var req domain.MenuDeleteREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "menu.delete", bindErr(err))
		return
	}
	if err := a.svc.Delete(c.Request.Context(), &req); err != nil {
		respondErr(c, "menu.delete", err)
		return
	}
	pkg.Ok(c, nil)
}

// PermCatalog 权限目录（GET /menu/permission/catalog，按分组返回已登记权限）。
func (a *MenuAPI) PermCatalog(c *gin.Context) {
	groups, err := a.svc.PermissionCatalog(c.Request.Context())
	if err != nil {
		respondErr(c, "menu.perm.catalog", err)
		return
	}
	pkg.Ok(c, groups)
}

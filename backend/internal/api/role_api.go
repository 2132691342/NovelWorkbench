// 本文件实现角色管理模块的 HTTP 处理器：角色列表、保存与删除。

package api

import (
	"github.com/gin-gonic/gin"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/service"
)

// RoleAPI 角色管理模块 HTTP 处理器。
type RoleAPI struct {
	svc *service.RoleService
}

// NewRoleAPI 构造角色管理处理器。
func NewRoleAPI(s *service.RoleService) *RoleAPI {
	return &RoleAPI{svc: s}
}

// List 分页查询角色（POST /admin/role/list）。
func (a *RoleAPI) List(c *gin.Context) {
	var req domain.RoleListREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "admin.role.list", bindErr(err))
		return
	}
	page, err := a.svc.ListPage(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "admin.role.list", err)
		return
	}
	pkg.Ok(c, page)
}

// Save 创建或更新角色（POST /admin/role/save）。
func (a *RoleAPI) Save(c *gin.Context) {
	var req domain.RoleSaveREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "admin.role.save", bindErr(err))
		return
	}
	vo, err := a.svc.Save(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "admin.role.save", err)
		return
	}
	pkg.Ok(c, vo)
}

// Delete 删除角色（POST /admin/role/delete）。
func (a *RoleAPI) Delete(c *gin.Context) {
	var req domain.RoleDeleteREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "admin.role.delete", bindErr(err))
		return
	}
	if err := a.svc.Delete(c.Request.Context(), &req); err != nil {
		respondErr(c, "admin.role.delete", err)
		return
	}
	pkg.Ok(c, nil)
}

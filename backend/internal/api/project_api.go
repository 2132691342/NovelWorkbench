// 本文件实现项目模块的 HTTP 处理器：项目 CRUD、列表、详情与目录树。

package api

import (
	"github.com/gin-gonic/gin"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/service"
)

// ProjectAPI 项目模块 HTTP 处理器。
type ProjectAPI struct {
	svc *service.ProjectService
}

// NewProjectAPI 构造项目处理器。
func NewProjectAPI(s *service.ProjectService) *ProjectAPI {
	return &ProjectAPI{svc: s}
}

// Create 创建项目（POST /project/create）。
func (a *ProjectAPI) Create(c *gin.Context) {
	var req domain.ProjectCreateREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "project.create", bindErr(err))
		return
	}
	vo, err := a.svc.Create(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "project.create", err)
		return
	}
	pkg.Ok(c, vo)
}

// Update 更新项目（POST /project/update）。
func (a *ProjectAPI) Update(c *gin.Context) {
	var req domain.ProjectUpdateREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "project.update", bindErr(err))
		return
	}
	vo, err := a.svc.Update(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "project.update", err)
		return
	}
	pkg.Ok(c, vo)
}

// Delete 删除项目（POST /project/delete，级联删除卷/章节/快照/统计）。
func (a *ProjectAPI) Delete(c *gin.Context) {
	var req domain.ProjectDeleteREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "project.delete", bindErr(err))
		return
	}
	if err := a.svc.Delete(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req); err != nil {
		respondErr(c, "project.delete", err)
		return
	}
	pkg.Ok(c, nil)
}

// List 分页查询项目列表（POST /project/list）。
func (a *ProjectAPI) List(c *gin.Context) {
	var req domain.ProjectListREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "project.list", bindErr(err))
		return
	}
	page, err := a.svc.List(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "project.list", err)
		return
	}
	pkg.Ok(c, page)
}

// Detail 查询项目详情（GET /project/detail/{ProjectULID}）。
func (a *ProjectAPI) Detail(c *gin.Context) {
	req := domain.ProjectDetailREQ{ProjectULID: c.Param("ProjectULID")}
	if err := c.ShouldBindQuery(&req); err != nil {
		respondErr(c, "project.detail", bindErr(err))
		return
	}
	vo, err := a.svc.Detail(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "project.detail", err)
		return
	}
	pkg.Ok(c, vo)
}

// Tree 查询项目目录树（GET /project/tree/{ProjectULID}）。
func (a *ProjectAPI) Tree(c *gin.Context) {
	req := domain.ProjectTreeREQ{ProjectULID: c.Param("ProjectULID")}
	if err := c.ShouldBindQuery(&req); err != nil {
		respondErr(c, "project.tree", bindErr(err))
		return
	}
	vo, err := a.svc.Tree(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "project.tree", err)
		return
	}
	pkg.Ok(c, vo)
}

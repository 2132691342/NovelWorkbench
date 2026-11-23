// 本文件实现工具模块的 HTTP 处理器：列表、保存、删除与试运行。

package api

import (
	"github.com/gin-gonic/gin"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/service"
)

// ToolAPI 工具模块 HTTP 处理器。
type ToolAPI struct {
	svc *service.ToolService
}

// NewToolAPI 构造工具处理器。
func NewToolAPI(s *service.ToolService) *ToolAPI {
	return &ToolAPI{svc: s}
}

// List 查询工具列表（GET /tool/list）。
func (a *ToolAPI) List(c *gin.Context) {
	var req domain.ToolListREQ
	if err := c.ShouldBindQuery(&req); err != nil {
		respondErr(c, "tool.list", bindErr(err))
		return
	}
	vos, err := a.svc.List(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "tool.list", err)
		return
	}
	pkg.Ok(c, vos)
}

// Save 新建/更新工具（POST /tool/save）。
func (a *ToolAPI) Save(c *gin.Context) {
	var req domain.ToolREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "tool.save", bindErr(err))
		return
	}
	vo, err := a.svc.Save(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "tool.save", err)
		return
	}
	pkg.Ok(c, vo)
}

// Delete 删除工具（POST /tool/delete，内置不可删）。
func (a *ToolAPI) Delete(c *gin.Context) {
	var req domain.ToolDeleteREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "tool.delete", bindErr(err))
		return
	}
	if err := a.svc.Delete(c.Request.Context(), &req); err != nil {
		respondErr(c, "tool.delete", err)
		return
	}
	pkg.Ok(c, nil)
}

// Invoke 试运行工具（POST /tool/invoke）。
func (a *ToolAPI) Invoke(c *gin.Context) {
	var req domain.ToolInvokeREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "tool.invoke", bindErr(err))
		return
	}
	dto, err := a.svc.Invoke(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "tool.invoke", err)
		return
	}
	pkg.Ok(c, dto)
}

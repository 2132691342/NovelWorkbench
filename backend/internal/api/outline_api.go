// 本文件实现创作规划模块的 HTTP 处理器：大纲节点树与场景卡维护。

package api

import (
	"github.com/gin-gonic/gin"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/service"
)

// OutlineAPI 创作规划模块 HTTP 处理器。
type OutlineAPI struct {
	svc *service.OutlineService
}

// NewOutlineAPI 构造创作规划处理器。
func NewOutlineAPI(s *service.OutlineService) *OutlineAPI {
	return &OutlineAPI{svc: s}
}

// SaveNode 保存大纲节点（POST /outline/node/save）。
func (a *OutlineAPI) SaveNode(c *gin.Context) {
	var req domain.OutlineNodeREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "outline.saveNode", bindErr(err))
		return
	}
	vo, err := a.svc.SaveNode(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "outline.saveNode", err)
		return
	}
	pkg.Ok(c, vo)
}

// DeleteNode 删除大纲节点（POST /outline/node/delete）。
func (a *OutlineAPI) DeleteNode(c *gin.Context) {
	var req domain.OutlineNodeDeleteREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "outline.deleteNode", bindErr(err))
		return
	}
	if err := a.svc.DeleteNode(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req); err != nil {
		respondErr(c, "outline.deleteNode", err)
		return
	}
	pkg.Ok(c, nil)
}

// ListTree 查询项目大纲树（POST /outline/list）。
func (a *OutlineAPI) ListTree(c *gin.Context) {
	var req domain.OutlineTreeREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "outline.listTree", bindErr(err))
		return
	}
	vos, err := a.svc.ListTree(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "outline.listTree", err)
		return
	}
	pkg.Ok(c, vos)
}

// SaveScene 保存详细场景卡片（POST /outline/scene/save）。
func (a *OutlineAPI) SaveScene(c *gin.Context) {
	var req domain.DetailedSceneREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "outline.saveScene", bindErr(err))
		return
	}
	vo, err := a.svc.SaveScene(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "outline.saveScene", err)
		return
	}
	pkg.Ok(c, vo)
}

// DeleteScene 删除详细场景卡片（POST /outline/scene/delete）。
func (a *OutlineAPI) DeleteScene(c *gin.Context) {
	var req domain.DetailedSceneDeleteREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "outline.deleteScene", bindErr(err))
		return
	}
	if err := a.svc.DeleteScene(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req); err != nil {
		respondErr(c, "outline.deleteScene", err)
		return
	}
	pkg.Ok(c, nil)
}

// ListScenes 查询场景列表（POST /outline/scene/list）。
func (a *OutlineAPI) ListScenes(c *gin.Context) {
	var req domain.DetailedSceneListREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "outline.listScenes", bindErr(err))
		return
	}
	vos, err := a.svc.ListScenes(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "outline.listScenes", err)
		return
	}
	pkg.Ok(c, vos)
}

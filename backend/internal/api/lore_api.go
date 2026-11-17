package api

import (
	"github.com/gin-gonic/gin"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/service"
)

// LoreAPI 设定模块 HTTP 处理器。
type LoreAPI struct {
	svc *service.LoreService
}

// NewLoreAPI 构造设定处理器。
func NewLoreAPI(s *service.LoreService) *LoreAPI {
	return &LoreAPI{svc: s}
}

// Save 新建/更新设定（POST /lore/save）。
func (a *LoreAPI) Save(c *gin.Context) {
	var req domain.LoreREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "lore.save", bindErr(err))
		return
	}
	vo, err := a.svc.Save(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "lore.save", err)
		return
	}
	pkg.Ok(c, vo)
}

// Delete 删除设定（POST /lore/delete）。
func (a *LoreAPI) Delete(c *gin.Context) {
	var req domain.LoreDeleteREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "lore.delete", bindErr(err))
		return
	}
	if err := a.svc.Delete(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req); err != nil {
		respondErr(c, "lore.delete", err)
		return
	}
	pkg.Ok(c, nil)
}

// List 查询设定列表（POST /lore/list）。
func (a *LoreAPI) List(c *gin.Context) {
	var req domain.LoreListREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "lore.list", bindErr(err))
		return
	}
	vos, err := a.svc.List(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "lore.list", err)
		return
	}
	pkg.Ok(c, vos)
}

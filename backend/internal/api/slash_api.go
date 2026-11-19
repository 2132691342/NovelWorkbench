// 本文件实现斜杠命令模块的 HTTP 处理器：技能列表、保存、删除与正文解析。

package api

import (
	"github.com/gin-gonic/gin"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/service"
)

// SlashAPI 斜杠命令模块 HTTP 处理器。
type SlashAPI struct {
	svc *service.SlashService
}

// NewSlashAPI 构造斜杠命令处理器。
func NewSlashAPI(s *service.SlashService) *SlashAPI {
	return &SlashAPI{svc: s}
}

// List 查询命令列表（GET /slash/list，?enabled_only=true 仅启用）。
func (a *SlashAPI) List(c *gin.Context) {
	var req domain.SlashListREQ
	if err := c.ShouldBindQuery(&req); err != nil {
		respondErr(c, "slash.list", bindErr(err))
		return
	}
	vos, err := a.svc.List(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "slash.list", err)
		return
	}
	pkg.Ok(c, vos)
}

// Save 新建/更新命令（POST /slash/save）。
func (a *SlashAPI) Save(c *gin.Context) {
	var req domain.SlashCommandREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "slash.save", bindErr(err))
		return
	}
	vo, err := a.svc.Save(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "slash.save", err)
		return
	}
	pkg.Ok(c, vo)
}

// Delete 删除命令（POST /slash/delete，内置不可删）。
func (a *SlashAPI) Delete(c *gin.Context) {
	var req domain.SlashDeleteREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "slash.delete", bindErr(err))
		return
	}
	if err := a.svc.Delete(c.Request.Context(), &req); err != nil {
		respondErr(c, "slash.delete", err)
		return
	}
	pkg.Ok(c, nil)
}

// Parse 解析正文斜杠命令（POST /slash/parse）。
func (a *SlashAPI) Parse(c *gin.Context) {
	var req domain.SlashParseREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "slash.parse", bindErr(err))
		return
	}
	vo, err := a.svc.Parse(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "slash.parse", err)
		return
	}
	pkg.Ok(c, vo)
}

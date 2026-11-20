// 本文件实现系统提示词档案模块的 HTTP 处理器：列表、详情、保存、删除。

package api

import (
	"github.com/gin-gonic/gin"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/service"
)

// PromptProfileAPI 提示词档案模块 HTTP 处理器。
type PromptProfileAPI struct {
	svc *service.PromptProfileService
}

// NewPromptProfileAPI 构造提示词档案处理器。
func NewPromptProfileAPI(s *service.PromptProfileService) *PromptProfileAPI {
	return &PromptProfileAPI{svc: s}
}

// List 查询档案列表（POST /prompt/profile/list）。
func (a *PromptProfileAPI) List(c *gin.Context) {
	var req domain.PromptProfileListREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "prompt.profile.list", bindErr(err))
		return
	}
	vos, err := a.svc.List(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "prompt.profile.list", err)
		return
	}
	pkg.Ok(c, vos)
}

// Detail 查询单个档案（GET /prompt/profile/detail/{PromptProfileULID}）。
func (a *PromptProfileAPI) Detail(c *gin.Context) {
	vo, err := a.svc.Detail(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), c.Param("PromptProfileULID"))
	if err != nil {
		respondErr(c, "prompt.profile.detail", err)
		return
	}
	pkg.Ok(c, vo)
}

// Save 新建/更新档案（POST /prompt/profile/save）。
func (a *PromptProfileAPI) Save(c *gin.Context) {
	var req domain.PromptProfileREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "prompt.profile.save", bindErr(err))
		return
	}
	vo, err := a.svc.Save(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "prompt.profile.save", err)
		return
	}
	pkg.Ok(c, vo)
}

// Delete 删除档案（POST /prompt/profile/delete）。
func (a *PromptProfileAPI) Delete(c *gin.Context) {
	var req domain.PromptProfileDeleteREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "prompt.profile.delete", bindErr(err))
		return
	}
	if err := a.svc.Delete(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req); err != nil {
		respondErr(c, "prompt.profile.delete", err)
		return
	}
	pkg.Ok(c, nil)
}

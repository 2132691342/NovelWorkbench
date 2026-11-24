// 本文件实现提示词模块的 HTTP 处理器：列表、详情、保存、删除与对话前置解析。

package api

import (
	"github.com/gin-gonic/gin"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/service"
)

// PromptAPI 提示词模块 HTTP 处理器。
type PromptAPI struct {
	svc *service.PromptService
}

// NewPromptAPI 构造提示词处理器。
func NewPromptAPI(s *service.PromptService) *PromptAPI {
	return &PromptAPI{svc: s}
}

// List 查询提示词列表（GET /prompt/list）。
func (a *PromptAPI) List(c *gin.Context) {
	var req domain.PromptListREQ
	if err := c.ShouldBindQuery(&req); err != nil {
		respondErr(c, "prompt.list", bindErr(err))
		return
	}
	vos, err := a.svc.List(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "prompt.list", err)
		return
	}
	pkg.Ok(c, vos)
}

// Detail 查询提示词详情（GET /prompt/detail/{PromptULID}）。
func (a *PromptAPI) Detail(c *gin.Context) {
	vo, err := a.svc.Detail(c.Request.Context(), c.Param("PromptULID"))
	if err != nil {
		respondErr(c, "prompt.detail", err)
		return
	}
	pkg.Ok(c, vo)
}

// Save 创建或更新提示词（POST /prompt/save）。
func (a *PromptAPI) Save(c *gin.Context) {
	var req domain.PromptREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "prompt.save", bindErr(err))
		return
	}
	vo, err := a.svc.Save(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "prompt.save", err)
		return
	}
	pkg.Ok(c, vo)
}

// Delete 删除提示词（POST /prompt/delete）。
func (a *PromptAPI) Delete(c *gin.Context) {
	var req domain.PromptDeleteREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "prompt.delete", bindErr(err))
		return
	}
	if err := a.svc.Delete(c.Request.Context(), &req); err != nil {
		respondErr(c, "prompt.delete", err)
		return
	}
	pkg.Ok(c, nil)
}

// Resolve 解析斜杠命令为提示词（POST /prompt/resolve，供对话前置预览）。
func (a *PromptAPI) Resolve(c *gin.Context) {
	var req domain.PromptResolveREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "prompt.resolve", bindErr(err))
		return
	}
	dto, err := a.svc.Resolve(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "prompt.resolve", err)
		return
	}
	pkg.Ok(c, dto)
}

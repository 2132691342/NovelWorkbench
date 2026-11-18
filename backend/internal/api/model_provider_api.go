package api

import (
	"github.com/gin-gonic/gin"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/service"
)

// ModelProviderAPI 模型供应商模块 HTTP 处理器（协议、密钥、默认切换、连通性测试）。
type ModelProviderAPI struct {
	svc *service.ModelProviderService
}

// NewModelProviderAPI 构造模型供应商处理器。
func NewModelProviderAPI(s *service.ModelProviderService) *ModelProviderAPI {
	return &ModelProviderAPI{svc: s}
}

// List 查询供应商列表（POST /model/provider/list，密钥仅回显掩码）。
func (a *ModelProviderAPI) List(c *gin.Context) {
	vos, err := a.svc.List(c.Request.Context())
	if err != nil {
		respondErr(c, "model.provider.list", err)
		return
	}
	pkg.Ok(c, vos)
}

// Save 新建/更新供应商（POST /model/provider/save）。
func (a *ModelProviderAPI) Save(c *gin.Context) {
	var req domain.ModelProviderREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "model.provider.save", bindErr(err))
		return
	}
	vo, err := a.svc.Save(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "model.provider.save", err)
		return
	}
	pkg.Ok(c, vo)
}

// Delete 删除供应商（POST /model/provider/delete，默认供应商需先切换）。
func (a *ModelProviderAPI) Delete(c *gin.Context) {
	var req domain.ModelProviderDeleteREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "model.provider.delete", bindErr(err))
		return
	}
	if err := a.svc.Delete(c.Request.Context(), &req); err != nil {
		respondErr(c, "model.provider.delete", err)
		return
	}
	pkg.Ok(c, nil)
}

// Test 连通性测试（POST /model/provider/test，结果回写供应商记录）。
func (a *ModelProviderAPI) Test(c *gin.Context) {
	var req domain.ModelProviderTestREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "model.provider.test", bindErr(err))
		return
	}
	dto, err := a.svc.Test(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "model.provider.test", err)
		return
	}
	pkg.Ok(c, dto)
}

// Capability 查询当前 AI 能力（GET /model/capability，前端据此提示可用性）。
func (a *ModelProviderAPI) Capability(c *gin.Context) {
	vo, err := a.svc.Capability(c.Request.Context())
	if err != nil {
		respondErr(c, "model.capability", err)
		return
	}
	pkg.Ok(c, vo)
}

// ChatOptions 查询可选对话供应商（GET /model/chat_options，AI 面板模型选择器消费）。
func (a *ModelProviderAPI) ChatOptions(c *gin.Context) {
	vo, err := a.svc.Capability(c.Request.Context())
	if err != nil {
		respondErr(c, "model.chat_options", err)
		return
	}
	options, err := a.svc.List(c.Request.Context())
	if err != nil {
		respondErr(c, "model.chat_options", err)
		return
	}
	pkg.Ok(c, gin.H{"capability_vo": vo, "provider_list": options})
}

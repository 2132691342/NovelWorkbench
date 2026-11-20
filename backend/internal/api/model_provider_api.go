// 本文件实现模型生态模块的 HTTP 处理器：供应商、模型目录、角色分配、连通性测试与能力探测。

package api

import (
	"github.com/gin-gonic/gin"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/service"
)

// ModelProviderAPI 模型生态模块 HTTP 处理器。
type ModelProviderAPI struct {
	svc *service.ModelProviderService
}

// NewModelProviderAPI 构造模型生态处理器。
func NewModelProviderAPI(s *service.ModelProviderService) *ModelProviderAPI {
	return &ModelProviderAPI{svc: s}
}

// ProviderList 查询供应商列表（POST /model/provider/list，密钥仅回显掩码）。
func (a *ModelProviderAPI) ProviderList(c *gin.Context) {
	vos, err := a.svc.ListProviders(c.Request.Context())
	if err != nil {
		respondErr(c, "model.provider.list", err)
		return
	}
	pkg.Ok(c, vos)
}

// ProviderSave 新建/更新供应商（POST /model/provider/save）。
func (a *ModelProviderAPI) ProviderSave(c *gin.Context) {
	var req domain.ModelProviderREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "model.provider.save", bindErr(err))
		return
	}
	vo, err := a.svc.SaveProvider(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "model.provider.save", err)
		return
	}
	pkg.Ok(c, vo)
}

// ProviderDelete 删除供应商（POST /model/provider/delete，其下仍有模型时拒绝）。
func (a *ModelProviderAPI) ProviderDelete(c *gin.Context) {
	var req domain.ModelProviderDeleteREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "model.provider.delete", bindErr(err))
		return
	}
	if err := a.svc.DeleteProvider(c.Request.Context(), &req); err != nil {
		respondErr(c, "model.provider.delete", err)
		return
	}
	pkg.Ok(c, nil)
}

// ProviderTest 供应商连通性测试（POST /model/provider/test）。
func (a *ModelProviderAPI) ProviderTest(c *gin.Context) {
	var req domain.ModelProviderTestREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "model.provider.test", bindErr(err))
		return
	}
	dto, err := a.svc.TestProvider(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "model.provider.test", err)
		return
	}
	pkg.Ok(c, dto)
}

// ModelList 查询模型目录（POST /model/catalog/list，provider_ulid 可空）。
func (a *ModelProviderAPI) ModelList(c *gin.Context) {
	var req domain.ModelCatalogListREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "model.catalog.list", bindErr(err))
		return
	}
	vos, err := a.svc.ListModels(c.Request.Context(), req.ProviderULID)
	if err != nil {
		respondErr(c, "model.catalog.list", err)
		return
	}
	pkg.Ok(c, vos)
}

// ModelSave 新建/更新模型（POST /model/catalog/save，含角色授权整体覆盖）。
func (a *ModelProviderAPI) ModelSave(c *gin.Context) {
	var req domain.ModelCatalogREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "model.catalog.save", bindErr(err))
		return
	}
	vo, err := a.svc.SaveModel(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "model.catalog.save", err)
		return
	}
	pkg.Ok(c, vo)
}

// ModelDelete 删除模型（POST /model/catalog/delete）。
func (a *ModelProviderAPI) ModelDelete(c *gin.Context) {
	var req domain.ModelCatalogDeleteREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "model.catalog.delete", bindErr(err))
		return
	}
	if err := a.svc.DeleteModel(c.Request.Context(), &req); err != nil {
		respondErr(c, "model.catalog.delete", err)
		return
	}
	pkg.Ok(c, nil)
}

// ModelTest 模型连通性测试（POST /model/catalog/test）。
func (a *ModelProviderAPI) ModelTest(c *gin.Context) {
	var req domain.ModelCatalogTestREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "model.catalog.test", bindErr(err))
		return
	}
	dto, err := a.svc.TestModel(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "model.catalog.test", err)
		return
	}
	pkg.Ok(c, dto)
}

// ModelSetDefault 设为系统默认模型（POST /model/catalog/set_default）。
func (a *ModelProviderAPI) ModelSetDefault(c *gin.Context) {
	var req domain.ModelCatalogTestREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "model.catalog.set_default", bindErr(err))
		return
	}
	if err := a.svc.SetDefaultModel(c.Request.Context(), &req); err != nil {
		respondErr(c, "model.catalog.set_default", err)
		return
	}
	pkg.Ok(c, nil)
}

// ModelGrant 整体覆盖模型的角色授权（POST /model/catalog/grant）。
func (a *ModelProviderAPI) ModelGrant(c *gin.Context) {
	var req domain.ModelGrantREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "model.catalog.grant", bindErr(err))
		return
	}
	if err := a.svc.GrantModel(c.Request.Context(), &req); err != nil {
		respondErr(c, "model.catalog.grant", err)
		return
	}
	pkg.Ok(c, nil)
}

// Capability 查询当前用户的 AI 能力（GET /model/capability）。
func (a *ModelProviderAPI) Capability(c *gin.Context) {
	vo, err := a.svc.Capability(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID))
	if err != nil {
		respondErr(c, "model.capability", err)
		return
	}
	pkg.Ok(c, vo)
}

package api

import (
	"github.com/gin-gonic/gin"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/service"
)

// ModelUsageAPI 模型用量模块 HTTP 处理器。
type ModelUsageAPI struct {
	svc *service.ModelUsageService
}

// NewModelUsageAPI 构造用量处理器。
func NewModelUsageAPI(s *service.ModelUsageService) *ModelUsageAPI {
	return &ModelUsageAPI{svc: s}
}

// Overview 查询区间用量总览（GET /model/usage/overview，?from_date=&to_date=）。
func (a *ModelUsageAPI) Overview(c *gin.Context) {
	var req domain.ModelUsageQueryREQ
	if err := c.ShouldBindQuery(&req); err != nil {
		respondErr(c, "model.usage.overview", bindErr(err))
		return
	}
	vo, err := a.svc.Overview(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "model.usage.overview", err)
		return
	}
	pkg.Ok(c, vo)
}

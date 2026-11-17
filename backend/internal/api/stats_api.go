package api

import (
	"github.com/gin-gonic/gin"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/service"
)

// StatsAPI 数据总览模块 HTTP 处理器。
type StatsAPI struct {
	svc *service.StatsService
}

// NewStatsAPI 构造数据总览处理器。
func NewStatsAPI(s *service.StatsService) *StatsAPI {
	return &StatsAPI{svc: s}
}

// Overview 查询数据总览（GET /stats/overview，?project_ulid= 可选全局）。
func (a *StatsAPI) Overview(c *gin.Context) {
	var req domain.StatsOverviewREQ
	if err := c.ShouldBindQuery(&req); err != nil {
		respondErr(c, "stats.overview", bindErr(err))
		return
	}
	vo, err := a.svc.Overview(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "stats.overview", err)
		return
	}
	pkg.Ok(c, vo)
}

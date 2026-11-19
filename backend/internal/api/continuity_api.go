// 本文件实现连续性看板模块的 HTTP 处理器：时间事实、伏笔钩子与章节摘要。

package api

import (
	"github.com/gin-gonic/gin"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/service"
)

// ContinuityAPI 连续性看板模块 HTTP 处理器。
type ContinuityAPI struct {
	svc *service.ContinuityService
}

// NewContinuityAPI 构造连续性看板处理器。
func NewContinuityAPI(s *service.ContinuityService) *ContinuityAPI {
	return &ContinuityAPI{svc: s}
}

// SaveFact 新建/更新时间事实（POST /continuity/fact/save）。
func (a *ContinuityAPI) SaveFact(c *gin.Context) {
	var req domain.TemporalFactSaveREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "continuity.fact.save", bindErr(err))
		return
	}
	vo, err := a.svc.SaveFact(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "continuity.fact.save", err)
		return
	}
	pkg.Ok(c, vo)
}

// ReviewFact 复核时间事实状态（POST /continuity/fact/review）。
func (a *ContinuityAPI) ReviewFact(c *gin.Context) {
	var req domain.TemporalFactReviewREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "continuity.fact.review", bindErr(err))
		return
	}
	vo, err := a.svc.ReviewFact(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "continuity.fact.review", err)
		return
	}
	pkg.Ok(c, vo)
}

// ListFacts 查询时间事实列表（POST /continuity/fact/list）。
func (a *ContinuityAPI) ListFacts(c *gin.Context) {
	var req domain.TemporalFactListREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "continuity.fact.list", bindErr(err))
		return
	}
	vos, err := a.svc.ListFacts(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "continuity.fact.list", err)
		return
	}
	pkg.Ok(c, vos)
}

// SaveHook 新建/更新伏笔钩子（POST /continuity/hook/save）。
func (a *ContinuityAPI) SaveHook(c *gin.Context) {
	var req domain.ForeshadowHookSaveREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "continuity.hook.save", bindErr(err))
		return
	}
	vo, err := a.svc.SaveHook(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "continuity.hook.save", err)
		return
	}
	pkg.Ok(c, vo)
}

// ListHooks 查询伏笔钩子列表（POST /continuity/hook/list）。
func (a *ContinuityAPI) ListHooks(c *gin.Context) {
	var req domain.ForeshadowHookListREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "continuity.hook.list", bindErr(err))
		return
	}
	vos, err := a.svc.ListHooks(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "continuity.hook.list", err)
		return
	}
	pkg.Ok(c, vos)
}

// SaveSummary 新建/更新章节摘要（POST /continuity/summary/save）。
func (a *ContinuityAPI) SaveSummary(c *gin.Context) {
	var req domain.ChapterSummarySaveREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "continuity.summary.save", bindErr(err))
		return
	}
	vo, err := a.svc.SaveSummary(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "continuity.summary.save", err)
		return
	}
	pkg.Ok(c, vo)
}

// ListSummaries 查询章节摘要列表（POST /continuity/summary/list）。
func (a *ContinuityAPI) ListSummaries(c *gin.Context) {
	var req domain.ChapterSummaryListREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "continuity.summary.list", bindErr(err))
		return
	}
	vos, err := a.svc.ListSummaries(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "continuity.summary.list", err)
		return
	}
	pkg.Ok(c, vos)
}

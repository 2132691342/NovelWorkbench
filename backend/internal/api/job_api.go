// 本文件实现异步任务中心的 HTTP 处理器：任务分页与失败重试。

package api

import (
	"github.com/gin-gonic/gin"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/service"
)

// JobAPI 异步任务中心 HTTP 处理器。
type JobAPI struct {
	svc *service.JobService
}

// NewJobAPI 构造任务中心处理器。
func NewJobAPI(s *service.JobService) *JobAPI {
	return &JobAPI{svc: s}
}

// List 分页查询异步任务（POST /admin/job/list）。
func (a *JobAPI) List(c *gin.Context) {
	var req domain.JobListREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "admin.job.list", bindErr(err))
		return
	}
	page, err := a.svc.ListPage(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "admin.job.list", err)
		return
	}
	pkg.Ok(c, page)
}

// Stats 查询任务状态概览（GET /admin/job/stats）。
func (a *JobAPI) Stats(c *gin.Context) {
	vo, err := a.svc.Stats(c.Request.Context())
	if err != nil {
		respondErr(c, "admin.job.stats", err)
		return
	}
	pkg.Ok(c, vo)
}

// Retry 重试失败任务（POST /admin/job/retry）。
func (a *JobAPI) Retry(c *gin.Context) {
	var req domain.JobRetryREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "admin.job.retry", bindErr(err))
		return
	}
	vo, err := a.svc.Retry(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "admin.job.retry", err)
		return
	}
	pkg.Ok(c, vo)
}

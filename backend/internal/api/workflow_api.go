// 本文件实现工作流模块的 HTTP 处理器：模板、运行创建/查询、审核门禁与执行控制。

package api

import (
	"context"

	"github.com/gin-gonic/gin"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/service"
)

// WorkflowAPI 工作流模块 HTTP 处理器。
type WorkflowAPI struct {
	svc *service.WorkflowService
}

// NewWorkflowAPI 构造工作流处理器。
func NewWorkflowAPI(s *service.WorkflowService) *WorkflowAPI {
	return &WorkflowAPI{svc: s}
}

// TemplateList 查询可用模板（GET /workflow/template/list）。
func (a *WorkflowAPI) TemplateList(c *gin.Context) {
	vos, err := a.svc.ListTemplates(c.Request.Context())
	if err != nil {
		respondErr(c, "workflow.template.list", err)
		return
	}
	pkg.Ok(c, vos)
}

// RunCreate 创建工作流运行（POST /workflow/run/create）。
func (a *WorkflowAPI) RunCreate(c *gin.Context) {
	var req domain.WorkflowRunCreateREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "workflow.run.create", bindErr(err))
		return
	}
	vo, err := a.svc.CreateRun(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "workflow.run.create", err)
		return
	}
	pkg.Ok(c, vo)
}

// RunList 分页查询运行实例（POST /workflow/run/list）。
func (a *WorkflowAPI) RunList(c *gin.Context) {
	var req domain.WorkflowRunListREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "workflow.run.list", bindErr(err))
		return
	}
	page, err := a.svc.ListRuns(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "workflow.run.list", err)
		return
	}
	pkg.Ok(c, page)
}

// RunDetail 查询运行详情含步骤时间线（GET /workflow/run/detail/{RunULID}）。
func (a *WorkflowAPI) RunDetail(c *gin.Context) {
	vo, err := a.svc.Detail(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), c.Param("RunULID"))
	if err != nil {
		respondErr(c, "workflow.run.detail", err)
		return
	}
	pkg.Ok(c, vo)
}

// RunStats 查询运行状态概览（GET /workflow/run/stats）。
func (a *WorkflowAPI) RunStats(c *gin.Context) {
	var req struct {
		ProjectULID string `form:"project_ulid"`
	}
	if err := c.ShouldBindQuery(&req); err != nil {
		respondErr(c, "workflow.run.stats", bindErr(err))
		return
	}
	vo, err := a.svc.Stats(c.Request.Context(), req.ProjectULID)
	if err != nil {
		respondErr(c, "workflow.run.stats", err)
		return
	}
	pkg.Ok(c, vo)
}

// RunApprove 人工审核通过（POST /workflow/run/approve）。
func (a *WorkflowAPI) RunApprove(c *gin.Context) {
	var req domain.WorkflowRunApproveREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "workflow.run.approve", bindErr(err))
		return
	}
	vo, err := a.svc.Approve(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "workflow.run.approve", err)
		return
	}
	pkg.Ok(c, vo)
}

// RunReject 人工审核退回（POST /workflow/run/reject）。
func (a *WorkflowAPI) RunReject(c *gin.Context) {
	var req domain.WorkflowRunRejectREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "workflow.run.reject", bindErr(err))
		return
	}
	vo, err := a.svc.Reject(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "workflow.run.reject", err)
		return
	}
	pkg.Ok(c, vo)
}

// RunPause 暂停运行（POST /workflow/run/pause）。
func (a *WorkflowAPI) RunPause(c *gin.Context) {
	a.runControl(c, "workflow.run.pause", a.svc.Pause)
}

// RunResume 继续已暂停的运行（POST /workflow/run/resume）。
func (a *WorkflowAPI) RunResume(c *gin.Context) {
	a.runControl(c, "workflow.run.resume", a.svc.Resume)
}

// RunCancel 取消运行（POST /workflow/run/cancel）。
func (a *WorkflowAPI) RunCancel(c *gin.Context) {
	a.runControl(c, "workflow.run.cancel", a.svc.Cancel)
}

// RunRetry 重试失败的运行（POST /workflow/run/retry）。
func (a *WorkflowAPI) RunRetry(c *gin.Context) {
	a.runControl(c, "workflow.run.retry", a.svc.Retry)
}

// runControl 执行控制类动作的统一绑定与错误出口（四个动作请求体一致）。
func (a *WorkflowAPI) runControl(c *gin.Context, op string,
	action func(ctx context.Context, userULID string, req *domain.WorkflowRunIDREQ) (domain.WorkflowRunVO, error)) {
	var req domain.WorkflowRunIDREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, op, bindErr(err))
		return
	}
	vo, err := action(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, op, err)
		return
	}
	pkg.Ok(c, vo)
}

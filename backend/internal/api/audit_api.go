// 本文件实现审计日志模块的 HTTP 处理器：分页查询、详情、链路时间线、动作码汇总与留存清理。

package api

import (
	"github.com/gin-gonic/gin"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/service"
)

// AuditAPI 审计日志模块 HTTP 处理器。
type AuditAPI struct {
	svc *service.AuditService
}

// NewAuditAPI 构造审计日志处理器。
func NewAuditAPI(s *service.AuditService) *AuditAPI {
	return &AuditAPI{svc: s}
}

// List 分页查询审计日志（POST /admin/audit/list）。
func (a *AuditAPI) List(c *gin.Context) {
	var req domain.AuditListREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "admin.audit.list", bindErr(err))
		return
	}
	page, err := a.svc.ListPage(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "admin.audit.list", err)
		return
	}
	pkg.Ok(c, page)
}

// TraceList 分页查询链路（POST /admin/audit/trace/list，审计页主视图）。
func (a *AuditAPI) TraceList(c *gin.Context) {
	var req domain.AuditTraceListREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "admin.audit.trace_list", bindErr(err))
		return
	}
	page, err := a.svc.ListTracePage(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "admin.audit.trace_list", err)
		return
	}
	pkg.Ok(c, page)
}

// Stats 返回区间内审计概览统计（POST /admin/audit/stats）。
func (a *AuditAPI) Stats(c *gin.Context) {
	var req domain.AuditTraceListREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "admin.audit.stats", bindErr(err))
		return
	}
	vo, err := a.svc.Stats(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "admin.audit.stats", err)
		return
	}
	pkg.Ok(c, vo)
}

// Detail 查询审计详情（GET /admin/audit/detail/{AuditULID}，含请求体与响应体原文）。
func (a *AuditAPI) Detail(c *gin.Context) {
	req := domain.AuditDetailREQ{AuditULID: c.Param("AuditULID")}
	if err := c.ShouldBindQuery(&req); err != nil {
		respondErr(c, "admin.audit.detail", bindErr(err))
		return
	}
	vo, err := a.svc.Detail(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "admin.audit.detail", err)
		return
	}
	pkg.Ok(c, vo)
}

// Trace 拉取链路时间线（GET /admin/audit/trace/{TraceID}）。
func (a *AuditAPI) Trace(c *gin.Context) {
	req := domain.AuditTraceREQ{TraceID: c.Param("TraceID")}
	if err := c.ShouldBindQuery(&req); err != nil {
		respondErr(c, "admin.audit.trace", bindErr(err))
		return
	}
	vo, err := a.svc.Trace(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "admin.audit.trace", err)
		return
	}
	pkg.Ok(c, vo)
}

// ActionOptions 返回动作码筛选项（GET /admin/audit/action_options）。
func (a *AuditAPI) ActionOptions(c *gin.Context) {
	vos, err := a.svc.ActionOptions(c.Request.Context())
	if err != nil {
		respondErr(c, "admin.audit.action_options", err)
		return
	}
	pkg.Ok(c, vos)
}

// Retention 查询留存策略现状（GET /admin/audit/retention）。
func (a *AuditAPI) Retention(c *gin.Context) {
	pkg.Ok(c, a.svc.Retention(c.Request.Context()))
}

// Purge 立即执行一次留存清理（POST /admin/audit/purge）。
func (a *AuditAPI) Purge(c *gin.Context) {
	vo, err := a.svc.Purge(c.Request.Context())
	if err != nil {
		respondErr(c, "admin.audit.purge", err)
		return
	}
	pkg.Ok(c, vo)
}

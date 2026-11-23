// 本文件实现审计日志模块的 HTTP 处理器：分页查询写操作审计记录。

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

// Detail 查询审计详情（GET /admin/audit/detail/{AuditULID}，含请求体原文）。
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

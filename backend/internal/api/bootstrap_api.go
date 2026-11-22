// 本文件实现 AI 开书模块的 HTTP 处理器：生成方案与按方案落库。

package api

import (
	"github.com/gin-gonic/gin"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/service"
)

// BootstrapAPI AI 开书模块 HTTP 处理器。
type BootstrapAPI struct {
	svc *service.BootstrapService
}

// NewBootstrapAPI 构造开书处理器。
func NewBootstrapAPI(s *service.BootstrapService) *BootstrapAPI {
	return &BootstrapAPI{svc: s}
}

// Plan 生成开书方案（POST /project/bootstrap/plan，仅生成不落库）。
func (a *BootstrapAPI) Plan(c *gin.Context) {
	var req domain.ProjectBootstrapREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "project.bootstrap.plan", bindErr(err))
		return
	}
	vo, err := a.svc.GeneratePlan(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "project.bootstrap.plan", err)
		return
	}
	pkg.Ok(c, vo)
}

// Apply 按方案落库（POST /project/bootstrap/apply，事务内创建项目与全部资产）。
func (a *BootstrapAPI) Apply(c *gin.Context) {
	var req domain.ProjectBootstrapApplyREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "project.bootstrap.apply", bindErr(err))
		return
	}
	vo, err := a.svc.ApplyPlan(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "project.bootstrap.apply", err)
		return
	}
	pkg.Ok(c, vo)
}

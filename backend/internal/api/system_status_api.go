package api

import (
	"github.com/gin-gonic/gin"

	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/service"
)

// SystemStatusAPI 系统运行状态模块 HTTP 处理器。
type SystemStatusAPI struct {
	svc *service.SystemStatusService
}

// NewSystemStatusAPI 构造运行状态处理器。
func NewSystemStatusAPI(s *service.SystemStatusService) *SystemStatusAPI {
	return &SystemStatusAPI{svc: s}
}

// Status 查询系统运行状态（GET /system/status，依赖探测并发执行）。
func (a *SystemStatusAPI) Status(c *gin.Context) {
	vo, err := a.svc.Status(c.Request.Context())
	if err != nil {
		respondErr(c, "system.status", err)
		return
	}
	pkg.Ok(c, vo)
}

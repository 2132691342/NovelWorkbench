package api

import (
	"github.com/gin-gonic/gin"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/service"
)

// SettingAPI 应用配置模块 HTTP 处理器。
type SettingAPI struct {
	svc *service.SettingService
}

// NewSettingAPI 构造应用配置处理器。
func NewSettingAPI(s *service.SettingService) *SettingAPI {
	return &SettingAPI{svc: s}
}

// List 查询配置列表（GET /setting/list）。
func (a *SettingAPI) List(c *gin.Context) {
	vos, err := a.svc.List(c.Request.Context())
	if err != nil {
		respondErr(c, "setting.list", err)
		return
	}
	pkg.Ok(c, vos)
}

// Update 更新配置（POST /setting/update）。
func (a *SettingAPI) Update(c *gin.Context) {
	var req domain.SettingUpdateREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "setting.update", bindErr(err))
		return
	}
	vo, err := a.svc.Update(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "setting.update", err)
		return
	}
	pkg.Ok(c, vo)
}

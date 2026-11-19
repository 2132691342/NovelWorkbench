// 本文件实现卷模块的 HTTP 处理器：卷的创建、更新与删除。

package api

import (
	"github.com/gin-gonic/gin"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/service"
)

// VolumeAPI 卷模块 HTTP 处理器。
type VolumeAPI struct {
	svc *service.VolumeService
}

// NewVolumeAPI 构造卷处理器。
func NewVolumeAPI(s *service.VolumeService) *VolumeAPI {
	return &VolumeAPI{svc: s}
}

// Create 创建卷（POST /volume/create）。
func (a *VolumeAPI) Create(c *gin.Context) {
	var req domain.VolumeCreateREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "volume.create", bindErr(err))
		return
	}
	vo, err := a.svc.Create(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "volume.create", err)
		return
	}
	pkg.Ok(c, vo)
}

// Update 更新卷（POST /volume/update）。
func (a *VolumeAPI) Update(c *gin.Context) {
	var req domain.VolumeUpdateREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "volume.update", bindErr(err))
		return
	}
	vo, err := a.svc.Update(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "volume.update", err)
		return
	}
	pkg.Ok(c, vo)
}

// Delete 删除卷（POST /volume/delete）。
func (a *VolumeAPI) Delete(c *gin.Context) {
	var req domain.VolumeDeleteREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "volume.delete", bindErr(err))
		return
	}
	if err := a.svc.Delete(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req); err != nil {
		respondErr(c, "volume.delete", err)
		return
	}
	pkg.Ok(c, nil)
}

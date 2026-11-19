// 本文件实现技能模块的 HTTP 处理器：列表、详情、保存、删除、解析与参考资料管理。

package api

import (
	"github.com/gin-gonic/gin"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/service"
)

// SkillAPI 技能模块 HTTP 处理器。
type SkillAPI struct {
	svc *service.SkillService
}

// NewSkillAPI 构造技能处理器。
func NewSkillAPI(s *service.SkillService) *SkillAPI {
	return &SkillAPI{svc: s}
}

// List 查询技能列表（GET /skill/list）。
func (a *SkillAPI) List(c *gin.Context) {
	var req domain.SkillListREQ
	if err := c.ShouldBindQuery(&req); err != nil {
		respondErr(c, "skill.list", bindErr(err))
		return
	}
	vos, err := a.svc.List(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "skill.list", err)
		return
	}
	pkg.Ok(c, vos)
}

// Detail 查询技能详情（GET /skill/detail/{SkillULID}）。
func (a *SkillAPI) Detail(c *gin.Context) {
	vo, err := a.svc.Detail(c.Request.Context(), c.Param("SkillULID"))
	if err != nil {
		respondErr(c, "skill.detail", err)
		return
	}
	pkg.Ok(c, vo)
}

// Save 创建或更新技能（POST /skill/save）。
func (a *SkillAPI) Save(c *gin.Context) {
	var req domain.SkillREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "skill.save", bindErr(err))
		return
	}
	vo, err := a.svc.Save(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "skill.save", err)
		return
	}
	pkg.Ok(c, vo)
}

// Delete 删除技能（POST /skill/delete）。
func (a *SkillAPI) Delete(c *gin.Context) {
	var req domain.SkillDeleteREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "skill.delete", bindErr(err))
		return
	}
	if err := a.svc.Delete(c.Request.Context(), &req); err != nil {
		respondErr(c, "skill.delete", err)
		return
	}
	pkg.Ok(c, nil)
}

// Resolve 解析技能与变量（POST /skill/resolve，供对话前置确认必填变量）。
func (a *SkillAPI) Resolve(c *gin.Context) {
	var req domain.SkillResolveREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "skill.resolve", bindErr(err))
		return
	}
	dto, err := a.svc.Resolve(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "skill.resolve", err)
		return
	}
	pkg.Ok(c, dto)
}

// ReferenceList 查询技能参考资料（GET /skill/reference/list）。
func (a *SkillAPI) ReferenceList(c *gin.Context) {
	var req domain.SkillListBySkillREQ
	if err := c.ShouldBindQuery(&req); err != nil {
		respondErr(c, "skill.reference.list", bindErr(err))
		return
	}
	vos, err := a.svc.ReferenceList(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "skill.reference.list", err)
		return
	}
	pkg.Ok(c, vos)
}

// ReferenceSave 保存技能参考资料（POST /skill/reference/save）。
func (a *SkillAPI) ReferenceSave(c *gin.Context) {
	var req domain.SkillReferenceREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "skill.reference.save", bindErr(err))
		return
	}
	vo, err := a.svc.SaveReference(c.Request.Context(), &req)
	if err != nil {
		respondErr(c, "skill.reference.save", err)
		return
	}
	pkg.Ok(c, vo)
}

// ReferenceDelete 删除技能参考资料（POST /skill/reference/delete）。
func (a *SkillAPI) ReferenceDelete(c *gin.Context) {
	var req domain.SkillReferenceDeleteREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "skill.reference.delete", bindErr(err))
		return
	}
	if err := a.svc.DeleteReference(c.Request.Context(), &req); err != nil {
		respondErr(c, "skill.reference.delete", err)
		return
	}
	pkg.Ok(c, nil)
}

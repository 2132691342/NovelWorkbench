// 本文件实现角色模块的 HTTP 处理器：角色卡 CRUD、关系图谱与关系边维护。

package api

import (
	"github.com/gin-gonic/gin"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/service"
)

// CharacterAPI 角色模块 HTTP 处理器。
type CharacterAPI struct {
	svc *service.CharacterService
}

// NewCharacterAPI 构造角色处理器。
func NewCharacterAPI(s *service.CharacterService) *CharacterAPI {
	return &CharacterAPI{svc: s}
}

// Save 新建/更新角色（POST /character/save）。
func (a *CharacterAPI) Save(c *gin.Context) {
	var req domain.CharacterREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "character.save", bindErr(err))
		return
	}
	vo, err := a.svc.Save(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "character.save", err)
		return
	}
	pkg.Ok(c, vo)
}

// Delete 删除角色（POST /character/delete，连带删关系边）。
func (a *CharacterAPI) Delete(c *gin.Context) {
	var req domain.CharacterDeleteREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "character.delete", bindErr(err))
		return
	}
	if err := a.svc.Delete(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req); err != nil {
		respondErr(c, "character.delete", err)
		return
	}
	pkg.Ok(c, nil)
}

// List 查询角色列表（POST /character/list）。
func (a *CharacterAPI) List(c *gin.Context) {
	var req domain.CharacterListREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "character.list", bindErr(err))
		return
	}
	vos, err := a.svc.List(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "character.list", err)
		return
	}
	pkg.Ok(c, vos)
}

// Graph 查询角色关系图（GET /character/graph/{ProjectULID}）。
func (a *CharacterAPI) Graph(c *gin.Context) {
	req := domain.ProjectDetailREQ{ProjectULID: c.Param("ProjectULID")}
	if err := c.ShouldBindQuery(&req); err != nil {
		respondErr(c, "character.graph", bindErr(err))
		return
	}
	vo, err := a.svc.Graph(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "character.graph", err)
		return
	}
	pkg.Ok(c, vo)
}

// RelationSave 新建角色关系边（POST /character/relation/save）。
func (a *CharacterAPI) RelationSave(c *gin.Context) {
	var req domain.CharacterRelationREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "character.relation.save", bindErr(err))
		return
	}
	vo, err := a.svc.RelationSave(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "character.relation.save", err)
		return
	}
	pkg.Ok(c, vo)
}

// RelationDelete 删除角色关系边（POST /character/relation/delete）。
func (a *CharacterAPI) RelationDelete(c *gin.Context) {
	var req domain.CharacterRelationDeleteREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "character.relation.delete", bindErr(err))
		return
	}
	if err := a.svc.RelationDelete(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req); err != nil {
		respondErr(c, "character.relation.delete", err)
		return
	}
	pkg.Ok(c, nil)
}

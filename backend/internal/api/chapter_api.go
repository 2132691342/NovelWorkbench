// 本文件实现章节模块的 HTTP 处理器：章节 CRUD、详情与快照留档/回滚。

package api

import (
	"github.com/gin-gonic/gin"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/service"
)

// ChapterAPI 章节模块 HTTP 处理器。
type ChapterAPI struct {
	svc *service.ChapterService
}

// NewChapterAPI 构造章节处理器。
func NewChapterAPI(s *service.ChapterService) *ChapterAPI {
	return &ChapterAPI{svc: s}
}

// Create 创建章节（POST /chapter/create）。
func (a *ChapterAPI) Create(c *gin.Context) {
	var req domain.ChapterCreateREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "chapter.create", bindErr(err))
		return
	}
	vo, err := a.svc.Create(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "chapter.create", err)
		return
	}
	pkg.Ok(c, vo)
}

// Update 更新章节（POST /chapter/update）。
func (a *ChapterAPI) Update(c *gin.Context) {
	var req domain.ChapterUpdateREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "chapter.update", bindErr(err))
		return
	}
	vo, err := a.svc.Update(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "chapter.update", err)
		return
	}
	pkg.Ok(c, vo)
}

// Delete 删除章节（POST /chapter/delete，级联删除快照并回补字数）。
func (a *ChapterAPI) Delete(c *gin.Context) {
	var req domain.ChapterDeleteREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "chapter.delete", bindErr(err))
		return
	}
	if err := a.svc.Delete(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req); err != nil {
		respondErr(c, "chapter.delete", err)
		return
	}
	pkg.Ok(c, nil)
}

// Detail 查询章节详情含正文（GET /chapter/detail）。
func (a *ChapterAPI) Detail(c *gin.Context) {
	var req domain.ChapterContentREQ
	if err := c.ShouldBindQuery(&req); err != nil {
		respondErr(c, "chapter.detail", bindErr(err))
		return
	}
	vo, err := a.svc.Detail(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "chapter.detail", err)
		return
	}
	pkg.Ok(c, vo)
}

// SnapshotList 查询章节快照列表（GET /chapter/snapshot/list）。
func (a *ChapterAPI) SnapshotList(c *gin.Context) {
	var req domain.ChapterSnapshotListREQ
	if err := c.ShouldBindQuery(&req); err != nil {
		respondErr(c, "chapter.snapshot.list", bindErr(err))
		return
	}
	vos, err := a.svc.SnapshotList(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "chapter.snapshot.list", err)
		return
	}
	pkg.Ok(c, vos)
}

// SnapshotCreate 生成章节快照（POST /chapter/snapshot/create）。
func (a *ChapterAPI) SnapshotCreate(c *gin.Context) {
	var req domain.ChapterSnapshotCreateREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "chapter.snapshot.create", bindErr(err))
		return
	}
	vo, err := a.svc.SnapshotCreate(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "chapter.snapshot.create", err)
		return
	}
	pkg.Ok(c, vo)
}

// SnapshotRestore 回滚章节到快照（POST /chapter/snapshot/restore）。
func (a *ChapterAPI) SnapshotRestore(c *gin.Context) {
	var req domain.ChapterSnapshotRestoreREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "chapter.snapshot.restore", bindErr(err))
		return
	}
	vo, err := a.svc.SnapshotRestore(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "chapter.snapshot.restore", err)
		return
	}
	pkg.Ok(c, vo)
}

package api

import (
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/service"
)

// contentTypeByType 文档类型 → 响应 Content-Type 映射。
var contentTypeByType = map[string]string{
	"txt":  "text/plain; charset=utf-8",
	"md":   "text/markdown; charset=utf-8",
	"docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	"pdf":  "application/pdf",
}

// KnowledgeAPI 知识库模块 HTTP 处理器。
type KnowledgeAPI struct {
	svc *service.KnowledgeService
}

// NewKnowledgeAPI 构造知识库处理器。
func NewKnowledgeAPI(s *service.KnowledgeService) *KnowledgeAPI {
	return &KnowledgeAPI{svc: s}
}

// Upload 上传知识库文档（POST /knowledge/upload，multipart/form-data：file + 表单字段）。
func (a *KnowledgeAPI) Upload(c *gin.Context) {
	var req domain.KnowledgeUploadREQ
	if err := c.ShouldBind(&req); err != nil {
		respondErr(c, "knowledge.upload", bindErr(err))
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		respondErr(c, "knowledge.upload", bindErr(err))
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		respondErr(c, "knowledge.upload", bindErr(err))
		return
	}
	defer file.Close()
	fileDTO := &domain.KnowledgeUploadFileDTO{
		FileName: fileHeader.Filename,
		FileSize: fileHeader.Size,
		Reader:   file,
	}
	vo, err := a.svc.Upload(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req, fileDTO)
	if err != nil {
		respondErr(c, "knowledge.upload", err)
		return
	}
	pkg.Ok(c, vo)
}

// List 文档分页（POST /knowledge/list）。
func (a *KnowledgeAPI) List(c *gin.Context) {
	var req domain.KnowledgeListREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "knowledge.list", bindErr(err))
		return
	}
	page, err := a.svc.List(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "knowledge.list", err)
		return
	}
	pkg.Ok(c, page)
}

// Delete 删除文档（POST /knowledge/delete，连带删原文/切片/索引）。
func (a *KnowledgeAPI) Delete(c *gin.Context) {
	var req domain.KnowledgeDeleteREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "knowledge.delete", bindErr(err))
		return
	}
	if err := a.svc.Delete(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req); err != nil {
		respondErr(c, "knowledge.delete", err)
		return
	}
	pkg.Ok(c, nil)
}

// Search 知识库检索（POST /knowledge/search）。
func (a *KnowledgeAPI) Search(c *gin.Context) {
	var req domain.KnowledgeSearchREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "knowledge.search", bindErr(err))
		return
	}
	vos, err := a.svc.Search(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "knowledge.search", err)
		return
	}
	pkg.Ok(c, vos)
}

// Reindex 重投索引任务（POST /knowledge/reindex）。
func (a *KnowledgeAPI) Reindex(c *gin.Context) {
	var req domain.KnowledgeReindexREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "knowledge.reindex", bindErr(err))
		return
	}
	if err := a.svc.Reindex(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req); err != nil {
		respondErr(c, "knowledge.reindex", err)
		return
	}
	pkg.Ok(c, nil)
}

// ChunkList 文档切片分页（GET /knowledge/chunk/list）。
func (a *KnowledgeAPI) ChunkList(c *gin.Context) {
	var req domain.KnowledgeChunkListREQ
	if err := c.ShouldBindQuery(&req); err != nil {
		respondErr(c, "knowledge.chunk.list", bindErr(err))
		return
	}
	page, err := a.svc.ChunkList(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "knowledge.chunk.list", err)
		return
	}
	pkg.Ok(c, page)
}

// Content 输出文档原文（GET /knowledge/content，供前端预览 docx/pdf/txt/md）。
func (a *KnowledgeAPI) Content(c *gin.Context) {
	var req domain.KnowledgeContentREQ
	if err := c.ShouldBindQuery(&req); err != nil {
		respondErr(c, "knowledge.content", bindErr(err))
		return
	}
	contentDTO, err := a.svc.GetContent(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "knowledge.content", err)
		return
	}
	contentType := contentTypeByType[contentDTO.DocTypeCode]
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Header("Content-Disposition", "inline; filename*=UTF-8''"+url.PathEscape(contentDTO.DocName))
	c.Data(http.StatusOK, contentType, contentDTO.ContentBytes)
}

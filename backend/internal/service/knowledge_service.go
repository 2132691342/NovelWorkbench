// 本文件实现知识库业务：文档上传/列表/删除/重投、检索与切片查询。

package service

import (
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/infra"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// rrfRankConstant RRF 融合常数（score = Σ 1/(60+rank)）。
const rrfRankConstant = 60.0

// KnowledgeIndexConfig 索引链路参数（infra 配置的标量子集，避免 service 依赖配置类型）。
type KnowledgeIndexConfig struct {
	UploadMaxMB    int    // 上传大小上限（MB），0 不限
	ChunkSize      int    // 切片字符数
	ChunkOverlap   int    // 切片重叠字符数
	RAGTopK        int    // RAG 默认召回条数
	ManticoreTable string // 全文索引表名（空回退 nw_chunks）
}

// KnowledgeService 知识库业务：文档上传/列表/删除/重投、检索与切片查看。
type KnowledgeService struct {
	knowledgeRepo *repo.KnowledgeRepo    // 知识库仓储
	objectRepo    *repo.ObjectRepo       // 对象存储仓储
	mqRepo        *repo.MQRepo           // 队列仓储
	jobRepo       *repo.JobRepo          // 任务仓储
	projectRepo   *repo.ProjectRepo      // 项目仓储（归属校验）
	settingRepo   *repo.SettingRepo      // 应用配置仓储（默认召回条数）
	aiClient      *ai.Client             // 模型网关（流式调用与用量上报）
	providerSvc   *ModelProviderService  // 供应商服务（模型选择）
	manticore     *infra.ManticoreClient // 全文检索客户端
	db            *gorm.DB               // 事务入口
	config        KnowledgeIndexConfig   // 索引链路参数
}

// NewKnowledgeService 构造知识库服务。
func NewKnowledgeService(knowledgeRepo *repo.KnowledgeRepo, objectRepo *repo.ObjectRepo, mqRepo *repo.MQRepo, jobRepo *repo.JobRepo, projectRepo *repo.ProjectRepo, settingRepo *repo.SettingRepo, aiClient *ai.Client, providerSvc *ModelProviderService, manticore *infra.ManticoreClient, db *gorm.DB, config KnowledgeIndexConfig) *KnowledgeService {
	return &KnowledgeService{
		knowledgeRepo: knowledgeRepo,
		objectRepo:    objectRepo,
		mqRepo:        mqRepo,
		jobRepo:       jobRepo,
		projectRepo:   projectRepo,
		settingRepo:   settingRepo,
		aiClient:      aiClient,
		providerSvc:   providerSvc,
		manticore:     manticore,
		db:            db,
		config:        config,
	}
}

// Upload 上传知识库文档：原文入 MinIO、元数据落库、投递异步索引任务。
func (s *KnowledgeService) Upload(ctx context.Context, userULID string, req *domain.KnowledgeUploadREQ, fileDTO *domain.KnowledgeUploadFileDTO) (domain.KnowledgeDocumentVO, error) {
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID); err != nil {
		return domain.KnowledgeDocumentVO{}, err
	}
	if fileDTO == nil || fileDTO.Reader == nil {
		return domain.KnowledgeDocumentVO{}, pkg.NewBizError(domain.RespCodeInvalidParam, "缺少上传文件")
	}
	docTypeCode := docTypeOfFileName(fileDTO.FileName)
	if docTypeCode == "" {
		return domain.KnowledgeDocumentVO{}, pkg.NewBizError(domain.RespCodeInvalidParam, "仅支持 txt/md/docx/pdf 格式")
	}
	if s.config.UploadMaxMB > 0 && fileDTO.FileSize > int64(s.config.UploadMaxMB)<<20 {
		return domain.KnowledgeDocumentVO{}, pkg.NewBizError(domain.RespCodeInvalidParam, "文件超过上传大小上限")
	}
	docULID := pkg.NewULID(domain.IDPrefixDocument)
	if err := s.objectRepo.PutObject(ctx, docULID, fileDTO.Reader, fileDTO.FileSize, "application/octet-stream"); err != nil {
		return domain.KnowledgeDocumentVO{}, pkg.WrapBizError(domain.RespCodeStorageError, "文档原文上传失败", err)
	}
	do := &domain.KnowledgeDocumentDO{
		DocULID:       docULID,
		ProjectULID:   req.ProjectULID,
		DocName:       req.DocName,
		DocTypeCode:   docTypeCode,
		ObjectKey:     docULID,
		DocStatusCode: "pending",
	}
	if err := s.knowledgeRepo.CreateDocument(ctx, do); err != nil {
		if removeErr := s.objectRepo.RemoveObject(ctx, docULID); removeErr != nil {
			pkg.SLogger().Warnw("文档记录创建失败，MinIO 原文回滚失败", "doc_ulid", docULID, "error", removeErr)
		}
		return domain.KnowledgeDocumentVO{}, pkg.WrapBizError(domain.RespCodeStorageError, "文档记录创建失败", err)
	}
	if err := s.enqueueIndexJob(ctx, "doc_index", docULID); err != nil {
		_ = s.knowledgeRepo.UpdateDocument(ctx, docULID, map[string]any{"doc_status_code": "failed", "error_message": "索引任务投递失败"})
		return domain.KnowledgeDocumentVO{}, err
	}
	return toDocumentVO(do), nil
}

// List 分页查询项目文档。
func (s *KnowledgeService) List(ctx context.Context, userULID string, req *domain.KnowledgeListREQ) (domain.PageDTO[domain.KnowledgeDocumentVO], error) {
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID); err != nil {
		return domain.PageDTO[domain.KnowledgeDocumentVO]{}, err
	}
	req.Normalize()
	dos, total, err := s.knowledgeRepo.ListDocuments(ctx, req.ProjectULID, req.DocStatusCode, req.PageNum, req.PageSize)
	if err != nil {
		return domain.PageDTO[domain.KnowledgeDocumentVO]{}, err
	}
	items := make([]domain.KnowledgeDocumentVO, 0, len(dos))
	for i := range dos {
		items = append(items, toDocumentVO(&dos[i]))
	}
	return domain.NewPageDTO(total, req.PageREQ, items), nil
}

// Delete 删除文档：MinIO 原文 + 切片 + Manticore 行 + 文档行。
func (s *KnowledgeService) Delete(ctx context.Context, userULID string, req *domain.KnowledgeDeleteREQ) error {
	doc, err := s.mustOwnDoc(ctx, userULID, req.DocULID)
	if err != nil {
		return err
	}
	if err := s.objectRepo.RemoveObject(ctx, doc.ObjectKey); err != nil {
		return pkg.WrapBizError(domain.RespCodeStorageError, "文档原文删除失败", err)
	}
	if err := deleteManticoreChunks(ctx, s.manticore, s.config.ManticoreTable, doc.DocULID); err != nil {
		pkg.SLogger().Warnw("Manticore 切片清理失败，全文索引降级", "doc_ulid", doc.DocULID, "error", err)
	}
	return repo.WithTx(s.db, func(tx *gorm.DB) error {
		txKnowledge := repo.NewKnowledgeRepo(tx)
		if err := txKnowledge.DeleteChunksByDoc(ctx, doc.DocULID); err != nil {
			return err
		}
		return txKnowledge.DeleteDocument(ctx, doc.DocULID)
	})
}

// Reindex 重投索引任务（仅 indexed/failed 文档；切片先清后写幂等）。
func (s *KnowledgeService) Reindex(ctx context.Context, userULID string, req *domain.KnowledgeReindexREQ) error {
	doc, err := s.mustOwnDoc(ctx, userULID, req.DocULID)
	if err != nil {
		return err
	}
	if doc.DocStatusCode != "indexed" && doc.DocStatusCode != "failed" {
		return pkg.NewBizError(domain.RespCodeBizFailed, "仅 indexed/failed 状态的文档可重投索引")
	}
	if err := s.knowledgeRepo.UpdateDocument(ctx, doc.DocULID, map[string]any{"doc_status_code": "pending", "error_message": ""}); err != nil {
		return pkg.WrapBizError(domain.RespCodeStorageError, "文档状态更新失败", err)
	}
	return s.enqueueIndexJob(ctx, "doc_reindex", doc.DocULID)
}

// ChunkList 分页查询文档切片。
func (s *KnowledgeService) ChunkList(ctx context.Context, userULID string, req *domain.KnowledgeChunkListREQ) (domain.PageDTO[domain.KnowledgeChunkVO], error) {
	if _, err := s.mustOwnDoc(ctx, userULID, req.DocULID); err != nil {
		return domain.PageDTO[domain.KnowledgeChunkVO]{}, err
	}
	req.Normalize()
	chunks, total, err := s.knowledgeRepo.ListChunksByDoc(ctx, req.DocULID, req.PageNum, req.PageSize)
	if err != nil {
		return domain.PageDTO[domain.KnowledgeChunkVO]{}, err
	}
	items := make([]domain.KnowledgeChunkVO, 0, len(chunks))
	for i := range chunks {
		items = append(items, domain.KnowledgeChunkVO{
			ChunkULID:  chunks[i].ChunkULID,
			DocULID:    chunks[i].DocULID,
			ChunkIndex: chunks[i].ChunkIndex,
			ChunkText:  chunks[i].ChunkText,
		})
	}
	return domain.NewPageDTO(total, req.PageREQ, items), nil
}

// contentMaxBytes 原文读取上限（上传上限 20MB，留余量防异常对象）。
const contentMaxBytes = 25 << 20

// GetContent 读取文档原文（归属校验后从对象存储取回，供前端预览）。
func (s *KnowledgeService) GetContent(ctx context.Context, userULID string, req *domain.KnowledgeContentREQ) (domain.KnowledgeContentDTO, error) {
	doc, err := s.mustOwnDoc(ctx, userULID, req.DocULID)
	if err != nil {
		return domain.KnowledgeContentDTO{}, err
	}
	reader, err := s.objectRepo.GetObject(ctx, doc.ObjectKey)
	if err != nil {
		return domain.KnowledgeContentDTO{}, pkg.WrapBizError(domain.RespCodeStorageError, "读取原文失败", err)
	}
	defer reader.Close()
	contentBytes, err := io.ReadAll(io.LimitReader(reader, contentMaxBytes))
	if err != nil {
		return domain.KnowledgeContentDTO{}, pkg.WrapBizError(domain.RespCodeStorageError, "读取原文失败", err)
	}
	return domain.KnowledgeContentDTO{DocName: doc.DocName, DocTypeCode: doc.DocTypeCode, ContentBytes: contentBytes}, nil
}

// enqueueIndexJob 创建任务留痕（queued）并投递 RabbitMQ 索引队列。
func (s *KnowledgeService) enqueueIndexJob(ctx context.Context, jobTypeCode, docULID string) error {
	payload, err := json.Marshal(domain.KnowledgeIndexJobDTO{DocULID: docULID})
	if err != nil {
		return pkg.WrapBizError(domain.RespCodeInternalError, "任务载荷序列化失败", err)
	}
	job := &domain.JobDO{
		JobULID:       pkg.NewULID(domain.IDPrefixJob),
		JobTypeCode:   jobTypeCode,
		PayloadJSON:   string(payload),
		JobStatusCode: "queued",
	}
	if err := s.jobRepo.Create(ctx, job); err != nil {
		return pkg.WrapBizError(domain.RespCodeStorageError, "任务留痕失败", err)
	}
	if err := s.mqRepo.PublishIndexJob(ctx, domain.KnowledgeIndexJobDTO{DocULID: docULID}); err != nil {
		_ = s.jobRepo.Update(ctx, job.JobULID, map[string]any{"job_status_code": "failed", "error_message": "队列投递失败"})
		return pkg.WrapBizError(domain.RespCodeStorageError, "索引任务投递失败", err)
	}
	return nil
}

// mustOwnDoc 加载文档并校验项目归属（全局库文档放行）。
func (s *KnowledgeService) mustOwnDoc(ctx context.Context, userULID, docULID string) (*domain.KnowledgeDocumentDO, error) {
	doc, err := s.knowledgeRepo.GetDocument(ctx, docULID)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, pkg.NewBizError(domain.RespCodeNotFound, "文档不存在")
	}
	if doc.ProjectULID != "" && doc.ProjectULID != "0" {
		if _, err := mustOwnProject(ctx, s.projectRepo, userULID, doc.ProjectULID); err != nil {
			return nil, err
		}
	}
	return doc, nil
}

// docTypeOfFileName 从文件扩展名推导文档类型（txt/md/docx/pdf 白名单），不支持返回空。
func docTypeOfFileName(fileName string) string {
	docTypeCode := strings.ToLower(strings.TrimPrefix(filepath.Ext(fileName), "."))
	switch docTypeCode {
	case "txt", "md", "docx", "pdf":
		return docTypeCode
	}
	return ""
}

// toDocumentVO 组装文档视图对象。
func toDocumentVO(do *domain.KnowledgeDocumentDO) domain.KnowledgeDocumentVO {
	return domain.KnowledgeDocumentVO{
		DocULID:         do.DocULID,
		ProjectULID:     do.ProjectULID,
		DocName:         do.DocName,
		DocTypeCode:     do.DocTypeCode,
		DocStatusCode:   do.DocStatusCode,
		ChunkCount:      do.ChunkCount,
		TotalTokenCount: do.TotalTokenCount,
		ErrorMessage:    do.ErrorMessage,
		CreatedAt:       pkg.FormatTime(do.CreatedAt),
	}
}

// Stats 查询项目知识库概览（文档状态与切片总量）。
func (s *KnowledgeService) Stats(ctx context.Context, userULID, projectULID string) (domain.KnowledgeStatsVO, error) {
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, projectULID); err != nil {
		return domain.KnowledgeStatsVO{}, err
	}
	return s.knowledgeRepo.StatsByProject(ctx, projectULID)
}

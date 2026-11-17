package service

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"path/filepath"
	"strings"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/infra"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// searchModeVector 语义检索通道。
const searchModeVector = "vector"

// searchModeKeyword 全文检索通道。
const searchModeKeyword = "keyword"

// searchModeHybrid 混合检索通道（默认，RRF 融合）。
const searchModeHybrid = "hybrid"

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
	knowledgeRepo *repo.KnowledgeRepo
	objectRepo    *repo.ObjectRepo
	mqRepo        *repo.MQRepo
	jobRepo       *repo.JobRepo
	projectRepo   *repo.ProjectRepo
	aiClient      *ai.Client
	manticore     *infra.ManticoreClient
	db            *gorm.DB
	config        KnowledgeIndexConfig
}

// NewKnowledgeService 构造知识库服务。
func NewKnowledgeService(knowledgeRepo *repo.KnowledgeRepo, objectRepo *repo.ObjectRepo, mqRepo *repo.MQRepo, jobRepo *repo.JobRepo, projectRepo *repo.ProjectRepo, aiClient *ai.Client, manticore *infra.ManticoreClient, db *gorm.DB, config KnowledgeIndexConfig) *KnowledgeService {
	return &KnowledgeService{
		knowledgeRepo: knowledgeRepo,
		objectRepo:    objectRepo,
		mqRepo:        mqRepo,
		jobRepo:       jobRepo,
		projectRepo:   projectRepo,
		aiClient:      aiClient,
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
		return domain.KnowledgeDocumentVO{}, pkg.WrapBizError(domain.RespCodeStorageError, "文档记录创建失败", err)
	}
	if err := s.enqueueIndexJob(ctx, "doc_index", docULID); err != nil {
		// 投递失败文档留库可重投，状态置 failed 供前端提示
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
		slog.Warn("Manticore 切片清理失败，全文索引降级", "doc_ulid", doc.DocULID, "error", err)
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

// Search 检索知识库（vector/keyword/hybrid；hybrid 默认 RRF 融合，单通道失败自动降级）。
func (s *KnowledgeService) Search(ctx context.Context, userULID string, req *domain.KnowledgeSearchREQ) ([]domain.KnowledgeChunkVO, error) {
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID); err != nil {
		return nil, err
	}
	mode := req.SearchMode
	if mode == "" {
		mode = searchModeHybrid
	}
	topK := req.TopKCount
	if topK <= 0 {
		topK = s.config.RAGTopK
	}
	if err := validateSearchREQ(mode, req); err != nil {
		return nil, err
	}
	switch mode {
	case searchModeVector:
		hits, err := s.searchVectorChannel(ctx, req, topK)
		if err != nil {
			return nil, err
		}
		return fuseSearchResults(hits, nil, topK), nil
	case searchModeKeyword:
		hits, err := s.searchKeywordChannel(ctx, req, topK)
		if err != nil {
			return nil, err
		}
		return fuseSearchResults(nil, hits, topK), nil
	default:
		vectorHits, vectorErr := s.searchVectorChannel(ctx, req, topK)
		keywordHits, keywordErr := s.searchKeywordChannel(ctx, req, topK)
		if vectorErr != nil && keywordErr != nil {
			return nil, vectorErr
		}
		if vectorErr != nil {
			slog.Warn("向量通道失败，降级 keyword", "error", vectorErr)
		}
		if keywordErr != nil {
			slog.Warn("关键词通道失败，降级 vector", "error", keywordErr)
		}
		return fuseSearchResults(vectorHits, keywordHits, topK), nil
	}
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

// searchVectorChannel 向量通道：查询文本向量化后按 pgvector 余弦距离召回。
func (s *KnowledgeService) searchVectorChannel(ctx context.Context, req *domain.KnowledgeSearchREQ, topK int) ([]domain.KnowledgeChunkVO, error) {
	vectors, err := s.aiClient.Embed(ctx, []string{req.QueryText})
	if err != nil {
		return nil, pkg.WrapBizError(domain.RespCodeAIProviderError, "查询向量化失败", err)
	}
	if len(vectors) == 0 {
		return nil, pkg.NewBizError(domain.RespCodeAIProviderError, "查询向量化失败")
	}
	hits, err := s.knowledgeRepo.SearchByVector(ctx, req.ProjectULID, vectors[0], topK, req.DocULIDList)
	if err != nil {
		return nil, pkg.WrapBizError(domain.RespCodeStorageError, "向量检索失败", err)
	}
	if hits == nil {
		hits = []domain.KnowledgeChunkVO{}
	}
	return hits, nil
}

// searchKeywordChannel 关键词通道：Manticore 全文命中后回表取切片原文。
func (s *KnowledgeService) searchKeywordChannel(ctx context.Context, req *domain.KnowledgeSearchREQ, topK int) ([]domain.KnowledgeChunkVO, error) {
	chunkULIDs, err := queryManticoreChunkIDs(ctx, s.manticore, s.config.ManticoreTable, req, topK)
	if err != nil {
		return nil, pkg.WrapBizError(domain.RespCodeStorageError, "全文检索失败", err)
	}
	if len(chunkULIDs) == 0 {
		return []domain.KnowledgeChunkVO{}, nil
	}
	hits, err := s.knowledgeRepo.GetChunksByULIDs(ctx, chunkULIDs)
	if err != nil {
		return nil, pkg.WrapBizError(domain.RespCodeStorageError, "切片回表失败", err)
	}
	byULID := make(map[string]domain.KnowledgeChunkVO, len(hits))
	for i := range hits {
		byULID[hits[i].ChunkULID] = hits[i]
	}
	ordered := make([]domain.KnowledgeChunkVO, 0, len(chunkULIDs))
	for _, chunkULID := range chunkULIDs {
		if hit, ok := byULID[chunkULID]; ok {
			ordered = append(ordered, hit)
		}
	}
	return ordered, nil
}

// validateSearchREQ 校验检索模式与入参的必填关系。
func validateSearchREQ(mode string, req *domain.KnowledgeSearchREQ) error {
	if mode == searchModeKeyword && strings.TrimSpace(req.KeywordText) == "" {
		return pkg.NewBizError(domain.RespCodeInvalidParam, "keyword 模式必须提供 keyword_text")
	}
	if (mode == searchModeVector || mode == searchModeHybrid) && strings.TrimSpace(req.QueryText) == "" {
		return pkg.NewBizError(domain.RespCodeInvalidParam, mode+" 模式必须提供 query_text")
	}
	return nil
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

// fuseSearchResults 以 RRF 融合双通道命中：score=Σ 1/(60+rank)，得分降序取 topK，
// 同分保持稳定顺序（向量通道命中在前），source_mode 标注命中通道。
func fuseSearchResults(vectorHits, keywordHits []domain.KnowledgeChunkVO, topK int) []domain.KnowledgeChunkVO {
	order := make([]string, 0, len(vectorHits)+len(keywordHits))
	fused := make(map[string]*domain.KnowledgeChunkVO, len(vectorHits)+len(keywordHits))
	add := func(hit domain.KnowledgeChunkVO, rank int, sourceMode string) {
		item, ok := fused[hit.ChunkULID]
		if !ok {
			hit.ScoreValue = 0
			hit.SourceMode = sourceMode
			fused[hit.ChunkULID] = &hit
			order = append(order, hit.ChunkULID)
			item = fused[hit.ChunkULID]
		} else if item.SourceMode != sourceMode {
			item.SourceMode = searchModeHybrid
		}
		item.ScoreValue += 1.0 / (rrfRankConstant + float64(rank))
	}
	for i := range vectorHits {
		add(vectorHits[i], i+1, searchModeVector)
	}
	for i := range keywordHits {
		add(keywordHits[i], i+1, searchModeKeyword)
	}
	results := make([]domain.KnowledgeChunkVO, 0, len(order))
	for _, chunkULID := range order {
		results = append(results, *fused[chunkULID])
	}
	sortByRRFScore(results)
	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}
	return results
}

// sortByRRFScore 按融合得分降序排序（稳定排序，同分保持入选顺序）。
func sortByRRFScore(results []domain.KnowledgeChunkVO) {
	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].ScoreValue > results[j-1].ScoreValue; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}
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
		CreatedAtMs:     do.CreatedAt.UnixMilli(),
	}
}

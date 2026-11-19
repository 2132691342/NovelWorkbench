package service

import (
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"

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
	knowledgeRepo *repo.KnowledgeRepo
	objectRepo    *repo.ObjectRepo
	mqRepo        *repo.MQRepo
	jobRepo       *repo.JobRepo
	projectRepo   *repo.ProjectRepo
	aiClient      *ai.Client
	providerSvc   *ModelProviderService
	manticore     *infra.ManticoreClient
	db            *gorm.DB
	config        KnowledgeIndexConfig
}

// NewKnowledgeService 构造知识库服务。
func NewKnowledgeService(knowledgeRepo *repo.KnowledgeRepo, objectRepo *repo.ObjectRepo, mqRepo *repo.MQRepo, jobRepo *repo.JobRepo, projectRepo *repo.ProjectRepo, aiClient *ai.Client, providerSvc *ModelProviderService, manticore *infra.ManticoreClient, db *gorm.DB, config KnowledgeIndexConfig) *KnowledgeService {
	return &KnowledgeService{
		knowledgeRepo: knowledgeRepo,
		objectRepo:    objectRepo,
		mqRepo:        mqRepo,
		jobRepo:       jobRepo,
		projectRepo:   projectRepo,
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

// Search 检索知识库（keyword/hybrid），供 RAG 与前端共用。
func (s *KnowledgeService) Search(ctx context.Context, userULID string, req *domain.KnowledgeSearchREQ) ([]domain.KnowledgeChunkVO, error) {
	dto, err := s.SearchWithTrace(ctx, userULID, req)
	if err != nil {
		return nil, err
	}
	return dto.ResultList, nil
}

// SearchWithTrace 检索并返回可解释的检索过程（query 构造、候选规模、入选 ID 与通道）。
func (s *KnowledgeService) SearchWithTrace(ctx context.Context, userULID string, req *domain.KnowledgeSearchREQ) (domain.KnowledgeSearchDTO, error) {
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID); err != nil {
		return domain.KnowledgeSearchDTO{}, err
	}
	mode := normalizeSearchMode(req.SearchMode)
	query := strings.TrimSpace(req.KeywordText)
	if query == "" {
		return domain.KnowledgeSearchDTO{}, pkg.NewBizError(domain.RespCodeInvalidParam, "keyword_text 不能为空")
	}
	topK := req.TopKCount
	if topK <= 0 {
		topK = s.config.RAGTopK
	}
	terms := extractQueryTerms(query)
	if len(terms) == 0 {
		terms = []string{query}
	}
	trace := domain.KnowledgeSearchTrace{
		QueryTerms:  terms,
		SourceMode:  mode,
		SelectedIDs: []string{},
	}
	candidateReq := &domain.KnowledgeSearchREQ{
		ProjectULID: req.ProjectULID,
		KeywordText: query,
		DocULIDList: req.DocULIDList,
	}
	termHits, termErr := s.searchKeywordChannel(ctx, candidateReq, topK, "term")
	if mode == domain.SearchModeKeyword {
		if termErr != nil {
			return domain.KnowledgeSearchDTO{}, termErr
		}
		results := s.maybeRerank(ctx, terms[0], termHits, topK, &trace)
		trace.QueryExpr = strings.Join(terms, " | ")
		trace.CandidateCount = len(termHits)
		trace.SelectedIDs = selectedIDs(results)
		return domain.KnowledgeSearchDTO{ResultList: results, Trace: trace}, nil
	}
	phraseHits, phraseErr := s.searchKeywordChannel(ctx, candidateReq, topK, "phrase")
	if termErr != nil && phraseErr != nil {
		return domain.KnowledgeSearchDTO{}, termErr
	}
	if termErr != nil {
		pkg.SLogger().Warnw("词条通道失败，降级短语通道", "error", termErr)
	}
	if phraseErr != nil {
		pkg.SLogger().Warnw("短语通道失败，降级词条通道", "error", phraseErr)
	}
	trace.QueryExpr = strings.Join(terms, " | ")
	trace.CandidateCount = len(termHits) + len(phraseHits)
	hits := fuseSearchResults(termHits, phraseHits, topK+8)
	results := s.maybeRerank(ctx, terms[0], hits, topK, &trace)
	trace.SelectedIDs = selectedIDs(results)
	return domain.KnowledgeSearchDTO{ResultList: results, Trace: trace}, nil
}

// maybeRerank 用默认对话模型对候选 ID 重排（失败时 fail-open 保留原顺序）。
func (s *KnowledgeService) maybeRerank(ctx context.Context, query string, hits []domain.KnowledgeChunkVO, topK int, trace *domain.KnowledgeSearchTrace) []domain.KnowledgeChunkVO {
	if len(hits) == 0 || s.aiClient == nil || s.providerSvc == nil {
		trace.RerankNote = "未配置重排模型，按检索得分排序"
		return trimHits(hits, topK)
	}
	spec, err := s.providerSvc.ChatSpec(ctx)
	if err != nil {
		trace.RerankNote = "重排跳过：" + err.Error()
		return trimHits(hits, topK)
	}
	candidates := make([]ai.RerankCandidate, 0, len(hits))
	for _, hit := range hits {
		candidates = append(candidates, ai.RerankCandidate{ID: hit.ChunkULID, Text: truncateRunes(hit.ChunkText, 160)})
	}
	rerankCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	ids, note := s.aiClient.Rerank(rerankCtx, spec, query, candidates)
	trace.Reranked = note == "reranked by chat model"
	trace.RerankNote = note
	byID := make(map[string]domain.KnowledgeChunkVO, len(hits))
	for _, hit := range hits {
		byID[hit.ChunkULID] = hit
	}
	ordered := make([]domain.KnowledgeChunkVO, 0, len(hits))
	for _, id := range ids {
		if hit, ok := byID[id]; ok {
			hit.SourceMode = domain.SearchModeHybrid
			ordered = append(ordered, hit)
		}
	}
	return trimHits(ordered, topK)
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

// searchKeywordChannel 单个 Manticore 词条/短语通道：返回带 BM25 相关度的切片。
func (s *KnowledgeService) searchKeywordChannel(ctx context.Context, req *domain.KnowledgeSearchREQ, topK int, channel string) ([]domain.KnowledgeChunkVO, error) {
	chunkULIDs, scoreMap, err := queryManticoreHits(ctx, s.manticore, s.config.ManticoreTable, req, topK, channel)
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
			hit.ScoreValue = scoreMap[chunkULID]
			hit.SourceMode = channel
			ordered = append(ordered, hit)
		}
	}
	return ordered, nil
}

// normalizeSearchMode 归一化检索模式：仅 hybrid 走双通道，其他模式按 keyword 处理。
func normalizeSearchMode(mode string) string {
	if mode == domain.SearchModeHybrid {
		return domain.SearchModeHybrid
	}
	return domain.SearchModeKeyword
}

// extractQueryTerms 从查询文本抽取检索词：CJK 连续串按 2-gram 切分，拉丁词按字母数字切分并去重。
func extractQueryTerms(query string) []string {
	seen := make(map[string]bool)
	terms := make([]string, 0, 8)
	add := func(term string) {
		term = strings.ToLower(strings.TrimSpace(term))
		if term == "" || seen[term] {
			return
		}
		seen[term] = true
		terms = append(terms, term)
	}
	var latin strings.Builder
	var cjk strings.Builder
	flushLatin := func() {
		if latin.Len() > 0 {
			add(latin.String())
			latin.Reset()
		}
	}
	flushCJK := func() {
		runes := []rune(cjk.String())
		for i := 0; i+1 < len(runes); i++ {
			add(string(runes[i : i+2]))
		}
		cjk.Reset()
	}
	for _, r := range query {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			flushCJK()
			latin.WriteRune(r)
		case r >= 0x4e00 && r <= 0x9fff:
			flushLatin()
			cjk.WriteRune(r)
		default:
			flushLatin()
			flushCJK()
		}
	}
	flushLatin()
	flushCJK()
	return terms
}

// fuseSearchResults 以 RRF 融合两个 lexical 通道命中：score=Σ 1/(60+rank)，得分降序取 topK。
func fuseSearchResults(termHits, phraseHits []domain.KnowledgeChunkVO, topK int) []domain.KnowledgeChunkVO {
	order := make([]string, 0, len(termHits)+len(phraseHits))
	fused := make(map[string]*domain.KnowledgeChunkVO, len(termHits)+len(phraseHits))
	add := func(hit domain.KnowledgeChunkVO, rank int, channel string) {
		item, ok := fused[hit.ChunkULID]
		if !ok {
			hit.ScoreValue = 0
			hit.SourceMode = channel
			fused[hit.ChunkULID] = &hit
			order = append(order, hit.ChunkULID)
			item = fused[hit.ChunkULID]
		} else if item.SourceMode != channel {
			item.SourceMode = domain.SearchModeHybrid
		}
		item.ScoreValue += 1.0 / (rrfRankConstant + float64(rank))
	}
	for i := range termHits {
		add(termHits[i], i+1, "term")
	}
	for i := range phraseHits {
		add(phraseHits[i], i+1, "phrase")
	}
	results := make([]domain.KnowledgeChunkVO, 0, len(order))
	for _, chunkULID := range order {
		results = append(results, *fused[chunkULID])
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].ScoreValue > results[j].ScoreValue })
	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}
	return results
}

// trimHits 截断命中列表并保证非 nil（前端直接遍历）。
func trimHits(hits []domain.KnowledgeChunkVO, topK int) []domain.KnowledgeChunkVO {
	if topK > 0 && len(hits) > topK {
		hits = hits[:topK]
	}
	if hits == nil {
		return []domain.KnowledgeChunkVO{}
	}
	return hits
}

// selectedIDs 抽取最终入选切片 ID（有序）。
func selectedIDs(hits []domain.KnowledgeChunkVO) []string {
	ids := make([]string, 0, len(hits))
	for _, hit := range hits {
		ids = append(ids, hit.ChunkULID)
	}
	return ids
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
		CreatedAtMs:     do.CreatedAt.UnixMilli(),
	}
}

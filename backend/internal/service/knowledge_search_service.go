// 本文件实现知识库检索：双词法通道召回、RRF 融合、模型重排与检索过程追踪。

package service

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
)

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
		topK = s.resolveRAGTopK(ctx)
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

// resolveRAGTopK 默认召回条数：应用配置 rag_top_k > 全局配置 app.rag_top_k。
func (s *KnowledgeService) resolveRAGTopK(ctx context.Context) int {
	if s.settingRepo != nil {
		setting, err := s.settingRepo.GetByKey(ctx, settingKeyRAGTopK)
		if err != nil {
			pkg.SLogger().Warnw("读取召回条数配置失败，回退全局配置", "error", err)
		} else if setting != nil {
			if value, convErr := strconv.Atoi(setting.SettingValue); convErr == nil && value > 0 {
				return value
			}
		}
	}
	return s.config.RAGTopK
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

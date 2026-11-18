package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"

	"regexp"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"
	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/infra"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// maxIndexRetryCount 单个索引任务的最大重试次数（首次失败后重试 ≤3 次）。
const maxIndexRetryCount = 3

// consumeReconnectInterval 索引队列消费中断后的重连间隔。
const consumeReconnectInterval = 5 * time.Second

// KnowledgeWorker 知识库索引 worker：消费队列完成提取/切片/向量化/入库与 Manticore 同步。
type KnowledgeWorker struct {
	knowledgeRepo *repo.KnowledgeRepo
	objectRepo    *repo.ObjectRepo
	mqRepo        *repo.MQRepo
	jobRepo       *repo.JobRepo
	aiClient      *ai.Client
	providerSvc   *ModelProviderService
	manticore     *infra.ManticoreClient
	db            *gorm.DB
	config        KnowledgeIndexConfig
}

// NewKnowledgeWorker 构造知识库索引 worker。
func NewKnowledgeWorker(knowledgeRepo *repo.KnowledgeRepo, objectRepo *repo.ObjectRepo, mqRepo *repo.MQRepo, jobRepo *repo.JobRepo, aiClient *ai.Client, providerSvc *ModelProviderService, manticore *infra.ManticoreClient, db *gorm.DB, config KnowledgeIndexConfig) *KnowledgeWorker {
	return &KnowledgeWorker{
		knowledgeRepo: knowledgeRepo,
		objectRepo:    objectRepo,
		mqRepo:        mqRepo,
		jobRepo:       jobRepo,
		aiClient:      aiClient,
		providerSvc:   providerSvc,
		manticore:     manticore,
		db:            db,
		config:        config,
	}
}

// Run 阻塞消费索引队列直至 ctx 取消（优雅退出）；消费连接中断按固定间隔重连。
func (w *KnowledgeWorker) Run(ctx context.Context) error {
	rebuilt, err := ensureManticoreSchema(ctx, w.manticore, w.config.ManticoreTable)
	if err != nil {
		pkg.SLogger().Warnw("Manticore 切片表初始化失败，keyword 通道降级", "error", err)
	} else if rebuilt {
		if err := w.rebuildManticoreIndex(ctx); err != nil {
			pkg.SLogger().Warnw("Manticore 全文索引重建失败，keyword 通道降级", "error", err)
		}
	}
	for {
		err := w.mqRepo.ConsumeIndexJobs(ctx, w.handleIndexJob)
		if err == nil || ctx.Err() != nil {
			return nil
		}
		pkg.SLogger().Warnw("索引队列消费中断，稍后重连", "error", err)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(consumeReconnectInterval):
		}
	}
}

// handleIndexJob 处理单条索引任务：内部重试 ≤3 次，仍失败置 failed 并 ack（毒消息不重投）。
func (w *KnowledgeWorker) handleIndexJob(ctx context.Context, job domain.KnowledgeIndexJobDTO) error {
	if job.DocULID == "" {
		return fmt.Errorf("index job payload missing doc_ulid")
	}
	jobDO, err := w.jobRepo.GetPendingByDoc(ctx, job.DocULID)
	if err != nil {
		pkg.SLogger().Warnw("任务留痕查询失败", "doc_ulid", job.DocULID, "error", err)
	}
	markJob := func(status, errorMessage string, retryCount int) {
		if jobDO == nil {
			return
		}
		fields := map[string]any{"job_status_code": status, "error_message": errorMessage, "retry_count": retryCount}
		if err := w.jobRepo.Update(ctx, jobDO.JobULID, fields); err != nil {
			pkg.SLogger().Warnw("任务状态更新失败", "job_ulid", jobDO.JobULID, "error", err)
		}
	}
	doc, err := w.knowledgeRepo.GetDocument(ctx, job.DocULID)
	if err != nil {
		return err // 查询异常 requeue 重投
	}
	if doc == nil {
		markJob("failed", "文档不存在", 0)
		return nil // 文档已删除的滞留消息直接 ack
	}
	if err := w.knowledgeRepo.UpdateDocument(ctx, doc.DocULID, map[string]any{"doc_status_code": "indexing", "error_message": ""}); err != nil {
		pkg.SLogger().Warnw("文档状态更新失败", "doc_ulid", doc.DocULID, "error", err)
	}
	var lastErr error
	for attempt := 0; attempt <= maxIndexRetryCount; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err() // 关闭中断，消息 requeue 待重启后重投
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}
		lastErr = w.indexDocument(ctx, doc)
		if lastErr == nil {
			markJob("success", "", attempt)
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		pkg.SLogger().Warnw("文档索引失败，将重试", "doc_ulid", doc.DocULID, "attempt", attempt+1, "error", lastErr)
	}
	errorMessage := truncateErrorMessage(lastErr)
	markJob("failed", errorMessage, maxIndexRetryCount)
	_ = w.knowledgeRepo.UpdateDocument(ctx, doc.DocULID, map[string]any{"doc_status_code": "failed", "error_message": errorMessage})
	return nil
}

// indexDocument 单次索引：提取文本 → 切片 → 向量化 → 事务写切片 → Manticore 同步 → 置 indexed。
func (w *KnowledgeWorker) indexDocument(ctx context.Context, doc *domain.KnowledgeDocumentDO) error {
	reader, err := w.objectRepo.GetObject(ctx, doc.ObjectKey)
	if err != nil {
		return pkg.WrapBizError(domain.RespCodeStorageError, "原文读取失败", err)
	}
	defer reader.Close()
	text, err := extractDocumentText(doc.DocTypeCode, reader)
	if err != nil {
		return pkg.WrapBizError(domain.RespCodeStorageError, "原文提取失败", err)
	}
	pieces := chunkText(text, w.config.ChunkSize, w.config.ChunkOverlap)
	if len(pieces) == 0 {
		return fmt.Errorf("文档内容为空，无法建立索引")
	}
	// 向量模型缺失或调用失败时只建关键词索引：文档仍可被全文检索，语义通道待配置后重投即可
	embeddingSpec, embeddingAvailable, specErr := w.providerSvc.EmbeddingSpec(ctx)
	var vectors [][]float32
	if specErr != nil {
		pkg.SLogger().Warnw("向量供应商解析失败，本次仅建关键词索引", "doc_ulid", doc.DocULID, "error", specErr)
	} else if !embeddingAvailable {
		pkg.SLogger().Warnw("未配置向量模型，本次仅建关键词索引", "doc_ulid", doc.DocULID)
	} else {
		embedded, embedErr := w.aiClient.Embed(ctx, embeddingSpec, pieces)
		if embedErr != nil {
			pkg.SLogger().Warnw("切片向量化失败，本次仅建关键词索引", "doc_ulid", doc.DocULID, "error", embedErr)
		} else if len(embedded) != len(pieces) {
			pkg.SLogger().Warnw("向量数量与切片数量不一致，本次仅建关键词索引",
				"doc_ulid", doc.DocULID, "vector_count", len(embedded), "chunk_count", len(pieces))
		} else {
			vectors = embedded
		}
	}
	chunks := make([]domain.KnowledgeChunkDO, 0, len(pieces))
	var totalTokenCount int64
	for i, piece := range pieces {
		tokenCount := int(pkg.CountWords(piece))
		totalTokenCount += int64(tokenCount)
		chunkDO := domain.KnowledgeChunkDO{
			ChunkULID:   pkg.NewULID(domain.IDPrefixChunk),
			DocULID:     doc.DocULID,
			ProjectULID: doc.ProjectULID,
			ChunkIndex:  i,
			ChunkText:   piece,
			TokenCount:  tokenCount,
		}
		if vectors != nil {
			chunkDO.EmbeddingVec = vectors[i]
		}
		chunks = append(chunks, chunkDO)
	}
	if err := repo.WithTx(w.db, func(tx *gorm.DB) error {
		txKnowledge := repo.NewKnowledgeRepo(tx)
		if err := txKnowledge.DeleteChunksByDoc(ctx, doc.DocULID); err != nil {
			return err
		}
		for i := range chunks {
			if vectors == nil {
				if err := txKnowledge.InsertChunkTextOnly(ctx, &chunks[i]); err != nil {
					return err
				}
				continue
			}
			if err := txKnowledge.InsertChunkWithVector(ctx, &chunks[i]); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return pkg.WrapBizError(domain.RespCodeStorageError, "切片入库失败", err)
	}
	if err := syncManticoreChunks(ctx, w.manticore, w.config.ManticoreTable, doc.DocULID, chunks); err != nil {
		pkg.SLogger().Warnw("Manticore 同步失败，keyword 通道降级", "doc_ulid", doc.DocULID, "error", err)
	}
	if err := w.knowledgeRepo.UpdateDocument(ctx, doc.DocULID, map[string]any{
		"doc_status_code":   "indexed",
		"chunk_count":       len(chunks),
		"total_token_count": totalTokenCount,
		"error_message":     "",
	}); err != nil {
		return pkg.WrapBizError(domain.RespCodeStorageError, "文档状态更新失败", err)
	}
	return nil
}

// extractDocumentText 按文档类型提取纯文本（txt/md 直读，docx/pdf 纯 Go 解析）。
func extractDocumentText(docTypeCode string, reader io.Reader) (string, error) {
	raw, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("read document source: %w", err)
	}
	switch docTypeCode {
	case "txt", "md":
		return strings.TrimPrefix(string(raw), string(rune(0xFEFF))), nil // 剥除 UTF-8 BOM
	case "docx":
		return extractDocxText(raw)
	case "pdf":
		return extractPDFText(raw)
	default:
		return "", fmt.Errorf("unsupported doc type %q", docTypeCode)
	}
}

var (
	// docxParagraphPattern 将 docx 段落结束标记转为换行。
	docxParagraphPattern = regexp.MustCompile(`</w:p>`)
	// xmlTagPattern 匹配 XML 标签（去标签取纯文本）。
	xmlTagPattern = regexp.MustCompile(`<[^>]*>`)
)

// extractDocxText 从 docx（zip 包）提取纯文本：word/document.xml 段落转行、去标签并还原 XML 实体。
func extractDocxText(raw []byte) (string, error) {
	zipReader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return "", fmt.Errorf("open docx archive: %w", err)
	}
	for _, file := range zipReader.File {
		if file.Name != "word/document.xml" {
			continue
		}
		entry, err := file.Open()
		if err != nil {
			return "", fmt.Errorf("open docx document.xml: %w", err)
		}
		xmlBytes, err := io.ReadAll(entry)
		entry.Close()
		if err != nil {
			return "", fmt.Errorf("read docx document.xml: %w", err)
		}
		text := docxParagraphPattern.ReplaceAllString(string(xmlBytes), "\n")
		text = xmlTagPattern.ReplaceAllString(text, "")
		return html.UnescapeString(text), nil
	}
	return "", fmt.Errorf("docx archive missing word/document.xml")
}

// extractPDFText 用纯 Go（ledongthuc/pdf）解析 PDF 提取纯文本。
func extractPDFText(raw []byte) (string, error) {
	pdfReader, err := pdf.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return "", fmt.Errorf("open pdf: %w", err)
	}
	plain, err := pdfReader.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("extract pdf text: %w", err)
	}
	var buffer bytes.Buffer
	if _, err := buffer.ReadFrom(plain); err != nil {
		return "", fmt.Errorf("read pdf text: %w", err)
	}
	return buffer.String(), nil
}

// chunkText 滑窗切片：窗口 chunkSize 字符、重叠 chunkSize-overlap 步进，
// 窗口内优先在段落/句末边界裁剪（边界不早于窗口中点），短文本返回单切片。
func chunkText(text string, chunkSize, overlap int) []string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}
	if chunkSize <= 0 {
		return []string{trimmed}
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= chunkSize {
		overlap = chunkSize - 1
	}
	runes := []rune(trimmed)
	var chunks []string
	start := 0
	for start < len(runes) {
		end := start + chunkSize
		if end >= len(runes) {
			end = len(runes)
		} else if cut := lastBoundary(runes[start:end]); cut > chunkSize/2 {
			end = start + cut
		}
		if piece := strings.TrimSpace(string(runes[start:end])); piece != "" {
			chunks = append(chunks, piece)
		}
		if end >= len(runes) {
			break
		}
		if next := end - overlap; next > start {
			start = next
		} else {
			start = end // 重叠过大时保证前进
		}
	}
	return chunks
}

// lastBoundary 返回窗口内最后一个段落/句末边界之后的位置（无边界返回 0）。
func lastBoundary(window []rune) int {
	for i := len(window) - 1; i >= 0; i-- {
		switch window[i] {
		case '\n', '。', '！', '？', '；', '!', '?', ';':
			return i + 1
		}
	}
	return 0
}

// ensureManticoreSchema 确保全文表存在且使用 CJK ngram 分词；旧结构重建并返回 true（需回灌索引）。
func ensureManticoreSchema(ctx context.Context, client *infra.ManticoreClient, table string) (bool, error) {
	tableName := manticoreTableName(table)
	ddl := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s(chunk_ulid string, doc_ulid string, project_ulid string, chunk_text text) charset_table='non_cjk,0..9,A..Z->a..z,a..z' ngram_len='1' ngram_chars='cjk'", tableName)
	if _, err := client.Exec(ctx, ddl); err != nil {
		return false, err
	}
	body, err := client.Exec(ctx, "SHOW CREATE TABLE "+tableName)
	if err != nil {
		return false, err
	}
	if strings.Contains(string(body), "ngram_len") {
		return false, nil
	}
	if _, err := client.Exec(ctx, "DROP TABLE "+tableName); err != nil {
		return false, err
	}
	if _, err := client.Exec(ctx, ddl); err != nil {
		return false, err
	}
	return true, nil
}

// rebuildManticoreIndex 从 PG 切片回灌全文索引（表结构升级后一次性执行）。
func (w *KnowledgeWorker) rebuildManticoreIndex(ctx context.Context) error {
	chunks, err := w.knowledgeRepo.ListAllChunksForIndex(ctx)
	if err != nil {
		return err
	}
	byDoc := make(map[string][]domain.KnowledgeChunkDO, 16)
	docOrder := make([]string, 0, 16)
	for i := range chunks {
		docULID := chunks[i].DocULID
		if _, ok := byDoc[docULID]; !ok {
			docOrder = append(docOrder, docULID)
		}
		byDoc[docULID] = append(byDoc[docULID], chunks[i])
	}
	for _, docULID := range docOrder {
		if err := syncManticoreChunks(ctx, w.manticore, w.config.ManticoreTable, docULID, byDoc[docULID]); err != nil {
			return err
		}
	}
	return nil
}

// syncManticoreChunks 先删后插全量同步某文档的切片全文行。
func syncManticoreChunks(ctx context.Context, client *infra.ManticoreClient, table, docULID string, chunks []domain.KnowledgeChunkDO) error {
	if err := deleteManticoreChunks(ctx, client, table, docULID); err != nil {
		return err
	}
	tableName := manticoreTableName(table)
	for i := range chunks {
		sql := fmt.Sprintf("INSERT INTO %s(chunk_ulid, doc_ulid, project_ulid, chunk_text) VALUES ('%s', '%s', '%s', '%s')",
			tableName,
			escapeManticoreString(chunks[i].ChunkULID),
			escapeManticoreString(chunks[i].DocULID),
			escapeManticoreString(chunks[i].ProjectULID),
			escapeManticoreString(chunks[i].ChunkText),
		)
		if _, err := client.Exec(ctx, sql); err != nil {
			return err
		}
	}
	return nil
}

// deleteManticoreChunks 删除某文档的全部切片全文行。
func deleteManticoreChunks(ctx context.Context, client *infra.ManticoreClient, table, docULID string) error {
	sql := fmt.Sprintf("DELETE FROM %s WHERE doc_ulid = '%s'", manticoreTableName(table), escapeManticoreString(docULID))
	_, err := client.Exec(ctx, sql)
	return err
}

// queryManticoreChunkIDs 关键词通道：Manticore MATCH 命中切片 ULID（保持相关度返回顺序）。
func queryManticoreChunkIDs(ctx context.Context, client *infra.ManticoreClient, table string, req *domain.KnowledgeSearchREQ, topK int) ([]string, error) {
	sql := fmt.Sprintf("SELECT chunk_ulid FROM %s WHERE MATCH('@chunk_text \"%s\"') AND project_ulid = '%s'",
		manticoreTableName(table), escapeManticoreString(req.KeywordText), escapeManticoreString(req.ProjectULID))
	if len(req.DocULIDList) > 0 {
		docValues := make([]string, len(req.DocULIDList))
		for i, docULID := range req.DocULIDList {
			docValues[i] = "'" + escapeManticoreString(docULID) + "'"
		}
		sql += " AND doc_ulid IN (" + strings.Join(docValues, ",") + ")"
	}
	sql += fmt.Sprintf(" LIMIT %d", topK)
	body, err := client.Exec(ctx, sql)
	if err != nil {
		return nil, err
	}
	return parseManticoreChunkIDs(body)
}

// parseManticoreChunkIDs 解析 Manticore raw SQL 响应中的 chunk_ulid 列（保持返回顺序）。
func parseManticoreChunkIDs(body []byte) ([]string, error) {
	var resultSets []struct {
		Data  []map[string]any `json:"data"`
		Error string           `json:"error"`
	}
	if err := json.Unmarshal(body, &resultSets); err != nil {
		return nil, fmt.Errorf("parse manticore response: %w", err)
	}
	chunkULIDs := make([]string, 0, 8)
	for _, resultSet := range resultSets {
		if resultSet.Error != "" {
			return nil, fmt.Errorf("manticore query error: %s", resultSet.Error)
		}
		for _, row := range resultSet.Data {
			if chunkULID, ok := row["chunk_ulid"].(string); ok {
				chunkULIDs = append(chunkULIDs, chunkULID)
			}
		}
	}
	return chunkULIDs, nil
}

// manticoreTableName 返回全文表名（空值回退 nw_chunks）。
func manticoreTableName(table string) string {
	if strings.TrimSpace(table) == "" {
		return "nw_chunks"
	}
	return table
}

// manticoreEscaper Manticore SQL 字符串/匹配串转义器。
var manticoreEscaper = strings.NewReplacer(`\`, `\\`, `'`, `\'`, `"`, `\"`)

// escapeManticoreString 转义 Manticore SQL 字符串/匹配串中的反斜杠与引号。
func escapeManticoreString(value string) string {
	return manticoreEscaper.Replace(value)
}

// truncateErrorMessage 截断错误信息以适配 error_message 列宽（512）。
func truncateErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	runes := []rune(err.Error())
	if len(runes) > 500 {
		return string(runes[:500])
	}
	return string(runes)
}

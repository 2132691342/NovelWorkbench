// 本文件实现知识库 Manticore 全文索引的表结构迁移、回灌、同步与查询辅助。

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/infra"
)

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

// queryManticoreHits 执行 Manticore MATCH，返回按相关度排序的 chunk_ulid 与得分。
func queryManticoreHits(ctx context.Context, client *infra.ManticoreClient, table string, req *domain.KnowledgeSearchREQ, topK int, channel string) ([]string, map[string]float64, error) {
	matchExpr := `"` + escapeManticoreString(req.KeywordText) + `"`
	if channel == "term" {
		terms := extractQueryTerms(req.KeywordText)
		if len(terms) == 0 {
			terms = []string{req.KeywordText}
		}
		quoted := make([]string, len(terms))
		for i, term := range terms {
			quoted[i] = `"` + escapeManticoreString(term) + `"`
		}
		matchExpr = strings.Join(quoted, " | ")
	}
	sql := fmt.Sprintf("SELECT chunk_ulid, WEIGHT() AS score FROM %s WHERE MATCH('@chunk_text %s') AND project_ulid = '%s'",
		manticoreTableName(table), matchExpr, escapeManticoreString(req.ProjectULID))
	if len(req.DocULIDList) > 0 {
		docValues := make([]string, len(req.DocULIDList))
		for i, docULID := range req.DocULIDList {
			docValues[i] = "'" + escapeManticoreString(docULID) + "'"
		}
		sql += " AND doc_ulid IN (" + strings.Join(docValues, ",") + ")"
	}
	sql += fmt.Sprintf(" ORDER BY score DESC LIMIT %d", topK)
	body, err := client.Exec(ctx, sql)
	if err != nil {
		return nil, nil, err
	}
	return parseManticoreHits(body)
}

// parseManticoreHits 解析 Manticore raw SQL 响应中的 chunk_ulid 与 score 列（保持返回顺序）。
func parseManticoreHits(body []byte) ([]string, map[string]float64, error) {
	var resultSets []struct {
		Data  []map[string]any `json:"data"`
		Error string           `json:"error"`
	}
	if err := json.Unmarshal(body, &resultSets); err != nil {
		return nil, nil, fmt.Errorf("parse manticore response: %w", err)
	}
	ids := make([]string, 0, 8)
	scores := make(map[string]float64, 8)
	for _, resultSet := range resultSets {
		if resultSet.Error != "" {
			return nil, nil, fmt.Errorf("manticore query error: %s", resultSet.Error)
		}
		for _, row := range resultSet.Data {
			chunkULID, ok := row["chunk_ulid"].(string)
			if !ok {
				continue
			}
			ids = append(ids, chunkULID)
			scores[chunkULID] = scoreOf(row["score"])
		}
	}
	return ids, scores, nil
}

func scoreOf(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case json.Number:
		parsed, _ := typed.Float64()
		return parsed
	case string:
		parsed, _ := strconv.ParseFloat(typed, 64)
		return parsed
	default:
		return 0
	}
}

func manticoreTableName(table string) string {
	if strings.TrimSpace(table) == "" {
		return "nw_chunks"
	}
	return table
}

var manticoreEscaper = strings.NewReplacer(`\`, `\\`, `'`, `\'`, `"`, `\"`)

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

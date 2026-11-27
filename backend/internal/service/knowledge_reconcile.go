// 本文件实现知识库外部存储的启动期对账：把「文档行已不存在」但仍残留在
// PostgreSQL 切片表、Manticore 全文表与 MinIO 对象桶里的数据清掉。
// 残留的孤儿切片会让全文检索召回已删除的文档，孤儿对象会持续占用存储。

package service

import (
	"context"
	"fmt"

	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
)

// ReconcileKnowledgeOrphans 对账并清理孤儿数据，返回清理条数（切片行、Manticore 行、对象）。
func (w *KnowledgeWorker) ReconcileKnowledgeOrphans(ctx context.Context) (chunkCount int64, manticoreCount int, objectCount int, err error) {
	docULIDList, err := w.knowledgeRepo.ListAllDocULIDs(ctx)
	if err != nil {
		return 0, 0, 0, err
	}
	docSet := make(map[string]bool, len(docULIDList))
	for _, docULID := range docULIDList {
		docSet[docULID] = true
	}

	// 1. PostgreSQL 切片表：文档行已删的切片是孤儿。
	chunkCount, err = w.knowledgeRepo.DeleteChunksByDocList(ctx, docULIDList)
	if err != nil {
		return 0, 0, 0, err
	}

	// 2. Manticore 全文表：doc_ulid 已不在文档表里的行是孤儿。
	manticoreCount, err = w.deleteOrphanManticoreChunks(ctx, docSet)
	if err != nil {
		return chunkCount, 0, 0, err
	}

	// 3. MinIO 对象桶：对象键不在文档表 object_key 里的对象是孤儿。
	objectCount, err = w.deleteOrphanObjects(ctx)
	if err != nil {
		return chunkCount, manticoreCount, 0, err
	}
	return chunkCount, manticoreCount, objectCount, nil
}

// deleteOrphanManticoreChunks 删除全文表里文档已不存在的行，返回删除行数。
func (w *KnowledgeWorker) deleteOrphanManticoreChunks(ctx context.Context, docSet map[string]bool) (int, error) {
	if w.manticore == nil {
		return 0, nil
	}
	table := manticoreTableName(w.config.ManticoreTable)
	rows, err := w.manticore.Query(ctx,
		fmt.Sprintf("SELECT doc_ulid, COUNT(*) AS total FROM %s GROUP BY doc_ulid", table))
	if err != nil {
		return 0, err
	}
	orphanList := make([]string, 0, 8)
	orphanCount := 0
	for i := range rows {
		docULID, _ := rows[i]["doc_ulid"].(string)
		if docULID == "" || docSet[docULID] {
			continue
		}
		orphanList = append(orphanList, docULID)
		if total, ok := rows[i]["total"].(int64); ok {
			orphanCount += int(total)
		}
	}
	for _, docULID := range orphanList {
		sql := fmt.Sprintf("DELETE FROM %s WHERE doc_ulid = '%s'", table, escapeManticoreString(docULID))
		if _, err := w.manticore.Exec(ctx, sql); err != nil {
			return orphanCount, err
		}
	}
	return orphanCount, nil
}

// deleteOrphanObjects 删除对象桶里没有文档行对应的对象，返回删除个数。
func (w *KnowledgeWorker) deleteOrphanObjects(ctx context.Context) (int, error) {
	if w.objectRepo == nil {
		return 0, nil
	}
	keyList, err := w.knowledgeRepo.ListAllObjectKeys(ctx)
	if err != nil {
		return 0, err
	}
	keySet := make(map[string]bool, len(keyList))
	for _, objectKey := range keyList {
		keySet[objectKey] = true
	}
	objectList, err := w.objectRepo.ListObjectKeys(ctx)
	if err != nil {
		return 0, err
	}
	orphanList := make([]string, 0, 8)
	for _, objectKey := range objectList {
		if keySet[objectKey] {
			continue
		}
		orphanList = append(orphanList, objectKey)
	}
	for _, objectKey := range orphanList {
		if err := w.objectRepo.RemoveObject(ctx, objectKey); err != nil {
			return len(orphanList), err
		}
	}
	return len(orphanList), nil
}

// logReconcileResult 打印对账结果；无残留时不打扰运维。
func logReconcileResult(chunkCount int64, manticoreCount int, objectCount int) {
	if chunkCount == 0 && manticoreCount == 0 && objectCount == 0 {
		return
	}
	pkg.SLogger().Infow("知识库孤儿数据已清理",
		"orphan_chunks", chunkCount, "orphan_manticore_rows", manticoreCount, "orphan_objects", objectCount)
}

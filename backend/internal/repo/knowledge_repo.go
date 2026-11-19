// 本文件实现知识库文档与切片的持久化仓储。

package repo

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// KnowledgeRepo 知识库文档与切片持久化仓储（全文索引写入 Manticore）。
type KnowledgeRepo struct {
	db *gorm.DB // 事务入口
}

// NewKnowledgeRepo 构造知识库仓储。
func NewKnowledgeRepo(db *gorm.DB) *KnowledgeRepo {
	return &KnowledgeRepo{db: db}
}

// CreateDocument 插入文档记录。
func (r *KnowledgeRepo) CreateDocument(ctx context.Context, do *domain.KnowledgeDocumentDO) error {
	if err := r.db.WithContext(ctx).Create(do).Error; err != nil {
		return fmt.Errorf("create knowledge document: %w", err)
	}
	return nil
}

// GetDocument 按 ULID 查询文档，不存在时返回 (nil, nil)。
func (r *KnowledgeRepo) GetDocument(ctx context.Context, docULID string) (*domain.KnowledgeDocumentDO, error) {
	var do domain.KnowledgeDocumentDO
	err := r.db.WithContext(ctx).Where("doc_ulid = ?", docULID).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get knowledge document: %w", err)
	}
	return &do, nil
}

// UpdateDocument 按列名增量更新文档（map 更新防零值丢失）。
func (r *KnowledgeRepo) UpdateDocument(ctx context.Context, docULID string, fields map[string]any) error {
	result := r.db.WithContext(ctx).Model(&domain.KnowledgeDocumentDO{}).
		Where("doc_ulid = ?", docULID).Updates(fields)
	if result.Error != nil {
		return fmt.Errorf("update knowledge document: %w", result.Error)
	}
	return nil
}

// DeleteDocument 删除文档记录。
func (r *KnowledgeRepo) DeleteDocument(ctx context.Context, docULID string) error {
	if err := r.db.WithContext(ctx).
		Where("doc_ulid = ?", docULID).Delete(&domain.KnowledgeDocumentDO{}).Error; err != nil {
		return fmt.Errorf("delete knowledge document: %w", err)
	}
	return nil
}

// ListDocuments 按项目分页查询文档（状态过滤，上传时间倒序），返回列表与总数。
func (r *KnowledgeRepo) ListDocuments(ctx context.Context, projectULID, docStatusCode string, pageNum, pageSize int) ([]domain.KnowledgeDocumentDO, int64, error) {
	query := r.db.WithContext(ctx).Model(&domain.KnowledgeDocumentDO{}).
		Where("project_ulid = ?", projectULID)
	if docStatusCode != "" {
		query = query.Where("doc_status_code = ?", docStatusCode)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count knowledge documents: %w", err)
	}
	var dos []domain.KnowledgeDocumentDO
	err := query.Order("created_at DESC").
		Offset((pageNum - 1) * pageSize).Limit(pageSize).
		Find(&dos).Error
	if err != nil {
		return nil, 0, fmt.Errorf("list knowledge documents: %w", err)
	}
	return dos, total, nil
}

// GetDocumentsByULIDs 批量查询文档（任务中心回填文档名，避免 N+1）。
func (r *KnowledgeRepo) GetDocumentsByULIDs(ctx context.Context, docULIDList []string) ([]domain.KnowledgeDocumentDO, error) {
	if len(docULIDList) == 0 {
		return []domain.KnowledgeDocumentDO{}, nil
	}
	var dos []domain.KnowledgeDocumentDO
	if err := r.db.WithContext(ctx).Where("doc_ulid IN ?", docULIDList).Find(&dos).Error; err != nil {
		return nil, fmt.Errorf("get documents by ulids: %w", err)
	}
	return dos, nil
}

// ListDocumentsByProject 查询项目下全部文档（项目删除前收集对象键与文档 ULID，供外部存储清理）。
func (r *KnowledgeRepo) ListDocumentsByProject(ctx context.Context, projectULID string) ([]domain.KnowledgeDocumentDO, error) {
	var dos []domain.KnowledgeDocumentDO
	if err := r.db.WithContext(ctx).
		Where("project_ulid = ?", projectULID).Find(&dos).Error; err != nil {
		return nil, fmt.Errorf("list knowledge documents by project: %w", err)
	}
	return dos, nil
}

// DeleteChunksByProject 删除项目下全部文档切片。
func (r *KnowledgeRepo) DeleteChunksByProject(ctx context.Context, projectULID string) error {
	if err := r.db.WithContext(ctx).
		Where("project_ulid = ?", projectULID).Delete(&domain.KnowledgeChunkDO{}).Error; err != nil {
		return fmt.Errorf("delete knowledge chunks by project: %w", err)
	}
	return nil
}

// DeleteDocumentsByProject 删除项目下全部文档记录。
func (r *KnowledgeRepo) DeleteDocumentsByProject(ctx context.Context, projectULID string) error {
	if err := r.db.WithContext(ctx).
		Where("project_ulid = ?", projectULID).Delete(&domain.KnowledgeDocumentDO{}).Error; err != nil {
		return fmt.Errorf("delete knowledge documents by project: %w", err)
	}
	return nil
}

// ListAllChunksForIndex 查询全部切片的索引字段（Manticore 表结构升级后重建全文索引）。
func (r *KnowledgeRepo) ListAllChunksForIndex(ctx context.Context) ([]domain.KnowledgeChunkDO, error) {
	var dos []domain.KnowledgeChunkDO
	if err := r.db.WithContext(ctx).Model(&domain.KnowledgeChunkDO{}).
		Select("chunk_ulid", "doc_ulid", "project_ulid", "chunk_text").
		Order("doc_ulid ASC, chunk_index ASC").Find(&dos).Error; err != nil {
		return nil, fmt.Errorf("list all knowledge chunks: %w", err)
	}
	return dos, nil
}

// DeleteChunksByDoc 删除某文档的全部切片。
func (r *KnowledgeRepo) DeleteChunksByDoc(ctx context.Context, docULID string) error {
	if err := r.db.WithContext(ctx).
		Where("doc_ulid = ?", docULID).Delete(&domain.KnowledgeChunkDO{}).Error; err != nil {
		return fmt.Errorf("delete knowledge chunks: %w", err)
	}
	return nil
}

// InsertChunk 插入一条文本切片（含内容哈希，全文索引由 Manticore 独立维护）。
func (r *KnowledgeRepo) InsertChunk(ctx context.Context, do *domain.KnowledgeChunkDO) error {
	if err := r.db.WithContext(ctx).Create(do).Error; err != nil {
		return fmt.Errorf("insert knowledge chunk: %w", err)
	}
	return nil
}

// GetChunksByULIDs 按切片 ULID 批量取切片（含文档名，供全文检索回表）。
func (r *KnowledgeRepo) GetChunksByULIDs(ctx context.Context, chunkULIDs []string) ([]domain.KnowledgeChunkVO, error) {
	if len(chunkULIDs) == 0 {
		return []domain.KnowledgeChunkVO{}, nil
	}
	sql := `SELECT c.chunk_ulid, c.doc_ulid, d.doc_name, c.chunk_index, c.chunk_text
		FROM knowledge_chunks c
		JOIN knowledge_documents d ON d.doc_ulid = c.doc_ulid
		WHERE c.chunk_ulid IN ?`
	var vos []domain.KnowledgeChunkVO
	if err := r.db.WithContext(ctx).Raw(sql, chunkULIDs).Scan(&vos).Error; err != nil {
		return nil, fmt.Errorf("get knowledge chunks: %w", err)
	}
	return vos, nil
}

// ListChunksByDoc 按文档分页查询切片（切片序号升序），返回列表与总数。
func (r *KnowledgeRepo) ListChunksByDoc(ctx context.Context, docULID string, pageNum, pageSize int) ([]domain.KnowledgeChunkDO, int64, error) {
	query := r.db.WithContext(ctx).Model(&domain.KnowledgeChunkDO{}).Where("doc_ulid = ?", docULID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count knowledge chunks: %w", err)
	}
	var dos []domain.KnowledgeChunkDO
	err := query.Select("chunk_ulid", "doc_ulid", "project_ulid", "chunk_index", "chunk_text", "token_count", "content_hash", "created_at", "updated_at").
		Order("chunk_index ASC").
		Offset((pageNum - 1) * pageSize).Limit(pageSize).
		Find(&dos).Error
	if err != nil {
		return nil, 0, fmt.Errorf("list knowledge chunks: %w", err)
	}
	return dos, total, nil
}

// ListChunksByDocAll 查询文档下全部切片（含内容哈希，供重索引增量复用）。
func (r *KnowledgeRepo) ListChunksByDocAll(ctx context.Context, docULID string) ([]domain.KnowledgeChunkDO, error) {
	var dos []domain.KnowledgeChunkDO
	if err := r.db.WithContext(ctx).Model(&domain.KnowledgeChunkDO{}).
		Where("doc_ulid = ?", docULID).
		Order("chunk_index ASC").Find(&dos).Error; err != nil {
		return nil, fmt.Errorf("list knowledge chunks by document: %w", err)
	}
	return dos, nil
}

// StatsByProject 汇总项目文档状态与切片总量（知识库页概览）。
func (r *KnowledgeRepo) StatsByProject(ctx context.Context, projectULID string) (domain.KnowledgeStatsVO, error) {
	var vo domain.KnowledgeStatsVO
	sql := `SELECT
		COUNT(*) AS total_count,
		COUNT(*) FILTER (WHERE doc_status_code = 'indexed') AS indexed_count,
		COUNT(*) FILTER (WHERE doc_status_code IN ('pending', 'indexing')) AS pending_count,
		COUNT(*) FILTER (WHERE doc_status_code = 'failed') AS failed_count,
		COALESCE(SUM(chunk_count), 0) AS chunk_count
	FROM knowledge_documents WHERE project_ulid = ?`
	if err := r.db.WithContext(ctx).Raw(sql, projectULID).Scan(&vo).Error; err != nil {
		return domain.KnowledgeStatsVO{}, fmt.Errorf("stats knowledge documents: %w", err)
	}
	return vo, nil
}

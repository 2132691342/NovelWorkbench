package domain

import "io"

// 知识库检索模式与来源通道常量（均为关键词/全文检索通道）。
const (
	SearchModeKeyword = "keyword" // Manticore MATCH 关键词/短语检索
	SearchModeHybrid  = "hybrid"  // 关键词与短语双通道 RRF 融合（默认）
)

// KnowledgeDocumentDO 知识库文档持久化对象（原文存 MinIO，索引存 PG/Manticore）。
type KnowledgeDocumentDO struct {
	BaseDO
	DocULID         string `json:"doc_ulid" gorm:"column:doc_ulid;primaryKey;size:40"`    // 文档 ULID（DOC_ 前缀）
	ProjectULID     string `json:"project_ulid" gorm:"column:project_ulid;size:40;index"` // 所属项目（0 表示全局库）
	DocName         string `json:"doc_name" gorm:"size:128"`                              // 文档名
	DocTypeCode     string `json:"doc_type_code" gorm:"size:16"`                          // 类型：txt/md/pdf/docx
	ObjectKey       string `json:"object_key" gorm:"size:256"`                            // MinIO 对象键
	DocStatusCode   string `json:"doc_status_code" gorm:"size:16;index"`                  // 状态：pending/indexing/indexed/failed
	ChunkCount      int    `json:"chunk_count"`                                           // 切片数量
	TotalTokenCount int64  `json:"total_token_count"`                                     // 估算 token 总量
	ErrorMessage    string `json:"error_message" gorm:"size:512"`                         // 失败原因
}

// TableName 指定表名。
func (KnowledgeDocumentDO) TableName() string { return "knowledge_documents" }

// KnowledgeChunkDO 知识库切片持久化对象（原文切片 + 内容哈希）。
type KnowledgeChunkDO struct {
	BaseDO
	ChunkULID   string `json:"chunk_ulid" gorm:"column:chunk_ulid;primaryKey;size:40"` // 切片 ULID（CHK_ 前缀）
	DocULID     string `json:"doc_ulid" gorm:"column:doc_ulid;size:40;index"`          // 所属文档
	ProjectULID string `json:"project_ulid" gorm:"column:project_ulid;size:40;index"`  // 所属项目
	ChunkIndex  int    `json:"chunk_index"`                                            // 切片序号（文档内升序）
	ChunkText   string `json:"chunk_text" gorm:"type:text"`                            // 切片文本
	TokenCount  int    `json:"token_count"`                                            // 估算 token 数
	ContentHash string `json:"content_hash" gorm:"column:content_hash;size:64;index"`  // 切片内容 SHA-256（去重/增量重建）
}

// TableName 指定表名。
func (KnowledgeChunkDO) TableName() string { return "knowledge_chunks" }

// KnowledgeUploadREQ 文档上传请求（multipart/form-data：file 字段 + 本结构）。
type KnowledgeUploadREQ struct {
	ProjectULID string `form:"project_ulid" binding:"required"`     // 所属项目
	DocName     string `form:"doc_name" binding:"required,max=128"` // 文档名
}

// KnowledgeListREQ 文档列表查询。
type KnowledgeListREQ struct {
	PageREQ
	ProjectULID   string `json:"project_ulid" binding:"required"`                                           // 所属项目
	DocStatusCode string `json:"doc_status_code" binding:"omitempty,oneof=pending indexing indexed failed"` // 状态过滤
}

// KnowledgeSearchREQ 知识库检索请求（RAG 召回 / 界面搜索共用）。
type KnowledgeSearchREQ struct {
	ProjectULID string   `json:"project_ulid" binding:"required"`              // 所属项目
	KeywordText string   `json:"keyword_text" binding:"omitempty,max=512"`     // 检索文本（关键词/语义问句均可）
	SearchMode  string   `json:"search_mode" binding:"omitempty,max=16"`       // 检索模式：keyword/hybrid（默认），其他值按 keyword 处理
	TopKCount   int      `json:"top_k_count" binding:"omitempty,min=1,max=20"` // 召回条数，默认 6
	DocULIDList []string `json:"doc_ulid_list"`                                // 限定文档范围，空为全部
}

// KnowledgeDocumentVO 文档视图对象。
type KnowledgeDocumentVO struct {
	DocULID         string `json:"doc_ulid"`          // 文档 ULID
	ProjectULID     string `json:"project_ulid"`      // 所属项目
	DocName         string `json:"doc_name"`          // 文档名
	DocTypeCode     string `json:"doc_type_code"`     // 类型
	DocStatusCode   string `json:"doc_status_code"`   // 状态
	ChunkCount      int    `json:"chunk_count"`       // 切片数量
	TotalTokenCount int64  `json:"total_token_count"` // token 总量
	ErrorMessage    string `json:"error_message"`     // 失败原因
	CreatedAtMs     int64  `json:"created_at_ms"`     // 上传时间（毫秒时间戳）
}

// KnowledgeChunkVO 切片视图对象（搜索结果，含可解释的检索过程信息）。
type KnowledgeChunkVO struct {
	ChunkULID  string  `json:"chunk_ulid"`  // 切片 ULID
	DocULID    string  `json:"doc_ulid"`    // 所属文档
	DocName    string  `json:"doc_name"`    // 文档名（联表得出）
	ChunkIndex int     `json:"chunk_index"` // 切片序号
	ChunkText  string  `json:"chunk_text"`  // 切片文本
	ScoreValue float64 `json:"score_value"` // 相关度得分（RRF 或 Manticore BM25）
	SourceMode string  `json:"source_mode"` // 命中通道：keyword/phrase/hybrid/rerank
}

// KnowledgeSearchTrace 检索过程说明（query 构造、候选规模、入选 ID 与通道）。
type KnowledgeSearchTrace struct {
	QueryTerms     []string `json:"query_terms"`     // 抽取的检索词
	QueryExpr      string   `json:"query_expr"`      // Manticore MATCH 表达式（关键词通道）
	CandidateCount int      `json:"candidate_count"` // 候选切片数
	Reranked       bool     `json:"reranked"`        // 是否由对话模型重排
	RerankNote     string   `json:"rerank_note"`     // 重排执行/降级说明
	SelectedIDs    []string `json:"selected_ids"`    // 最终入选切片 ULID（有序）
	SourceMode     string   `json:"source_mode"`     // 主检索通道：keyword/hybrid
}

// KnowledgeDeleteREQ 删除文档请求（删原文 + 切片 + 索引）。
type KnowledgeDeleteREQ struct {
	DocULID string `json:"doc_ulid" form:"doc_ulid" binding:"required"` // 文档 ULID
}

// KnowledgeReindexREQ 重新索引请求。
type KnowledgeReindexREQ struct {
	DocULID string `json:"doc_ulid" form:"doc_ulid" binding:"required"` // 文档 ULID
}

// KnowledgeSearchDTO 检索结果与可解释检索过程（API 响应）。
type KnowledgeSearchDTO struct {
	ResultList []KnowledgeChunkVO   `json:"result_list"` // 检索结果（有序）
	Trace      KnowledgeSearchTrace `json:"trace"`       // 检索过程说明
}

// KnowledgeChunkListREQ 文档切片分页查询（GET query）。
type KnowledgeChunkListREQ struct {
	PageREQ
	DocULID string `json:"doc_ulid" form:"doc_ulid" binding:"required"` // 文档 ULID
}

// KnowledgeUploadFileDTO 上传文件流（api → service 传递 multipart 文件内容）。
type KnowledgeUploadFileDTO struct {
	FileName string        `json:"file_name" form:"file_name"` // 原始文件名（含扩展名）
	FileSize int64         `json:"file_size" form:"file_size"` // 文件字节数
	Reader   io.ReadCloser `json:"-" form:"-"`                 // 文件内容流（调用方负责 Close）
}

// KnowledgeIndexJobDTO 索引任务载荷（RabbitMQ 消息体）。
type KnowledgeIndexJobDTO struct {
	DocULID string `json:"doc_ulid"` // 待索引文档
}

// KnowledgeContentREQ 拉取文档原文请求（GET /knowledge/content，预览用）。
type KnowledgeContentREQ struct {
	DocULID string `json:"doc_ulid" form:"doc_ulid" binding:"required"` // 文档 ULID
}

// KnowledgeContentDTO 文档原文（service → api 二进制透传）。
type KnowledgeContentDTO struct {
	DocName      string // 文档名
	DocTypeCode  string // 文档类型（txt/md/docx/pdf）
	ContentBytes []byte // 原文字节
}

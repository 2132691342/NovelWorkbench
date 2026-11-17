package domain

// ChapterDO 章节持久化对象。
type ChapterDO struct {
	BaseDO
	ChapterULID  string `json:"chapter_ulid" gorm:"column:chapter_ulid;primaryKey;size:40"` // 章节 ULID（CHP_ 前缀）
	ProjectULID  string `json:"project_ulid" gorm:"column:project_ulid;size:40;index"`     // 所属项目
	VolumeULID   string `json:"volume_ulid" gorm:"column:volume_ulid;size:40;index"`      // 所属卷
	ChapterName  string `json:"chapter_name" gorm:"size:128"`          // 章节名
	ChapterOrder int    `json:"chapter_order" gorm:"index"`            // 排序号，升序
	ContentText  string `json:"content_text" gorm:"type:text"`         // 正文
	SummaryText  string `json:"summary_text" gorm:"size:2048"`         // 章节梗概
	WordCount    int64  `json:"word_count"`                            // 正文字数
	ChapterStatus string `json:"chapter_status" gorm:"size:16"`        // 状态：draft/revised/locked
}

// TableName 指定表名。
func (ChapterDO) TableName() string { return "chapters" }

// ChapterSnapshotDO 章节快照（人工/AI 修改前留档，可回滚）。
type ChapterSnapshotDO struct {
	BaseDO
	SnapshotULID string `json:"snapshot_ulid" gorm:"column:snapshot_ulid;primaryKey;size:40"` // 快照 ULID（SNP_ 前缀）
	ChapterULID  string `json:"chapter_ulid" gorm:"column:chapter_ulid;size:40;index"`      // 所属章节
	ContentText  string `json:"content_text" gorm:"type:text"`          // 快照正文
	SnapshotType string `json:"snapshot_type" gorm:"size:16"`           // 来源：manual/ai_before/ai_after
	ChangeNote   string `json:"change_note" gorm:"size:256"`            // 变更说明
}

// TableName 指定表名。
func (ChapterSnapshotDO) TableName() string { return "chapter_snapshots" }

// ChapterCreateREQ 创建章节请求。
type ChapterCreateREQ struct {
	ProjectULID string `json:"project_ulid" form:"project_ulid" binding:"required"`                 // 项目 ULID
	VolumeULID  string `json:"volume_ulid" form:"volume_ulid" binding:"required"`                   // 卷 ULID
	ChapterName string `json:"chapter_name" form:"chapter_name" binding:"required,min=1,max=128"`   // 章节名
}

// ChapterUpdateREQ 更新章节请求（内容整体覆盖）。
type ChapterUpdateREQ struct {
	ChapterULID   string `json:"chapter_ulid" form:"chapter_ulid" binding:"required"`                             // 章节 ULID
	ChapterName   string `json:"chapter_name" form:"chapter_name" binding:"omitempty,max=128"`                    // 章节名
	VolumeULID    string `json:"volume_ulid" form:"volume_ulid" binding:"omitempty"`                             // 所属卷
	ContentText   *string `json:"content_text" form:"content_text" binding:"omitempty,max=200000"`                 // 正文（缺省不改，空串清空）
	SummaryText   *string `json:"summary_text" form:"summary_text" binding:"omitempty,max=2048"`                   // 梗概（缺省不改，空串清空）
	ChapterStatus string `json:"chapter_status" form:"chapter_status" binding:"omitempty,oneof=draft revised locked"` // 状态
}

// ChapterContentREQ 拉取章节正文（编辑器载入）。
type ChapterContentREQ struct {
	ChapterULID string `json:"chapter_ulid" form:"chapter_ulid" binding:"required"` // 章节 ULID
}

// ChapterDeleteREQ 删除章节请求。
type ChapterDeleteREQ struct {
	ChapterULID string `json:"chapter_ulid" form:"chapter_ulid" binding:"required"` // 章节 ULID
}

// ChapterSnapshotListREQ 章节快照列表查询（GET query）。
type ChapterSnapshotListREQ struct {
	ChapterULID string `json:"chapter_ulid" form:"chapter_ulid" binding:"required"` // 章节 ULID
}

// ChapterSnapshotCreateREQ 生成章节快照请求。
type ChapterSnapshotCreateREQ struct {
	ChapterULID string `json:"chapter_ulid" form:"chapter_ulid" binding:"required"`          // 章节 ULID
	ChangeNote  string `json:"change_note" form:"change_note" binding:"omitempty,max=256"`   // 变更说明
}

// ChapterSnapshotRestoreREQ 回滚章节到快照请求。
type ChapterSnapshotRestoreREQ struct {
	SnapshotULID string `json:"snapshot_ulid" form:"snapshot_ulid" binding:"required"` // 快照 ULID
}

// ChapterBriefVO 章节摘要视图（目录树用，不含正文）。
type ChapterBriefVO struct {
	ChapterULID   string `json:"chapter_ulid"`   // 章节 ULID
	ChapterName   string `json:"chapter_name"`   // 章节名
	ChapterOrder  int    `json:"chapter_order"`  // 排序号
	WordCount     int64  `json:"word_count"`     // 字数
	ChapterStatus string `json:"chapter_status"` // 状态
}

// ChapterDetailVO 章节详情视图（含正文）。
type ChapterDetailVO struct {
	ChapterBriefVO
	ProjectULID  string `json:"project_ulid"`  // 所属项目
	VolumeULID   string `json:"volume_ulid"`   // 所属卷
	ContentText  string `json:"content_text"`  // 正文
	SummaryText  string `json:"summary_text"`  // 梗概
	UpdatedAtMs  int64  `json:"updated_at_ms"` // 最后编辑时间（毫秒时间戳）
}

// ChapterSnapshotVO 快照视图（历史版本列表/回滚）。
type ChapterSnapshotVO struct {
	SnapshotULID string `json:"snapshot_ulid"` // 快照 ULID
	ChapterULID  string `json:"chapter_ulid"`  // 章节 ULID
	SnapshotType string `json:"snapshot_type"` // 来源
	ChangeNote   string `json:"change_note"`   // 变更说明
	WordCount    int64  `json:"word_count"`    // 快照字数
	CreatedAtMs  int64  `json:"created_at_ms"` // 生成时间（毫秒时间戳）
}

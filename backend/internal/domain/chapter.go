// 本文件定义章节与章节快照的持久化对象、请求与视图对象。

package domain

// ChapterDO 章节持久化对象。
type ChapterDO struct {
	BaseDO
	ChapterULID   string `json:"chapter_ulid" gorm:"column:chapter_ulid;primaryKey;size:40"` // 章节 ULID（CHP_ 前缀）
	ProjectULID   string `json:"project_ulid" gorm:"column:project_ulid;size:40;index"`      // 所属项目
	VolumeULID    string `json:"volume_ulid" gorm:"column:volume_ulid;size:40;index"`        // 所属卷
	ChapterName   string `json:"chapter_name" gorm:"size:128"`                               // 章节名
	ChapterOrder  int    `json:"chapter_order" gorm:"index"`                                 // 排序号，升序
	ContentText   string `json:"content_text" gorm:"type:text"`                              // 正文
	SummaryText   string `json:"summary_text" gorm:"size:2048"`                              // 章节梗概
	WordCount     int64  `json:"word_count"`                                                 // 正文字数
	ChapterStatus string `json:"chapter_status" gorm:"size:16"`                              // 状态：draft/revised/locked
}

// TableName 指定表名。
func (ChapterDO) TableName() string { return "chapters" }

// ChapterSnapshotDO 章节快照（人工/AI 修改前留档，可回滚）。
type ChapterSnapshotDO struct {
	BaseDO
	SnapshotULID string `json:"snapshot_ulid" gorm:"column:snapshot_ulid;primaryKey;size:40"` // 快照 ULID（SNP_ 前缀）
	ChapterULID  string `json:"chapter_ulid" gorm:"column:chapter_ulid;size:40;index"`        // 所属章节
	ContentText  string `json:"content_text" gorm:"type:text"`                                // 快照正文
	SnapshotType string `json:"snapshot_type" gorm:"size:16"`                                 // 来源：manual（人工留档，当前仅此一种）
	ChangeNote   string `json:"change_note" gorm:"size:256"`                                  // 变更说明
}

// TableName 指定表名。
func (ChapterSnapshotDO) TableName() string { return "chapter_snapshots" }

// ChapterCreateREQ 创建章节请求。
type ChapterCreateREQ struct {
	ProjectULID string `json:"project_ulid" form:"project_ulid" binding:"required"`               // 项目 ULID
	VolumeULID  string `json:"volume_ulid" form:"volume_ulid" binding:"required"`                 // 卷 ULID
	ChapterName string `json:"chapter_name" form:"chapter_name" binding:"required,min=1,max=128"` // 章节名
}

// ChapterUpdateREQ 更新章节请求（内容整体覆盖）。
type ChapterUpdateREQ struct {
	ChapterULID   string  `json:"chapter_ulid" form:"chapter_ulid" binding:"required"`                                 // 章节 ULID
	ChapterName   string  `json:"chapter_name" form:"chapter_name" binding:"omitempty,max=128"`                        // 章节名
	VolumeULID    string  `json:"volume_ulid" form:"volume_ulid" binding:"omitempty"`                                  // 所属卷
	ContentText   *string `json:"content_text" form:"content_text" binding:"omitempty,max=200000"`                     // 正文（缺省不改，空串清空）
	SummaryText   *string `json:"summary_text" form:"summary_text" binding:"omitempty,max=2048"`                       // 梗概（缺省不改，空串清空）
	ChapterStatus string  `json:"chapter_status" form:"chapter_status" binding:"omitempty,oneof=draft revised locked"` // 状态
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
	ChapterULID string `json:"chapter_ulid" form:"chapter_ulid" binding:"required"`        // 章节 ULID
	ChangeNote  string `json:"change_note" form:"change_note" binding:"omitempty,max=256"` // 变更说明
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
	ProjectULID string `json:"project_ulid"` // 所属项目
	VolumeULID  string `json:"volume_ulid"`  // 所属卷
	ContentText string `json:"content_text"` // 正文
	SummaryText string `json:"summary_text"` // 梗概
	UpdatedAt   string `json:"updated_at"`   // 最后编辑时间（RFC3339）
}

// ChapterSnapshotVO 快照视图（历史版本列表/回滚）。
type ChapterSnapshotVO struct {
	SnapshotULID string `json:"snapshot_ulid"` // 快照 ULID
	ChapterULID  string `json:"chapter_ulid"`  // 章节 ULID
	SnapshotType string `json:"snapshot_type"` // 来源
	ChangeNote   string `json:"change_note"`   // 变更说明
	WordCount    int64  `json:"word_count"`    // 快照字数
	CreatedAt    string `json:"created_at"`    // 生成时间（RFC3339）
}

// ChapterCheckREQ 章节体检请求（content_text 为空时取库内正文）。
type ChapterCheckREQ struct {
	ChapterULID string `json:"chapter_ulid" binding:"omitempty,max=40"`     // 章节 ULID（可空）
	ContentText string `json:"content_text" binding:"omitempty,max=200000"` // 待检正文（优先于库内正文）
}

// ChapterCheckVO 章节体检视图（check.Report 的对外投影，避免 domain 依赖 check 包）。
type ChapterCheckVO struct {
	WordCount        int64                 `json:"word_count"`         // 字数
	ParagraphCount   int                   `json:"paragraph_count"`    // 段落数
	DialogueRatio    float64               `json:"dialogue_ratio"`     // 对话段占比（0-1）
	DashCount        int                   `json:"dash_count"`         // 破折号数量
	BannedWordCount  int                   `json:"banned_word_count"`  // 零容忍词命中数
	WarnWordCount    int                   `json:"warn_word_count"`    // 警示词命中数
	EmotionWordCount int                   `json:"emotion_word_count"` // 情绪命名命中数
	BareTagCount     int                   `json:"bare_tag_count"`     // 裸对话标签数
	HookPresent      bool                  `json:"hook_present"`       // 章末是否有钩子
	ScoreValue       int                   `json:"score_value"`        // 综合分（0-100）
	VerdictCode      string                `json:"verdict_code"`       // 结论：pass/revise/block
	IssueList        []ChapterCheckIssueVO `json:"issue_list"`         // 问题清单
}

// ChapterCheckIssueVO 体检问题条目。
type ChapterCheckIssueVO struct {
	IssueCode  string `json:"issue_code"`  // 问题编码
	LevelCode  string `json:"level_code"`  // 级别：block/warn/info
	TitleText  string `json:"title_text"`  // 问题标题
	DetailText string `json:"detail_text"` // 说明与改法
	HitCount   int    `json:"hit_count"`   // 命中次数
	SampleText string `json:"sample_text"` // 命中示例
}

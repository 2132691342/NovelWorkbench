// 本文件定义连续性看板（时间事实、伏笔钩子、章节摘要）的持久化对象、请求与视图对象。

package domain

// 连续性看板 ULID 业务前缀：主键形如 FCT_01HXXX，便于肉眼识别。
const (
	IDPrefixTemporalFact   = "FCT_" // 时间事实
	IDPrefixForeshadowHook = "HOK_" // 伏笔钩子
	IDPrefixChapterSummary = "CSM_" // 章节摘要
)

// 时间事实状态码（候选/已确认/已被取代/已过期）。
const (
	FactStatusCodeCandidate  = "candidate"  // 候选，待人工复核
	FactStatusCodeConfirmed  = "confirmed"  // 已确认
	FactStatusCodeSuperseded = "superseded" // 已被新事实取代
	FactStatusCodeStale      = "stale"      // 已过期
)

// 伏笔钩子状态码（计划中/已埋设/已呼应/已回收/已放弃）。
const (
	HookStatusCodePlanned   = "planned"   // 计划中
	HookStatusCodePlanted   = "planted"   // 已埋设
	HookStatusCodeEchoed    = "echoed"    // 已呼应
	HookStatusCodeResolved  = "resolved"  // 已回收
	HookStatusCodeAbandoned = "abandoned" // 已放弃
)

// 伏笔重要程度编码（高/中/低）。
const (
	ImportanceCodeHigh   = "high"   // 高
	ImportanceCodeMedium = "medium" // 中
	ImportanceCodeLow    = "low"    // 低
)

// 章节摘要状态码（待核验/已核验/已过期）。
const (
	SummaryStatusCodePending  = "pending"  // 待核验
	SummaryStatusCodeVerified = "verified" // 已核验
	SummaryStatusCodeStale    = "stale"    // 已过期
)

// TemporalFactDO 时间事实持久化对象（角色状态、物品位置等带有效区间的事实）。
type TemporalFactDO struct {
	BaseDO
	FactULID              string `json:"fact_ulid" gorm:"column:fact_ulid;primaryKey;size:40"`                    // 事实 ULID（FCT_ 前缀）
	ProjectULID           string `json:"project_ulid" gorm:"column:project_ulid;size:40;index"`                   // 所属项目
	SubjectText           string `json:"subject_text" gorm:"column:subject_text;size:128"`                        // 主体
	PredicateCode         string `json:"predicate_code" gorm:"column:predicate_code;size:64;index"`               // 谓词编码
	ObjectText            string `json:"object_text" gorm:"column:object_text;size:256"`                          // 客体
	FactStatusCode        string `json:"fact_status_code" gorm:"column:fact_status_code;size:16;index"`           // 事实状态
	ValidFromChapterULID  string `json:"valid_from_chapter_ulid" gorm:"column:valid_from_chapter_ulid;size:40"`   // 生效起始章节
	ValidUntilChapterULID string `json:"valid_until_chapter_ulid" gorm:"column:valid_until_chapter_ulid;size:40"` // 生效截止章节
	SourceChapterULID     string `json:"source_chapter_ulid" gorm:"column:source_chapter_ulid;size:40;index"`     // 证据来源章节
	SourceQuoteText       string `json:"source_quote_text" gorm:"column:source_quote_text;type:text"`             // 原文证据
	NoteText              string `json:"note_text" gorm:"column:note_text;type:text"`                             // 备注
}

// TableName 指定表名。
func (TemporalFactDO) TableName() string { return "temporal_facts" }

// ForeshadowHookDO 伏笔钩子持久化对象（计划、埋设、呼应、回收全过程跟踪）。
type ForeshadowHookDO struct {
	BaseDO
	HookULID           string `json:"hook_ulid" gorm:"column:hook_ulid;primaryKey;size:40"`                // 钩子 ULID（HOK_ 前缀）
	ProjectULID        string `json:"project_ulid" gorm:"column:project_ulid;size:40;index"`               // 所属项目
	HookStatusCode     string `json:"hook_status_code" gorm:"column:hook_status_code;size:16;index"`       // 钩子状态
	DescriptionText    string `json:"description_text" gorm:"column:description_text;type:text"`           // 伏笔描述
	ExpectedPayoffText string `json:"expected_payoff_text" gorm:"column:expected_payoff_text;type:text"`   // 预期回收方式
	SetupChapterULID   string `json:"setup_chapter_ulid" gorm:"column:setup_chapter_ulid;size:40;index"`   // 埋设章节
	PayoffChapterULID  string `json:"payoff_chapter_ulid" gorm:"column:payoff_chapter_ulid;size:40;index"` // 回收章节
	ImportanceCode     string `json:"importance_code" gorm:"column:importance_code;size:16;index"`         // 重要程度
	NoteText           string `json:"note_text" gorm:"column:note_text;type:text"`                         // 备注
}

// TableName 指定表名。
func (ForeshadowHookDO) TableName() string { return "foreshadow_hooks" }

// ChapterSummaryDO 章节摘要持久化对象（供连续性核对使用的章节级摘要）。
type ChapterSummaryDO struct {
	BaseDO
	SummaryULID       string `json:"summary_ulid" gorm:"column:summary_ulid;primaryKey;size:40"`          // 摘要 ULID（CSM_ 前缀）
	ProjectULID       string `json:"project_ulid" gorm:"column:project_ulid;size:40;index"`               // 所属项目
	ChapterULID       string `json:"chapter_ulid" gorm:"column:chapter_ulid;size:40;index"`               // 所属章节
	SummaryText       string `json:"summary_text" gorm:"column:summary_text;type:text"`                   // 摘要正文
	SourceHashText    string `json:"source_hash_text" gorm:"column:source_hash_text;size:128"`            // 摘要生成时正文指纹
	SummaryStatusCode string `json:"summary_status_code" gorm:"column:summary_status_code;size:16;index"` // 摘要状态
}

// TableName 指定表名。
func (ChapterSummaryDO) TableName() string { return "chapter_summaries" }

// TemporalFactSaveREQ 时间事实新建/更新请求（fact_ulid 为空表示新建）。
type TemporalFactSaveREQ struct {
	FactULID              string `json:"fact_ulid" binding:"omitempty"`                                                   // 事实 ULID，更新时必填
	ProjectULID           string `json:"project_ulid" binding:"required"`                                                 // 所属项目
	SubjectText           string `json:"subject_text" binding:"required,min=1,max=128"`                                   // 主体
	PredicateCode         string `json:"predicate_code" binding:"required,min=1,max=64"`                                  // 谓词编码
	ObjectText            string `json:"object_text" binding:"omitempty,max=256"`                                         // 客体
	FactStatusCode        string `json:"fact_status_code" binding:"omitempty,oneof=candidate confirmed superseded stale"` // 事实状态，缺省 candidate
	ValidFromChapterULID  string `json:"valid_from_chapter_ulid" binding:"omitempty,max=40"`                              // 生效起始章节
	ValidUntilChapterULID string `json:"valid_until_chapter_ulid" binding:"omitempty,max=40"`                             // 生效截止章节
	SourceChapterULID     string `json:"source_chapter_ulid" binding:"required"`                                          // 证据来源章节
	SourceQuoteText       string `json:"source_quote_text" binding:"omitempty,max=8000"`                                  // 原文证据
	NoteText              string `json:"note_text" binding:"omitempty,max=4000"`                                          // 备注
}

// TemporalFactReviewREQ 时间事实复核请求（确认/取代/标记过期）。
type TemporalFactReviewREQ struct {
	FactULID       string `json:"fact_ulid" binding:"required"`                                         // 事实 ULID
	ProjectULID    string `json:"project_ulid" binding:"required"`                                      // 所属项目
	FactStatusCode string `json:"fact_status_code" binding:"required,oneof=confirmed superseded stale"` // 复核目标状态
	NoteText       string `json:"note_text" binding:"omitempty,max=4000"`                               // 复核备注（非空时覆盖）
}

// TemporalFactListREQ 时间事实列表查询请求。
type TemporalFactListREQ struct {
	ProjectULID    string `json:"project_ulid" binding:"required"`                                                 // 所属项目
	FactStatusCode string `json:"fact_status_code" binding:"omitempty,oneof=candidate confirmed superseded stale"` // 状态过滤
	ChapterULID    string `json:"chapter_ulid" binding:"omitempty,max=40"`                                         // 章节过滤
}

// ForeshadowHookSaveREQ 伏笔钩子新建/更新请求（hook_ulid 为空表示新建）。
type ForeshadowHookSaveREQ struct {
	HookULID           string `json:"hook_ulid" binding:"omitempty"`                                                       // 钩子 ULID，更新时必填
	ProjectULID        string `json:"project_ulid" binding:"required"`                                                     // 所属项目
	HookStatusCode     string `json:"hook_status_code" binding:"required,oneof=planned planted echoed resolved abandoned"` // 钩子状态
	DescriptionText    string `json:"description_text" binding:"required,min=1,max=4000"`                                  // 伏笔描述
	ExpectedPayoffText string `json:"expected_payoff_text" binding:"omitempty,max=4000"`                                   // 预期回收方式
	SetupChapterULID   string `json:"setup_chapter_ulid" binding:"omitempty,max=40"`                                       // 埋设章节
	PayoffChapterULID  string `json:"payoff_chapter_ulid" binding:"omitempty,max=40"`                                      // 回收章节
	ImportanceCode     string `json:"importance_code" binding:"omitempty,oneof=high medium low"`                           // 重要程度，缺省 medium
	NoteText           string `json:"note_text" binding:"omitempty,max=4000"`                                              // 备注
}

// ForeshadowHookListREQ 伏笔钩子列表查询请求。
type ForeshadowHookListREQ struct {
	ProjectULID    string `json:"project_ulid" binding:"required"`                                                      // 所属项目
	HookStatusCode string `json:"hook_status_code" binding:"omitempty,oneof=planned planted echoed resolved abandoned"` // 状态过滤
}

// ChapterSummarySaveREQ 章节摘要新建/更新请求（章节已存在摘要时自动更新）。
type ChapterSummarySaveREQ struct {
	SummaryULID       string `json:"summary_ulid" binding:"omitempty"`                                     // 摘要 ULID，指定时按 ULID 更新
	ProjectULID       string `json:"project_ulid" binding:"required"`                                      // 所属项目
	ChapterULID       string `json:"chapter_ulid" binding:"required"`                                      // 所属章节
	SummaryText       string `json:"summary_text" binding:"required,min=1,max=20000"`                      // 摘要正文
	SourceHashText    string `json:"source_hash_text" binding:"omitempty,max=128"`                         // 正文指纹
	SummaryStatusCode string `json:"summary_status_code" binding:"omitempty,oneof=pending verified stale"` // 摘要状态，缺省 pending
}

// ChapterSummaryListREQ 章节摘要列表查询请求。
type ChapterSummaryListREQ struct {
	ProjectULID       string `json:"project_ulid" binding:"required"`                                      // 所属项目
	SummaryStatusCode string `json:"summary_status_code" binding:"omitempty,oneof=pending verified stale"` // 状态过滤
	ChapterULID       string `json:"chapter_ulid" binding:"omitempty,max=40"`                              // 章节过滤
}

// TemporalFactVO 时间事实视图对象。
type TemporalFactVO struct {
	FactULID              string `json:"fact_ulid"`                // 事实 ULID
	ProjectULID           string `json:"project_ulid"`             // 所属项目
	SubjectText           string `json:"subject_text"`             // 主体
	PredicateCode         string `json:"predicate_code"`           // 谓词编码
	ObjectText            string `json:"object_text"`              // 客体
	FactStatusCode        string `json:"fact_status_code"`         // 事实状态
	ValidFromChapterULID  string `json:"valid_from_chapter_ulid"`  // 生效起始章节
	ValidUntilChapterULID string `json:"valid_until_chapter_ulid"` // 生效截止章节
	SourceChapterULID     string `json:"source_chapter_ulid"`      // 证据来源章节
	SourceQuoteText       string `json:"source_quote_text"`        // 原文证据
	NoteText              string `json:"note_text"`                // 备注
	UpdatedAtMs           int64  `json:"updated_at_ms"`            // 更新时间（毫秒时间戳）
}

// ForeshadowHookVO 伏笔钩子视图对象。
type ForeshadowHookVO struct {
	HookULID           string `json:"hook_ulid"`            // 钩子 ULID
	ProjectULID        string `json:"project_ulid"`         // 所属项目
	HookStatusCode     string `json:"hook_status_code"`     // 钩子状态
	DescriptionText    string `json:"description_text"`     // 伏笔描述
	ExpectedPayoffText string `json:"expected_payoff_text"` // 预期回收方式
	SetupChapterULID   string `json:"setup_chapter_ulid"`   // 埋设章节
	PayoffChapterULID  string `json:"payoff_chapter_ulid"`  // 回收章节
	ImportanceCode     string `json:"importance_code"`      // 重要程度
	NoteText           string `json:"note_text"`            // 备注
	UpdatedAtMs        int64  `json:"updated_at_ms"`        // 更新时间（毫秒时间戳）
}

// ChapterSummaryVO 章节摘要视图对象。
type ChapterSummaryVO struct {
	SummaryULID       string `json:"summary_ulid"`        // 摘要 ULID
	ProjectULID       string `json:"project_ulid"`        // 所属项目
	ChapterULID       string `json:"chapter_ulid"`        // 所属章节
	SummaryText       string `json:"summary_text"`        // 摘要正文
	SourceHashText    string `json:"source_hash_text"`    // 正文指纹
	SummaryStatusCode string `json:"summary_status_code"` // 摘要状态
	UpdatedAtMs       int64  `json:"updated_at_ms"`       // 更新时间（毫秒时间戳）
}

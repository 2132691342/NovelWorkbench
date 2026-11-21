// 本文件定义小说项目与卷的持久化对象、请求与视图对象（含目录树）。

package domain

// ProjectDO 小说项目持久化对象。
type ProjectDO struct {
	BaseDO
	ProjectULID    string `json:"project_ulid" gorm:"column:project_ulid;primaryKey;size:40"` // 项目 ULID（PRJ_ 前缀）
	UserULID       string `json:"user_ulid" gorm:"column:user_ulid;size:40;index"`            // 所属用户
	ProjectName    string `json:"project_name" gorm:"size:128"`                               // 书名
	GenreCode      string `json:"genre_code" gorm:"size:32"`                                  // 题材编码（fantasy/scifi/romance...）
	StyleText      string `json:"style_text" gorm:"size:1024"`                                // 文风设定提示词
	SynopsisText   string `json:"synopsis_text" gorm:"size:4096"`                             // 简介
	ProjectStatus  string `json:"project_status" gorm:"size:16;index"`                        // 状态：draft/serial/completed
	TotalWordCount int64  `json:"total_word_count"`                                           // 累计字数
}

// TableName 指定表名。
func (ProjectDO) TableName() string { return "projects" }

// VolumeDO 卷持久化对象。
type VolumeDO struct {
	BaseDO
	VolumeULID  string `json:"volume_ulid" gorm:"column:volume_ulid;primaryKey;size:40"` // 卷 ULID（VOL_ 前缀）
	ProjectULID string `json:"project_ulid" gorm:"column:project_ulid;size:40;index"`    // 所属项目
	VolumeName  string `json:"volume_name" gorm:"size:128"`                              // 卷名
	VolumeOrder int    `json:"volume_order"`                                             // 排序号，升序
	SummaryText string `json:"summary_text" gorm:"size:2048"`                            // 卷概要
}

// TableName 指定表名。
func (VolumeDO) TableName() string { return "volumes" }

// ProjectCreateREQ 创建项目请求。
type ProjectCreateREQ struct {
	ProjectName  string `json:"project_name" form:"project_name" binding:"required,min=1,max=128"` // 书名
	GenreCode    string `json:"genre_code" form:"genre_code" binding:"omitempty,max=32"`           // 题材编码
	StyleText    string `json:"style_text" form:"style_text" binding:"omitempty,max=1024"`         // 文风设定
	SynopsisText string `json:"synopsis_text" form:"synopsis_text" binding:"omitempty,max=4096"`   // 简介
}

// ProjectUpdateREQ 更新项目请求（整体覆盖可编辑字段）。
type ProjectUpdateREQ struct {
	ProjectULID   string `json:"project_ulid" form:"project_ulid" binding:"required"`                                   // 项目 ULID
	ProjectName   string `json:"project_name" form:"project_name" binding:"required,min=1,max=128"`                     // 书名
	GenreCode     string `json:"genre_code" form:"genre_code" binding:"omitempty,max=32"`                               // 题材编码
	StyleText     string `json:"style_text" form:"style_text" binding:"omitempty,max=1024"`                             // 文风设定
	SynopsisText  string `json:"synopsis_text" form:"synopsis_text" binding:"omitempty,max=4096"`                       // 简介
	ProjectStatus string `json:"project_status" form:"project_status" binding:"omitempty,oneof=draft serial completed"` // 状态
}

// ProjectListREQ 项目列表查询。
type ProjectListREQ struct {
	PageREQ
	KeywordText   string `json:"keyword_text" form:"keyword_text" binding:"omitempty,max=64"`                           // 名称模糊搜索
	ProjectStatus string `json:"project_status" form:"project_status" binding:"omitempty,oneof=draft serial completed"` // 状态过滤
}

// ProjectVO 项目视图对象。
type ProjectVO struct {
	ProjectULID    string `json:"project_ulid"`     // 项目 ULID
	ProjectName    string `json:"project_name"`     // 书名
	GenreCode      string `json:"genre_code"`       // 题材编码
	StyleText      string `json:"style_text"`       // 文风设定
	SynopsisText   string `json:"synopsis_text"`    // 简介
	ProjectStatus  string `json:"project_status"`   // 状态
	TotalWordCount int64  `json:"total_word_count"` // 累计字数
	VolumeCount    int    `json:"volume_count"`     // 卷数
	ChapterCount   int    `json:"chapter_count"`    // 章节数
	CreatedAt      string `json:"created_at"`       // 创建时间（RFC3339）
	UpdatedAt      string `json:"updated_at"`       // 更新时间（RFC3339）
}

// VolumeVO 卷视图对象。
type VolumeVO struct {
	VolumeULID  string `json:"volume_ulid"`  // 卷 ULID
	VolumeName  string `json:"volume_name"`  // 卷名
	VolumeOrder int    `json:"volume_order"` // 排序号
	SummaryText string `json:"summary_text"` // 卷概要
}

// ProjectDetailREQ 项目详情查询（GET query）。
type ProjectDetailREQ struct {
	ProjectULID string `json:"project_ulid" form:"project_ulid" binding:"required"` // 项目 ULID
}

// ProjectTreeREQ 项目目录树查询（GET query）。
type ProjectTreeREQ struct {
	ProjectULID string `json:"project_ulid" form:"project_ulid" binding:"required"` // 项目 ULID
}

// ProjectDeleteREQ 删除项目请求。
type ProjectDeleteREQ struct {
	ProjectULID string `json:"project_ulid" form:"project_ulid" binding:"required"` // 项目 ULID
}

// VolumeCreateREQ 创建卷请求。
type VolumeCreateREQ struct {
	ProjectULID string `json:"project_ulid" form:"project_ulid" binding:"required"`             // 所属项目 ULID
	VolumeName  string `json:"volume_name" form:"volume_name" binding:"required,min=1,max=128"` // 卷名
	VolumeOrder int    `json:"volume_order" form:"volume_order" binding:"omitempty,min=0"`      // 排序号，0 表示自动取项目内最大值 +1
	SummaryText string `json:"summary_text" form:"summary_text" binding:"omitempty,max=2048"`   // 卷概要
}

// VolumeUpdateREQ 更新卷请求（仅覆盖非空字段）。
type VolumeUpdateREQ struct {
	VolumeULID  string `json:"volume_ulid" form:"volume_ulid" binding:"required"`             // 卷 ULID
	VolumeName  string `json:"volume_name" form:"volume_name" binding:"omitempty,max=128"`    // 卷名
	VolumeOrder int    `json:"volume_order" form:"volume_order" binding:"omitempty,min=0"`    // 排序号，0 表示不调整
	SummaryText string `json:"summary_text" form:"summary_text" binding:"omitempty,max=2048"` // 卷概要
}

// VolumeDeleteREQ 删除卷请求。
type VolumeDeleteREQ struct {
	VolumeULID string `json:"volume_ulid" form:"volume_ulid" binding:"required"` // 卷 ULID
}

// ProjectTreeVO 项目目录树（卷 + 章节），供写作工作台使用。
type ProjectTreeVO struct {
	ProjectVO  ProjectVO          `json:"project_vo"`  // 项目信息
	VolumeList []VolumeTreeNodeVO `json:"volume_list"` // 卷节点列表
}

// VolumeTreeNodeVO 目录树的卷节点（含章节）。
type VolumeTreeNodeVO struct {
	VolumeVO    VolumeVO         `json:"volume_vo"`    // 卷信息
	ChapterList []ChapterBriefVO `json:"chapter_list"` // 章节列表
}

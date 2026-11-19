// 本文件定义创作规划的大纲节点与场景卡的持久化对象、请求与视图对象。

package domain

// 创作规划模块 ULID 前缀：大纲节点与详细场景卡片。
const (
	IDPrefixOutline = "OUT_" // 大纲节点（卷/篇章/情节块/章节）
	IDPrefixScene   = "SCN_" // 详细场景卡片
)

// OutlineNodeDO 创作规划大纲节点持久化对象（树形结构）。
type OutlineNodeDO struct {
	BaseDO
	OutlineULID       string `json:"outline_ulid" gorm:"column:outline_ulid;primaryKey;size:40"`                     // 节点 ULID（OUT_ 前缀）
	ProjectULID       string `json:"project_ulid" gorm:"column:project_ulid;size:40;index"`                          // 所属项目
	ParentOutlineULID string `json:"parent_outline_ulid" gorm:"column:parent_outline_ulid;size:40;index;default:''"` // 父节点 ULID，根节点为空串
	NodeTypeCode      string `json:"node_type_code" gorm:"column:node_type_code;size:32;index"`                      // 类型：volume/arc/storyBlock/chapter
	TitleText         string `json:"title_text" gorm:"column:title_text;size:128"`                                   // 节点标题
	SummaryText       string `json:"summary_text" gorm:"column:summary_text;size:4096"`                              // 节点概要
	NodeStatusCode    string `json:"node_status_code" gorm:"column:node_status_code;size:16;index"`                  // 状态：draft/planning/ready/done
	SortOrder         int    `json:"sort_order" gorm:"column:sort_order;index"`                                      // 同级排序号，升序
}

// TableName 指定大纲节点表名。
func (OutlineNodeDO) TableName() string { return "outline_nodes" }

// OutlineNodeREQ 大纲节点保存请求（有 outline_ulid 为更新，否则新建）。
type OutlineNodeREQ struct {
	OutlineULID       string `json:"outline_ulid" binding:"omitempty"`                                      // 节点 ULID，更新时必填
	ProjectULID       string `json:"project_ulid" binding:"required"`                                       // 所属项目
	ParentOutlineULID string `json:"parent_outline_ulid" binding:"omitempty"`                               // 父节点 ULID，根节点留空
	NodeTypeCode      string `json:"node_type_code" binding:"required,oneof=volume arc storyBlock chapter"` // 节点类型
	TitleText         string `json:"title_text" binding:"required,min=1,max=128"`                           // 节点标题
	SummaryText       string `json:"summary_text" binding:"omitempty,max=4096"`                             // 节点概要
	NodeStatusCode    string `json:"node_status_code" binding:"omitempty,oneof=draft planning ready done"`  // 节点状态
	SortOrder         int    `json:"sort_order"`                                                            // 排序号，0 表示追加到末尾
}

// OutlineNodeDeleteREQ 大纲节点删除请求。
type OutlineNodeDeleteREQ struct {
	ProjectULID string `json:"project_ulid" binding:"required"` // 所属项目
	OutlineULID string `json:"outline_ulid" binding:"required"` // 待删除节点 ULID
}

// OutlineTreeREQ 项目大纲树查询请求。
type OutlineTreeREQ struct {
	ProjectULID string `json:"project_ulid" binding:"required"` // 所属项目
}

// OutlineNodeVO 大纲节点视图（Children 递归承载子树）。
type OutlineNodeVO struct {
	OutlineULID       string          `json:"outline_ulid"`        // 节点 ULID
	ProjectULID       string          `json:"project_ulid"`        // 所属项目
	ParentOutlineULID string          `json:"parent_outline_ulid"` // 父节点 ULID
	NodeTypeCode      string          `json:"node_type_code"`      // 节点类型
	TitleText         string          `json:"title_text"`          // 节点标题
	SummaryText       string          `json:"summary_text"`        // 节点概要
	NodeStatusCode    string          `json:"node_status_code"`    // 节点状态
	SortOrder         int             `json:"sort_order"`          // 排序号
	UpdatedAtMs       int64           `json:"updated_at_ms"`       // 更新时间（毫秒时间戳）
	Children          []OutlineNodeVO `json:"children,omitempty"`  // 子节点（按排序号升序）
}

// DetailedSceneDO 详细场景卡片持久化对象，挂靠在某个大纲节点下。
type DetailedSceneDO struct {
	BaseDO
	SceneULID         string `json:"scene_ulid" gorm:"column:scene_ulid;primaryKey;size:40"`          // 场景 ULID（SCN_ 前缀）
	ProjectULID       string `json:"project_ulid" gorm:"column:project_ulid;size:40;index"`           // 所属项目
	OutlineULID       string `json:"outline_ulid" gorm:"column:outline_ulid;size:40;index"`           // 所属大纲节点
	CharacterULIDList string `json:"character_ulid_list" gorm:"column:character_ulid_list;type:text"` // 出场角色 ULID JSON 数组
	LocationText      string `json:"location_text" gorm:"column:location_text;size:256"`              // 场景地点
	ConflictText      string `json:"conflict_text" gorm:"column:conflict_text;type:text"`             // 核心冲突
	PaceCode          string `json:"pace_code" gorm:"column:pace_code;size:16;index"`                 // 节奏：slow/medium/fast/climax
	EstimatedWords    int64  `json:"estimated_words" gorm:"column:estimated_words"`                   // 预计字数
	EntryStateText    string `json:"entry_state_text" gorm:"column:entry_state_text;type:text"`       // 入场状态
	GoalText          string `json:"goal_text" gorm:"column:goal_text;type:text"`                     // 角色目标
	KeyChoiceText     string `json:"key_choice_text" gorm:"column:key_choice_text;type:text"`         // 关键选择
	OutcomeText       string `json:"outcome_text" gorm:"column:outcome_text;type:text"`               // 场景结果
	ExitStateText     string `json:"exit_state_text" gorm:"column:exit_state_text;type:text"`         // 出场状态
	SortOrder         int    `json:"sort_order" gorm:"column:sort_order;index"`                       // 场景排序号，升序
}

// TableName 指定详细场景表名。
func (DetailedSceneDO) TableName() string { return "detailed_scenes" }

// DetailedSceneREQ 详细场景保存请求（有 scene_ulid 为更新，否则新建）。
type DetailedSceneREQ struct {
	SceneULID         string   `json:"scene_ulid" binding:"omitempty"`                              // 场景 ULID，更新时必填
	ProjectULID       string   `json:"project_ulid" binding:"required"`                             // 所属项目
	OutlineULID       string   `json:"outline_ulid" binding:"required"`                             // 所属大纲节点
	CharacterULIDList []string `json:"character_ulid_list" binding:"omitempty,dive,max=40"`         // 出场角色 ULID 数组
	LocationText      string   `json:"location_text" binding:"omitempty,max=256"`                   // 场景地点
	ConflictText      string   `json:"conflict_text" binding:"omitempty,max=4096"`                  // 核心冲突
	PaceCode          string   `json:"pace_code" binding:"omitempty,oneof=slow medium fast climax"` // 节奏编码
	EstimatedWords    int64    `json:"estimated_words" binding:"omitempty,min=0"`                   // 预计字数
	EntryStateText    string   `json:"entry_state_text" binding:"omitempty,max=4096"`               // 入场状态
	GoalText          string   `json:"goal_text" binding:"omitempty,max=4096"`                      // 角色目标
	KeyChoiceText     string   `json:"key_choice_text" binding:"omitempty,max=4096"`                // 关键选择
	OutcomeText       string   `json:"outcome_text" binding:"omitempty,max=4096"`                   // 场景结果
	ExitStateText     string   `json:"exit_state_text" binding:"omitempty,max=4096"`                // 出场状态
	SortOrder         int      `json:"sort_order"`                                                  // 排序号，0 表示追加到末尾
}

// DetailedSceneDeleteREQ 详细场景删除请求。
type DetailedSceneDeleteREQ struct {
	ProjectULID string `json:"project_ulid" binding:"required"` // 所属项目
	SceneULID   string `json:"scene_ulid" binding:"required"`   // 待删除场景 ULID
}

// DetailedSceneListREQ 详细场景列表查询请求。
type DetailedSceneListREQ struct {
	ProjectULID string `json:"project_ulid" binding:"required"`  // 所属项目
	OutlineULID string `json:"outline_ulid" binding:"omitempty"` // 节点过滤，留空表示项目下全部场景
}

// DetailedSceneVO 详细场景视图对象。
type DetailedSceneVO struct {
	SceneULID         string   `json:"scene_ulid"`          // 场景 ULID
	ProjectULID       string   `json:"project_ulid"`        // 所属项目
	OutlineULID       string   `json:"outline_ulid"`        // 所属大纲节点
	CharacterULIDList []string `json:"character_ulid_list"` // 出场角色 ULID 数组
	LocationText      string   `json:"location_text"`       // 场景地点
	ConflictText      string   `json:"conflict_text"`       // 核心冲突
	PaceCode          string   `json:"pace_code"`           // 节奏编码
	EstimatedWords    int64    `json:"estimated_words"`     // 预计字数
	EntryStateText    string   `json:"entry_state_text"`    // 入场状态
	GoalText          string   `json:"goal_text"`           // 角色目标
	KeyChoiceText     string   `json:"key_choice_text"`     // 关键选择
	OutcomeText       string   `json:"outcome_text"`        // 场景结果
	ExitStateText     string   `json:"exit_state_text"`     // 出场状态
	SortOrder         int      `json:"sort_order"`          // 排序号
	UpdatedAtMs       int64    `json:"updated_at_ms"`       // 更新时间（毫秒时间戳）
}

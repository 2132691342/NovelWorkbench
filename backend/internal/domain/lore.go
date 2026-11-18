package domain

// LoreDO 世界观设定条目持久化对象（世界规则、地点、物品、事件等）。
type LoreDO struct {
	BaseDO
	LoreULID     string `json:"lore_ulid" gorm:"column:lore_ulid;primaryKey;size:40"`  // 设定 ULID（LOR_ 前缀）
	ProjectULID  string `json:"project_ulid" gorm:"column:project_ulid;size:40;index"` // 所属项目
	LoreName     string `json:"lore_name" gorm:"size:128"`                             // 设定名
	LoreTypeCode string `json:"lore_type_code" gorm:"size:16;index"`                   // 类型：world/rule/item/place/event/note（note=创作笔记）
	ContentText  string `json:"content_text" gorm:"type:text"`                         // 设定内容
	SortOrder    int    `json:"sort_order"`                                            // 排序号
}

// TableName 指定表名。
func (LoreDO) TableName() string { return "lore_settings" }

// LoreREQ 创建/更新设定请求（Create 时 LoreULID 置空）。
type LoreREQ struct {
	LoreULID     string `json:"lore_ulid" binding:"omitempty"`                                             // 设定 ULID，更新时必填
	ProjectULID  string `json:"project_ulid" binding:"required"`                                           // 所属项目
	LoreName     string `json:"lore_name" binding:"required,min=1,max=128"`                                // 设定名
	LoreTypeCode string `json:"lore_type_code" binding:"omitempty,oneof=world rule item place event note"` // 类型
	ContentText  string `json:"content_text" binding:"omitempty,max=20000"`                                // 内容
	SortOrder    int    `json:"sort_order"`                                                                // 排序号
}

// LoreListREQ 设定列表查询。
type LoreListREQ struct {
	ProjectULID  string `json:"project_ulid" binding:"required"`                                           // 所属项目
	LoreTypeCode string `json:"lore_type_code" binding:"omitempty,oneof=world rule item place event note"` // 类型过滤
	KeywordText  string `json:"keyword_text" binding:"omitempty,max=64"`                                   // 名称/内容模糊搜索
}

// LoreDeleteREQ 删除设定请求。
type LoreDeleteREQ struct {
	LoreULID string `json:"lore_ulid" form:"lore_ulid" binding:"required"` // 设定 ULID
}

// LoreVO 设定视图对象。
type LoreVO struct {
	LoreULID     string `json:"lore_ulid"`      // 设定 ULID
	ProjectULID  string `json:"project_ulid"`   // 所属项目
	LoreName     string `json:"lore_name"`      // 设定名
	LoreTypeCode string `json:"lore_type_code"` // 类型
	ContentText  string `json:"content_text"`   // 内容
	SortOrder    int    `json:"sort_order"`     // 排序号
	UpdatedAtMs  int64  `json:"updated_at_ms"`  // 更新时间（毫秒时间戳）
}

// 本文件定义角色卡、角色关系边及关系图谱的持久化对象、请求与视图对象。

package domain

// CharacterDO 角色卡持久化对象。
type CharacterDO struct {
	BaseDO
	CharacterULID   string `json:"character_ulid" gorm:"column:character_ulid;primaryKey;size:40"` // 角色 ULID（CHR_ 前缀）
	ProjectULID     string `json:"project_ulid" gorm:"column:project_ulid;size:40;index"`          // 所属项目
	CharacterName   string `json:"character_name" gorm:"size:64"`                                  // 姓名
	AliasText       string `json:"alias_text" gorm:"size:128"`                                     // 别名
	RoleTypeCode    string `json:"role_type_code" gorm:"size:16"`                                  // 角色类型：protagonist/antagonist/supporting/minor
	PersonalityText string `json:"personality_text" gorm:"size:2048"`                              // 性格
	BackgroundText  string `json:"background_text" gorm:"size:4096"`                               // 背景经历
	AppearanceText  string `json:"appearance_text" gorm:"size:2048"`                               // 外貌
	GoalText        string `json:"goal_text" gorm:"size:2048"`                                     // 目标动机
}

// TableName 指定表名。
func (CharacterDO) TableName() string { return "characters" }

// CharacterRelationDO 角色关系边（图谱数据）。
type CharacterRelationDO struct {
	BaseDO
	RelationULID      string `json:"relation_ulid" gorm:"column:relation_ulid;primaryKey;size:40"`        // 关系 ULID（RLT_ 前缀）
	ProjectULID       string `json:"project_ulid" gorm:"column:project_ulid;size:40;index"`               // 所属项目
	FromCharacterULID string `json:"from_character_ulid" gorm:"column:from_character_ulid;size:40;index"` // 起点角色
	ToCharacterULID   string `json:"to_character_ulid" gorm:"column:to_character_ulid;size:40;index"`     // 终点角色
	RelationTypeCode  string `json:"relation_type_code" gorm:"size:32"`                                   // 关系类型：friend/enemy/lover/kin/teacher...
	RelationNote      string `json:"relation_note" gorm:"size:256"`                                       // 关系说明
}

// TableName 指定表名。
func (CharacterRelationDO) TableName() string { return "character_relations" }

// CharacterCreateREQ 创建/更新角色请求（Create 复用，ULID 置空）。
type CharacterREQ struct {
	CharacterULID   string `json:"character_ulid" binding:"omitempty"`                                               // 角色 ULID，更新时必填
	ProjectULID     string `json:"project_ulid" binding:"required"`                                                  // 所属项目
	CharacterName   string `json:"character_name" binding:"required,min=1,max=64"`                                   // 姓名
	AliasText       string `json:"alias_text" binding:"omitempty,max=128"`                                           // 别名
	RoleTypeCode    string `json:"role_type_code" binding:"omitempty,oneof=protagonist antagonist supporting minor"` // 角色类型
	PersonalityText string `json:"personality_text" binding:"omitempty,max=2048"`                                    // 性格
	BackgroundText  string `json:"background_text" binding:"omitempty,max=4096"`                                     // 背景经历
	AppearanceText  string `json:"appearance_text" binding:"omitempty,max=2048"`                                     // 外貌
	GoalText        string `json:"goal_text" binding:"omitempty,max=2048"`                                           // 目标动机
}

// CharacterVO 角色视图对象。
type CharacterVO struct {
	CharacterULID   string `json:"character_ulid"`   // 角色 ULID
	ProjectULID     string `json:"project_ulid"`     // 所属项目
	CharacterName   string `json:"character_name"`   // 姓名
	AliasText       string `json:"alias_text"`       // 别名
	RoleTypeCode    string `json:"role_type_code"`   // 角色类型
	PersonalityText string `json:"personality_text"` // 性格
	BackgroundText  string `json:"background_text"`  // 背景经历
	AppearanceText  string `json:"appearance_text"`  // 外貌
	GoalText        string `json:"goal_text"`        // 目标动机
}

// CharacterListREQ 角色列表查询。
type CharacterListREQ struct {
	ProjectULID  string `json:"project_ulid" form:"project_ulid" binding:"required"`                                                    // 所属项目
	RoleTypeCode string `json:"role_type_code" form:"role_type_code" binding:"omitempty,oneof=protagonist antagonist supporting minor"` // 类型过滤
	KeywordText  string `json:"keyword_text" form:"keyword_text" binding:"omitempty,max=64"`                                            // 姓名/别名模糊搜索
}

// CharacterDeleteREQ 删除角色请求（连带删关系边）。
type CharacterDeleteREQ struct {
	CharacterULID string `json:"character_ulid" form:"character_ulid" binding:"required"` // 角色 ULID
}

// CharacterRelationDeleteREQ 删除角色关系边请求。
type CharacterRelationDeleteREQ struct {
	RelationULID string `json:"relation_ulid" form:"relation_ulid" binding:"required"` // 关系 ULID
}

// CharacterRelationREQ 创建角色关系请求。
type CharacterRelationREQ struct {
	ProjectULID       string `json:"project_ulid" binding:"required"`              // 所属项目
	FromCharacterULID string `json:"from_character_ulid" binding:"required"`       // 起点角色
	ToCharacterULID   string `json:"to_character_ulid" binding:"required"`         // 终点角色
	RelationTypeCode  string `json:"relation_type_code" binding:"required,max=32"` // 关系类型
	RelationNote      string `json:"relation_note" binding:"omitempty,max=256"`    // 关系说明
}

// CharacterGraphVO 角色关系图（AntV X6 消费的节点+边）。
type CharacterGraphVO struct {
	NodeList []CharacterNodeVO `json:"node_list"` // 节点列表
	EdgeList []CharacterEdgeVO `json:"edge_list"` // 边列表
}

// CharacterNodeVO 关系图节点。
type CharacterNodeVO struct {
	CharacterULID string `json:"character_ulid"` // 角色 ULID
	CharacterName string `json:"character_name"` // 姓名
	RoleTypeCode  string `json:"role_type_code"` // 角色类型（决定节点配色）
}

// CharacterEdgeVO 关系图边。
type CharacterEdgeVO struct {
	RelationULID      string `json:"relation_ulid"`       // 关系 ULID
	FromCharacterULID string `json:"from_character_ulid"` // 起点角色
	ToCharacterULID   string `json:"to_character_ulid"`   // 终点角色
	RelationTypeCode  string `json:"relation_type_code"`  // 关系类型
	RelationNote      string `json:"relation_note"`       // 关系说明
}

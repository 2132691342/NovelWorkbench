// 本文件定义 AI 开书（创意 → 项目结构方案）的请求、方案与结果视图对象。

package domain

// ProjectBootstrapREQ AI 开书生成方案请求：一句创意 + 可选题材与目标章数。
type ProjectBootstrapREQ struct {
	IdeaText           string `json:"idea_text" binding:"required,min=6,max=2000"`             // 一句创意或题材描述
	GenreCode          string `json:"genre_code" binding:"omitempty,max=32"`                   // 题材编码，留空由模型判断
	TargetChapterCount int    `json:"target_chapter_count" binding:"omitempty,min=3,max=60"`   // 期望章节数，留空默认 12
	ModelULID          string `json:"model_ulid" binding:"omitempty,max=40"`                   // 指定模型 ULID，留空用当前账号默认模型
}

// BootstrapChapterVO 开书方案中的章节条目。
type BootstrapChapterVO struct {
	ChapterName   string `json:"chapter_name"`   // 章节名
	SummaryText   string `json:"summary_text"`   // 本章要发生什么、章末停在哪
	ChapterOrder  int    `json:"chapter_order"`  // 章节序号，从 1 开始
}

// BootstrapVolumeVO 开书方案中的卷条目（含章节）。
type BootstrapVolumeVO struct {
	VolumeName  string               `json:"volume_name"`  // 卷名
	SummaryText string               `json:"summary_text"` // 本卷从哪到哪、卷末翻什么牌
	VolumeOrder int                  `json:"volume_order"` // 卷序号，从 1 开始
	ChapterList []BootstrapChapterVO `json:"chapter_list"` // 章节列表
}

// BootstrapCharacterVO 开书方案中的角色卡条目。
type BootstrapCharacterVO struct {
	CharacterName   string `json:"character_name"`   // 姓名
	RoleTypeCode    string `json:"role_type_code"`   // 角色类型：protagonist/antagonist/supporting/minor
	PersonalityText string `json:"personality_text"` // 性格
	GoalText        string `json:"goal_text"`        // 目标动机
	BackgroundText  string `json:"background_text"`  // 背景经历
}

// BootstrapLoreVO 开书方案中的世界观设定条目。
type BootstrapLoreVO struct {
	LoreName     string `json:"lore_name"`      // 设定名
	LoreTypeCode string `json:"lore_type_code"` // 类型：world/rule/item/place/event
	ContentText  string `json:"content_text"`   // 规则卡正文（规则 / 边界 / 代价）
}

// ProjectBootstrapPlanVO 开书方案（预览与落库共用同一结构，前端可直接编辑后提交）。
type ProjectBootstrapPlanVO struct {
	ProjectName     string                `json:"project_name"`      // 书名
	GenreCode       string                `json:"genre_code"`        // 题材编码
	SellingPointText string               `json:"selling_point_text"` // 卖点：一个什么人，因为什么事，必须做到什么
	SynopsisText    string                `json:"synopsis_text"`     // 简介
	StyleText       string                `json:"style_text"`        // 文风设定
	VolumeList      []BootstrapVolumeVO   `json:"volume_list"`       // 卷与章
	CharacterList   []BootstrapCharacterVO `json:"character_list"`   // 角色卡
	LoreList        []BootstrapLoreVO     `json:"lore_list"`         // 世界观设定
}

// ProjectBootstrapApplyREQ 按方案落库请求（plan 由前端回传，允许人工改动）。
type ProjectBootstrapApplyREQ struct {
	Plan ProjectBootstrapPlanVO `json:"plan" binding:"required"` // 开书方案
}

// ProjectBootstrapResultVO 开书落库结果：项目视图 + 各类资产数量。
type ProjectBootstrapResultVO struct {
	ProjectVO     ProjectVO `json:"project_vo"`     // 创建出的项目
	VolumeCount   int       `json:"volume_count"`   // 卷数
	ChapterCount  int       `json:"chapter_count"`  // 章节数
	CharacterCount int      `json:"character_count"` // 角色数
	LoreCount     int       `json:"lore_count"`     // 设定数
}

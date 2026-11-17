package ai

import (
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// builtinSkillList 内置斜杠命令资产（启动时播种进 slash_commands 表，BuiltinFlag=true 不可删）。
var builtinSkillList = []domain.SlashCommandDO{
	{
		CommandCode:     "continue",
		CommandName:     "续写",
		CategoryCode:    "write",
		DescriptionText: "依据当前章节与上下文续写下一段正文",
		SortOrder:       10,
		PromptTemplate: `请续写下面的小说正文，从我给出的位置直接往下写 500 字左右。
要求：衔接自然、保持既有文风与人物口吻、推进情节、不要重复已有内容、只输出续写正文。

【上下文】
{{context}}

【当前正文】
{{chapter_content}}

【续写要求】
{{user_input}}`,
	},
	{
		CommandCode:     "outline",
		CommandName:     "生成大纲",
		CategoryCode:    "outline",
		DescriptionText: "根据需求生成章节大纲或情节梗概",
		SortOrder:       20,
		PromptTemplate: `你是小说策划，请根据需求输出章节大纲。
要求：分条列出情节节点（起承转合），每条一句话，标注涉及角色与预期字数，只输出大纲本身。

【上下文】
{{context}}

【策划需求】
{{user_input}}`,
	},
	{
		CommandCode:     "polish",
		CommandName:     "润色",
		CategoryCode:    "revise",
		DescriptionText: "润色选中文字：更流畅、更有画面感",
		SortOrder:       30,
		PromptTemplate: `请润色下面的文字：保持原意与人称视角，提升流畅度与画面感，去除翻译腔与重复用词，只输出润色后的文字。

【原文】
{{chapter_content}}

【润色侧重】
{{user_input}}`,
	},
	{
		CommandCode:     "expand",
		CommandName:     "扩写",
		CategoryCode:    "revise",
		DescriptionText: "扩写选中文字：补充细节与心理描写",
		SortOrder:       40,
		PromptTemplate: `请扩写下面的文字：补充动作、神态、环境与心理细节，让场景更饱满，保持文风一致，只输出扩写后的文字。

【原文】
{{chapter_content}}

【扩写侧重】
{{user_input}}`,
	},
	{
		CommandCode:     "condense",
		CommandName:     "缩写",
		CategoryCode:    "revise",
		DescriptionText: "缩写选中文字：保留关键信息更紧凑",
		SortOrder:       50,
		PromptTemplate: `请缩写下面的文字：删除冗余与重复，保留关键情节、信息与人物表现，节奏更紧凑，只输出缩写后的文字。

【原文】
{{chapter_content}}

【缩写要求】
{{user_input}}`,
	},
	{
		CommandCode:     "summary",
		CommandName:     "生成梗概",
		CategoryCode:    "outline",
		DescriptionText: "为章节或选中文字生成一句话到一段话的梗概",
		SortOrder:       60,
		PromptTemplate: `请为下面的内容生成梗概：先一句话总述，再用 3 句话分述主要情节与结果，只输出梗概本身。

【内容】
{{chapter_content}}

【梗概要求】
{{user_input}}`,
	},
	{
		CommandCode:     "review",
		CommandName:     "审稿",
		CategoryCode:    "revise",
		DescriptionText: "审阅章节：指出逻辑、节奏、一致性问题",
		SortOrder:       70,
		PromptTemplate: `你是小说责编，请审阅下面的章节内容。
要求：按【逻辑硬伤 / 节奏问题 / 人物一致性 / 用词建议】四类分条列出问题与修改建议，不重写全文，只输出审阅意见。

【上下文】
{{context}}

【章节内容】
{{chapter_content}}

【审阅侧重】
{{user_input}}`,
	},
	{
		CommandCode:     "name",
		CommandName:     "取名",
		CategoryCode:    "setting",
		DescriptionText: "为角色、地名、功法等生成候选名字",
		SortOrder:       80,
		PromptTemplate: `请为小说生成候选名字。
要求：给出 10 个候选，每个附一行寓意说明，贴合题材与世界观，只输出候选列表。

【上下文】
{{context}}

【取名对象与要求】
{{user_input}}`,
	},
	{
		CommandCode:     "setting",
		CommandName:     "设定问答",
		CategoryCode:    "setting",
		DescriptionText: "基于世界观设定与知识库回答设定问题",
		SortOrder:       90,
		PromptTemplate: `你是小说设定管理员，请严格依据给定设定回答问题；设定中没有的，明确回答"设定中未提及"，不要编造。

【设定与资料】
{{context}}

【问题】
{{user_input}}`,
	},
}

// BuiltinSkills 返回内置斜杠命令定义（只读副本）。
func BuiltinSkills() []domain.SlashCommandDO {
	skills := make([]domain.SlashCommandDO, len(builtinSkillList))
	copy(skills, builtinSkillList)
	return skills
}

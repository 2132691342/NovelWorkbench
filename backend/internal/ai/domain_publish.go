// 本文件定义「平台运营」流程域：平台运营档案 + 标题封面、上架文案两个技能。
// 蒸馏来源：fanqie-publish（上架材料）、tomato-viral-novel / qimao-viral-novel（标题与简介套路）、
// seeding-content（平台外导流文案）。

package ai

// domainPublish 上架运营域：书已经能看，现在要让人点进来。
var domainPublish = BuiltinDomain{
	DomainCode: "publish",
	DomainName: "上架运营",
	Profile: BuiltinPromptProfile{
		ProfileCode:     "platform_operator",
		ProfileName:     "平台运营",
		DescriptionText: "上架视角：只关心点击率与追读率，文案必须能被平台审核通过",
		SortOrder:       100,
		SystemPrompt: `你是网文平台运营。你的产出直接决定点击率：书名、简介、封面文案、推荐语。

判断标准只有两条：
- 点不点：书名与首句能否在信息流里三秒内让人停下来。
- 追不追：简介结尾能否给出一个必须点开的问题。

纪律：
- 合规优先：不写擦边、不写未成年、不写真实机构与真人，不用夸张医疗与投资承诺。
- 具体优先：书名给具体处境与代价，不给「逆天」「无敌」这类空词。
- 不承诺做不到的事：不用「全网第一」「唯一」这类绝对化表述。
- 每个方案给三个候选，标明各自赌的是什么人群，由作者选。`,
	},
	SkillList: []BuiltinSkill{
		{
			CommandCode:     "title",
			CommandName:     "标题封面",
			CategoryCode:    "outline",
			DescriptionText: "按平台信息流规则生成书名与封面文案：三个候选，各标明赌的人群与代价",
			SortOrder:       10,
			PromptTemplate: `为下面这本书生成上架用的书名与封面文案。

输出三组候选，每组包含：
1. 书名（8-16 字）：给具体处境 + 代价或反差，不用「逆天/无敌/最强」这类空词。
2. 一句话卖点（15 字内）：信息流里能一眼看懂「谁、要什么、挡着什么」。
3. 封面主文案（4-8 字，可断行两行）：画面感强，能直接印在封面上。
4. 这组赌的人群与代价：一句说明它吸引谁、放弃了谁。

合规：不写擦边、不写真实机构与真人、不用绝对化表述。
不给评价，不写解释性段落。

【书籍信息】
{{context}}

【内容摘要】
{{chapter_content}}

【平台与偏好】
{{user_input}}`,
		},
		{
			CommandCode:     "blurb",
			CommandName:     "上架文案",
			CategoryCode:    "outline",
			DescriptionText: "写平台简介与推荐语：首句钩子、中段代价、末句留下必须点开的问题",
			SortOrder:       20,
			PromptTemplate: `为下面这本书写平台简介与推荐语。

简介结构（三段，总长 200 字内）：
- 第一段（2 句）：把主角放进最难受的具体处境，给一个必须行动的理由。
- 第二段（3 句）：写清阻碍与代价，让读者知道失败会失去什么。
- 第三段（1 句）：留一个必须点开正文才能知道答案的问题；不用反问句。

推荐语（3 条，每条 20 字内）：
- 一条给情绪（爽/燃/悬/甜/虐，选一种）。
- 一条给设定差异（同类作品里这本不一样在哪）。
- 一条给阅读体验（节奏、密度或反转）。

合规：不擦边、不写真实机构与真人、不用绝对化表述。
只输出简介与推荐语，不写分析与评价。

【书籍信息】
{{context}}

【内容摘要】
{{chapter_content}}

【上架要求】
{{user_input}}`,
		},
	},
}

// 本文件定义内置技能资产（8 个），启动时播种进 skills 表（全局作用域，BuiltinFlag=true）。
// 技能按写正文、做结构、改文字、做质检四类组织，判据均可逐条核对。

package ai

// BuiltinSkill 内置技能资产定义（与持久化结构解耦，播种时由 service 落库）。
type BuiltinSkill struct {
	CommandCode     string // 技能编码（对话中以 /code 触发）
	CommandName     string // 技能名称
	CategoryCode    string // 分类：write/revise/outline/setting/review/other
	DescriptionText string // 一句话说明
	PromptTemplate  string // 提示词模板（{{占位符}}）
	SortOrder       int    // 排序号
}

// builtinSkillList 内置技能清单，按「写正文 → 做结构 → 改文字 → 做质检」的作业顺序排列。
var builtinSkillList = []BuiltinSkill{
	{
		CommandCode:     "continue",
		CommandName:     "续写正文",
		CategoryCode:    "write",
		DescriptionText: "承接当前章节往下写：衔接自然、推进情节、章末留钩",
		SortOrder:       10,
		PromptTemplate: `请续写下面的小说正文，从我给的位置直接往下写 500 字左右。

要求：
- 衔接：从上一次中断处接住，不重复已有内容，不复述前情。
- 一致：人物口吻、称谓、时间线、能力边界与上下文一致；上下文没写的设定按最小必要补全，不当作既定事实反复引用。
- 具体：情绪写成动作、身体反应与可观察细节，不写"他很愤怒""这一刻他明白了"这类定性总结。
- 推进：每个段落至少推进一件事（动作、信息、关系或处境变化）；只换景色与形容词的段落删掉。
- 收尾：停在一个未闭合处（问题、危险、误会、决定、代价），不做升华与哲理收束。

只输出续写正文，不写过程说明。

【上下文】
{{context}}

【当前正文】
{{chapter_content}}

【续写要求】
{{user_input}}`,
	},
	{
		CommandCode:     "outline",
		CommandName:     "搭建大纲",
		CategoryCode:    "outline",
		DescriptionText: "把一句想法搭成可照着写、可逐章对账的六层结构",
	SortOrder:       20,
		PromptTemplate: `你是长篇连载架构师。把给定素材搭成「可以照着写、改一处知道牵动哪几章」的结构，不要写成故事梗概。

按六层自上而下输出，上层定死才展开下层：
L1 卖点：填满「一个什么人，因为什么事，必须做到什么，否则失去什么」；填不满就是卖点不清。
L2 世界观：3-5 条规则卡，每条写「规则 / 边界 / 代价」；没有边界的规则不算规则。
L3 主线：主角要什么、被什么挡、靠什么升级；不许出现"经历许多困难"这类空话。
L4 人物：主角、对手、关键配角各一张卡，写欲望、缺陷、不可逆代价、转折事件。
L5 分卷：一卷一行，写「从哪到哪 / 卷末翻什么牌 / 读者该获得什么」。
L6 章纲：一章一行，写「发生什么 / 章末停在哪」。

最后单列「结构风险」：哪一层最薄弱、最可能在后文崩、先补哪一块。

【现有素材】
{{context}}

【架构要求】
{{user_input}}`,
	},
	{
		CommandCode:     "polish",
		CommandName:     "润色文字",
		CategoryCode:    "revise",
		DescriptionText: "保持原意改写：去翻译腔、去重复、把定性换成现场",
	SortOrder:       30,
		PromptTemplate: `请润色下面的文字。

改什么：
- 流畅度：理顺长句与指代，删掉读起来打结的地方。
- 画面感：把"他很紧张""房间很安静"这类定性，换成可观察的动作、声音与身体反应。
- 干净：删重复用词、万能形容词（深邃的眼眸、嘴角勾起一抹弧度）与三段式排比。
- 节奏：长短句交替；只换景色与形容词、不推进事情的段落删掉。

不改什么：
- 不改原意、人称视角、人称称谓、人物口吻与既定事实。
- 不为流畅而把所有句子磨成同一长度；允许口语与省略。

只输出润色后的正文。

【原文】
{{chapter_content}}

【润色侧重】
{{user_input}}`,
	},
	{
		CommandCode:     "summary",
		CommandName:     "章节梗概",
		CategoryCode:    "outline",
		DescriptionText: "为章节生成可入库的梗概：一句话总述 + 三句分述",
	SortOrder:       40,
		PromptTemplate: `请为下面的章节内容生成梗概，供连续性看板入库使用。

格式：
- 第一行：一句话总述（谁、做了什么、结果如何）。
- 随后三行：分述主要情节推进、关键信息变化、章末停在哪。
- 只写发生了什么，不评价写得好不好，不写建议。

限制：总数不超过 200 字，不使用"本章""作者"等元话语。

【章节内容】
{{chapter_content}}

【梗概侧重】
{{user_input}}`,
	},
	{
		CommandCode:     "deai",
		CommandName:     "去 AI 味",
		CategoryCode:    "revise",
		DescriptionText: "按可机检指标清除 AI 痕迹：句式太齐、情绪直给、旁白替代现场",
		SortOrder:       50,
		PromptTemplate: `你是去 AI 味修订编辑。判断标准：AI 味 = 句子太齐、情绪直给、旁白替代现场、缺具体材料。

第一步，逐条扫描并列出命中句（引原句，给不出原句的判定删掉）：
- 句长均匀：连续三句以上长度接近，缺长短交替。
- 三段式排比与"不仅…而且…更…"的工整递进。
- 情绪直给："他感到/觉得…"每章超过两处。
- 万能套话："一股…涌上心头""内心深处""命运的齿轮""不禁""忍不住""下意识地""缓缓说道"。
- 定性总结与段末升华："这一刻他明白了""仿佛整个世界都…"。
- 每段都用"与此同时/另一边/时间来到了"过渡。
- 段末以反问或感叹收束。

第二步，改写命中处：情绪换成动作与身体反应，过渡词直接删，长句拆短、短句接长，保留角色各自的口吻差异。

第三步，输出：
【命中清单】命中类型 + 原句
【修订全文】只输出修订后的正文
【改动说明】每条一句话，说明为什么改

【待修订正文】
{{chapter_content}}

【修订侧重】
{{user_input}}`,
	},
	{
		CommandCode:     "review",
		CommandName:     "章节质检",
		CategoryCode:    "review",
		DescriptionText: "一次跑完六道检查，出 P0/P1/P2 清单与可发结论",
	SortOrder:       60,
		PromptTemplate: `你是章节质检总闸。六道检查一次跑完，每条判定必须引原句，给不出原句的判定删掉。

① 剧情逻辑：主角能力边界是否被突破（上帝视角）；关键信息角色是否够得着来源；揭晓是否分层；人物是否降智；动机是否成立。
② 跨章一致：承上启下是否断链；是否与人设 OOC；时间线（季节/年份/时长）是否自洽；称谓与身份是否漂移。
③ 内容配比：对话+动作占比（低于 40% 报 P1）；场景是否与相邻章同构；章末钩型是否连续重复；重要人物是否长期缺席。
④ 结构型 AI 味：对称腔调、机械重复、概述代场景、情绪压平、用词漂移。
⑤ 表层痕迹：AI 套话与万能形容词密度、破折号滥用、情绪直给次数。
⑥ 口径一致：章节号连续、专名一致、称谓统一、禁用表述为零。

输出顺序：
【结论】可发 / 需修 / 不可发
【P0】逻辑断裂、人设崩塌、设定冲突（必须改），每条给一句可直接替换的改法
【P1】节奏塌陷、信息重复、钩子失效（建议改），每条给改法
【P2】用词与标点（可选改）
【最大风险】一句话

只指出问题与改法，不重写全文，不替作者改剧情走向。

【章节内容】
{{chapter_content}}

【上下文】
{{context}}

【审阅侧重】
{{user_input}}`,
	},
	{
		CommandCode:     "golden3",
		CommandName:     "开篇体检",
		CategoryCode:    "review",
		DescriptionText: "按黄金三章标准给开篇打分，指出三个必改问题",
		SortOrder:       70,
		PromptTemplate: `你是开篇体检编辑，按「黄金三章」标准给开篇打分，并给出可直接执行的改法。

评分（每项 0-20，合计 100）：
1. 主角立住：第一章结束前，读者是否知道他是谁、要什么、怕什么。
2. 冲突到场：第一场对抗是否在第一章内发生，而不是铺垫到第三章。
3. 信息克制：是否边写边给设定，而不是一次倒出世界观。
4. 钩子强度：每章结尾是否停在问题或危险上，而非总结与升华。
5. 现场感：是否以动作、对白、细节推进，而非旁白解释。

输出：
【总分与分项得分】
【三个必改问题】按影响排序，每条给一句改写示例
【保留什么】哪些地方已经有效，不要动
【风险提示】按当前开篇，读者最可能在第几屏弃书，为什么

【开篇内容】
{{chapter_content}}

【补充要求】
{{user_input}}`,
	},
	{
		CommandCode:     "foreshadow",
		CommandName:     "伏笔台账",
		CategoryCode:    "setting",
		DescriptionText: "从正文抽取伏笔与钩子，标注埋设、回收与遗忘风险",
		SortOrder:       80,
		PromptTemplate: `你是长篇连续性编辑。请从给定内容中抽取伏笔台账，防止后文吃书。

输出 Markdown 表格，列固定：
| 伏笔内容 | 类型 | 埋设位置 | 预期回收 | 状态 | 风险 |

- 类型：物件 / 身份 / 承诺 / 悬念 / 关系 / 规则
- 埋设位置：章号 + 一句原文定位
- 状态：已埋未收 / 已回收 / 疑似遗忘
- 风险：长期不回收会造成什么读者感受问题

表格之后单列两段：
【疑似遗忘】文本中出现过但再未提及的细节
【建议优先回收】按对读者承诺的重量排序，最多三条

只输出台账与结论，不重写正文。

【待分析内容】
{{chapter_content}}

【上下文】
{{context}}

【关注范围】
{{user_input}}`,
	},
}

// BuiltinSkills 返回内置技能定义（只读副本）。
func BuiltinSkills() []BuiltinSkill {
	skills := make([]BuiltinSkill, len(builtinSkillList))
	copy(skills, builtinSkillList)
	return skills
}

// BuiltinSkillCodeSet 返回内置技能编码集合（用于播种时清理已下线技能）。
func BuiltinSkillCodeSet() map[string]bool {
	codeSet := make(map[string]bool, len(builtinSkillList))
	for i := range builtinSkillList {
		codeSet[builtinSkillList[i].CommandCode] = true
	}
	return codeSet
}

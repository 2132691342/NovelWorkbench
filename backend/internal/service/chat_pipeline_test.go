// AI 对话链路的场景回归：技能解析 → 上下文装配 → 消息成型 → 流式事件契约。
// 四者是同一次请求的上下游，拆开测会漏掉环节间的因果（作用域覆盖影响谁、
// 预算裁剪丢哪些块、事件顺序影响前端展示），因此合并到本文件按链路组织。

package service

import (
	"strings"
	"testing"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// skillDO 构造测试用技能行。
func skillDO(ulid, scope, projectULID, code string, enabled bool) domain.SkillDO {
	return domain.SkillDO{
		SkillULID: ulid, ScopeCode: scope, ProjectULID: projectULID,
		SkillCode: code, EnabledFlag: enabled,
	}
}

// chatMessageOf 构造测试用聊天消息。
func chatMessageOf(ulid, roleCode, contentText string) domain.ChatMessageDO {
	return domain.ChatMessageDO{MessageULID: ulid, RoleCode: roleCode, ContentText: contentText}
}

// findBlock 按块键查找上下文块。
func findBlock(blocks []ai.ContextBlock, blockKey string) *ai.ContextBlock {
	for i := range blocks {
		if blocks[i].BlockKey == blockKey {
			return &blocks[i]
		}
	}
	return nil
}

// eventSink 收集 SSE 事件的测试替身。
type eventSink struct{ eventList []domain.ChatStreamEventDTO }

// Event 记录一条事件。
func (s *eventSink) Event(dto domain.ChatStreamEventDTO) error {
	s.eventList = append(s.eventList, dto)
	return nil
}

// Ping 记录一次心跳。
func (s *eventSink) Ping() error { return nil }

// TestSkillResolutionChain 覆盖一次请求里技能从「命中」到「可渲染」的完整解析链路：
// 编码解析 → 作用域优先级 → 变量取值 → 模板占位符校验 → 参考资料命中与预算 → 全局覆盖标记。
func TestSkillResolutionChain(t *testing.T) {
	// 步骤1：编码解析。显式编码优先于正文首 token，正文命令不在开头时不命中。
	cases := []struct {
		caseName   string
		explicit   string
		content    string
		wantCode   string
		wantRemain string
	}{
		{"正文命令", "", "/continue 以林澈视角", "continue", "以林澈视角"},
		{"空白归一", "", "  /polish \t 加强画面感  \n", "polish", "加强画面感"},
		{"显式优先且正文不动", "polish", "/continue 以林澈视角", "polish", "/continue 以林澈视角"},
		{"无命令", "", "普通的正文开头，不该命中", "", "普通的正文开头，不该命中"},
		{"命令不在开头", "", "续写 /continue", "", "续写 /continue"},
	}
	for _, testCase := range cases {
		code, remain := resolveSkillCode(testCase.explicit, testCase.content)
		if code != testCase.wantCode || remain != testCase.wantRemain {
			t.Fatalf("resolveSkillCode = %q %q, want %q %q", code, remain, testCase.wantCode, testCase.wantRemain)
		}
	}

	// 步骤2：作用域优先级。项目技能胜过全局；项目技能停用回退全局；全局停用则不命中。
	project := skillDO("SKL_P", domain.SkillScopeProject, "PRJ_1", "polish", true)
	global := skillDO("SKL_G", domain.SkillScopeGlobal, "", "polish", true)
	disabledProject := skillDO("SKL_P", domain.SkillScopeProject, "PRJ_1", "polish", false)
	disabledGlobal := skillDO("SKL_G", domain.SkillScopeGlobal, "", "polish", false)

	if got := pickSkillByScope(&project, &global); got.SkillULID != "SKL_P" {
		t.Fatalf("项目技能应优先: %s", got.SkillULID)
	}
	if got := pickSkillByScope(&disabledProject, &global); got.SkillULID != "SKL_G" {
		t.Fatalf("停用项目技能应回退全局: %s", got.SkillULID)
	}
	if got := pickSkillByScope(nil, &disabledGlobal); got != nil {
		t.Fatalf("停用全局技能不应命中: %+v", got)
	}

	// 步骤3：变量取值。显式值 > 声明缺省 > 必填缺失；显式空值回落缺省；内置变量自动注入。
	declared := []domain.SkillVariableDO{
		{VariableName: "tone", VariableLabel: "语气", DefaultValue: "克制"},
		{VariableName: "scene", VariableLabel: "场景", RequiredFlag: true},
	}
	valueMap, missing := resolveSkillVariables("polish", declared, map[string]string{"scene": "雨夜"}, "正文")
	if valueMap["tone"] != "克制" || valueMap["scene"] != "雨夜" ||
		valueMap["user_input"] != "正文" || valueMap["skill_code"] != "polish" || len(missing) != 0 {
		t.Fatalf("变量取值错误: %+v, missing=%v", valueMap, missing)
	}
	if _, missing = resolveSkillVariables("polish", declared, nil, "正文"); len(missing) != 1 || missing[0] != "scene" {
		t.Fatalf("必填缺失应被记录: %v", missing)
	}
	if valueMap, _ = resolveSkillVariables("polish", declared, map[string]string{"tone": ""}, "正文"); valueMap["tone"] != "克制" {
		t.Fatalf("显式空值应回落缺省: %+v", valueMap)
	}

	// 步骤4：模板占位符白名单。已声明放行，未声明拒绝。
	if err := validateSkillTemplate("润色 {{user_input}} 与 {{tone}}", declared); err != nil {
		t.Fatalf("已声明占位符不应报错: %v", err)
	}
	if err := validateSkillTemplate("润色 {{unknown_var}}", declared); err == nil {
		t.Fatal("未声明占位符必须被拒绝")
	}

	// 步骤5：参考资料命中与预算。空关键词总命中；多关键词任一命中；超预算不注入。
	if !matchSkillReferenceKeywords("", []string{"任意正文"}) {
		t.Fatal("空关键词应总是命中")
	}
	if matchSkillReferenceKeywords("雨夜", []string{"晴天的午后"}) {
		t.Fatal("关键词未出现不应命中")
	}
	if !matchSkillReferenceKeywords("雨夜,追车", []string{"晴天的午后", "追车开始"}) {
		t.Fatal("任一关键词命中即应命中")
	}
	longRef := domain.SkillReferenceDO{
		RefULID: "REF_1", SkillULID: "SKL_G", RefName: "长文",
		RefText: strings.Repeat("x", skillReferenceBudgetChars+100),
	}
	if got := selectSkillReferences([]domain.SkillReferenceDO{longRef}, []string{"任意正文"}, skillReferenceBudgetChars); len(got) != 0 {
		t.Fatalf("超预算资料不应注入: %v", got)
	}
	selected := selectSkillReferences([]domain.SkillReferenceDO{
		{RefULID: "REF_2", SkillULID: "SKL_G", RefName: "短文", RefText: "样例正文"},
	}, []string{"任意正文"}, skillReferenceBudgetChars)
	if len(selected) != 1 || !strings.Contains(selected[0], "【短文】") || !strings.Contains(selected[0], "样例正文") {
		t.Fatalf("参考资料注入格式错误: %v", selected)
	}

	// 步骤6：全局覆盖标记。同编码且启用的项目技能覆盖全局；停用或异编码不覆盖。
	otherCode := skillDO("SKL_O", domain.SkillScopeProject, "PRJ_1", "continue", true)
	if !isOverriddenGlobalSkill(&global, []domain.SkillDO{global, project}) {
		t.Fatal("同编码项目技能应标记覆盖")
	}
	if isOverriddenGlobalSkill(&global, []domain.SkillDO{global, disabledProject}) {
		t.Fatal("停用项目技能不应覆盖")
	}
	if isOverriddenGlobalSkill(&global, []domain.SkillDO{global, otherCode}) {
		t.Fatal("异编码项目技能不应覆盖")
	}
}

// TestChatContextChain 覆盖一次请求里上下文从「素材」到「消息序列」的完整装配链路：
// 技能模板渲染 → 引用分优先级 → 摘要与档案注入 → 预算裁剪 → 历史选取 → 消息成型 → 引用校验。
func TestChatContextChain(t *testing.T) {
	// 步骤1：技能命中时渲染模板并产出最终指令；未命中时直接用用户正文。
	blocks, instruction := buildContextBlocks(chatContextInput{
		SkillTemplate: "润色指令：{{user_input}}｜语气：{{tone}}｜章节：{{chapter_content}}｜依据：{{context}}",
		SkillVariable: map[string]string{"tone": "克制"},
		SkillRefList:  []string{"【文风样例】\n短句为主"},
		UserInput:     "这段话",
		ChapterList:   []chapterSource{{ChapterName: "第一章", ContentText: "章节正文", IsCurrent: true}},
		DocList:       []docSource{{DocName: "设定集", FullText: "文档全文"}},
	})
	skillBlock := findBlock(blocks, "skill")
	if skillBlock == nil || skillBlock.Priority != ai.PrioritySkill || instruction != skillBlock.ContentText {
		t.Fatalf("技能块缺失或指令未取渲染结果: %+v", skillBlock)
	}
	for _, want := range []string{"润色指令：这段话", "语气：克制", "章节：章节正文", "文档全文"} {
		if !strings.Contains(instruction, want) {
			t.Fatalf("渲染结果缺少 %q: %q", want, instruction)
		}
	}
	refBlock := findBlock(blocks, "skill_reference")
	if refBlock == nil || refBlock.Priority != ai.PrioritySkillRef || !strings.Contains(refBlock.ContentText, "短句为主") {
		t.Fatalf("技能参考资料块错误: %+v", refBlock)
	}
	freeBlocks, freeInstruction := buildContextBlocks(chatContextInput{
		UserInput:   "自由提问原文",
		ChapterList: []chapterSource{{ChapterName: "第一章", ContentText: "章节正文", IsCurrent: true}},
	})
	if freeInstruction != "自由提问原文" || findBlock(freeBlocks, "skill") != nil {
		t.Fatalf("技能未命中应直接用用户正文: %q", freeInstruction)
	}

	// 步骤2：引用分优先级。当前章 90，引用章与文档 80，私人笔记不进上下文。
	blocks, _ = buildContextBlocks(chatContextInput{
		UserInput: "提问",
		ChapterList: []chapterSource{
			{ChapterName: "第一章", ContentText: "当前章正文", IsCurrent: true},
			{ChapterName: "第二章", ContentText: "引用章正文"},
		},
		DocList: []docSource{{DocName: "设定集", FullText: "文档全部切片拼接"}},
		LoreList: []domain.LoreDO{
			{LoreName: "灵力体系", LoreTypeCode: "rule", ContentText: "设定内容正文"},
			{LoreName: "私人笔记", LoreTypeCode: loreTypeCodeNote, ContentText: "笔记内容正文"},
		},
	})
	if got := findBlock(blocks, "current_chapter"); got == nil || got.Priority != ai.PriorityCurrentChapter ||
		!strings.Contains(got.ContentText, "当前章正文") {
		t.Fatalf("当前章块错误: %+v", got)
	}
	if got := findBlock(blocks, "ref_chapter"); got == nil || got.Priority != ai.PriorityExplicitRef ||
		!strings.Contains(got.ContentText, "引用章正文") {
		t.Fatalf("引用章块错误: %+v", got)
	}
	if got := findBlock(blocks, "ref_doc"); got == nil || got.Priority != ai.PriorityExplicitRef ||
		!strings.Contains(got.ContentText, "文档全部切片拼接") {
		t.Fatalf("引用文档块错误: %+v", got)
	}
	lore := findBlock(blocks, "lore")
	if lore == nil || lore.Priority != ai.PriorityCharacter || !strings.Contains(lore.ContentText, "设定内容正文") {
		t.Fatalf("lore 块错误: %+v", lore)
	}
	if strings.Contains(lore.ContentText, "笔记内容正文") {
		t.Fatalf("私人笔记不应注入: %q", lore.ContentText)
	}

	// 步骤3：显式引用摘要与档案按 80 注入，与自动梗概（60）并存；未引用时不产生空块。
	blocks, _ = buildContextBlocks(chatContextInput{
		UserInput:      "提问",
		RefSummaryList: []string{"第一章：主角取得钥匙"},
		RefPromptList:  []string{"《连载作者》\n你是连载作者，保持悬念"},
		SummaryList:    []string{"前章梗概"},
	})
	summaryBlock := findBlock(blocks, "ref_summary")
	if summaryBlock == nil || summaryBlock.Priority != ai.PriorityExplicitRef ||
		!strings.Contains(summaryBlock.ContentText, "主角取得钥匙") {
		t.Fatalf("引用摘要块错误: %+v", summaryBlock)
	}
	promptBlock := findBlock(blocks, "ref_prompt")
	if promptBlock == nil || promptBlock.Priority != ai.PriorityExplicitRef ||
		!strings.Contains(promptBlock.ContentText, "保持悬念") {
		t.Fatalf("引用提示词块错误: %+v", promptBlock)
	}
	autoBlock := findBlock(blocks, "chapter_summary")
	if autoBlock == nil || autoBlock.Priority != ai.PriorityChapterSummary {
		t.Fatalf("自动梗概块应保留且优先级为 60: %+v", autoBlock)
	}
	emptyBlocks, _ := buildContextBlocks(chatContextInput{UserInput: "提问"})
	if findBlock(emptyBlocks, "ref_summary") != nil || findBlock(emptyBlocks, "ref_prompt") != nil {
		t.Fatal("未引用摘要/提示词时不应产生对应块")
	}

	// 步骤4：预算裁剪。同优先级保持输入顺序；超预算按优先级丢块；历史不重复注入。
	blocks, _ = buildContextBlocks(chatContextInput{
		UserInput:     "提问",
		CharacterList: []domain.CharacterDO{{CharacterName: "林澈"}},
		LoreList:      []domain.LoreDO{{LoreName: "灵力体系", LoreTypeCode: "rule", ContentText: "设定内容正文"}},
	})
	assembled, _ := ai.AssembleContext(blocks, ai.ContextBudgetChars)
	if strings.Index(assembled, "林澈") > strings.Index(assembled, "灵力体系") {
		t.Fatalf("同优先级应保持输入顺序: %q", assembled)
	}
	blocks, _ = buildContextBlocks(chatContextInput{
		UserInput:   "提问",
		DocList:     []docSource{{DocName: "A", FullText: strings.Repeat("d", 100)}},
		RagList:     []domain.KnowledgeChunkVO{{DocName: "B", ChunkText: strings.Repeat("r", 100)}},
		SummaryList: []string{strings.Repeat("s", 100)},
		LoreList:    []domain.LoreDO{{LoreName: "L", ContentText: strings.Repeat("l", 100)}},
	})
	assembled, dropped := ai.AssembleContext(blocks, 300)
	droppedText := strings.Join(dropped, ",")
	if !strings.Contains(droppedText, "chapter_summary") || !strings.Contains(droppedText, "lore") {
		t.Fatalf("应丢弃 chapter_summary 与 lore: %v", dropped)
	}
	if strings.Contains(droppedText, "ref_doc") || strings.Contains(droppedText, "rag") {
		t.Fatalf("高优先级块不应被丢弃: %v", dropped)
	}
	if !strings.Contains(assembled, strings.Repeat("d", 100)) || strings.Contains(assembled, strings.Repeat("s", 100)) {
		t.Fatalf("装配文本应只保留高优先级块: %q", assembled)
	}
	if strings.Contains(assembled, "【对话历史】") {
		t.Fatalf("历史不应注入上下文块: %q", assembled)
	}

	// 步骤5：历史选取。最近 N 条旧到新；显式引用去重并置尾；N 为 0 时只注入引用消息。
	messages := []domain.ChatMessageDO{
		chatMessageOf("MSG_1", domain.ChatRoleUser, "一"),
		chatMessageOf("MSG_2", domain.ChatRoleAssistant, "二"),
		chatMessageOf("MSG_3", domain.ChatRoleUser, "三"),
		chatMessageOf("MSG_4", domain.ChatRoleAssistant, "四"),
		chatMessageOf("MSG_5", domain.ChatRoleUser, "五"),
	}
	if got := selectHistoryMessages(messages, nil, 2); len(got) != 2 ||
		got[0].MessageULID != "MSG_4" || got[1].MessageULID != "MSG_5" {
		t.Fatalf("最近 N 条应旧到新返回: %v", got)
	}
	got := selectHistoryMessages(messages, []string{"MSG_1", "MSG_4", "MSG_1"}, 2)
	want := []string{"MSG_4", "MSG_5", "MSG_1"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].MessageULID != want[i] {
			t.Fatalf("selected[%d] = %s, want %s", i, got[i].MessageULID, want[i])
		}
	}
	if got = selectHistoryMessages(messages, []string{"MSG_2"}, 0); len(got) != 1 || got[0].MessageULID != "MSG_2" {
		t.Fatalf("N 为 0 只应注入引用消息: %v", got)
	}

	// 步骤6：消息序列成型。system(含文风) → 上下文 → 历史 role 映射 → 最终指令。
	history := []domain.ChatMessageDO{
		chatMessageOf("MSG_1", domain.ChatRoleUser, "历史问"),
		chatMessageOf("MSG_2", domain.ChatRoleAssistant, "历史答"),
	}
	out := buildChatMessages("", "文风约束", "装配文本", "最终指令", history)
	if len(out) != 5 {
		t.Fatalf("len = %d, want 5", len(out))
	}
	if !strings.Contains(out[0].Content, "文风约束") {
		t.Fatal("system 应注入项目文风")
	}
	if !strings.HasPrefix(out[1].Content, "【上下文】") || !strings.Contains(out[1].Content, "装配文本") {
		t.Fatalf("第二条应为上下文: %q", out[1].Content)
	}
	if out[2].Role != "user" || out[2].Content != "历史问" ||
		out[3].Role != "assistant" || out[3].Content != "历史答" {
		t.Fatalf("历史 role 映射错误: %+v", out[2:4])
	}
	if out[4].Content != "最终指令" {
		t.Fatalf("末条应为最终指令: %q", out[4].Content)
	}
	if out = buildChatMessages("", "文风约束", "", "最终指令", nil); len(out) != 2 || out[1].Content != "最终指令" {
		t.Fatalf("装配文本为空应省略上下文消息: %+v", out)
	}

	// 步骤7：显式引用必须属于当前会话，去重后保序。
	sessionMessages := []domain.ChatMessageDO{
		chatMessageOf("MSG_1", domain.ChatRoleUser, "一"),
		chatMessageOf("MSG_2", domain.ChatRoleAssistant, "二"),
	}
	validated, err := validateRefMessages(sessionMessages, []string{"MSG_2", "MSG_1", "MSG_2"})
	if err != nil || len(validated) != 2 || validated[0] != "MSG_2" || validated[1] != "MSG_1" {
		t.Fatalf("引用应去重保序: %v, err=%v", validated, err)
	}
	if _, err := validateRefMessages(sessionMessages, []string{"MSG_999"}); err == nil {
		t.Fatal("会话外引用必须失败")
	}
}

// TestChatStreamContract 覆盖流式事件契约：thinking/chunk/done 顺序、seq 从 1 递增、
// done 携带完整思考过程与空数组引用（前端据此切换「等待→思考→正文→完成」）。
func TestChatStreamContract(t *testing.T) {
	sink := &eventSink{}
	writer := &chatEventWriter{writer: sink, sessionULID: "SES_1", messageULID: "MSG_1"}
	if err := writer.emit(domain.StreamEventThinking, "先想一步", "", nil); err != nil {
		t.Fatalf("thinking emit: %v", err)
	}
	if err := writer.emit(domain.StreamEventChunk, "正文", "", nil); err != nil {
		t.Fatalf("chunk emit: %v", err)
	}
	if err := writer.emitWithReasoning(domain.StreamEventDone, "正文完", "先想一步", "", nil); err != nil {
		t.Fatalf("done emit: %v", err)
	}
	if len(sink.eventList) != 3 {
		t.Fatalf("事件数 = %d, want 3", len(sink.eventList))
	}
	if sink.eventList[0].EventType != domain.StreamEventThinking || sink.eventList[0].ContentText != "先想一步" {
		t.Fatalf("首条应为 thinking 增量: %+v", sink.eventList[0])
	}
	done := sink.eventList[2]
	if done.EventType != domain.StreamEventDone || done.ReasoningText != "先想一步" || done.ReferenceList == nil {
		t.Fatalf("done 应携带思考过程与空数组引用: %+v", done)
	}
	for i := range sink.eventList {
		if sink.eventList[i].Seq != int64(i+1) {
			t.Fatalf("seq 应单调递增: %+v", sink.eventList)
		}
	}
}

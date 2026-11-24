// AI 对话链路的场景回归：斜杠命令解析 → 提示词作用域 → 上下文装配 → 消息序列成型 → 工具触发 → 流式契约。
// 这些环节是同一次请求的上下游，拆开测会漏掉环节间的因果（作用域覆盖影响谁、
// 预算裁剪丢哪些块、提示词进入哪一级消息、事件顺序影响前端展示），因此按链路组织。

package service

import (
	"strings"
	"testing"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// promptDO 构造测试用提示词行。
func promptDO(ulid, scope, projectULID, code, kind string, enabled bool) domain.PromptDO {
	return domain.PromptDO{
		PromptULID: ulid, ScopeCode: scope, ProjectULID: projectULID,
		PromptCode: code, PromptKindCode: kind, EnabledFlag: enabled,
	}
}

// toolDO 构造测试用工具行。
func toolDO(code string, enabled bool, keywords string) domain.ToolDO {
	return domain.ToolDO{
		ToolULID: "TOL_" + code, ToolCode: code, ToolName: code + "工具",
		TriggerKeywords: keywords, EnabledFlag: enabled, BuiltinFlag: true,
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

// TestPromptResolutionChain 覆盖一次请求里提示词从「命中」到「可下发」的完整解析链路：
// 编码解析 → 作用域优先级 → 占位符校验 → 全局覆盖标记。
func TestPromptResolutionChain(t *testing.T) {
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
		{"命令不在开头", "", "续写 /continue", "", "续写 /continue"},
	}
	for _, testCase := range cases {
		code, remain := resolveSlashCode(testCase.explicit, testCase.content)
		if code != testCase.wantCode || remain != testCase.wantRemain {
			t.Fatalf("%s: resolveSlashCode = %q %q, want %q %q", testCase.caseName,
				code, remain, testCase.wantCode, testCase.wantRemain)
		}
	}

	// 步骤2：作用域优先级。项目提示词胜过全局；项目停用回退全局；全局停用则不命中。
	project := promptDO("PRM_P", domain.PromptScopeProject, "PRJ_1", "polish", domain.PromptKindCommand, true)
	global := promptDO("PRM_G", domain.PromptScopeGlobal, "", "polish", domain.PromptKindCommand, true)
	disabledProject := promptDO("PRM_P", domain.PromptScopeProject, "PRJ_1", "polish", domain.PromptKindCommand, false)
	disabledGlobal := promptDO("PRM_G", domain.PromptScopeGlobal, "", "polish", domain.PromptKindCommand, false)

	if got := pickPromptByScope(&project, &global); got.PromptULID != "PRM_P" {
		t.Fatalf("项目提示词应优先: %s", got.PromptULID)
	}
	if got := pickPromptByScope(&disabledProject, &global); got.PromptULID != "PRM_G" {
		t.Fatalf("停用项目提示词应回退全局: %s", got.PromptULID)
	}
	if got := pickPromptByScope(nil, &disabledGlobal); got != nil {
		t.Fatalf("停用全局提示词不应命中: %+v", got)
	}

	// 步骤3：占位符校验。内置占位符放行，未登记占位符拒绝（渲染后会静默丢内容）。
	if err := validatePromptPlaceholders("续写 {{chapter_content}}，要求 {{user_input}}，依据 {{context}}"); err != nil {
		t.Fatalf("内置占位符不应报错: %v", err)
	}
	if err := validatePromptPlaceholders("续写 {{unknown_var}}"); err == nil {
		t.Fatal("未登记占位符必须被拒绝")
	}

	// 步骤4：全局覆盖标记。同编码且启用的项目提示词覆盖全局；停用或异编码不覆盖。
	otherCode := promptDO("PRM_O", domain.PromptScopeProject, "PRJ_1", "continue", domain.PromptKindCommand, true)
	if !isOverriddenGlobalPrompt(&global, []domain.PromptDO{global, project}) {
		t.Fatal("同编码项目提示词应标记覆盖")
	}
	if isOverriddenGlobalPrompt(&global, []domain.PromptDO{global, disabledProject}) ||
		isOverriddenGlobalPrompt(&global, []domain.PromptDO{global, otherCode}) {
		t.Fatal("停用或异编码项目提示词不应覆盖")
	}
}

// TestChatContextChain 覆盖一次请求里上下文从「素材」到「消息序列」的完整装配链路：
// 引用分优先级 → 摘要与提示词注入 → 预算裁剪 → 历史选取 → 消息成型（提示词为系统级）。
func TestChatContextChain(t *testing.T) {
	// 步骤1：引用分优先级。当前章 100，引用章与文档 90，设定 50，私人笔记不进上下文。
	blocks := buildContextBlocks(chatContextInput{
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
	if got := findBlock(blocks, "current_chapter"); got == nil ||
		got.Priority != ai.PriorityCurrentChapter || !strings.Contains(got.ContentText, "当前章正文") {
		t.Fatalf("当前章块错误: %+v", got)
	}
	if got := findBlock(blocks, "ref_chapter"); got == nil ||
		got.Priority != ai.PriorityExplicitRef || !strings.Contains(got.ContentText, "引用章正文") {
		t.Fatalf("引用章块错误: %+v", got)
	}
	if got := findBlock(blocks, "ref_doc"); got == nil ||
		got.Priority != ai.PriorityExplicitRef || !strings.Contains(got.ContentText, "文档全部切片拼接") {
		t.Fatalf("引用文档块错误: %+v", got)
	}
	lore := findBlock(blocks, "lore")
	if lore == nil || lore.Priority != ai.PriorityCharacter || !strings.Contains(lore.ContentText, "设定内容正文") {
		t.Fatalf("lore 块错误: %+v", lore)
	}
	if strings.Contains(lore.ContentText, "笔记内容正文") {
		t.Fatalf("私人笔记不应注入: %q", lore.ContentText)
	}

	// 步骤2：显式引用摘要与提示词按 90 注入，与自动梗概（60）并存；未引用时不产生空块。
	blocks = buildContextBlocks(chatContextInput{
		UserInput:      "提问",
		RefSummaryList: []string{"第一章：主角取得钥匙"},
		RefPromptList:  []string{"《连载作者》\n你是连载作者，保持悬念"},
		SummaryList:    []string{"前章梗概"},
	})
	if got := findBlock(blocks, "ref_summary"); got == nil ||
		got.Priority != ai.PriorityExplicitRef || !strings.Contains(got.ContentText, "主角取得钥匙") {
		t.Fatalf("引用摘要块错误: %+v", got)
	}
	if got := findBlock(blocks, "ref_prompt"); got == nil ||
		got.Priority != ai.PriorityExplicitRef || !strings.Contains(got.ContentText, "保持悬念") {
		t.Fatalf("引用提示词块错误: %+v", got)
	}
	if got := findBlock(blocks, "chapter_summary"); got == nil || got.Priority != ai.PriorityChapterSummary {
		t.Fatalf("自动梗概块应保留且优先级为 60: %+v", got)
	}
	emptyBlocks := buildContextBlocks(chatContextInput{UserInput: "提问"})
	if findBlock(emptyBlocks, "ref_summary") != nil || findBlock(emptyBlocks, "ref_prompt") != nil {
		t.Fatal("未引用摘要/提示词时不应产生对应块")
	}

	// 步骤3：预算裁剪。同优先级保持输入顺序；超预算按优先级丢块；历史不重复注入。
	blocks = buildContextBlocks(chatContextInput{
		UserInput:     "提问",
		CharacterList: []domain.CharacterDO{{CharacterName: "林澈"}},
		LoreList:      []domain.LoreDO{{LoreName: "灵力体系", LoreTypeCode: "rule", ContentText: "设定内容正文"}},
	})
	assembled, _ := ai.AssembleContext(blocks, ai.ContextBudgetChars)
	if strings.Index(assembled, "林澈") > strings.Index(assembled, "灵力体系") {
		t.Fatalf("同优先级应保持输入顺序: %q", assembled)
	}
	blocks = buildContextBlocks(chatContextInput{
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

	// 步骤4：历史选取。最近 N 条旧到新；显式引用去重并置尾；N 为 0 时只注入引用消息。
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

	// 步骤5：消息序列成型。身份（系统）→ 上下文（系统）→ 指令提示词（系统）→ 历史 role 映射 → 用户输入（用户）。
	history := []domain.ChatMessageDO{
		chatMessageOf("MSG_1", domain.ChatRoleUser, "历史问"),
		chatMessageOf("MSG_2", domain.ChatRoleAssistant, "历史答"),
	}
	out := buildChatMessages(chatMessageInput{
		SystemPromptText: "你是连载作者",
		StyleText:        "文风约束",
		ContextText:      "装配文本",
		CommandText:      "续写指令",
		HistoryList:      history,
		UserText:         "写下去",
	})
	if len(out) != 6 {
		t.Fatalf("len = %d, want 6", len(out))
	}
	if out[0].Role != "system" || !strings.Contains(out[0].Content, "文风约束") ||
		!strings.Contains(out[0].Content, "你是连载作者") {
		t.Fatalf("首条应为身份提示词（含文风）: %+v", out[0])
	}
	if out[1].Role != "system" || !strings.HasPrefix(out[1].Content, "【上下文】") ||
		!strings.Contains(out[1].Content, "装配文本") {
		t.Fatalf("第二条应为系统级上下文: %+v", out[1])
	}
	if out[2].Role != "system" || out[2].Content != "续写指令" {
		t.Fatalf("第三条应为系统级指令提示词: %+v", out[2])
	}
	if out[3].Role != "user" || out[3].Content != "历史问" ||
		out[4].Role != "assistant" || out[4].Content != "历史答" {
		t.Fatalf("历史 role 映射错误: %+v", out[3:5])
	}
	if out[5].Role != "user" || out[5].Content != "写下去" {
		t.Fatalf("末条应为用户输入: %+v", out[5])
	}
	if out = buildChatMessages(chatMessageInput{
		SystemPromptText: "你是连载作者", UserText: "自由提问",
	}); len(out) != 2 || out[1].Role != "user" || out[1].Content != "自由提问" {
		t.Fatalf("无上下文与指令时应只保留身份与用户输入: %+v", out)
	}

	// 步骤6：显式引用必须属于当前会话，去重后保序。
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

// TestToolTriggerChain 覆盖一次「正文带 #code 的对话」里工具从解析到执行的选取链路。
func TestToolTriggerChain(t *testing.T) {
	// 环节1：正文解析。行首/空白后的 #code 命中，正文中间粘连、大写、含连字符的不命中；
	// 触发词被剥离且不破坏换行与缩进。
	cases := []struct {
		caseName    string
		content     string
		wantCodes   int
		wantRemain  string
		wantFirstOf string
	}{
		{"行首触发", "#chapter_check 看看这章", 1, "看看这章", "chapter_check"},
		{"行中触发", "帮我 #deai_scan 扫一下", 1, "帮我 扫一下", "deai_scan"},
		{"多个触发", "#chapter_check #deai_scan 检查", 2, "检查", "chapter_check"},
		{"保留换行", "#chapter_check\n第一段\n第二段", 1, "第一段\n第二段", "chapter_check"},
		{"词内井号不命中", "C# 是语言", 0, "C# 是语言", ""},
		{"大写不命中", "#Chapter_Check 检查", 0, "#Chapter_Check 检查", ""},
	}
	for _, testCase := range cases {
		codeList, remainText := parseHashToolCodes(testCase.content)
		if len(codeList) != testCase.wantCodes || remainText != testCase.wantRemain {
			t.Fatalf("%s: 命中 %v 剩余 %q, want %d 个 / %q", testCase.caseName,
				codeList, remainText, testCase.wantCodes, testCase.wantRemain)
		}
		if testCase.wantCodes > 0 && codeList[0] != testCase.wantFirstOf {
			t.Fatalf("%s: 首个编码 = %q, want %q", testCase.caseName, codeList[0], testCase.wantFirstOf)
		}
	}

	// 环节2：编码合并。请求体显式编码在前、内联编码在后，去重保序并截断到上限。
	merged := mergeToolCodes([]string{"chapter_check", "chapter_check"}, []string{"deai_scan", "chapter_check"})
	if len(merged) != 2 || merged[0] != "chapter_check" || merged[1] != "deai_scan" {
		t.Fatalf("合并结果错误: %v", merged)
	}
	if overflow := mergeToolCodes([]string{"a", "b", "c", "d"}, nil); len(overflow) != toolExplicitLimit {
		t.Fatalf("超出上限未截断: %v", overflow)
	}

	// 环节3：显式选取。编码不存在或已停用记入 missing，不能静默跳过。
	fixtures := []domain.ToolDO{
		toolDO("chapter_check", true, "体检,检查这章"),
		toolDO("deai_scan", true, "去ai味"),
		toolDO("lore_lookup", false, "设定"),
	}
	voList, missingList := selectToolsByCode(fixtures, []string{"chapter_check", "lore_lookup", "not_exist"})
	if len(voList) != 1 || voList[0].ToolCode != "chapter_check" || len(missingList) != 2 {
		t.Fatalf("显式选取错误: %+v, missing=%v", voList, missingList)
	}

	// 环节4：关键词补位。已显式选中的不重复补，停用的不参与，补到余量为止。
	excludeSet := map[string]bool{"chapter_check": true}
	autoList := matchToolsByKeyword(fixtures, "这章有没有去ai味", excludeSet, 2)
	if len(autoList) != 1 || autoList[0].ToolCode != "deai_scan" {
		t.Fatalf("关键词补位错误: %+v", autoList)
	}
	if got := matchToolsByKeyword(fixtures, "设定是什么", excludeSet, 2); len(got) != 0 {
		t.Fatalf("停用工具不应自动触发: %+v", got)
	}
	if got := matchToolsByKeyword(fixtures, "这章有没有去ai味", excludeSet, 0); len(got) != 0 {
		t.Fatalf("余量为 0 时不应补位: %+v", got)
	}
}

// TestChatStreamContract 覆盖流式事件契约：thinking/chunk/done 顺序、seq 从 1 递增、
// done 携带完整思考过程、召回依据与本轮工具编码（前端据此切换「等待→思考→正文→完成」）。
func TestChatStreamContract(t *testing.T) {
	sink := &eventSink{}
	writer := &chatEventWriter{
		writer: sink, sessionULID: "SES_1", messageULID: "MSG_1",
		toolCodeList: []string{"chapter_check"},
	}
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
	// 工具编码随 done 下发：前端据此标记本轮真正跑过的工具（关键词自动触发的也算）。
	if len(done.ToolCodeList) != 1 || done.ToolCodeList[0] != "chapter_check" {
		t.Fatalf("done 应携带本轮工具编码: %+v", done.ToolCodeList)
	}
	for i := range sink.eventList {
		if sink.eventList[i].Seq != int64(i+1) {
			t.Fatalf("seq 应单调递增: %+v", sink.eventList)
		}
	}
}

// TestSlashNamespaceChain 覆盖斜杠命令的命名空间边界：写作台本地动作与内置提示词共用 `/` 触发，
// 两者编码一旦撞车，同一句输入会依实现顺序走向「执行动作」或「注入提示词」两种结果。
func TestSlashNamespaceChain(t *testing.T) {
	// 环节1：保留编码不被内置提示词占用。
	builtinCodes := ai.BuiltinCommandCodeSet()
	for code := range ai.BuiltinSystemCodeSet() {
		builtinCodes[code] = true
	}
	for promptCode := range builtinCodes {
		if domain.IsReservedActionCode(promptCode) {
			t.Fatalf("内置提示词编码 %s 与写作台动作冲突", promptCode)
		}
	}

	// 环节2：保留编码能被识别（大小写敏感，/Help 不是动作）。
	for _, actionCode := range []string{"help", "clear", "history", "new", "attach", "copy", "check", "focus", "snapshot"} {
		if !domain.IsReservedActionCode(actionCode) {
			t.Fatalf("动作编码 %s 未登记", actionCode)
		}
	}
	if domain.IsReservedActionCode("Help") || domain.IsReservedActionCode("polish") {
		t.Fatal("非动作编码不应被误判")
	}

	// 环节3：动作不被解析成提示词请求。前端拦下后正文原样保留，后端也不会凭空命中提示词。
	if code, _ := parseSlashToken("/help 怎么用"); code != "help" {
		t.Fatalf("首 token 解析应得到 help: %q", code)
	}
	if !domain.IsReservedActionCode("help") {
		t.Fatal("/help 必须识别为本地动作")
	}
}

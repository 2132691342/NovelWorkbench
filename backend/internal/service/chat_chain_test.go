// AI 对话链路的场景回归：斜杠命令解析 → 提示词作用域 → 上下文装配 → 消息序列成型 → 工具触发 → 流式契约。
// 这些环节是同一次请求的上下游，拆开测会漏掉环节间的因果（作用域覆盖影响谁、
// 预算裁剪丢哪些块、提示词进入哪一级消息、事件顺序影响前端展示），因此按链路组织。

package service

import (
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"

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
func toolDO(code string, enabled bool) domain.ToolDO {
	return domain.ToolDO{
		ToolULID: "TOL_" + code, ToolCode: code, ToolName: code + "工具",
		EnabledFlag: enabled, BuiltinFlag: true,
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

	// 步骤2：显式引用摘要按 90 注入，与自动梗概（60）并存；未引用时不产生空块。
	blocks = buildContextBlocks(chatContextInput{
		UserInput:      "提问",
		RefSummaryList: []string{"第一章：主角取得钥匙"},
		SummaryList:    []string{"前章梗概"},
	})
	if got := findBlock(blocks, "ref_summary"); got == nil ||
		got.Priority != ai.PriorityExplicitRef || !strings.Contains(got.ContentText, "主角取得钥匙") {
		t.Fatalf("引用摘要块错误: %+v", got)
	}
	if got := findBlock(blocks, "chapter_summary"); got == nil || got.Priority != ai.PriorityChapterSummary {
		t.Fatalf("自动梗概块应保留且优先级为 60: %+v", got)
	}
	emptyBlocks := buildContextBlocks(chatContextInput{UserInput: "提问"})
	if findBlock(emptyBlocks, "ref_summary") != nil || findBlock(emptyBlocks, "ref_session") != nil {
		t.Fatal("未引用摘要/会话时不应产生对应块")
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

	// 步骤4：历史选取。最近 N 条旧到新返回；N 为 0 表示不注入常规历史。
	messages := []domain.ChatMessageDO{
		chatMessageOf("MSG_1", domain.ChatRoleUser, "一"),
		chatMessageOf("MSG_2", domain.ChatRoleAssistant, "二"),
		chatMessageOf("MSG_3", domain.ChatRoleUser, "三"),
		chatMessageOf("MSG_4", domain.ChatRoleAssistant, "四"),
		chatMessageOf("MSG_5", domain.ChatRoleUser, "五"),
	}
	if got := selectHistoryMessages(messages, 2); len(got) != 2 ||
		got[0].MessageULID != "MSG_4" || got[1].MessageULID != "MSG_5" {
		t.Fatalf("最近 N 条应旧到新返回: %v", got)
	}
	if got := selectHistoryMessages(messages, 0); len(got) != 0 {
		t.Fatalf("N 为 0 不应注入常规历史: %v", got)
	}
	if got := selectHistoryMessages(messages, 10); len(got) != len(messages) {
		t.Fatalf("N 大于总数时应返回全部: %v", got)
	}

	// 步骤5：消息序列成型。身份（系统）→ 上下文（系统）→ 历史摘要（系统）→ 指令提示词（系统）→ 历史 role 映射 → 用户输入（用户）。
	history := []domain.ChatMessageDO{
		chatMessageOf("MSG_1", domain.ChatRoleUser, "历史问"),
		chatMessageOf("MSG_2", domain.ChatRoleAssistant, "历史答"),
	}
	out := buildChatMessages(chatMessageInput{
		SystemPromptText: "你是连载作者",
		StyleText:        "文风约束",
		ContextText:      "装配文本",
		DigestText:       "【历史摘要（本地压缩）】\n用户: 早先的问题",
		CommandText:      "续写指令",
		HistoryList:      history,
		UserText:         "写下去",
	})
	if len(out) != 7 {
		t.Fatalf("len = %d, want 7", len(out))
	}
	if out[0].Role != "system" || !strings.Contains(out[0].Content, "文风约束") ||
		!strings.Contains(out[0].Content, "你是连载作者") {
		t.Fatalf("首条应为身份提示词（含文风）: %+v", out[0])
	}
	if out[1].Role != "system" || !strings.HasPrefix(out[1].Content, "【上下文】") ||
		!strings.Contains(out[1].Content, "装配文本") {
		t.Fatalf("第二条应为系统级上下文: %+v", out[1])
	}
	// 摘要必须在指令提示词之前：它描述已发生的事实，指令描述本次要求
	if out[2].Role != "system" || !strings.Contains(out[2].Content, "历史摘要") {
		t.Fatalf("第三条应为系统级历史摘要: %+v", out[2])
	}
	if out[3].Role != "system" || out[3].Content != "续写指令" {
		t.Fatalf("第四条应为系统级指令提示词: %+v", out[3])
	}
	if out[4].Role != "user" || out[4].Content != "历史问" ||
		out[5].Role != "assistant" || out[5].Content != "历史答" {
		t.Fatalf("历史 role 映射错误: %+v", out[4:6])
	}
	if out[6].Role != "user" || out[6].Content != "写下去" {
		t.Fatalf("末条应为用户输入: %+v", out[6])
	}
	if out = buildChatMessages(chatMessageInput{
		SystemPromptText: "你是连载作者", UserText: "自由提问",
	}); len(out) != 2 || out[1].Role != "user" || out[1].Content != "自由提问" {
		t.Fatalf("无上下文与指令时应只保留身份与用户输入: %+v", out)
	}

	// 步骤6：@ 素材来源。角色与设定走「显式引用」优先级，历史会话整段压缩后注入。
	refBlocks := buildContextBlocks(chatContextInput{
		UserInput: "提问",
		RefCharacterList: []domain.CharacterDO{
			{CharacterName: "林澈", PersonalityText: "寡言", BackgroundText: "邮差"},
		},
		RefLoreList: []domain.LoreDO{
			{LoreName: "灵力体系", LoreTypeCode: "rule", ContentText: "设定正文"},
		},
		RefSessionList: []sessionSource{
			{SessionName: "上一轮讨论", ContentText: "用户: 主角动机\nAI: 是赎罪"},
		},
	})
	if block := findBlock(refBlocks, "ref_character"); block == nil ||
		block.Priority != ai.PriorityExplicitRef || !strings.Contains(block.ContentText, "林澈") {
		t.Fatalf("引用角色应进显式引用块: %+v", block)
	}
	if block := findBlock(refBlocks, "ref_lore"); block == nil ||
		block.Priority != ai.PriorityExplicitRef || !strings.Contains(block.ContentText, "灵力体系") {
		t.Fatalf("引用设定应进显式引用块: %+v", block)
	}
	sessionBlock := findBlock(refBlocks, "ref_session")
	if sessionBlock == nil || !strings.Contains(sessionBlock.ContentText, "上一轮讨论") ||
		!strings.Contains(sessionBlock.ContentText, "赎罪") {
		t.Fatalf("引用会话应带上会话名与压缩摘要: %+v", sessionBlock)
	}
}

// TestToolSelectionChain 覆盖「模型自行选择工具」的链路：工具声明 → 参数解码 → 流式调用归并 → 执行分流。
// 作者无法手动点名工具，服务端也不再按关键词猜测，一切由模型基于声明决定。
func TestToolSelectionChain(t *testing.T) {
	// 环节1：工具声明的 function name 用工具编码，参数按声明透出，供模型据此调用。
	infoList := ai.ToolInfos([]ai.ToolSpec{{
		ToolCode: "chapter_digest", ToolName: "章节摘要", DescriptionText: "抽取式生成章节摘要",
		ParamList: []ai.ToolParam{{ParamName: "max_sentences", RequiredFlag: false, DescriptionText: "保留句数"}},
	}})
	if len(infoList) != 1 || infoList[0].Name != "chapter_digest" {
		t.Fatalf("工具声明名称应为工具编码: %+v", infoList)
	}
	if infoList[0].ParamsOneOf == nil {
		t.Fatal("有参数的工具必须带上参数声明")
	}

	// 环节2：模型回传的参数是 JSON，需按字符串解出（数字与布尔也要能落到工具参数里）。
	paramValue := ai.DecodeToolArguments(`{"content_text":"正文","max_sentences":6,"strict":true}`)
	if paramValue["content_text"] != "正文" || paramValue["max_sentences"] != "6" || paramValue["strict"] != "true" {
		t.Fatalf("参数解码错误: %v", paramValue)
	}
	if len(ai.DecodeToolArguments("not json")) != 0 {
		t.Fatal("非法参数 JSON 应退化为空表而不是报错")
	}

	// 环节3：流式分片里的工具调用按 index 归并（名称与参数可能跨多个分片）。
	index0, index1 := 0, 1
	mergedCalls, err := mergeToolCallChunks([]schema.ToolCall{
		{Index: &index0, ID: "call_1", Type: "function", Function: schema.FunctionCall{Name: "chapter_check"}},
		{Index: &index1, ID: "call_2", Type: "function", Function: schema.FunctionCall{Name: "deai_scan"}},
		{Index: &index0, Function: schema.FunctionCall{Arguments: `{"content_text":`}},
		{Index: &index0, Function: schema.FunctionCall{Arguments: `"正文"}`}},
	})
	if err != nil || len(mergedCalls) != 2 {
		t.Fatalf("工具调用归并失败: %v, err=%v", mergedCalls, err)
	}
	if mergedCalls[0].Function.Name != "chapter_check" || mergedCalls[0].Function.Arguments != `{"content_text":"正文"}` {
		t.Fatalf("首个调用应完整合并: %+v", mergedCalls[0])
	}
	if mergedCalls[1].Function.Name != "deai_scan" || mergedCalls[1].Function.Arguments != "" {
		t.Fatalf("第二个调用不应串到第一个的参数: %+v", mergedCalls[1])
	}
}

// TestHistoryCompactChain 覆盖历史压缩链路：门槛判定 → 折叠规则 → 预算裁剪 → 与既有摘要拼接。
// 压缩必须完全本地完成（不调用模型），且同一输入必然得到同一输出。
func TestHistoryCompactChain(t *testing.T) {
	// 环节1：短历史不压缩。总字符未过门槛时原样返回，避免小题大做丢细节。
	shortList := []ai.CompactMessage{
		{RoleCode: "user", ContentText: "短问题"},
		{RoleCode: "assistant", ContentText: "短回答"},
	}
	shortResult := ai.CompactHistory(shortList, ai.DefaultCompactConfig())
	if shortResult.Compacted || len(shortResult.KeptList) != 2 || shortResult.DigestText != "" {
		t.Fatalf("短历史不应压缩: %+v", shortResult)
	}

	// 环节2：长历史折叠。最近 KeepRecentCount 条保留原文，更早的折成「角色: 内容」单行。
	longText := strings.Repeat("这是一段足够长的历史正文。", 100)
	longList := make([]ai.CompactMessage, 0, 14)
	for i := range 12 {
		roleCode := "user"
		if i%2 == 1 {
			roleCode = "assistant"
		}
		longList = append(longList, ai.CompactMessage{RoleCode: roleCode, ContentText: longText})
	}
	longResult := ai.CompactHistory(longList, ai.CompactConfig{
		KeepRecentCount: 4, PerMessageRunes: 20, BudgetChars: 4000, MinTotalChars: 100,
	})
	if !longResult.Compacted {
		t.Fatal("长历史必须压缩")
	}
	if len(longResult.KeptList) != 4 {
		t.Fatalf("应保留最近 4 条，实际 %d", len(longResult.KeptList))
	}
	if !strings.Contains(longResult.DigestText, "用户: ") || !strings.Contains(longResult.DigestText, "AI: ") {
		t.Fatalf("摘要应带角色标签: %q", longResult.DigestText)
	}
	if strings.Count(longResult.DigestText, "\n") < 4 {
		t.Fatalf("每条折叠消息应占一行: %q", longResult.DigestText)
	}

	// 环节3：预算裁剪。摘要超预算时从最旧一条开始丢弃，保留离当前更近的历史。
	tightResult := ai.CompactHistory(longList, ai.CompactConfig{
		KeepRecentCount: 4, PerMessageRunes: 60, BudgetChars: 120, MinTotalChars: 100,
	})
	if tightResult.DroppedCount == 0 {
		t.Fatalf("预算 120 字符下必然丢弃旧摘要: %+v", tightResult)
	}
	if tightResult.FoldedCount+tightResult.DroppedCount != len(longList)-4 {
		t.Fatalf("折叠数+丢弃数应等于被折叠总数: %+v", tightResult)
	}

	// 环节4：确定性。同一输入两次压缩结果必须逐字节相同（否则缓存与落库会漂移）。
	if again := ai.CompactHistory(longList, ai.DefaultCompactConfig()); again.DigestText !=
		ai.CompactHistory(longList, ai.DefaultCompactConfig()).DigestText {
		t.Fatal("压缩结果必须确定")
	}

	// 环节5：与既有摘要拼接。会话已有压缩点时新摘要接在其后，不覆盖旧结论。
	joined := ai.JoinDigest("【历史摘要（本地压缩）】\n用户: 早期结论", "用户: 本轮新增")
	if !strings.Contains(joined, "早期结论") || !strings.HasSuffix(joined, "本轮新增") {
		t.Fatalf("拼接顺序错误: %q", joined)
	}
	if ai.JoinDigest("", "只有新的") != "只有新的" || ai.JoinDigest("只有旧的", "") != "只有旧的" {
		t.Fatal("空串应自动忽略")
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
	for _, actionCode := range []string{"help", "clear", "history", "new", "attach", "copy", "check", "focus", "snapshot", "compact"} {
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

// AI 对话装配链路的回归：斜杠命令解析 → 提示词作用域 → 上下文块装配 → 消息序列成型 →
// 工具声明与流式事件 → 历史全量注入与归档。
// 这六步是同一次请求的上下游，合并为一条用例；每步失败都表现为「模型没按预期回答」，
// 步骤用「步骤N」标注，失败时直接定位到断点。

package service

import (
	"strconv"
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

// TestChatAssemblyChain 覆盖一次 AI 对话请求从输入到流式输出的装配链。
func TestChatAssemblyChain(t *testing.T) {
	// 步骤1：斜杠命令解析。显式编码优先于正文首 token，正文命令不在开头时不命中。
	slashCases := []struct {
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
	for _, testCase := range slashCases {
		code, remain := resolveSlashCode(testCase.explicit, testCase.content)
		if code != testCase.wantCode || remain != testCase.wantRemain {
			t.Fatalf("%s: resolveSlashCode = %q %q, want %q %q", testCase.caseName,
				code, remain, testCase.wantCode, testCase.wantRemain)
		}
	}

	// 步骤2：提示词作用域。项目提示词胜过全局；项目停用回退全局；全局停用则不命中。
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
	// 同编码且启用的项目提示词覆盖全局，全局条目标记为已覆盖。
	if !isOverriddenGlobalPrompt(&global, []domain.PromptDO{global, project}) {
		t.Fatal("同编码项目提示词应标记覆盖")
	}
	otherCode := promptDO("PRM_O", domain.PromptScopeProject, "PRJ_1", "continue", domain.PromptKindCommand, true)
	if isOverriddenGlobalPrompt(&global, []domain.PromptDO{global, disabledProject}) ||
		isOverriddenGlobalPrompt(&global, []domain.PromptDO{global, otherCode}) {
		t.Fatal("停用或异编码项目提示词不应覆盖")
	}
	// 占位符未登记必须被拒绝：渲染后会静默丢上下文。
	if err := validatePromptPlaceholders("续写 {{chapter_content}}，要求 {{user_input}}，依据 {{context}}"); err != nil {
		t.Fatalf("内置占位符不应报错: %v", err)
	}
	if err := validatePromptPlaceholders("续写 {{unknown_var}}"); err == nil {
		t.Fatal("未登记占位符必须被拒绝")
	}

	// 步骤3：上下文块优先级与排除。当前章 100，引用章与文档 90，设定 50，私人笔记不进上下文。
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

	// 步骤3.1：显式引用摘要按 90 注入，与自动梗概（60）并存；未引用时不产生空块。
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

	// 步骤3.2：装配不丢块。同优先级保持输入顺序；所有块都进上下文，超长只在块内保留首尾。
	blocks = buildContextBlocks(chatContextInput{
		UserInput:     "提问",
		CharacterList: []domain.CharacterDO{{CharacterName: "林澈"}},
		LoreList:      []domain.LoreDO{{LoreName: "灵力体系", LoreTypeCode: "rule", ContentText: "设定内容正文"}},
	})
	assembled, _ := ai.AssembleContext(blocks)
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
	assembled, condensed := ai.AssembleContext(blocks)
	if len(condensed) != 0 {
		t.Fatalf("未超单块上限时不应收敛任何块: %v", condensed)
	}
	for _, want := range []string{strings.Repeat("d", 100), strings.Repeat("r", 100),
		strings.Repeat("s", 100), strings.Repeat("l", 100)} {
		if !strings.Contains(assembled, want) {
			t.Fatalf("所有块都应进上下文，缺少 %q: %q", want[:1], assembled)
		}
	}
	if strings.Contains(assembled, "【对话历史】") {
		t.Fatalf("历史不应注入上下文块: %q", assembled)
	}

	// 步骤4：消息序列成型。身份（系统）→ 上下文（系统）→ 任务提示词（系统）→ 历史 role 映射 → 用户输入（用户）。
	out := buildChatMessages(chatMessageInput{
		SystemPromptText: "你是连载作者",
		StyleText:        "文风约束",
		ContextText:      "装配文本",
		CommandText:      "续写指令",
		HistoryList: []domain.ChatMessageDO{
			chatMessageOf("MSG_1", domain.ChatRoleUser, "历史问"),
			chatMessageOf("MSG_2", domain.ChatRoleAssistant, "历史答"),
		},
		UserText: "写下去",
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
		t.Fatalf("第三条应为系统级任务提示词: %+v", out[2])
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

	// 步骤5：@ 素材来源。角色与设定走「显式引用」优先级，历史会话整段归档后注入。
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
		t.Fatalf("引用会话应带上会话名与归档正文: %+v", sessionBlock)
	}

	// 步骤6：工具声明与参数解码。工具编码即 function name，参数按字符串解出。
	infoList := ai.ToolInfos([]ai.ToolSpec{{
		ToolCode: "chapter_digest", ToolName: "章节摘要", DescriptionText: "抽取式生成章节摘要",
		ParamList: []ai.ToolParam{{ParamName: "max_sentences", RequiredFlag: false, DescriptionText: "保留句数"}},
	}})
	if len(infoList) != 1 || infoList[0].Name != "chapter_digest" || infoList[0].ParamsOneOf == nil {
		t.Fatalf("工具声明应为工具编码并带参数声明: %+v", infoList)
	}
	paramValue := ai.DecodeToolArguments(`{"content_text":"正文","max_sentences":6,"strict":true}`)
	if paramValue["content_text"] != "正文" || paramValue["max_sentences"] != "6" || paramValue["strict"] != "true" {
		t.Fatalf("参数解码错误: %v", paramValue)
	}
	if len(ai.DecodeToolArguments("not json")) != 0 {
		t.Fatal("非法参数 JSON 应退化为空表而不是报错")
	}

	// 步骤7：流式工具调用按 index 归并（名称与参数可能跨多个分片）。
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

	// 步骤8：流式事件契约。thinking → chunk → done 顺序，seq 从 1 递增，
	// done 携带完整思考过程、召回依据与本轮工具编码（前端据此切换「等待→思考→正文→完成」）。
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
	if len(done.ToolCodeList) != 1 || done.ToolCodeList[0] != "chapter_check" {
		t.Fatalf("done 应携带本轮工具编码: %+v", done.ToolCodeList)
	}
	for i := range sink.eventList {
		if sink.eventList[i].Seq != int64(i+1) {
			t.Fatalf("seq 应单调递增: %+v", sink.eventList)
		}
	}

	// 步骤9：历史全量注入。条数一条不删，短正文原样返回。
	messages := []domain.ChatMessageDO{
		chatMessageOf("MSG_1", domain.ChatRoleUser, "一"),
		chatMessageOf("MSG_2", domain.ChatRoleAssistant, "二"),
		chatMessageOf("MSG_3", domain.ChatRoleUser, "三"),
		chatMessageOf("MSG_4", domain.ChatRoleAssistant, "四"),
		chatMessageOf("MSG_5", domain.ChatRoleUser, "五"),
	}
	injected, condensedCount := condenseHistory(messages)
	if len(injected) != len(messages) || injected[0].MessageULID != "MSG_1" ||
		injected[4].MessageULID != "MSG_5" {
		t.Fatalf("历史应全量旧到新注入: %v", injected)
	}
	if condensedCount != 0 || injected[0].ContentText != "一" {
		t.Fatalf("短正文不应被改写: %+v", injected[0])
	}

	// 步骤10：单条超长只保留首尾，条数仍然一条不删，并标注省略字数。
	longList := []domain.ChatMessageDO{
		chatMessageOf("MSG_1", domain.ChatRoleUser, strings.Repeat("甲", 5000)),
		chatMessageOf("MSG_2", domain.ChatRoleAssistant, "短回答"),
	}
	injected, condensedCount = condenseHistory(longList)
	if len(injected) != 2 || condensedCount != 1 {
		t.Fatalf("只应收敛超长的那一条: %d, %d", len(injected), condensedCount)
	}
	if !strings.HasPrefix(injected[0].ContentText, "甲甲") ||
		!strings.Contains(injected[0].ContentText, "此处省略") ||
		!strings.HasSuffix(injected[0].ContentText, "甲甲") {
		t.Fatalf("超长正文应保留首尾并标注省略: %q", injected[0].ContentText)
	}
	if longList[0].ContentText != strings.Repeat("甲", 5000) {
		t.Fatal("收敛不得改写原消息")
	}

	// 步骤11：归档全量。逐条带序号、角色标签、工具标签与时间，不丢条。
	archiveList := make([]ai.ArchiveMessage, 0, 6)
	for i := range 6 {
		archiveList = append(archiveList, ai.ArchiveMessage{
			Seq: i + 1, RoleCode: domain.ChatRoleUser, ContentText: "内容",
			CreatedAt: "2026-11-25T10:00:00+08:00",
		})
	}
	archiveList[1].RoleCode = domain.ChatRoleAssistant
	archiveList[2].ToolCodeCSV = "chapter_check"
	archiveText := ai.ArchiveHistory(archiveList, ai.ArchiveConfig{WithHeader: true})
	if !strings.Contains(archiveText, "共 6 条") {
		t.Fatalf("归档应标明总条数: %q", archiveText)
	}
	for i := range 6 {
		if !strings.Contains(archiveText, strconv.Itoa(i+1)+". ") {
			t.Fatalf("归档应逐条编号，第 %d 条缺失: %q", i+1, archiveText)
		}
	}
	if !strings.Contains(archiveText, "作者") || !strings.Contains(archiveText, "AI") {
		t.Fatalf("归档应带角色标签: %q", archiveText)
	}
	if !strings.Contains(archiveText, "［工具：chapter_check］") {
		t.Fatalf("归档应保留工具标签: %q", archiveText)
	}
	if !strings.Contains(archiveText, "2026-11-25T10:00:00+08:00") {
		t.Fatalf("归档应保留时间: %q", archiveText)
	}
	if ai.ArchiveHistory(nil, ai.ArchiveConfig{}) != "" {
		t.Fatal("空历史应返回空串")
	}
}

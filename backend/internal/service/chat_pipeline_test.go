// 本文件覆盖 AI 对话的三条长链路：技能解析、上下文装配、流式事件契约。
// 技能决定模板与参考资料，装配决定模型最终看到的块与消息序列，事件契约决定前端能否
// 从「思考中」走到「完成」；任一环节变化都会改变产出或让界面卡在等待态。

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
		SkillULID:      ulid,
		SkillCode:      code,
		SkillName:      code,
		ScopeCode:      scope,
		ProjectULID:    projectULID,
		EnabledFlag:    enabled,
		BuiltinFlag:    scope == domain.SkillScopeGlobal,
		PromptTemplate: "{{user_input}}",
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
type eventSink struct {
	eventList []domain.ChatStreamEventDTO
}

// Event 记录一条事件。
func (s *eventSink) Event(dto domain.ChatStreamEventDTO) error {
	s.eventList = append(s.eventList, dto)
	return nil
}

// Ping 记录一次心跳。
func (s *eventSink) Ping() error { return nil }

// TestSkillResolutionChain 覆盖技能链路：编码解析 → 作用域优先级 → 变量取值 →
// 模板占位符校验 → 参考资料关键词命中与预算截断 → 全局技能覆盖标记。
func TestSkillResolutionChain(t *testing.T) {
	// ---- 环节一：编码解析（显式编码优先；否则取正文首个 /code） ----
	code, remainder := resolveSkillCode("", "/continue 以林澈视角")
	if code != "continue" || remainder != "以林澈视角" {
		t.Fatalf("正文命令解析失败: %q %q", code, remainder)
	}
	code, remainder = resolveSkillCode("", "  /polish \t 加强画面感  \n")
	if code != "polish" || remainder != "加强画面感" {
		t.Fatalf("空白归一失败: %q %q", code, remainder)
	}
	// 显式编码由请求给出时正文不剥离，避免用户正文被静默改写。
	code, remainder = resolveSkillCode("polish", "/continue 以林澈视角")
	if code != "polish" || remainder != "/continue 以林澈视角" {
		t.Fatalf("显式编码应优先且不剥离正文: %q %q", code, remainder)
	}
	for _, contentText := range []string{"普通的正文开头，不该命中", "续写 /continue", "/continue"} {
		if parsedCode, _ := resolveSkillCode("", contentText); parsedCode != "" {
			t.Fatalf("content %q 未命中时不应产出技能编码: %q", contentText, parsedCode)
		}
	}

	// ---- 环节二：作用域优先级（启用项目技能 > 启用全局技能；停用一律不命中） ----
	projectSkill := skillDO("SKL_P", domain.SkillScopeProject, "PRJ_1", "polish", true)
	globalSkill := skillDO("SKL_G", domain.SkillScopeGlobal, "", "polish", true)
	if winner := pickSkillByScope(&projectSkill, &globalSkill); winner.SkillULID != "SKL_P" {
		t.Fatalf("项目技能应优先: %s", winner.SkillULID)
	}
	disabledProject := skillDO("SKL_P", domain.SkillScopeProject, "PRJ_1", "polish", false)
	if winner := pickSkillByScope(&disabledProject, &globalSkill); winner.SkillULID != "SKL_G" {
		t.Fatalf("停用的项目技能应回退全局技能: %s", winner.SkillULID)
	}
	disabledGlobal := skillDO("SKL_G", domain.SkillScopeGlobal, "", "polish", false)
	if winner := pickSkillByScope(nil, &disabledGlobal); winner != nil {
		t.Fatalf("停用的全局技能不应命中: %+v", winner)
	}

	// ---- 环节三：变量取值（显式传值 > 声明缺省值 > 必填缺失记入 missing） ----
	variableList := []domain.SkillVariableDO{
		{VariableName: "tone", VariableLabel: "语气", DefaultValue: "克制"},
		{VariableName: "scene", VariableLabel: "场景", RequiredFlag: true},
	}
	valueMap, missing := resolveSkillVariables("polish", variableList, map[string]string{"scene": "雨夜"}, "正文")
	if valueMap["tone"] != "克制" || valueMap["scene"] != "雨夜" ||
		valueMap["user_input"] != "正文" || valueMap["skill_code"] != "polish" {
		t.Fatalf("变量取值错误: %+v", valueMap)
	}
	if len(missing) != 0 {
		t.Fatalf("必填变量已提供时不应缺失: %v", missing)
	}
	_, missing = resolveSkillVariables("polish", variableList, nil, "正文")
	if len(missing) != 1 || missing[0] != "scene" {
		t.Fatalf("必填变量缺失应被记录: %v", missing)
	}
	// 显式空值不算已提供，回落到声明缺省值。
	valueMap, _ = resolveSkillVariables("polish", variableList, map[string]string{"tone": ""}, "正文")
	if valueMap["tone"] != "克制" {
		t.Fatalf("显式空值应回落缺省值: %+v", valueMap)
	}

	// ---- 环节四：模板占位符校验（未声明变量必须拒绝） ----
	if err := validateSkillTemplate("润色 {{user_input}} 与 {{tone}}", variableList); err != nil {
		t.Fatalf("已声明占位符不应报错: %v", err)
	}
	if err := validateSkillTemplate("润色 {{unknown_var}}", variableList); err == nil {
		t.Fatal("未声明占位符必须被拒绝")
	}

	// ---- 环节五：参考资料（空关键词总是命中；任一关键词出现即命中；超预算不注入） ----
	if !matchSkillReferenceKeywords("", []string{"任意正文"}) {
		t.Fatal("空关键词应总是命中")
	}
	if matchSkillReferenceKeywords("雨夜", []string{"晴天的午后"}) {
		t.Fatal("关键词未出现时不应命中")
	}
	if !matchSkillReferenceKeywords("雨夜,追车", []string{"晴天的午后", "追车开始"}) {
		t.Fatal("任一关键词命中即应命中")
	}
	longText := strings.Repeat("x", skillReferenceBudgetChars+100)
	if selected := selectSkillReferences([]domain.SkillReferenceDO{
		{RefULID: "REF_1", SkillULID: "SKL_G", RefName: "长文", RefText: longText},
	}, []string{"任意正文"}, skillReferenceBudgetChars); len(selected) != 0 {
		t.Fatalf("超预算资料不应被注入: %v", selected)
	}
	selected := selectSkillReferences([]domain.SkillReferenceDO{
		{RefULID: "REF_1", SkillULID: "SKL_G", RefName: "短文", RefText: "样例正文"},
	}, []string{"任意正文"}, skillReferenceBudgetChars)
	if len(selected) != 1 || !strings.Contains(selected[0], "【短文】") || !strings.Contains(selected[0], "样例正文") {
		t.Fatalf("参考资料注入格式错误: %v", selected)
	}

	// ---- 环节六：全局技能被同编码项目技能覆盖时须打标记 ----
	if !isOverriddenGlobalSkill(&globalSkill, []domain.SkillDO{globalSkill, projectSkill}) {
		t.Fatal("全局技能应被同编码项目技能标记为已覆盖")
	}
	if isOverriddenGlobalSkill(&globalSkill, []domain.SkillDO{globalSkill, disabledProject}) {
		t.Fatal("停用的项目技能不应覆盖全局技能")
	}
	if isOverriddenGlobalSkill(&globalSkill, []domain.SkillDO{globalSkill, skillDO("SKL_O", domain.SkillScopeProject, "PRJ_1", "continue", true)}) {
		t.Fatal("不同编码的项目技能不应覆盖全局技能")
	}
}

// TestChatContextChain 覆盖上下文装配链路：技能命中渲染 → @引用分优先级 → 设定注入排除私人笔记 →
// 同优先级稳定顺序 → 预算裁剪 → 历史选取 → 消息序列成型 → 显式引用会话归属校验。
func TestChatContextChain(t *testing.T) {
	// ---- 环节一：技能命中，模板占位符用其余块装配文本与变量填充 ----
	blocks, instruction := buildContextBlocks(chatContextInput{
		SkillTemplate: "润色指令：{{user_input}}｜语气：{{tone}}｜章节：{{chapter_content}}｜依据：{{context}}",
		SkillVariable: map[string]string{"tone": "克制"},
		SkillRefList:  []string{"【文风样例】\n短句为主"},
		UserInput:     "这段话",
		ChapterList:   []chapterSource{{ChapterName: "第一章", ContentText: "章节正文", IsCurrent: true}},
		DocList:       []docSource{{DocName: "设定集", FullText: "文档全文"}},
	})
	skillBlock := findBlock(blocks, "skill")
	if skillBlock == nil || skillBlock.Priority != ai.PrioritySkill {
		t.Fatalf("技能命中时应产出 skill 块且优先级最高: %+v", skillBlock)
	}
	if instruction != skillBlock.ContentText {
		t.Fatal("最终指令应为技能渲染结果")
	}
	for _, want := range []string{"润色指令：这段话", "语气：克制", "章节：章节正文", "文档全文"} {
		if !strings.Contains(instruction, want) {
			t.Fatalf("渲染结果缺少 %q: %q", want, instruction)
		}
	}
	refBlock := findBlock(blocks, "skill_reference")
	if refBlock == nil || refBlock.Priority != ai.PrioritySkillRef || !strings.Contains(refBlock.ContentText, "短句为主") {
		t.Fatalf("技能参考资料块缺失或优先级错误: %+v", refBlock)
	}

	// ---- 环节二：自由对话（技能未命中）直接用用户正文 ----
	freeBlocks, freeInstruction := buildContextBlocks(chatContextInput{
		UserInput:   "自由提问原文",
		ChapterList: []chapterSource{{ChapterName: "第一章", ContentText: "章节正文", IsCurrent: true}},
	})
	if freeInstruction != "自由提问原文" || findBlock(freeBlocks, "skill") != nil {
		t.Fatalf("技能未命中时应直接用用户正文且无 skill 块: %q", freeInstruction)
	}

	// ---- 环节三：@引用按当前章(90)/引用章与引用文档(80)分优先级 ----
	blocks, _ = buildContextBlocks(chatContextInput{
		UserInput: "提问",
		ChapterList: []chapterSource{
			{ChapterName: "第一章", ContentText: "当前章正文", IsCurrent: true},
			{ChapterName: "第二章", ContentText: "引用章正文"},
		},
		DocList: []docSource{{DocName: "设定集", FullText: "文档全部切片拼接"}},
	})
	if current := findBlock(blocks, "current_chapter"); current == nil ||
		current.Priority != ai.PriorityCurrentChapter || !strings.Contains(current.ContentText, "当前章正文") {
		t.Fatalf("当前章块缺失或优先级错误: %+v", current)
	}
	if refChapter := findBlock(blocks, "ref_chapter"); refChapter == nil ||
		refChapter.Priority != ai.PriorityExplicitRef || !strings.Contains(refChapter.ContentText, "引用章正文") {
		t.Fatalf("引用章块缺失或优先级错误: %+v", refChapter)
	}
	if refDoc := findBlock(blocks, "ref_doc"); refDoc == nil ||
		refDoc.Priority != ai.PriorityExplicitRef || !strings.Contains(refDoc.ContentText, "文档全部切片拼接") {
		t.Fatalf("引用文档块缺失或优先级错误: %+v", refDoc)
	}

	// ---- 环节四：设定条目注入，私人笔记(note)必须被排除 ----
	blocks, _ = buildContextBlocks(chatContextInput{
		UserInput: "提问",
		LoreList: []domain.LoreDO{
			{LoreName: "灵力体系", LoreTypeCode: "rule", ContentText: "设定内容正文"},
			{LoreName: "私人笔记", LoreTypeCode: loreTypeCodeNote, ContentText: "笔记内容正文"},
		},
	})
	lore := findBlock(blocks, "lore")
	if lore == nil || lore.Priority != ai.PriorityCharacter ||
		!strings.Contains(lore.ContentText, "设定内容正文") {
		t.Fatalf("lore 块缺失或内容错误: %+v", lore)
	}
	if strings.Contains(lore.ContentText, "笔记内容正文") {
		t.Fatalf("note 私人笔记禁止注入: %q", lore.ContentText)
	}

	// ---- 环节五：同优先级保持输入顺序（角色卡在设定之前），保证输出确定性 ----
	blocks, _ = buildContextBlocks(chatContextInput{
		UserInput:     "提问",
		CharacterList: []domain.CharacterDO{{CharacterName: "林澈"}},
		LoreList:      []domain.LoreDO{{LoreName: "灵力体系", LoreTypeCode: "rule", ContentText: "设定内容正文"}},
	})
	assembled, _ := ai.AssembleContext(blocks, ai.ContextBudgetChars)
	if strings.Index(assembled, "林澈") > strings.Index(assembled, "灵力体系") {
		t.Fatalf("同优先级应保持输入顺序: %q", assembled)
	}

	// ---- 环节六：预算超限时按优先级丢块并记录块键 ----
	blocks, _ = buildContextBlocks(chatContextInput{
		UserInput:   "提问",
		DocList:     []docSource{{DocName: "A", FullText: strings.Repeat("d", 100)}},
		RagList:     []domain.KnowledgeChunkVO{{DocName: "B", ChunkText: strings.Repeat("r", 100)}},
		SummaryList: []string{strings.Repeat("s", 100)},
		HistoryList: []domain.ChatMessageDO{chatMessageOf("MSG_1", domain.ChatRoleUser, strings.Repeat("h", 100))},
	})
	assembled, dropped := ai.AssembleContext(blocks, 300)
	droppedText := strings.Join(dropped, ",")
	if !strings.Contains(droppedText, "chapter_summary") || !strings.Contains(droppedText, "history") {
		t.Fatalf("应丢弃 chapter_summary 与 history，实际 %v", dropped)
	}
	if strings.Contains(droppedText, "ref_doc") || strings.Contains(droppedText, "rag") {
		t.Fatalf("高优先级块不应被丢弃，实际 %v", dropped)
	}
	if !strings.Contains(assembled, strings.Repeat("d", 100)) || strings.Contains(assembled, strings.Repeat("h", 100)) {
		t.Fatalf("装配文本应只保留高优先级块: %q", assembled)
	}

	// ---- 环节七：历史选取（最近 N 条 + 显式引用必注入且去重置尾） ----
	messages := []domain.ChatMessageDO{
		chatMessageOf("MSG_1", domain.ChatRoleUser, "一"),
		chatMessageOf("MSG_2", domain.ChatRoleAssistant, "二"),
		chatMessageOf("MSG_3", domain.ChatRoleUser, "三"),
		chatMessageOf("MSG_4", domain.ChatRoleAssistant, "四"),
		chatMessageOf("MSG_5", domain.ChatRoleUser, "五"),
	}
	selected := selectHistoryMessages(messages, nil, 2)
	if len(selected) != 2 || selected[0].MessageULID != "MSG_4" || selected[1].MessageULID != "MSG_5" {
		t.Fatalf("最近 N 条应旧到新返回 [MSG_4 MSG_5]，实际 %v", selected)
	}
	selected = selectHistoryMessages(messages, []string{"MSG_1", "MSG_4", "MSG_1"}, 2)
	wantSelected := []string{"MSG_4", "MSG_5", "MSG_1"}
	if len(selected) != len(wantSelected) {
		t.Fatalf("len = %d, want %d", len(selected), len(wantSelected))
	}
	for i := range wantSelected {
		if selected[i].MessageULID != wantSelected[i] {
			t.Fatalf("selected[%d] = %s, want %s", i, selected[i].MessageULID, wantSelected[i])
		}
	}
	if selected = selectHistoryMessages(messages, []string{"MSG_2"}, 0); len(selected) != 1 || selected[0].MessageULID != "MSG_2" {
		t.Fatalf("N 为 0 时只应注入引用消息，实际 %v", selected)
	}

	// ---- 环节八：消息序列 system → 【上下文】 → 历史 role 映射 → 最终指令 ----
	history := []domain.ChatMessageDO{
		chatMessageOf("MSG_1", domain.ChatRoleUser, "历史问"),
		chatMessageOf("MSG_2", domain.ChatRoleAssistant, "历史答"),
	}
	messagesOut := buildChatMessages("", "文风约束", "装配文本", "最终指令", history)
	if len(messagesOut) != 5 {
		t.Fatalf("len = %d, want 5", len(messagesOut))
	}
	if !strings.Contains(messagesOut[0].Content, "文风约束") {
		t.Fatal("首条 system 提示词应注入项目文风")
	}
	if !strings.HasPrefix(messagesOut[1].Content, "【上下文】") || !strings.Contains(messagesOut[1].Content, "装配文本") {
		t.Fatalf("第二条应为【上下文】装配文本: %q", messagesOut[1].Content)
	}
	if messagesOut[2].Role != "user" || messagesOut[2].Content != "历史问" ||
		messagesOut[3].Role != "assistant" || messagesOut[3].Content != "历史答" {
		t.Fatalf("历史应按 role 映射注入: %+v", messagesOut[2:4])
	}
	if messagesOut[4].Content != "最终指令" {
		t.Fatalf("末条应为最终指令: %q", messagesOut[4].Content)
	}
	if messagesOut = buildChatMessages("", "文风约束", "", "最终指令", nil); len(messagesOut) != 2 || messagesOut[1].Content != "最终指令" {
		t.Fatalf("装配文本为空时应省略上下文消息: %+v", messagesOut)
	}

	// ---- 环节九：显式引用消息的会话归属校验（去重保序，会话外必须拒绝） ----
	sessionMessages := []domain.ChatMessageDO{
		chatMessageOf("MSG_1", domain.ChatRoleUser, "一"),
		chatMessageOf("MSG_2", domain.ChatRoleAssistant, "二"),
	}
	validated, err := validateRefMessages(sessionMessages, []string{"MSG_2", "MSG_1", "MSG_2"})
	if err != nil {
		t.Fatalf("会话内引用应通过校验: %v", err)
	}
	if len(validated) != 2 || validated[0] != "MSG_2" || validated[1] != "MSG_1" {
		t.Fatalf("引用应去重保序: %v", validated)
	}
	if _, err := validateRefMessages(sessionMessages, []string{"MSG_999"}); err == nil {
		t.Fatal("引用会话外消息必须失败")
	}
}

// TestChatStreamContract 覆盖流式事件契约：thinking/chunk/done 顺序、seq 从 1 单调递增、
// done 携带完整思考过程与空数组引用。
// 前端据此在「等待 → 思考 → 正文 → 完成」之间切换；done 缺 reasoning_text 会让重开会话丢失思考，
// reference_list 为 null 会让依据面板报错。
func TestChatStreamContract(t *testing.T) {
	sink := &eventSink{}
	writer := &chatEventWriter{writer: sink, sessionULID: "SES_1", messageULID: "MSG_1"}
	if err := writer.emit(domain.StreamEventThinking, "先想一步", "", nil); err != nil {
		t.Fatalf("thinking emit 失败: %v", err)
	}
	if err := writer.emit(domain.StreamEventChunk, "正文", "", nil); err != nil {
		t.Fatalf("chunk emit 失败: %v", err)
	}
	if err := writer.emitWithReasoning(domain.StreamEventDone, "正文完", "先想一步", "", nil); err != nil {
		t.Fatalf("done emit 失败: %v", err)
	}
	if len(sink.eventList) != 3 {
		t.Fatalf("事件数 = %d, want 3", len(sink.eventList))
	}
	if sink.eventList[0].EventType != domain.StreamEventThinking || sink.eventList[0].ContentText != "先想一步" {
		t.Fatalf("首条应为 thinking 增量: %+v", sink.eventList[0])
	}
	doneEvent := sink.eventList[2]
	if doneEvent.EventType != domain.StreamEventDone || doneEvent.ReasoningText != "先想一步" {
		t.Fatalf("done 应携带完整思考过程: %+v", doneEvent)
	}
	if doneEvent.ReferenceList == nil {
		t.Fatal("done 的 reference_list 应为空数组而非 null")
	}
	for i := range sink.eventList {
		if sink.eventList[i].Seq != int64(i+1) {
			t.Fatalf("seq 应单调递增: %+v", sink.eventList)
		}
	}
}

// 本文件覆盖 AI 对话请求准备链路的全部环节：
// 正文解析（斜杠命令）→ 技能命中渲染 → @引用分优先级注入 → 设定注入且私人笔记排除 →
// 预算裁剪丢块 → 历史选取 → 消息序列成型 → 显式引用会话归属校验。
// 这条链路决定了模型实际看到的全部输入，任何一段变化都会改变 AI 产出，因此合并为一条用例。

package service

import (
	"strings"
	"testing"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

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

// TestChatRequestPreparationChain 覆盖 AI 对话请求准备链路。
func TestChatRequestPreparationChain(t *testing.T) {
	// ---- 环节一：正文首 token 的 /code 解析，命中才剥离命令与参数 ----
	skillCode, remainder := parseSlashToken("/continue 以林澈视角")
	if skillCode != "continue" || remainder != "以林澈视角" {
		t.Fatalf("命令与参数分离失败: %q %q", skillCode, remainder)
	}
	skillCode, remainder = parseSlashToken("  /polish \t 加强画面感  \n")
	if skillCode != "polish" || remainder != "加强画面感" {
		t.Fatalf("空白归一失败: %q %q", skillCode, remainder)
	}
	for _, contentText := range []string{"普通的正文开头，不该命中", "续写 /continue", "/continue"} {
		parsedCode, parsedRemainder := parseSlashToken(contentText)
		if parsedCode != "" {
			t.Fatalf("content %q 未命中时不应产出技能编码: %q", contentText, parsedCode)
		}
		if contentText != "/continue" && parsedRemainder != contentText {
			t.Fatalf("content %q 未命中时应原样保留: %q", contentText, parsedRemainder)
		}
	}

	// ---- 环节一之二：技能变量取值（显式传值 > 声明缺省值 > 必填缺失） ----
	variableList := []domain.SkillVariableDO{
		{VariableName: "tone", VariableLabel: "语气", DefaultValue: "克制"},
		{VariableName: "scene", VariableLabel: "场景", RequiredFlag: true},
	}
	valueMap, missing := resolveSkillVariables("polish", variableList, map[string]string{"scene": "雨夜"}, "正文")
	if valueMap["tone"] != "克制" || valueMap["scene"] != "雨夜" || valueMap["user_input"] != "正文" {
		t.Fatalf("变量取值错误: %+v", valueMap)
	}
	if len(missing) != 0 {
		t.Fatalf("必填变量已提供时不应缺失: %v", missing)
	}
	_, missing = resolveSkillVariables("polish", variableList, nil, "正文")
	if len(missing) != 1 || missing[0] != "scene" {
		t.Fatalf("必填变量缺失应被记录: %v", missing)
	}

	// ---- 环节一之三：参考资料关键词命中，空关键词总是注入 ----
	if !matchSkillReferenceKeywords("", []string{"任意正文"}) {
		t.Fatal("空关键词应总是命中")
	}
	if !matchSkillReferenceKeywords("雨夜, 追车", []string{"这是一个雨夜"}) {
		t.Fatal("关键词出现在正文中应命中")
	}
	if matchSkillReferenceKeywords("雨夜", []string{"晴天的午后"}) {
		t.Fatal("关键词未出现时不应命中")
	}

	// ---- 环节二：技能命中，模板占位符用其余块装配文本与变量填充 ----
	input := chatContextInput{
		SkillTemplate: "润色指令：{{user_input}}｜语气：{{tone}}｜章节：{{chapter_content}}｜依据：{{context}}",
		SkillVariable: map[string]string{"tone": "克制"},
		SkillRefList:  []string{"【文风样例】\n短句为主"},
		UserInput:     "这段话",
		ChapterList:   []chapterSource{{ChapterName: "第一章", ContentText: "章节正文", IsCurrent: true}},
		DocList:       []docSource{{DocName: "设定集", FullText: "文档全文"}},
	}
	blocks, instruction := buildContextBlocks(input)
	skillBlock := findBlock(blocks, "skill")
	if skillBlock == nil {
		t.Fatal("技能命中时应产出 skill 块")
	}
	if skillBlock.Priority != ai.PrioritySkill {
		t.Fatalf("skill 块优先级 = %d, want %d", skillBlock.Priority, ai.PrioritySkill)
	}
	if instruction != skillBlock.ContentText {
		t.Fatal("最终指令应为技能渲染结果")
	}
	for _, want := range []string{"润色指令：这段话", "语气：克制", "章节：章节正文", "依据：", "文档全文"} {
		if !strings.Contains(instruction, want) {
			t.Fatalf("渲染结果缺少 %q: %q", want, instruction)
		}
	}
	refBlock := findBlock(blocks, "skill_reference")
	if refBlock == nil || refBlock.Priority != ai.PrioritySkillRef || !strings.Contains(refBlock.ContentText, "短句为主") {
		t.Fatalf("技能参考资料块缺失或优先级错误: %+v", refBlock)
	}

	// ---- 环节三：自由对话（技能未命中）直接用用户正文 ----
	freeBlocks, freeInstruction := buildContextBlocks(chatContextInput{
		UserInput:   "自由提问原文",
		ChapterList: []chapterSource{{ChapterName: "第一章", ContentText: "章节正文", IsCurrent: true}},
	})
	if freeInstruction != "自由提问原文" {
		t.Fatalf("最终指令 = %q, want 自由提问原文", freeInstruction)
	}
	if findBlock(freeBlocks, "skill") != nil {
		t.Fatal("技能未命中时不应有 skill 块")
	}

	// ---- 环节四：@引用按当前章(90)/引用章与引用文档(80)分优先级 ----
	blocks, _ = buildContextBlocks(chatContextInput{
		UserInput: "提问",
		ChapterList: []chapterSource{
			{ChapterName: "第一章", ContentText: "当前章正文", IsCurrent: true},
			{ChapterName: "第二章", ContentText: "引用章正文"},
		},
		DocList: []docSource{{DocName: "设定集", FullText: "文档全部切片拼接"}},
	})
	current := findBlock(blocks, "current_chapter")
	if current == nil || current.Priority != ai.PriorityCurrentChapter || !strings.Contains(current.ContentText, "当前章正文") {
		t.Fatalf("当前章块缺失或优先级错误: %+v", current)
	}
	refChapter := findBlock(blocks, "ref_chapter")
	if refChapter == nil || refChapter.Priority != ai.PriorityExplicitRef || !strings.Contains(refChapter.ContentText, "引用章正文") {
		t.Fatalf("引用章块缺失或优先级错误: %+v", refChapter)
	}
	refDoc := findBlock(blocks, "ref_doc")
	if refDoc == nil || refDoc.Priority != ai.PriorityExplicitRef || !strings.Contains(refDoc.ContentText, "文档全部切片拼接") {
		t.Fatalf("引用文档块缺失或优先级错误: %+v", refDoc)
	}

	// ---- 环节五：设定条目注入，私人笔记(note)必须被排除 ----
	blocks, _ = buildContextBlocks(chatContextInput{
		UserInput: "提问",
		LoreList: []domain.LoreDO{
			{LoreName: "灵力体系", LoreTypeCode: "rule", ContentText: "设定内容正文"},
			{LoreName: "私人笔记", LoreTypeCode: loreTypeCodeNote, ContentText: "笔记内容正文"},
		},
	})
	lore := findBlock(blocks, "lore")
	if lore == nil {
		t.Fatal("有可用设定条目时应产出 lore 块")
	}
	if lore.Priority != ai.PriorityCharacter {
		t.Fatalf("lore 块优先级 = %d, want %d", lore.Priority, ai.PriorityCharacter)
	}
	if !strings.Contains(lore.ContentText, "灵力体系") || !strings.Contains(lore.ContentText, "设定内容正文") {
		t.Fatalf("lore 块应含设定名与内容: %q", lore.ContentText)
	}
	if strings.Contains(lore.ContentText, "笔记内容正文") || strings.Contains(lore.ContentText, "私人笔记") {
		t.Fatalf("note 私人笔记禁止注入: %q", lore.ContentText)
	}
	blocks, _ = buildContextBlocks(chatContextInput{
		UserInput: "提问",
		LoreList:  []domain.LoreDO{{LoreName: "私人笔记", LoreTypeCode: loreTypeCodeNote, ContentText: "笔记内容正文"}},
	})
	if findBlock(blocks, "lore") != nil {
		t.Fatal("全部为 note 时不应产出 lore 块")
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
	if !strings.Contains(assembled, strings.Repeat("d", 100)) || !strings.Contains(assembled, strings.Repeat("r", 100)) {
		t.Fatal("装配文本应保留高优先级块")
	}
	if strings.Contains(assembled, strings.Repeat("h", 100)) {
		t.Fatal("被丢弃块不应进入装配文本")
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
	selected = selectHistoryMessages(messages, []string{"MSG_2"}, 0)
	if len(selected) != 1 || selected[0].MessageULID != "MSG_2" {
		t.Fatalf("N 为 0 时只应注入引用消息，实际 %v", selected)
	}

	// ---- 环节八：消息序列 system → 【上下文】 → 历史 role 映射 → 最终指令 ----
	history := []domain.ChatMessageDO{
		chatMessageOf("MSG_1", domain.ChatRoleUser, "历史问"),
		chatMessageOf("MSG_2", domain.ChatRoleAssistant, "历史答"),
	}
	messagesOut := buildChatMessages("文风约束", "装配文本", "最终指令", history)
	if len(messagesOut) != 5 {
		t.Fatalf("len = %d, want 5", len(messagesOut))
	}
	if messagesOut[0].Content != ai.SystemPrompt("文风约束") {
		t.Fatal("首条应为 system 提示词")
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
	messagesOut = buildChatMessages("文风约束", "", "最终指令", nil)
	if len(messagesOut) != 2 || messagesOut[1].Content != "最终指令" {
		t.Fatalf("装配文本为空时应省略上下文消息: %+v", messagesOut)
	}

	// ---- 环节九：显式引用消息的会话归属校验 ----
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

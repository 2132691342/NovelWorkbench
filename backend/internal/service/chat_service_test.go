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

// TestBuildContextBlocksSkillHit 技能命中：占位符渲染进技能块与最终指令，技能块优先级最高。
func TestBuildContextBlocksSkillHit(t *testing.T) {
	input := chatContextInput{
		SkillTemplate: "润色指令：{{user_input}}｜章节：{{chapter_content}}｜依据：{{context}}",
		UserInput:     "这段话",
		ChapterList:   []chapterSource{{ChapterName: "第一章", ContentText: "章节正文", IsCurrent: true}},
		DocList:       []docSource{{DocName: "设定集", FullText: "文档全文"}},
	}
	blocks, instruction := buildContextBlocks(input)
	skillBlock := findBlock(blocks, "skill")
	if skillBlock == nil {
		t.Fatalf("技能命中时应有 skill 块")
	}
	if skillBlock.Priority != ai.PrioritySkill {
		t.Fatalf("技能块优先级 = %d, want %d", skillBlock.Priority, ai.PrioritySkill)
	}
	if instruction != skillBlock.ContentText {
		t.Fatalf("最终指令应为技能渲染结果")
	}
	for _, want := range []string{"润色指令：这段话", "章节：章节正文", "依据："} {
		if !strings.Contains(instruction, want) {
			t.Fatalf("渲染结果缺少 %q: %q", want, instruction)
		}
	}
	if !strings.Contains(instruction, "文档全文") {
		t.Fatalf("context 占位符应为其余块先装配文本: %q", instruction)
	}
}

// TestBuildContextBlocksSkillMiss 技能未命中：无技能块，最终指令为用户正文原文。
func TestBuildContextBlocksSkillMiss(t *testing.T) {
	input := chatContextInput{
		UserInput:   "自由提问原文",
		ChapterList: []chapterSource{{ChapterName: "第一章", ContentText: "章节正文", IsCurrent: true}},
	}
	blocks, instruction := buildContextBlocks(input)
	if instruction != "自由提问原文" {
		t.Fatalf("最终指令 = %q, want 自由提问原文", instruction)
	}
	if findBlock(blocks, "skill") != nil {
		t.Fatalf("技能未命中时不应有 skill 块")
	}
}

// TestBuildContextBlocksRefInjection @引用注入：当前章 90、引用章/引用文档 80，内容进入对应块。
func TestBuildContextBlocksRefInjection(t *testing.T) {
	input := chatContextInput{
		UserInput: "提问",
		ChapterList: []chapterSource{
			{ChapterName: "第一章", ContentText: "当前章正文", IsCurrent: true},
			{ChapterName: "第二章", ContentText: "引用章正文"},
		},
		DocList: []docSource{{DocName: "设定集", FullText: "文档全部切片拼接"}},
	}
	blocks, _ := buildContextBlocks(input)
	current := findBlock(blocks, "current_chapter")
	if current == nil || current.Priority != ai.PriorityCurrentChapter || !strings.Contains(current.ContentText, "当前章正文") {
		t.Fatalf("当前章块缺失或优先级错误: %+v", current)
	}
	ref := findBlock(blocks, "ref_chapter")
	if ref == nil || ref.Priority != ai.PriorityExplicitRef || !strings.Contains(ref.ContentText, "引用章正文") {
		t.Fatalf("引用章块缺失或优先级错误: %+v", ref)
	}
	doc := findBlock(blocks, "ref_doc")
	if doc == nil || doc.Priority != ai.PriorityExplicitRef || !strings.Contains(doc.ContentText, "文档全部切片拼接") {
		t.Fatalf("引用文档块缺失或优先级错误: %+v", doc)
	}
}

// TestBuildContextBlocksLore 设定条目注入：lore 块含设定名/内容（截断），note 私人笔记排除。
func TestBuildContextBlocksLore(t *testing.T) {
	input := chatContextInput{
		UserInput: "提问",
		LoreList: []domain.LoreDO{
			{LoreName: "灵力体系", LoreTypeCode: "rule", ContentText: "设定内容正文"},
			{LoreName: "私人笔记", LoreTypeCode: loreTypeCodeNote, ContentText: "笔记内容正文"},
		},
	}
	blocks, _ := buildContextBlocks(input)
	lore := findBlock(blocks, "lore")
	if lore == nil {
		t.Fatalf("有可用设定条目时应有 lore 块")
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

	t.Run("全部为 note 时无 lore 块", func(t *testing.T) {
		input.LoreList = []domain.LoreDO{{LoreName: "私人笔记", LoreTypeCode: loreTypeCodeNote, ContentText: "笔记内容正文"}}
		blocks, _ := buildContextBlocks(input)
		if findBlock(blocks, "lore") != nil {
			t.Fatalf("全部为 note 时不应有 lore 块")
		}
	})
}

// TestSelectHistoryMessages 历史选择：最近 N 条截断（旧→新）、显式引用必注入、去重后置尾。
func TestSelectHistoryMessages(t *testing.T) {
	messages := []domain.ChatMessageDO{
		chatMessageOf("MSG_1", domain.ChatRoleUser, "一"),
		chatMessageOf("MSG_2", domain.ChatRoleAssistant, "二"),
		chatMessageOf("MSG_3", domain.ChatRoleUser, "三"),
		chatMessageOf("MSG_4", domain.ChatRoleAssistant, "四"),
		chatMessageOf("MSG_5", domain.ChatRoleUser, "五"),
	}

	t.Run("最近 N 条旧到新", func(t *testing.T) {
		selected := selectHistoryMessages(messages, nil, 2)
		if len(selected) != 2 || selected[0].MessageULID != "MSG_4" || selected[1].MessageULID != "MSG_5" {
			t.Fatalf("selected = %v, want [MSG_4 MSG_5]", selected)
		}
	})

	t.Run("显式引用必注入且去重置尾", func(t *testing.T) {
		selected := selectHistoryMessages(messages, []string{"MSG_1", "MSG_4", "MSG_1"}, 2)
		want := []string{"MSG_4", "MSG_5", "MSG_1"}
		if len(selected) != len(want) {
			t.Fatalf("len = %d, want %d", len(selected), len(want))
		}
		for i := range want {
			if selected[i].MessageULID != want[i] {
				t.Fatalf("selected[%d] = %s, want %s", i, selected[i].MessageULID, want[i])
			}
		}
	})

	t.Run("N 为 0 仅注入引用", func(t *testing.T) {
		selected := selectHistoryMessages(messages, []string{"MSG_2"}, 0)
		if len(selected) != 1 || selected[0].MessageULID != "MSG_2" {
			t.Fatalf("selected = %v, want [MSG_2]", selected)
		}
	})

	t.Run("N 超过总数时全量注入", func(t *testing.T) {
		selected := selectHistoryMessages(messages, nil, 50)
		if len(selected) != len(messages) {
			t.Fatalf("len = %d, want %d", len(selected), len(messages))
		}
	})
}

// TestAssembleBudgetDrop 预算超限：低优先级块丢弃且标注块键。
func TestAssembleBudgetDrop(t *testing.T) {
	input := chatContextInput{
		UserInput:   "提问",
		DocList:     []docSource{{DocName: "A", FullText: strings.Repeat("d", 100)}},
		RagList:     []domain.KnowledgeChunkVO{{DocName: "B", ChunkText: strings.Repeat("r", 100)}},
		SummaryList: []string{strings.Repeat("s", 100)},
		HistoryList: []domain.ChatMessageDO{chatMessageOf("MSG_1", domain.ChatRoleUser, strings.Repeat("h", 100))},
	}
	blocks, _ := buildContextBlocks(input)
	assembled, dropped := ai.AssembleContext(blocks, 300)
	droppedKeys := strings.Join(dropped, ",")
	if !strings.Contains(droppedKeys, "chapter_summary") || !strings.Contains(droppedKeys, "history") {
		t.Fatalf("期望丢弃 chapter_summary 与 history，实际 %v", dropped)
	}
	if strings.Contains(droppedKeys, "ref_doc") || strings.Contains(droppedKeys, "rag") {
		t.Fatalf("高优先级块不应被丢弃，实际 %v", dropped)
	}
	if !strings.Contains(assembled, strings.Repeat("d", 100)) || !strings.Contains(assembled, strings.Repeat("r", 100)) {
		t.Fatalf("装配文本应保留高优先级块")
	}
	if strings.Contains(assembled, strings.Repeat("h", 100)) {
		t.Fatalf("被丢弃块不应进入装配文本")
	}
}

// TestBuildChatMessages 消息序列：system → 【上下文】 → 历史 role 映射 → 最终指令，上下文为空省略。
func TestBuildChatMessages(t *testing.T) {
	history := []domain.ChatMessageDO{
		chatMessageOf("MSG_1", domain.ChatRoleUser, "历史问"),
		chatMessageOf("MSG_2", domain.ChatRoleAssistant, "历史答"),
	}
	messages := buildChatMessages("文风约束", "装配文本", "最终指令", history)
	if len(messages) != 5 {
		t.Fatalf("len = %d, want 5", len(messages))
	}
	if messages[0].Content != ai.SystemPrompt("文风约束") {
		t.Fatalf("首条应为 system 提示词")
	}
	if !strings.HasPrefix(messages[1].Content, "【上下文】") || !strings.Contains(messages[1].Content, "装配文本") {
		t.Fatalf("第二条应为【上下文】装配文本: %q", messages[1].Content)
	}
	if messages[2].Role != "user" || messages[2].Content != "历史问" || messages[3].Role != "assistant" || messages[3].Content != "历史答" {
		t.Fatalf("历史应按 role 映射注入: %+v", messages[2:4])
	}
	if messages[4].Content != "最终指令" {
		t.Fatalf("末条应为最终指令: %q", messages[4].Content)
	}

	t.Run("装配文本为空省略上下文消息", func(t *testing.T) {
		messages := buildChatMessages("文风约束", "", "最终指令", nil)
		if len(messages) != 2 {
			t.Fatalf("len = %d, want 2", len(messages))
		}
		if messages[1].Content != "最终指令" {
			t.Fatalf("末条应为最终指令: %q", messages[1].Content)
		}
	})
}

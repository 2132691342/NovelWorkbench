// 本文件实现 AI 对话的上下文装配的纯函数部分：上下文块构造、历史收敛与消息序列成型。
// 关键约定：注入的提示词与上下文一律以系统级消息进入消息序列，用户输入始终是用户级消息。
// 素材加载（数据库与检索访问）在 chat_loader.go。

package service

import (
	"strings"
	"unicode/utf8"

	"github.com/cloudwego/eino/schema"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

const (
	characterPersonalityRunes = 120 // 角色性格截断
	characterBackgroundRunes  = 200 // 角色背景截断
	loreContentRunes          = 300 // 设定条目内容截断
	sessionNameRunes          = 20  // 会话缺省名取首条消息字数
)

// loreTypeCodeNote 创作笔记类型：作者私人笔记，禁止注入 AI 上下文。
const loreTypeCodeNote = "note"

// chapterSource 章节上下文来源。
type chapterSource struct {
	ChapterULID string // 章节 ULID（模型调工具未给章节时补齐用）
	ChapterName string // 章节名
	ContentText string // 章节正文
	IsCurrent   bool   // 是否当前章（当前章优先级 90，引用章 80）
}

// currentChapterULIDOf 取当前章 ULID（无当前章返回空，工具据此退回用正文）。
func currentChapterULIDOf(input chatContextInput) string {
	for i := range input.ChapterList {
		if input.ChapterList[i].IsCurrent {
			return input.ChapterList[i].ChapterULID
		}
	}
	return ""
}

// docSource @引用文档上下文来源。
type docSource struct {
	DocName  string // 文档名
	FullText string // 全部切片拼接文本
}

// chatContextInput 上下文装配输入（纯数据，供 buildContextBlocks 消费，便于测试注入）。
type chatContextInput struct {
	UserInput        string                    // 解析后的用户正文（提示词占位符用）
	ChapterList      []chapterSource           // 当前章与引用章（首项为当前章）
	DocList          []docSource               // @文档
	RagList          []domain.KnowledgeChunkVO // 知识库召回切片
	RefSummaryList   []string                  // @章节摘要正文
	RefCharacterList []domain.CharacterDO      // @角色卡（仅显式引用的那几张）
	RefLoreList      []domain.LoreDO           // @设定条目（仅显式引用的那几条）
	RefSessionList   []sessionSource           // @历史会话（已本地压缩的摘要文本）
	SummaryList      []string                  // 当前章前后章梗概
	CharacterList    []domain.CharacterDO      // 项目角色卡（全量，低优先级）
	LoreList         []domain.LoreDO           // 项目设定条目（全量，低优先级；note 类型禁止注入）
}

// sessionSource @历史会话上下文来源。
type sessionSource struct {
	SessionName string // 会话名
	ContentText string // 压缩后的会话摘要
}

// buildContextBlocks 组装上下文块（纯函数）。
// 块顺序即同优先级稳定顺序：章节 → 文档 → 摘要 → 角色 → 设定 → 历史会话 → 召回 → 梗概 → 全量角色 → 全量设定。
func buildContextBlocks(input chatContextInput) []ai.ContextBlock {
	blocks := make([]ai.ContextBlock, 0, len(input.ChapterList)+len(input.DocList)+8)
	for i := range input.ChapterList {
		chapter := &input.ChapterList[i]
		if strings.TrimSpace(chapter.ContentText) == "" {
			continue
		}
		block := ai.ContextBlock{BlockKey: "ref_chapter", Priority: ai.PriorityExplicitRef,
			ContentText: "【引用章节】" + chapter.ChapterName + "\n" + chapter.ContentText}
		if chapter.IsCurrent {
			block.BlockKey = "current_chapter"
			block.Priority = ai.PriorityCurrentChapter
			block.ContentText = "【当前章节】" + chapter.ChapterName + "\n" + chapter.ContentText
		}
		blocks = append(blocks, block)
	}
	for i := range input.DocList {
		doc := &input.DocList[i]
		if strings.TrimSpace(doc.FullText) == "" {
			continue
		}
		blocks = append(blocks, ai.ContextBlock{
			BlockKey:    "ref_doc",
			Priority:    ai.PriorityExplicitRef,
			ContentText: "【引用文档】" + doc.DocName + "\n" + doc.FullText,
		})
	}
	if len(input.RefSummaryList) > 0 {
		blocks = append(blocks, ai.ContextBlock{
			BlockKey:    "ref_summary",
			Priority:    ai.PriorityExplicitRef,
			ContentText: "【引用章节摘要】\n" + strings.Join(input.RefSummaryList, "\n"),
		})
	}
	if len(input.RefCharacterList) > 0 {
		blocks = append(blocks, ai.ContextBlock{
			BlockKey:    "ref_character",
			Priority:    ai.PriorityExplicitRef,
			ContentText: formatCharacterBlock(input.RefCharacterList),
		})
	}
	if loreText := formatLoreBlock(input.RefLoreList); loreText != "" {
		blocks = append(blocks, ai.ContextBlock{
			BlockKey:    "ref_lore",
			Priority:    ai.PriorityExplicitRef,
			ContentText: loreText,
		})
	}
	// 历史会话整段压缩后注入：作者 @ 的是「那次聊过什么」，不需要逐条原文
	if len(input.RefSessionList) > 0 {
		var builder strings.Builder
		builder.WriteString("【引用历史会话】")
		for i := range input.RefSessionList {
			builder.WriteString("\n《" + input.RefSessionList[i].SessionName + "》\n")
			builder.WriteString(input.RefSessionList[i].ContentText)
		}
		blocks = append(blocks, ai.ContextBlock{
			BlockKey:    "ref_session",
			Priority:    ai.PriorityExplicitRef,
			ContentText: builder.String(),
		})
	}
	if len(input.RagList) > 0 {
		var builder strings.Builder
		builder.WriteString("【知识库召回】")
		for i := range input.RagList {
			builder.WriteString("\n《" + input.RagList[i].DocName + "》" + input.RagList[i].ChunkText)
		}
		blocks = append(blocks, ai.ContextBlock{BlockKey: "rag", Priority: ai.PriorityRAG, ContentText: builder.String()})
	}
	if len(input.SummaryList) > 0 {
		blocks = append(blocks, ai.ContextBlock{
			BlockKey:    "chapter_summary",
			Priority:    ai.PriorityChapterSummary,
			ContentText: "【章节梗概】\n" + strings.Join(input.SummaryList, "\n"),
		})
	}
	if len(input.CharacterList) > 0 {
		blocks = append(blocks, ai.ContextBlock{
			BlockKey:    "character",
			Priority:    ai.PriorityCharacter,
			ContentText: formatCharacterBlock(input.CharacterList),
		})
	}
	if loreText := formatLoreBlock(input.LoreList); loreText != "" {
		blocks = append(blocks, ai.ContextBlock{BlockKey: "lore", Priority: ai.PriorityCharacter, ContentText: loreText})
	}
	// 历史不注入上下文块：它以原生 role 消息追加到消息序列，两处同时注入会让模型看到两份对话。
	// 工具结果同样不在这里：模型主动调用的工具结果以 tool 消息回到模型，作者显式 #code 的结果并入指令消息。
	return blocks
}

// joinChapterContent 拼接引用章节正文（当前章在前）。
func joinChapterContent(chapters []chapterSource) string {
	parts := make([]string, 0, len(chapters))
	for i := range chapters {
		if text := strings.TrimSpace(chapters[i].ContentText); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n\n")
}

// formatCharacterBlock 拼接角色卡（姓名/性格/背景，超长截断）。
func formatCharacterBlock(characters []domain.CharacterDO) string {
	var builder strings.Builder
	builder.WriteString("【角色卡】")
	for i := range characters {
		character := &characters[i]
		builder.WriteString("\n姓名：" + character.CharacterName)
		if character.PersonalityText != "" {
			builder.WriteString("｜性格：" + truncateRunes(character.PersonalityText, characterPersonalityRunes))
		}
		if character.BackgroundText != "" {
			builder.WriteString("｜背景：" + truncateRunes(character.BackgroundText, characterBackgroundRunes))
		}
	}
	return builder.String()
}

// formatLoreBlock 拼接设定条目（名称+内容截断；note 为私人笔记，禁止注入），无可用条目返回空。
func formatLoreBlock(lores []domain.LoreDO) string {
	var builder strings.Builder
	builder.WriteString("【设定条目】")
	count := 0
	for i := range lores {
		if lores[i].LoreTypeCode == loreTypeCodeNote {
			continue // 私人笔记严禁进入提示词
		}
		builder.WriteString("\n设定名：" + lores[i].LoreName + "｜内容：" + truncateRunes(lores[i].ContentText, loreContentRunes))
		count++
	}
	if count == 0 {
		return ""
	}
	return builder.String()
}

// condenseHistory 把会话历史全部注入，只收敛单条超长正文。
// 不按条数截取、不设总量预算：作者要的是完整脉络，过长的是单条而不是整段对话。
// 返回（注入历史，被收敛正文的消息条数）；纯函数，同输入同输出。
func condenseHistory(messages []domain.ChatMessageDO) ([]domain.ChatMessageDO, int) {
	injected := make([]domain.ChatMessageDO, len(messages))
	copy(injected, messages)
	condensed := 0
	for i := range injected {
		shortened := ai.CondenseText(injected[i].ContentText)
		if shortened != injected[i].ContentText {
			injected[i].ContentText = shortened
			condensed++
		}
	}
	return injected, condensed
}

// chatMessageInput 消息序列装配输入（纯数据，便于单测）。
type chatMessageInput struct {
	SystemPromptText string                 // 身份提示词正文（空则用内置兜底）
	StyleText        string                 // 项目文风约束
	ContextText      string                 // 装配后的上下文文本（空则省略）
	CommandText      string                 // 命中的任务提示词渲染文本（空则省略）
	HistoryList      []domain.ChatMessageDO // 注入历史（旧→新）
	UserText         string                 // 用户输入
}

// buildChatMessages 构造发往模型的消息序列（纯函数）：
// 身份提示词（系统）→ 上下文（系统）→ 任务提示词（系统）→ 历史（原生角色）→ 用户输入（用户）。
// 历史全量按原生角色注入，不做条数截取也不做摘要替代：模型看到的就是作者说过的话。
func buildChatMessages(input chatMessageInput) []*schema.Message {
	messages := make([]*schema.Message, 0, len(input.HistoryList)+4)
	messages = append(messages, &schema.Message{
		Role:    schema.System,
		Content: ai.ComposeSystemPrompt(input.SystemPromptText, input.StyleText),
	})
	if input.ContextText != "" {
		messages = append(messages, &schema.Message{Role: schema.System, Content: "【上下文】\n" + input.ContextText})
	}
	if input.CommandText != "" {
		messages = append(messages, &schema.Message{Role: schema.System, Content: input.CommandText})
	}
	for i := range input.HistoryList {
		messages = append(messages, &schema.Message{
			Role:    chatRoleOf(input.HistoryList[i].RoleCode),
			Content: input.HistoryList[i].ContentText,
		})
	}
	messages = append(messages, &schema.Message{Role: schema.User, Content: input.UserText})
	return messages
}

// chatRoleOf 将消息角色编码映射为 eino 角色（未知角色按 user 处理）。
func chatRoleOf(roleCode string) schema.RoleType {
	switch roleCode {
	case domain.ChatRoleAssistant:
		return schema.Assistant
	case domain.ChatRoleSystem:
		return schema.System
	default:
		return schema.User
	}
}

// countMessagesRunes 统计消息序列入参总 rune 数（token 估算用）。
func countMessagesRunes(messages []*schema.Message) int {
	total := 0
	for _, message := range messages {
		total += utf8.RuneCountInString(message.Content)
	}
	return total
}

// estimateTokenCount 按 rune 数 0.75 估算 token（入参+出参）。
func estimateTokenCount(inputRunes, outputRunes int) int {
	return int(float64(inputRunes+outputRunes) * 0.75)
}

// truncateRunes 按 rune 截断文本，超长追加省略标记。
func truncateRunes(text string, maxRunes int) string {
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes]) + "…"
}

// firstRunes 取文本前 n 个 rune（会话缺省名用）。
func firstRunes(text string, n int) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= n {
		return string(runes)
	}
	return string(runes[:n])
}

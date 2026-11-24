// 本文件实现 AI 对话的上下文装配：素材加载、上下文块构造、历史选取与消息序列成型。
// 关键约定：注入的提示词与上下文一律以系统级消息进入消息序列，用户输入始终是用户级消息。

package service

import (
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/cloudwego/eino/schema"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
)

// 上下文块内容截断上限（rune）：单条素材超长即截断，整体预算由 AssembleContext 裁剪。
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
	CommandText      string                 // 命中的指令提示词渲染文本（空则省略）
	HistoryList      []domain.ChatMessageDO // 注入历史（旧→新）
	UserText         string                 // 用户输入
}

// buildChatMessages 构造发往模型的消息序列（纯函数）：
// 身份提示词（系统）→ 上下文（系统）→ 指令提示词（系统）→ 历史（原生角色）→ 用户输入（用户）。
// 历史全量按原生角色注入，不做条数截取也不做摘要替代：模型看到的就是作者说过的话。
// 注入内容全部是系统级消息，用户输入始终是用户级消息，二者在消息列表里可区分。
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

// loadContextInput 装配上下文素材：章节/文档/召回/梗概/角色/设定。
// RAG 异常降级为空，不阻断生成；显式引用不存在或越权直接报错。
func (s *ChatService) loadContextInput(ctx context.Context, userULID string, req *domain.ChatSendREQ, userInput string) (chatContextInput, error) {
	var input chatContextInput
	var currentChapter *domain.ChapterDO
	for i, chapterULID := range req.RefChapterULIDList {
		chapter, err := s.chapterRepo.GetByULID(ctx, chapterULID)
		if err != nil {
			return input, err
		}
		if chapter == nil || chapter.ProjectULID != req.ProjectULID {
			return input, pkg.NewBizError(domain.RespCodeInvalidParam, "引用章节不存在")
		}
		input.ChapterList = append(input.ChapterList, chapterSource{
			ChapterULID: chapter.ChapterULID,
			ChapterName: chapter.ChapterName,
			ContentText: chapter.ContentText,
			IsCurrent:   i == 0,
		})
		if i == 0 {
			currentChapter = chapter
		}
	}
	if currentChapter != nil {
		summaries, err := s.loadNeighborSummaries(ctx, req.ProjectULID, currentChapter.ChapterULID)
		if err != nil {
			return input, err
		}
		input.SummaryList = summaries
	}
	for _, docULID := range req.RefDocULIDList {
		doc, err := s.knowledgeRepo.GetDocument(ctx, docULID)
		if err != nil {
			return input, err
		}
		if doc == nil {
			return input, pkg.NewBizError(domain.RespCodeInvalidParam, "引用文档不存在")
		}
		if doc.ProjectULID != req.ProjectULID && doc.ProjectULID != globalDocProjectID {
			return input, pkg.NewBizError(domain.RespCodeForbidden, "无权引用该文档")
		}
		fullText, err := s.loadDocFullText(ctx, docULID)
		if err != nil {
			return input, pkg.WrapBizError(domain.RespCodeStorageError, "引用文档读取失败", err)
		}
		input.DocList = append(input.DocList, docSource{DocName: doc.DocName, FullText: fullText})
	}
	for _, summaryULID := range req.RefSummaryULIDList {
		summary, err := s.continuityRepo.GetSummaryByULID(ctx, summaryULID)
		if err != nil {
			return input, err
		}
		if summary == nil || summary.ProjectULID != req.ProjectULID {
			return input, pkg.NewBizError(domain.RespCodeInvalidParam, "引用章节摘要不存在")
		}
		if text := strings.TrimSpace(summary.SummaryText); text != "" {
			input.RefSummaryList = append(input.RefSummaryList, text)
		}
	}
	for _, characterULID := range req.RefCharacterULIDList {
		character, err := s.characterRepo.GetByULID(ctx, characterULID)
		if err != nil {
			return input, err
		}
		if character == nil || character.ProjectULID != req.ProjectULID {
			return input, pkg.NewBizError(domain.RespCodeInvalidParam, "引用角色不存在")
		}
		input.RefCharacterList = append(input.RefCharacterList, *character)
	}
	for _, loreULID := range req.RefLoreULIDList {
		lore, err := s.loreRepo.GetByULID(ctx, loreULID)
		if err != nil {
			return input, err
		}
		if lore == nil || lore.ProjectULID != req.ProjectULID {
			return input, pkg.NewBizError(domain.RespCodeInvalidParam, "引用设定不存在")
		}
		// 私人笔记禁止进入 AI 上下文，显式引用也不例外
		if lore.LoreTypeCode == loreTypeCodeNote {
			return input, pkg.NewBizError(domain.RespCodeInvalidParam, "创作笔记不能引用给 AI")
		}
		input.RefLoreList = append(input.RefLoreList, *lore)
	}
	for _, sessionULID := range req.RefSessionULIDList {
		if sessionULID == req.SessionULID {
			return input, pkg.NewBizError(domain.RespCodeInvalidParam, "不能引用当前会话自身")
		}
		source, err := s.loadSessionArchive(ctx, userULID, sessionULID)
		if err != nil {
			return input, err
		}
		input.RefSessionList = append(input.RefSessionList, source)
	}
	if strings.TrimSpace(userInput) != "" {
		// TopKCount 留空，由知识库服务按应用配置 rag_top_k 解析
		hits, err := s.knowledgeService.Search(ctx, userULID, &domain.KnowledgeSearchREQ{
			ProjectULID: req.ProjectULID,
			KeywordText: userInput,
			SearchMode:  domain.SearchModeHybrid,
		})
		if err != nil {
			pkg.SLogger().Warnw("RAG 召回失败，降级为空上下文", "error", err)
		} else {
			input.RagList = hits
		}
	}
	characters, err := s.characterRepo.ListByProject(ctx, req.ProjectULID, "", "")
	if err != nil {
		return input, err
	}
	input.CharacterList = characters
	lores, err := s.chatRepo.ListLoreBriefsByProject(ctx, req.ProjectULID)
	if err != nil {
		return input, err
	}
	input.LoreList = lores
	return input, nil
}

// loadSessionArchive 读取被 @ 的历史会话并导出逐条归档文本。
// 归属校验与当前会话同口径：只能引用自己名下的会话；导出不调用模型，一条消息都不删。
func (s *ChatService) loadSessionArchive(ctx context.Context, userULID, sessionULID string) (sessionSource, error) {
	session, err := s.mustOwnSession(ctx, userULID, sessionULID)
	if err != nil {
		return sessionSource{}, err
	}
	messages, err := s.chatRepo.ListMessagesBySession(ctx, session.SessionULID)
	if err != nil {
		return sessionSource{}, pkg.WrapBizError(domain.RespCodeStorageError, "引用会话读取失败", err)
	}
	archiveList := make([]ai.ArchiveMessage, 0, len(messages))
	for i := range messages {
		archiveList = append(archiveList, ai.ArchiveMessage{
			Seq:         i + 1,
			RoleCode:    messages[i].RoleCode,
			ContentText: messages[i].ContentText,
			ToolCodeCSV: strings.Join(decodeStringList(messages[i].ToolCodeJSON), "、"),
			CreatedAt:   pkg.FormatTime(messages[i].CreatedAt),
		})
	}
	// 引用整段会话：逐条编号导出，只有单条正文超长时才保留首尾
	archiveText := ai.ArchiveHistory(archiveList, ai.ArchiveConfig{WithHeader: true})
	sessionName := session.SessionName
	if strings.TrimSpace(sessionName) == "" {
		sessionName = "未命名会话"
	}
	if strings.TrimSpace(archiveText) == "" {
		archiveText = "（该会话没有可引用的内容）"
	}
	return sessionSource{SessionName: sessionName, ContentText: archiveText}, nil
}

// loadNeighborSummaries 取当前章相邻前后章的梗概（按章节排序号相邻，缺位跳过）。
func (s *ChatService) loadNeighborSummaries(ctx context.Context, projectULID, currentChapterULID string) ([]string, error) {
	metas, err := s.chatRepo.ListChapterMetaByProject(ctx, projectULID)
	if err != nil {
		return nil, err
	}
	summaries := make([]string, 0, 2)
	for i := range metas {
		if metas[i].ChapterULID != currentChapterULID {
			continue
		}
		for _, neighbor := range []*domain.ChapterDO{chapterAt(metas, i-1), chapterAt(metas, i+1)} {
			if neighbor != nil && neighbor.SummaryText != "" {
				summaries = append(summaries, neighbor.SummaryText)
			}
		}
		break
	}
	return summaries, nil
}

// chapterAt 按下标取章节，越界返回 nil。
func chapterAt(metas []domain.ChapterDO, index int) *domain.ChapterDO {
	if index < 0 || index >= len(metas) {
		return nil
	}
	return &metas[index]
}

// loadDocFullText 拼接文档全部切片文本（chunk_index 升序）。
func (s *ChatService) loadDocFullText(ctx context.Context, docULID string) (string, error) {
	parts := make([]string, 0, 8)
	for pageNum := 1; ; pageNum++ {
		chunks, _, err := s.knowledgeRepo.ListChunksByDoc(ctx, docULID, pageNum, docFullTextPageSize)
		if err != nil {
			return "", err
		}
		for i := range chunks {
			parts = append(parts, chunks[i].ChunkText)
		}
		if len(chunks) < docFullTextPageSize {
			break
		}
	}
	return strings.Join(parts, "\n"), nil
}

// encodeStringList 将字符串列表编码为 JSON 数组字符串（空列表为 []）。
func encodeStringList(values []string) string {
	if values == nil {
		values = []string{}
	}
	data, err := json.Marshal(values)
	if err != nil {
		return "[]"
	}
	return string(data)
}

// decodeStringList 解码 JSON 数组字符串为字符串列表（非法文本返回空列表）。
func decodeStringList(jsonText string) []string {
	values := []string{}
	if jsonText == "" {
		return values
	}
	if err := json.Unmarshal([]byte(jsonText), &values); err != nil {
		return []string{}
	}
	return values
}

// toChatMessageVO 组装消息视图对象（ref_*_json 解码为 ULID 列表）。
func toChatMessageVO(do *domain.ChatMessageDO) domain.ChatMessageVO {
	return domain.ChatMessageVO{
		MessageULID:          do.MessageULID,
		SessionULID:          do.SessionULID,
		RoleCode:             do.RoleCode,
		ContentText:          do.ContentText,
		ReasoningText:        do.ReasoningText,
		PromptCode:           do.PromptCode,
		ToolCodeList:         decodeStringList(do.ToolCodeJSON),
		RefDocULIDList:       decodeStringList(do.RefDocJSON),
		RefChapterULIDList:   decodeStringList(do.RefChapterJSON),
		RefSummaryULIDList:   decodeStringList(do.RefSummaryJSON),
		RefCharacterULIDList: decodeStringList(do.RefCharacterJSON),
		RefLoreULIDList:      decodeStringList(do.RefLoreJSON),
		RefSessionULIDList:   decodeStringList(do.RefSessionJSON),
		TokenUsedCount:       do.TokenUsedCount,
		CreatedAt:            pkg.FormatTime(do.CreatedAt),
	}
}

// 本文件实现 AI 对话的上下文装配：素材加载、块构造、截断与消息序列成型（纯函数为主）。

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
	ChapterName string // 章节名
	ContentText string // 章节正文
	IsCurrent   bool   // 是否当前章（当前章优先级 90，引用章 80）
}

// docSource @引用文档上下文来源。
type docSource struct {
	DocName  string // 文档名
	FullText string // 全部切片拼接文本
}

// chatContextInput 上下文装配输入（纯数据，供 buildContextBlocks 消费，便于测试注入）。
type chatContextInput struct {
	SkillTemplate  string                    // 命中技能的提示词模板（空为自由对话）
	SkillVariable  map[string]string         // 技能变量取值（渲染模板用）
	SkillRefList   []string                  // 技能参考资料正文（命中后注入）
	UserInput      string                    // 解析后的用户正文（技能参数）
	ChapterList    []chapterSource           // 当前章与引用章（首项为当前章）
	DocList        []docSource               // @引用文档
	RagList        []domain.KnowledgeChunkVO // 知识库召回切片
	RefSummaryList []string                  // @显式引用的章节摘要正文
	RefPromptList  []string                  // @显式引用的提示词档案正文
	SummaryList    []string                  // 当前章前后章梗概
	CharacterList  []domain.CharacterDO      // 项目角色卡
	LoreList       []domain.LoreDO           // 项目设定条目（note 类型禁止注入）
	HistoryList    []domain.ChatMessageDO    // 注入历史（旧→新）
}

// buildContextBlocks 组装上下文块并返回最终指令文本（纯函数）。
// 技能命中时渲染 {{user_input}}/{{chapter_content}}/{{context}}（context 为其余块先装配文本）；
// 块顺序即同优先级稳定顺序：章节 → 文档 → 召回 → 梗概 → 角色 → 设定 → 历史。
func buildContextBlocks(input chatContextInput) ([]ai.ContextBlock, string) {
	blocks := make([]ai.ContextBlock, 0, len(input.ChapterList)+len(input.DocList)+6)
	for i := range input.ChapterList {
		chapter := &input.ChapterList[i]
		if strings.TrimSpace(chapter.ContentText) == "" {
			continue
		}
		block := ai.ContextBlock{BlockKey: "ref_chapter", Priority: ai.PriorityExplicitRef, ContentText: "【引用章节】" + chapter.ChapterName + "\n" + chapter.ContentText}
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
	if len(input.RefPromptList) > 0 {
		blocks = append(blocks, ai.ContextBlock{
			BlockKey:    "ref_prompt",
			Priority:    ai.PriorityExplicitRef,
			ContentText: "【引用提示词档案】\n" + strings.Join(input.RefPromptList, "\n\n"),
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
	if len(input.SkillRefList) > 0 {
		blocks = append(blocks, ai.ContextBlock{
			BlockKey:    "skill_reference",
			Priority:    ai.PrioritySkillRef,
			ContentText: "【技能参考资料】\n" + strings.Join(input.SkillRefList, "\n\n"),
		})
	}
	if input.SkillTemplate == "" {
		return blocks, input.UserInput
	}
	// 技能渲染块优先级最高：context 取其余块先装配文本，chapter_content 超预算由装配裁剪。
	contextText, _ := ai.AssembleContext(blocks, ai.ContextBudgetChars)
	valueMap := map[string]string{
		"user_input":      input.UserInput,
		"chapter_content": truncateRunes(joinChapterContent(input.ChapterList), ai.ContextBudgetChars),
		"context":         contextText,
	}
	for key, value := range input.SkillVariable {
		valueMap[key] = value
	}
	rendered := ai.RenderPrompt(input.SkillTemplate, valueMap)
	skillBlock := ai.ContextBlock{BlockKey: "skill", Priority: ai.PrioritySkill, ContentText: rendered}
	return append([]ai.ContextBlock{skillBlock}, blocks...), rendered
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

// formatHistoryBlock 拼接对话历史块（旧→新，角色标注）。
func formatHistoryBlock(messages []domain.ChatMessageDO) string {
	var builder strings.Builder
	builder.WriteString("【对话历史】")
	for i := range messages {
		roleLabel := "用户"
		if messages[i].RoleCode == domain.ChatRoleAssistant {
			roleLabel = "助手"
		}
		builder.WriteString("\n" + roleLabel + "：" + messages[i].ContentText)
	}
	return builder.String()
}

// selectHistoryMessages 选取注入历史（纯函数）：取最近 n 条为主干（旧→新，n<=0 不注入常规历史），
// 显式引用消息必注入（不受 n 限制，去重后按时间旧→新追加到尾部）。
func selectHistoryMessages(messages []domain.ChatMessageDO, refMessageULIDs []string, n int) []domain.ChatMessageDO {
	mainStart := 0
	switch {
	case n <= 0:
		mainStart = len(messages) // 不注入常规历史
	case len(messages) > n:
		mainStart = len(messages) - n
	}
	selected := make([]domain.ChatMessageDO, 0, len(messages))
	included := make(map[string]bool, len(messages))
	for _, message := range messages[mainStart:] {
		selected = append(selected, message)
		included[message.MessageULID] = true
	}
	refSet := make(map[string]bool, len(refMessageULIDs))
	for _, ulid := range refMessageULIDs {
		refSet[ulid] = true
	}
	for _, message := range messages {
		if refSet[message.MessageULID] && !included[message.MessageULID] {
			selected = append(selected, message)
			included[message.MessageULID] = true
		}
	}
	return selected
}

// buildChatMessages 构造发往模型的消息序列（纯函数）：
// 系统提示词（档案正文 + 项目文风约束）→ 【上下文】装配文本（为空省略）→ 历史（role 映射）→ 最终指令。
func buildChatMessages(systemPromptText, styleText, contextText, instructionText string, historyList []domain.ChatMessageDO) []*schema.Message {
	messages := make([]*schema.Message, 0, len(historyList)+3)
	messages = append(messages, &schema.Message{Role: schema.System, Content: ai.ComposeSystemPrompt(systemPromptText, styleText)})
	if contextText != "" {
		messages = append(messages, &schema.Message{Role: schema.User, Content: "【上下文】\n" + contextText})
	}
	for i := range historyList {
		messages = append(messages, &schema.Message{Role: chatRoleOf(historyList[i].RoleCode), Content: historyList[i].ContentText})
	}
	messages = append(messages, &schema.Message{Role: schema.User, Content: instructionText})
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
	for _, promptULID := range req.RefPromptULIDList {
		profile, err := s.promptProfileSvc.mustReadableProfile(ctx, userULID, promptULID)
		if err != nil {
			return input, err
		}
		if text := strings.TrimSpace(profile.SystemPromptText); text != "" {
			input.RefPromptList = append(input.RefPromptList, "《"+profile.PromptProfileName+"》\n"+text)
		}
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

// encodeULIDList 将 ULID 列表编码为 JSON 数组字符串（空列表为 []）。
func encodeULIDList(ulids []string) string {
	if ulids == nil {
		ulids = []string{}
	}
	data, err := json.Marshal(ulids)
	if err != nil {
		return "[]"
	}
	return string(data)
}

// decodeULIDList 解码 JSON 数组字符串为 ULID 列表（非法文本返回空列表）。
func decodeULIDList(jsonText string) []string {
	ulids := []string{}
	if jsonText == "" {
		return ulids
	}
	if err := json.Unmarshal([]byte(jsonText), &ulids); err != nil {
		return []string{}
	}
	return ulids
}

// toChatMessageVO 组装消息视图对象（ref_*_json 解码为 ULID 列表）。
func toChatMessageVO(do *domain.ChatMessageDO) domain.ChatMessageVO {
	return domain.ChatMessageVO{
		MessageULID:        do.MessageULID,
		SessionULID:        do.SessionULID,
		RoleCode:           do.RoleCode,
		ContentText:        do.ContentText,
		ReasoningText:      do.ReasoningText,
		SkillCode:          do.SkillCode,
		RefDocULIDList:     decodeULIDList(do.RefDocJSON),
		RefChapterULIDList: decodeULIDList(do.RefChapterJSON),
		RefMessageULIDList: decodeULIDList(do.RefMessageJSON),
		RefSummaryULIDList: decodeULIDList(do.RefSummaryJSON),
		RefPromptULIDList:  decodeULIDList(do.RefPromptJSON),
		TokenUsedCount:     do.TokenUsedCount,
		CreatedAt:          pkg.FormatTime(do.CreatedAt),
	}
}

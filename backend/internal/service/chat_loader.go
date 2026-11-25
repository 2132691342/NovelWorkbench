// 本文件实现 AI 对话的素材加载：从仓储与检索服务取章节、文档、摘要、角色、设定与历史会话。
// RAG 异常降级为空，不阻断生成；显式引用不存在或越权直接报错。

package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
)

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

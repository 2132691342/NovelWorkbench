// 本文件实现 AI 对话业务：会话管理、提示词解析、上下文装配、流式生成与消息落库。

package service

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"

	"github.com/cloudwego/eino/schema"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// 对话链路常量。
const (
	settingKeyRAGTopK     = "rag_top_k"      // 应用配置键：知识库默认召回条数
	globalDocProjectID    = "0"              // 全局知识库文档的项目标识
	chatPingInterval      = 15 * time.Second // 流式无输出心跳间隔
	chatStreamIdleTimeout = 6 * time.Minute  // 流式连续无分片的上限：超时判定网关已断
	docFullTextPageSize   = 200              // 文档切片全量拼接分页大小
	chatToolRoundLimit    = 4                // 工具往返轮数上限：防模型反复调用同一工具打转
)

// ChatStreamWriter SSE 输出接口（api 层实现为 text/event-stream 写入）。
type ChatStreamWriter interface {
	Event(dto domain.ChatStreamEventDTO) error // 输出一条事件（data: {json}）
	Ping() error                               // 输出心跳注释行（: ping）
}

// chatEventWriter 流式事件封装：seq 从 1 单调递增，统一封装事件体。
type chatEventWriter struct {
	writer             ChatStreamWriter   // SSE 输出实现
	sessionULID        string             // 会话 ULID
	messageULID        string             // 助手消息 ULID
	userULID           string             // 当前用户（工具调用归属校验）
	projectULID        string             // 当前项目（工具调用上下文）
	currentChapterULID string             // 当前章 ULID（模型调工具未给章节时补齐）
	currentChapterText string             // 当前章正文（同上）
	userInput          string             // 作者本轮原话（检索类工具兜底检索词）
	toolInfoList       []*schema.ToolInfo // 下发给模型的工具声明（空表示不提供工具）
	toolCodeList       []string           // 本轮模型实际调用的工具编码（done 事件下发）
	seq                int64              // 事件序号（单调递增）
}

// emit 输出一条流式事件（reference_list 缺省为空数组）。
func (e *chatEventWriter) emit(eventType, contentText, errorMessage string, referenceList []domain.KnowledgeChunkVO) error {
	return e.emitWithReasoning(eventType, contentText, "", errorMessage, referenceList)
}

// emitWithReasoning 输出一条带完整思考过程的流式事件（done 事件携带 reasoning_text）。
func (e *chatEventWriter) emitWithReasoning(eventType, contentText, reasoningText, errorMessage string,
	referenceList []domain.KnowledgeChunkVO) error {
	e.seq++
	if referenceList == nil {
		referenceList = []domain.KnowledgeChunkVO{}
	}
	toolCodeList := e.toolCodeList
	if toolCodeList == nil {
		toolCodeList = []string{}
	}
	return e.writer.Event(domain.ChatStreamEventDTO{
		Seq:           e.seq,
		EventType:     eventType,
		SessionULID:   e.sessionULID,
		MessageULID:   e.messageULID,
		ContentText:   contentText,
		ReasoningText: reasoningText,
		ReferenceList: referenceList,
		ToolCodeList:  toolCodeList,
		ErrorMessage:  errorMessage,
	})
}

// ChatService AI 对话业务：会话管理、上下文装配、流式生成与消息落库。
type ChatService struct {
	chatRepo         *repo.ChatRepo        // 会话与消息仓储
	projectRepo      *repo.ProjectRepo     // 项目仓储（归属校验）
	chapterRepo      *repo.ChapterRepo     // 章节仓储
	characterRepo    *repo.CharacterRepo   // 角色仓储
	loreRepo         *repo.LoreRepo        // 设定仓储（@引用设定条目）
	knowledgeRepo    *repo.KnowledgeRepo   // 知识库仓储
	continuityRepo   *repo.ContinuityRepo  // 章节摘要仓储（@引用摘要）
	settingRepo      *repo.SettingRepo     // 应用配置仓储
	promptSvc        *PromptService        // 提示词服务（身份提示词与斜杠指令解析）
	knowledgeService *KnowledgeService     // 知识库服务（RAG 召回）
	toolService      *ToolService          // 工具服务（关键词自动触发与执行）
	providerSvc      *ModelProviderService // 供应商服务（模型选择）
	aiClient         *ai.Client            // 模型网关（流式调用与用量上报）
	db               *gorm.DB              // 事务入口
	auditRecorder    pkg.AuditRecorder     // 审计写入（AI 调用留痕，可为 nil）
}

// NewChatService 构造对话服务。
func NewChatService(chatRepo *repo.ChatRepo, projectRepo *repo.ProjectRepo, chapterRepo *repo.ChapterRepo,
	characterRepo *repo.CharacterRepo, loreRepo *repo.LoreRepo, knowledgeRepo *repo.KnowledgeRepo,
	continuityRepo *repo.ContinuityRepo, settingRepo *repo.SettingRepo, promptSvc *PromptService,
	knowledgeService *KnowledgeService, toolService *ToolService, providerSvc *ModelProviderService,
	aiClient *ai.Client, db *gorm.DB) *ChatService {
	return &ChatService{
		chatRepo: chatRepo, projectRepo: projectRepo, chapterRepo: chapterRepo,
		characterRepo: characterRepo, loreRepo: loreRepo,
		knowledgeRepo: knowledgeRepo, continuityRepo: continuityRepo,
		settingRepo: settingRepo, promptSvc: promptSvc, knowledgeService: knowledgeService,
		toolService: toolService, providerSvc: providerSvc, aiClient: aiClient, db: db,
	}
}

// SetAuditRecorder 注入审计写入器（AI 对话留痕，链路 ID 取请求的 X-Request-ID）。
func (s *ChatService) SetAuditRecorder(recorder pkg.AuditRecorder) {
	s.auditRecorder = recorder
}

// SessionCreate 创建会话（会话名缺省时由首条消息前 20 字回填）。
func (s *ChatService) SessionCreate(ctx context.Context, userULID string, req *domain.ChatSessionCreateREQ) (domain.ChatSessionVO, error) {
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID); err != nil {
		return domain.ChatSessionVO{}, err
	}
	do := &domain.ChatSessionDO{
		SessionULID: pkg.NewULID(domain.IDPrefixSession),
		ProjectULID: req.ProjectULID,
		UserULID:    userULID,
		SessionName: req.SessionName,
	}
	if err := s.chatRepo.CreateSession(ctx, do); err != nil {
		return domain.ChatSessionVO{}, pkg.WrapBizError(domain.RespCodeStorageError, "会话创建失败", err)
	}
	return domain.ChatSessionVO{
		SessionULID:  do.SessionULID,
		ProjectULID:  do.ProjectULID,
		SessionName:  do.SessionName,
		MessageCount: 0,
		UpdatedAt:    pkg.FormatTime(do.UpdatedAt),
	}, nil
}

// SessionList 查询项目会话列表（按最后活跃倒序）。
func (s *ChatService) SessionList(ctx context.Context, userULID string, req *domain.ChatSessionListREQ) ([]domain.ChatSessionVO, error) {
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID); err != nil {
		return nil, err
	}
	vos, err := s.chatRepo.ListSessionsByProject(ctx, req.ProjectULID)
	if err != nil {
		return nil, pkg.WrapBizError(domain.RespCodeStorageError, "会话列表读取失败", err)
	}
	if vos == nil {
		vos = []domain.ChatSessionVO{}
	}
	return vos, nil
}

// SessionDelete 删除会话（级联删除其全部消息）。
func (s *ChatService) SessionDelete(ctx context.Context, userULID string, req *domain.ChatSessionDeleteREQ) error {
	session, err := s.chatRepo.GetSession(ctx, req.SessionULID)
	if err != nil {
		return err
	}
	if session == nil {
		return nil // 幂等：会话已不存在，视为删除成功
	}
	if session.UserULID != userULID {
		return pkg.NewBizError(domain.RespCodeForbidden, "无权操作该会话")
	}
	if err := s.chatRepo.DeleteSessionWithMessages(ctx, req.SessionULID); err != nil {
		return pkg.WrapBizError(domain.RespCodeStorageError, "会话删除失败", err)
	}
	return nil
}

// History 会话历史：created_at 升序全量返回（PageDTO 包络，total=条数）。
func (s *ChatService) History(ctx context.Context, userULID string, req *domain.ChatHistoryREQ) (domain.PageDTO[domain.ChatMessageVO], error) {
	if _, err := s.mustOwnSession(ctx, userULID, req.SessionULID); err != nil {
		return domain.PageDTO[domain.ChatMessageVO]{}, err
	}
	messages, err := s.chatRepo.ListMessagesBySession(ctx, req.SessionULID)
	if err != nil {
		return domain.PageDTO[domain.ChatMessageVO]{}, pkg.WrapBizError(domain.RespCodeStorageError, "会话历史读取失败", err)
	}
	items := make([]domain.ChatMessageVO, 0, len(messages))
	for i := range messages {
		items = append(items, toChatMessageVO(&messages[i]))
	}
	return domain.PageDTO[domain.ChatMessageVO]{Total: int64(len(items)), PageNum: 1, PageSize: len(items), Items: items}, nil
}

// Send 流式对话主链路：提示词解析 → 上下文装配 → 历史压缩 → 流式生成（含工具往返）→ 落库与统计。
// 流开始前的失败返回 error（api 输出 JSON 包络）；流开始后失败经 SSE error 事件下发并返回 nil。
func (s *ChatService) Send(ctx context.Context, userULID string, req *domain.ChatSendREQ, writer ChatStreamWriter) error {
	project, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID)
	if err != nil {
		return err
	}
	prompt, err := s.resolvePrompt(ctx, req)
	if err != nil {
		return err
	}
	promptCode, promptText, userInput := prompt.PromptCode, prompt.PromptText, prompt.RemainderText
	session, err := s.resolveSession(ctx, userULID, req)
	if err != nil {
		return err
	}
	sessionMessages, err := s.chatRepo.ListMessagesBySession(ctx, session.SessionULID)
	if err != nil {
		return pkg.WrapBizError(domain.RespCodeStorageError, "会话历史读取失败", err)
	}
	historyList, condensedCount := condenseHistory(sessionMessages)
	if condensedCount > 0 {
		pkg.SLogger().Infow("历史中超长正文已保留首尾", "session_ulid", session.SessionULID,
			"message_count", len(sessionMessages), "condensed_count", condensedCount)
	}
	contextInput, err := s.loadContextInput(ctx, userULID, req, userInput)
	if err != nil {
		return err
	}
	contextInput.UserInput = userInput
	blocks := buildContextBlocks(contextInput)
	assembledText, condensedKeys := ai.AssembleContext(blocks)
	if len(condensedKeys) > 0 {
		pkg.SLogger().Infow("上下文块超长已保留首尾", "session_ulid", session.SessionULID, "condensed_keys", condensedKeys)
	}
	commandText := ""
	if promptText != "" {
		commandText = ai.RenderPrompt(promptText, map[string]string{
			"user_input":      userInput,
			"chapter_content": joinChapterContent(contextInput.ChapterList),
			"context":         assembledText,
			"prompt_code":     promptCode,
		})
	}
	// 工具不由命令注入：模型拿到工具声明后自行决定是否调用，作者无需（也无法）手动点名
	userMessage := &domain.ChatMessageDO{
		MessageULID:      pkg.NewULID(domain.IDPrefixMessage),
		SessionULID:      session.SessionULID,
		RoleCode:         domain.ChatRoleUser,
		ContentText:      req.ContentText,
		PromptCode:       promptCode,
		ToolCodeJSON:     "[]",
		RefDocJSON:       encodeStringList(req.RefDocULIDList),
		RefChapterJSON:   encodeStringList(req.RefChapterULIDList),
		RefSummaryJSON:   encodeStringList(req.RefSummaryULIDList),
		RefCharacterJSON: encodeStringList(req.RefCharacterULIDList),
		RefLoreJSON:      encodeStringList(req.RefLoreULIDList),
		RefSessionJSON:   encodeStringList(req.RefSessionULIDList),
		TokenUsedCount:   estimateTokenCount(utf8.RuneCountInString(req.ContentText), 0),
	}
	if err := s.chatRepo.CreateMessage(ctx, userMessage); err != nil {
		return pkg.WrapBizError(domain.RespCodeStorageError, "消息保存失败", err)
	}
	if session.SessionName == "" {
		if err := s.chatRepo.UpdateSessionName(ctx, session.SessionULID, firstRunes(req.ContentText, sessionNameRunes)); err != nil {
			pkg.SLogger().Warnw("会话名回填失败", "session_ulid", session.SessionULID, "error", err)
		}
	}
	systemPromptText := s.promptSvc.SystemPromptOf(ctx, req.ProjectULID, req.SystemPromptCode)
	messages := buildChatMessages(chatMessageInput{
		SystemPromptText: systemPromptText,
		StyleText:        project.StyleText,
		ContextText:      assembledText,
		CommandText:      commandText,
		HistoryList:      historyList,
		UserText:         userInput,
	})
	providerSpec, err := s.providerSvc.ChatSpecOf(ctx, userULID, req.ModelULID)
	if err != nil {
		return err // 供应商不可用：流未开始，api 层输出 JSON 包络
	}
	toolInfoList, err := s.toolDeclarations(ctx)
	if err != nil {
		// 工具声明读失败不该阻断对话：退化为不带工具，模型仍能基于上下文作答
		pkg.SLogger().Warnw("工具声明读取失败，本轮不提供工具", "error", err)
	}
	events := &chatEventWriter{
		writer:             writer,
		sessionULID:        session.SessionULID,
		messageULID:        pkg.NewULID(domain.IDPrefixMessage),
		userULID:           userULID,
		projectULID:        req.ProjectULID,
		currentChapterULID: currentChapterULIDOf(contextInput),
		currentChapterText: joinChapterContent(contextInput.ChapterList),
		userInput:          userInput,
		toolInfoList:       toolInfoList,
	}
	chatStartedAt := time.Now()
	replyText, reasoningText, completed := s.streamChat(ctx, events, messages, providerSpec)
	// AI 调用留痕：一次对话一条 ai_call span（只记模型、token 与耗时，不落提示词原文）
	chatLatency := time.Since(chatStartedAt)
	chatTokenCount := estimateTokenCount(countMessagesRunes(messages),
		utf8.RuneCountInString(replyText)+utf8.RuneCountInString(reasoningText))
	var chatCallErr error
	if !completed {
		chatCallErr = pkg.NewBizError(domain.RespCodeAIProviderError, "AI 生成未完成（客户端断开或模型失败）")
	}
	recordAICall(ctx, s.auditRecorder, aiCallMeta{
		ActionCode:  domain.AuditActionAIChat,
		ActionName:  "AI 对话",
		TraceID:     pkg.RequestIDFromContext(ctx),
		UserULID:    userULID,
		TargetType:  "chat_session",
		TargetULID:  session.SessionULID,
		ModelName:   providerSpec.ChatModelName,
		PromptChars: countMessagesRunes(messages),
	}, chatLatency, chatTokenCount, chatCallErr)
	if !completed {
		return nil // 客户端断开或生成失败：不落 assistant 消息，用户消息保留对话痕迹
	}
	assistantMessage := &domain.ChatMessageDO{
		MessageULID:      events.messageULID,
		SessionULID:      session.SessionULID,
		RoleCode:         domain.ChatRoleAssistant,
		ContentText:      replyText,
		ReasoningText:    reasoningText,
		PromptCode:       promptCode,
		ToolCodeJSON:     encodeStringList(events.toolCodeList),
		RefDocJSON:       encodeStringList(req.RefDocULIDList),
		RefChapterJSON:   encodeStringList(req.RefChapterULIDList),
		RefSummaryJSON:   encodeStringList(req.RefSummaryULIDList),
		RefCharacterJSON: encodeStringList(req.RefCharacterULIDList),
		RefLoreJSON:      encodeStringList(req.RefLoreULIDList),
		RefSessionJSON:   encodeStringList(req.RefSessionULIDList),
		TokenUsedCount:   estimateTokenCount(countMessagesRunes(messages), utf8.RuneCountInString(replyText)),
	}
	// assistant 消息与今日 AI 字数统计同事务，保证两者一致落库
	if err := repo.WithTx(s.db, func(tx *gorm.DB) error {
		txChat := repo.NewChatRepo(tx)
		if err := txChat.CreateMessage(ctx, assistantMessage); err != nil {
			return err
		}
		return txChat.UpsertAIWordDelta(ctx, req.ProjectULID, time.Now().Format("2006-01-02"), pkg.CountWords(replyText))
	}); err != nil {
		pkg.SLogger().Errorw("助手消息落库失败", "session_ulid", session.SessionULID, "error", err)
		_ = events.emit(domain.StreamEventError, "", "回复保存失败，请重试", nil)
		return nil
	}
	_ = events.emitWithReasoning(domain.StreamEventDone, replyText, reasoningText, "", contextInput.RagList)
	return nil
}

// chatPrompt 对话链路的提示词解析结果（正文保留占位符，装配时再渲染）。
type chatPrompt struct {
	PromptCode    string // 提示词编码（空为自由对话）
	PromptText    string // 提示词正文（含 {{context}} 等占位符）
	RemainderText string // 去掉斜杠命令后的用户正文
}

// resolvePrompt 解析提示词与用户正文：prompt_code 优先，否则解析正文首 token 的 /code；
// 项目提示词覆盖同编码全局提示词，未命中提示词时按自由对话处理。
func (s *ChatService) resolvePrompt(ctx context.Context, req *domain.ChatSendREQ) (chatPrompt, error) {
	resolved, err := s.promptSvc.Resolve(ctx, &domain.PromptResolveREQ{
		ProjectULID: req.ProjectULID,
		ContentText: req.ContentText,
		PromptCode:  req.PromptCode,
	})
	if err != nil {
		return chatPrompt{}, err
	}
	return chatPrompt{
		PromptCode:    resolved.PromptCode,
		PromptText:    resolved.PromptText,
		RemainderText: resolved.RemainderText,
	}, nil
}

// resolveSession 解析会话：session_ulid 为空自动创建，否则校验归属与项目匹配。
func (s *ChatService) resolveSession(ctx context.Context, userULID string, req *domain.ChatSendREQ) (*domain.ChatSessionDO, error) {
	if req.SessionULID == "" {
		do := &domain.ChatSessionDO{
			SessionULID: pkg.NewULID(domain.IDPrefixSession),
			ProjectULID: req.ProjectULID,
			UserULID:    userULID,
		}
		if err := s.chatRepo.CreateSession(ctx, do); err != nil {
			return nil, pkg.WrapBizError(domain.RespCodeStorageError, "会话创建失败", err)
		}
		return do, nil
	}
	session, err := s.mustOwnSession(ctx, userULID, req.SessionULID)
	if err != nil {
		return nil, err
	}
	if session.ProjectULID != req.ProjectULID {
		return nil, pkg.NewBizError(domain.RespCodeInvalidParam, "会话与项目不匹配")
	}
	return session, nil
}

// mustOwnSession 加载会话并校验归属当前用户。
func (s *ChatService) mustOwnSession(ctx context.Context, userULID, sessionULID string) (*domain.ChatSessionDO, error) {
	session, err := s.chatRepo.GetSession(ctx, sessionULID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, pkg.NewBizError(domain.RespCodeNotFound, "会话不存在")
	}
	if session.UserULID != userULID {
		return nil, pkg.NewBizError(domain.RespCodeForbidden, "无权操作该会话")
	}
	return session, nil
}

// toolDeclarations 取本轮下发给模型的工具声明（模型据此自行决定调不调、调哪个）。
func (s *ChatService) toolDeclarations(ctx context.Context) ([]*schema.ToolInfo, error) {
	if s.toolService == nil {
		return nil, nil
	}
	specList, err := s.toolService.ToolDeclarations(ctx)
	if err != nil {
		return nil, err
	}
	return ai.ToolInfos(specList), nil
}

// invokeModelToolCall 执行模型请求的一次工具调用，返回写给模型的 tool 消息正文。
// 模型通常只给出 tool_code（它看不到章节 ULID），因此工作区上下文由服务端补齐：
// 提到「这章」用具当前章，正文缺省用当前章正文，检索词缺省用作者那句话。
func (s *ChatService) invokeModelToolCall(ctx context.Context, events *chatEventWriter,
	toolCode, argumentsJSON string) (string, error) {
	if s.toolService == nil {
		return "", pkg.NewBizError(domain.RespCodeBizFailed, "工具服务不可用")
	}
	paramValue := ai.DecodeToolArguments(argumentsJSON)
	contentText := paramValue["content_text"]
	chapterULID := paramValue["chapter_ulid"]
	if contentText == "" && chapterULID == "" {
		if events.currentChapterULID != "" {
			chapterULID = events.currentChapterULID
		} else {
			contentText = events.currentChapterText
		}
	}
	paramValue["query_text"] = firstNonEmpty(paramValue["query_text"], events.userInput)
	dto, err := s.toolService.Invoke(ctx, events.userULID, &domain.ToolInvokeREQ{
		ProjectULID: events.projectULID,
		ToolCode:    toolCode,
		ChapterULID: chapterULID,
		ContentText: contentText,
		ParamValue:  paramValue,
	})
	if err != nil {
		return "", err
	}
	return dto.ResultText, nil
}

// firstNonEmpty 返回第一个非空字符串（缺省值兜底用）。
func firstNonEmpty(valueList ...string) string {
	for _, value := range valueList {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

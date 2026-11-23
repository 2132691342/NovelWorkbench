// 本文件实现 AI 对话业务：会话管理、上下文装配、流式生成与消息落库。

package service

import (
	"context"
	"strconv"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// 对话链路常量。
const (
	settingKeyChatHistoryCount = "chat_history_count" // 应用配置键：历史注入条数
	settingKeyRAGTopK          = "rag_top_k"          // 应用配置键：知识库默认召回条数
	globalDocProjectID         = "0"                  // 全局知识库文档的项目标识
	chatPingInterval           = 15 * time.Second     // 流式无输出心跳间隔
	chatStreamIdleTimeout      = 6 * time.Minute      // 流式连续无分片的上限：超时判定网关已断，报错而非无限等待
	docFullTextPageSize        = 200                  // 文档切片全量拼接分页大小
)

// ChatStreamWriter SSE 输出接口（api 层实现为 text/event-stream 写入）。
type ChatStreamWriter interface {
	Event(dto domain.ChatStreamEventDTO) error // 输出一条事件（data: {json}）
	Ping() error                               // 输出心跳注释行（: ping）
}

// chatEventWriter 流式事件封装：seq 从 1 单调递增，统一封装事件体。
type chatEventWriter struct {
	writer      ChatStreamWriter // SSE 输出实现
	sessionULID string           // 会话 ULID
	messageULID string           // 助手消息 ULID
	seq         int64            // 事件序号（单调递增）
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
	return e.writer.Event(domain.ChatStreamEventDTO{
		Seq:           e.seq,
		EventType:     eventType,
		SessionULID:   e.sessionULID,
		MessageULID:   e.messageULID,
		ContentText:   contentText,
		ReasoningText: reasoningText,
		ReferenceList: referenceList,
		ErrorMessage:  errorMessage,
	})
}

// ChatConfig 对话链路参数（infra 配置的标量子集，避免 service 依赖配置类型）。
type ChatConfig struct {
	HistoryCount int // 历史注入默认条数（应用配置未命中时兜底）
}

// ChatService AI 对话业务：会话管理、上下文装配、流式生成与消息落库。
type ChatService struct {
	chatRepo         *repo.ChatRepo        // 会话与消息仓储
	projectRepo      *repo.ProjectRepo     // 项目仓储（归属校验）
	chapterRepo      *repo.ChapterRepo     // 章节仓储
	characterRepo    *repo.CharacterRepo   // 角色仓储
	knowledgeRepo    *repo.KnowledgeRepo   // 知识库仓储
	continuityRepo   *repo.ContinuityRepo  // 章节摘要仓储（@引用摘要）
	settingRepo      *repo.SettingRepo     // 应用配置仓储
	skillService     *SkillService         // 技能服务（解析、变量与参考资料）
	promptProfileSvc *PromptProfileService // 提示词档案服务（系统提示词解析）
	knowledgeService *KnowledgeService     // 知识库服务（RAG 召回）
	toolService      *ToolService          // 工具服务（关键词自动触发与执行）
	providerSvc      *ModelProviderService // 供应商服务（模型选择）
	aiClient         *ai.Client            // 模型网关（流式调用与用量上报）
	db               *gorm.DB              // 事务入口
	config           ChatConfig            // 对话链路参数
}

// NewChatService 构造对话服务。
func NewChatService(chatRepo *repo.ChatRepo, projectRepo *repo.ProjectRepo, chapterRepo *repo.ChapterRepo,
	characterRepo *repo.CharacterRepo, knowledgeRepo *repo.KnowledgeRepo, continuityRepo *repo.ContinuityRepo, settingRepo *repo.SettingRepo,
	skillService *SkillService, promptProfileSvc *PromptProfileService, knowledgeService *KnowledgeService, toolService *ToolService, providerSvc *ModelProviderService,
	aiClient *ai.Client, db *gorm.DB, config ChatConfig) *ChatService {
	return &ChatService{
		chatRepo: chatRepo, projectRepo: projectRepo, chapterRepo: chapterRepo,
		characterRepo: characterRepo, knowledgeRepo: knowledgeRepo, continuityRepo: continuityRepo, settingRepo: settingRepo,
		skillService: skillService, promptProfileSvc: promptProfileSvc, knowledgeService: knowledgeService,
		toolService: toolService, providerSvc: providerSvc, aiClient: aiClient,
		db: db, config: config,
	}
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
	if _, err := s.mustOwnSession(ctx, userULID, req.SessionULID); err != nil {
		return err
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

// Send 流式对话主链路：技能解析 → 上下文装配 → 流式生成 → 落库与统计。
// 流开始前的失败返回 error（api 输出 JSON 包络）；流开始后失败经 SSE error 事件下发并返回 nil。
func (s *ChatService) Send(ctx context.Context, userULID string, req *domain.ChatSendREQ, writer ChatStreamWriter) error {
	project, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID)
	if err != nil {
		return err
	}
	skill, err := s.resolveSkill(ctx, req)
	if err != nil {
		return err
	}
	skillCode, skillTemplate, userInput := skill.SkillCode, skill.PromptTemplate, skill.RemainderText
	session, err := s.resolveSession(ctx, userULID, req)
	if err != nil {
		return err
	}
	sessionMessages, err := s.chatRepo.ListMessagesBySession(ctx, session.SessionULID)
	if err != nil {
		return pkg.WrapBizError(domain.RespCodeStorageError, "会话历史读取失败", err)
	}
	refMessageULIDs, err := validateRefMessages(sessionMessages, req.RefMessageULIDList)
	if err != nil {
		return err
	}
	historyList := selectHistoryMessages(sessionMessages, refMessageULIDs, s.resolveHistoryLimit(ctx, req.HistoryCount))
	contextInput, err := s.loadContextInput(ctx, userULID, req, userInput)
	if err != nil {
		return err
	}
	contextInput.SkillTemplate = skillTemplate
	contextInput.SkillVariable = skill.VariableMap
	contextInput.SkillRefList = skill.RefTextList
	contextInput.UserInput = userInput
	contextInput.HistoryList = historyList
	contextInput.ToolResultList = s.runAutoTools(ctx, userULID, req, userInput, contextInput)
	blocks, instructionText := buildContextBlocks(contextInput)
	assembledText, droppedKeys := ai.AssembleContext(blocks, ai.ContextBudgetChars)
	if len(droppedKeys) > 0 {
		pkg.SLogger().Warnw("上下文块超出预算被丢弃", "session_ulid", session.SessionULID, "dropped_keys", droppedKeys)
	}
	userMessage := &domain.ChatMessageDO{
		MessageULID:    pkg.NewULID(domain.IDPrefixMessage),
		SessionULID:    session.SessionULID,
		RoleCode:       domain.ChatRoleUser,
		ContentText:    req.ContentText,
		SkillCode:      skillCode,
		RefDocJSON:     encodeULIDList(req.RefDocULIDList),
		RefChapterJSON: encodeULIDList(req.RefChapterULIDList),
		RefMessageJSON: encodeULIDList(req.RefMessageULIDList),
		RefSummaryJSON: encodeULIDList(req.RefSummaryULIDList),
		RefPromptJSON:  encodeULIDList(req.RefPromptULIDList),
		TokenUsedCount: estimateTokenCount(utf8.RuneCountInString(req.ContentText), 0),
	}
	if err := s.chatRepo.CreateMessage(ctx, userMessage); err != nil {
		return pkg.WrapBizError(domain.RespCodeStorageError, "消息保存失败", err)
	}
	if session.SessionName == "" {
		if err := s.chatRepo.UpdateSessionName(ctx, session.SessionULID, firstRunes(req.ContentText, sessionNameRunes)); err != nil {
			pkg.SLogger().Warnw("会话名回填失败", "session_ulid", session.SessionULID, "error", err)
		}
	}
	systemPromptText := s.promptProfileSvc.SystemPromptOf(ctx, req.ProjectULID, req.PromptProfileULID)
	messages := buildChatMessages(systemPromptText, project.StyleText, assembledText, instructionText, historyList)
	providerSpec, err := s.providerSvc.ChatSpecOf(ctx, userULID, req.ModelULID)
	if err != nil {
		return err // 供应商不可用：流未开始，api 层输出 JSON 包络
	}
	events := &chatEventWriter{
		writer:      writer,
		sessionULID: session.SessionULID,
		messageULID: pkg.NewULID(domain.IDPrefixMessage),
	}
	replyText, reasoningText, completed := s.streamChat(ctx, events, messages, providerSpec, countMessagesRunes(messages))
	if !completed {
		return nil // 客户端断开或生成失败：不落 assistant 消息，用户消息保留对话痕迹
	}
	assistantMessage := &domain.ChatMessageDO{
		MessageULID:    events.messageULID,
		SessionULID:    session.SessionULID,
		RoleCode:       domain.ChatRoleAssistant,
		ContentText:    replyText,
		ReasoningText:  reasoningText,
		SkillCode:      skillCode,
		RefDocJSON:     encodeULIDList(req.RefDocULIDList),
		RefChapterJSON: encodeULIDList(req.RefChapterULIDList),
		RefMessageJSON: encodeULIDList(req.RefMessageULIDList),
		RefSummaryJSON: encodeULIDList(req.RefSummaryULIDList),
		RefPromptJSON:  encodeULIDList(req.RefPromptULIDList),
		TokenUsedCount: estimateTokenCount(countMessagesRunes(messages), utf8.RuneCountInString(replyText)),
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

// chatSkill 对话链路的技能解析结果（模板与变量分离，供上下文装配渲染）。
type chatSkill struct {
	SkillCode      string            // 技能编码（空为自由对话）
	PromptTemplate string            // 提示词模板（保留 {{context}} 等内置占位符）
	VariableMap    map[string]string // 声明的变量取值
	RefTextList    []string          // 命中的参考资料正文
	RemainderText  string            // 去掉斜杠命令后的用户正文
}

// resolveSkill 解析技能与用户正文：skill_code 优先，否则解析正文首 token 的 /code；
// 项目技能覆盖同编码全局技能，未命中技能时按自由对话处理。
func (s *ChatService) resolveSkill(ctx context.Context, req *domain.ChatSendREQ) (chatSkill, error) {
	resolved, err := s.skillService.Resolve(ctx, &domain.SkillResolveREQ{
		ProjectULID:   req.ProjectULID,
		ContentText:   req.ContentText,
		SkillCode:     req.SkillCode,
		VariableValue: req.SkillVariableMap,
	})
	if err != nil {
		return chatSkill{}, err
	}
	result := chatSkill{
		SkillCode:     resolved.SkillCode,
		RemainderText: resolved.RemainderText,
		VariableMap:   resolved.VariableMap,
		RefTextList:   resolved.RefTextList,
	}
	if resolved.SkillCode == "" {
		return result, nil
	}
	skill, err := s.skillService.SkillTemplate(ctx, req.ProjectULID, resolved.SkillCode)
	if err != nil {
		return chatSkill{}, err
	}
	result.PromptTemplate = skill.PromptTemplate
	return result, nil
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

// runAutoTools 按正文关键词自动触发工具并把结果转为上下文块（失败只告警，不阻断对话）。
func (s *ChatService) runAutoTools(ctx context.Context, userULID string, req *domain.ChatSendREQ,
	userInput string, contextInput chatContextInput) []string {
	if s.toolService == nil {
		return nil
	}
	matchedList, err := s.toolService.ResolveAutoTools(ctx, req.ContentText)
	if err != nil {
		pkg.SLogger().Warnw("工具关键词匹配失败", "error", err)
		return nil
	}
	if len(matchedList) == 0 {
		return nil
	}
	contentText := userInput
	if len(contextInput.ChapterList) > 0 {
		contentText = contextInput.ChapterList[0].ContentText
	}
	paramValue := map[string]string{"content_text": contentText, "query_text": req.ContentText}
	resultList := make([]string, 0, len(matchedList))
	for i := range matchedList {
		dto, invokeErr := s.toolService.Invoke(ctx, userULID, &domain.ToolInvokeREQ{
			ProjectULID: req.ProjectULID,
			ToolCode:    matchedList[i].ToolCode,
			ContentText: contentText,
			ParamValue:  paramValue,
		})
		if invokeErr != nil {
			pkg.SLogger().Warnw("工具执行失败", "tool_code", matchedList[i].ToolCode, "error", invokeErr)
			continue
		}
		resultList = append(resultList, dto.ResultText)
	}
	return resultList
}

// resolveHistoryLimit 历史注入条数：请求值 > 应用配置 chat_history_count > 全局配置。
func (s *ChatService) resolveHistoryLimit(ctx context.Context, reqCount int) int {
	if reqCount > 0 {
		return reqCount
	}
	setting, err := s.settingRepo.GetByKey(ctx, settingKeyChatHistoryCount)
	if err != nil {
		pkg.SLogger().Warnw("读取历史条数配置失败，回退全局配置", "error", err)
	} else if setting != nil {
		if value, convErr := strconv.Atoi(setting.SettingValue); convErr == nil && value > 0 {
			return value
		}
	}
	return s.config.HistoryCount
}

// validateRefMessages 校验显式引用消息均属于当前会话（去重保序）。
func validateRefMessages(sessionMessages []domain.ChatMessageDO, refMessageULIDs []string) ([]string, error) {
	existing := make(map[string]bool, len(sessionMessages))
	for i := range sessionMessages {
		existing[sessionMessages[i].MessageULID] = true
	}
	seen := make(map[string]bool, len(refMessageULIDs))
	result := make([]string, 0, len(refMessageULIDs))
	for _, ulid := range refMessageULIDs {
		if seen[ulid] {
			continue
		}
		if !existing[ulid] {
			return nil, pkg.NewBizError(domain.RespCodeInvalidParam, "引用消息不存在或不属于该会话")
		}
		seen[ulid] = true
		result = append(result, ulid)
	}
	return result, nil
}

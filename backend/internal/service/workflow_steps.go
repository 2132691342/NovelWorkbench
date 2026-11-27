// 本文件实现工作流步骤的收尾动作：摘要落账、定稿入库、审核门禁、完成/失败通知与模型调用。

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cloudwego/eino/schema"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/ai/check"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
)

// stepSummary 生成章节摘要并写入连续性账本（供后续章节上下文注入）。
func (s *WorkflowService) stepSummary(ctx context.Context, runULID string, chapter *domain.ChapterDO,
	stepDef domain.WorkflowStepDefDTO, params domain.WorkflowRunParamsDTO) (workflowStepResult, string, string, error) {
	runDO, err := s.workflowRepo.GetRun(ctx, runULID)
	if err != nil || runDO == nil {
		return workflowStepResult{}, "", "", pkg.NewBizError(domain.RespCodeStorageError, "运行实例读取失败")
	}
	text := runDO.CandidateText
	if strings.TrimSpace(text) == "" {
		text = chapter.ContentText
	}
	if strings.TrimSpace(text) == "" {
		return workflowStepResult{}, "", "", nil
	}
	promptCode := firstNonEmpty(stepDef.PromptCode, "summary")
	promptText, err := s.promptSvc.PromptTextOf(ctx, chapter.ProjectULID, promptCode)
	if err != nil {
		return workflowStepResult{}, "", "", err
	}
	rendered := ai.RenderPrompt(promptText, map[string]string{
		"chapter_content": condenseChapterText(text),
		"context":         "",
		"user_input":      "",
		"prompt_code":     promptCode,
	})
	spec, err := s.providerSvc.ChatSpecOf(ctx, runDO.UserULID, params.ModelULID)
	if err != nil {
		return workflowStepResult{}, "", "", err
	}
	stepCtx, cancel := context.WithTimeout(ctx, workflowStepTimeout)
	defer cancel()
	summaryText, err := s.generateText(stepCtx, spec, workflowGenericSystemPrompt, rendered,
		domain.WorkflowStepTypeSummary, runDO.TraceID, runDO.RunULID)
	if err != nil {
		return workflowStepResult{}, "", "", err
	}
	if err := s.saveChapterSummary(ctx, chapter, summaryText); err != nil {
		return workflowStepResult{}, "", "", err
	}
	return workflowStepResult{OutputText: summaryText}, pkg.TruncateText(summaryText, 2000), `{}`, nil
}

// stepApply 定稿入库：把候选正文写回章节（含快照留档与字数账目）。
func (s *WorkflowService) stepApply(ctx context.Context, runULID string) (workflowStepResult, string, string, error) {
	runDO, err := s.workflowRepo.GetRun(ctx, runULID)
	if err != nil || runDO == nil {
		return workflowStepResult{}, "", "", pkg.NewBizError(domain.RespCodeStorageError, "运行实例读取失败")
	}
	if strings.TrimSpace(runDO.CandidateText) == "" {
		return workflowStepResult{}, "候选正文已由人工审核写入章节，无需重复入库", `{}`, nil
	}
	if err := s.applyCandidate(ctx, runDO); err != nil {
		return workflowStepResult{}, "", "", err
	}
	wordCount := pkg.CountWords(runDO.CandidateText)
	return workflowStepResult{OutputText: "已写回章节"}, fmt.Sprintf("已写回章节，%d 字", wordCount),
		fmt.Sprintf(`{"word_count":%d}`, wordCount), nil
}

// enterReviewGate 进入人工审核门禁：记录待审核步骤并置运行状态。
func (s *WorkflowService) enterReviewGate(ctx context.Context, runULID string, stepIndex int,
	stepDef domain.WorkflowStepDefDTO) error {
	runDO, err := s.workflowRepo.GetRun(ctx, runULID)
	if err != nil || runDO == nil {
		return pkg.NewBizError(domain.RespCodeStorageError, "运行实例读取失败")
	}
	if err := s.recordStep(ctx, runDO, stepIndex, domain.WorkflowStepTypeHumanReview, stepDef.StepName,
		domain.WorkflowStepStatusRunning, "等待人工审核", "", 0); err != nil {
		return err
	}
	return s.workflowRepo.UpdateRun(ctx, runULID, map[string]any{
		"run_status_code":    domain.WorkflowRunStatusWaitingReview,
		"current_step_index": stepIndex,
	})
}

// stopRequested 判断是否收到暂停或取消请求。
func (s *WorkflowService) stopRequested(ctx context.Context, runULID string) bool {
	runDO, err := s.workflowRepo.GetRun(ctx, runULID)
	if err != nil || runDO == nil {
		return true
	}
	return runDO.CancelRequestedFlag || runDO.PauseRequestedFlag
}

// handleStop 在步骤边界响应暂停/取消，返回对应结果。
func (s *WorkflowService) handleStop(ctx context.Context, runULID string) chapterOutcome {
	runDO, err := s.workflowRepo.GetRun(ctx, runULID)
	if err != nil || runDO == nil {
		return chapterOutcomeFailed
	}
	if runDO.CancelRequestedFlag {
		_ = s.workflowRepo.UpdateRun(ctx, runULID, map[string]any{"finished_at": time.Now()})
		return chapterOutcomeCanceled
	}
	if err := s.workflowRepo.UpdateRun(ctx, runULID, map[string]any{
		"run_status_code": domain.WorkflowRunStatusPaused,
	}); err != nil {
		return chapterOutcomeFailed
	}
	return chapterOutcomePaused
}

// finishRun 标记运行全部完成并发出站内通知。
func (s *WorkflowService) finishRun(ctx context.Context, runULID string) {
	runDO, err := s.workflowRepo.GetRun(ctx, runULID)
	if err != nil || runDO == nil {
		return
	}
	if err := s.workflowRepo.UpdateRun(ctx, runULID, map[string]any{
		"run_status_code":        domain.WorkflowRunStatusSucceeded,
		"finished_chapter_count": runDO.TotalChapterCount,
		"finished_at":            time.Now(),
	}); err != nil {
		return
	}
	if s.notifySvc != nil {
		_ = s.notifySvc.Notify(ctx, domain.NotificationCreateDTO{
			UserULID:              runDO.UserULID,
			NotificationTypeCode:  domain.NotificationTypeTask,
			NotificationTitleText: "工作流已完成：" + runDO.TemplateName,
			NotificationBodyText:  fmt.Sprintf("共 %d 章全部完成，可到工作流中心查看产出。", runDO.TotalChapterCount),
			RefTypeCode:           "workflow_run",
			RefULID:               runDO.RunULID,
		})
	}
	s.recordRunAudit(ctx, runDO, domain.WorkflowRunStatusSucceeded,
		fmt.Sprintf("共 %d 章全部完成", runDO.TotalChapterCount))
}

// failRun 标记运行失败并发出站内通知。
func (s *WorkflowService) failRun(ctx context.Context, runDO *domain.WorkflowRunDO, message string) {
	message = pkg.TruncateText(message, 500)
	if err := s.workflowRepo.UpdateRun(ctx, runDO.RunULID, map[string]any{
		"run_status_code": domain.WorkflowRunStatusFailed,
		"error_message":   message,
		"finished_at":     time.Now(),
	}); err != nil {
		pkg.SLogger().Errorw("工作流失败状态写入失败", "run_ulid", runDO.RunULID, "error", err)
	}
	if s.notifySvc != nil {
		_ = s.notifySvc.Notify(ctx, domain.NotificationCreateDTO{
			UserULID:              runDO.UserULID,
			NotificationTypeCode:  domain.NotificationTypeTask,
			NotificationTitleText: "工作流失败：" + runDO.TemplateName,
			NotificationBodyText:  message,
			RefTypeCode:           "workflow_run",
			RefULID:               runDO.RunULID,
		})
	}
	s.recordRunAudit(ctx, runDO, domain.WorkflowRunStatusFailed, message)
}

// recordStepOf 按步骤定义落一条步骤记录（步骤索引与类型来自模板）。
// 同时写一条审计：步骤是后台链路，不经过 HTTP 中间件，不显式留痕的话
// 「启动工作流」的链路时间线里只有一次创建请求，看不到后续每一步。
func (s *WorkflowService) recordStepOf(ctx context.Context, runULID, chapterULID string, stepIndex int,
	stepDef domain.WorkflowStepDefDTO, statusCode, outputText, metricJSON string, latencyMs int64) error {
	now := time.Now()
	s.recordStepAudit(ctx, runULID, chapterULID, stepDef, statusCode, latencyMs)
	return s.workflowRepo.CreateStep(ctx, &domain.WorkflowStepDO{
		StepULID:       pkg.NewULID(domain.IDPrefixWorkflowStep),
		RunULID:        runULID,
		ChapterULID:    chapterULID,
		StepIndex:      stepIndex,
		StepTypeCode:   stepDef.StepTypeCode,
		StepName:       stepDef.StepName,
		StepStatusCode: statusCode,
		PromptCode:     stepDef.PromptCode,
		OutputText:     pkg.TruncateText(outputText, 20000),
		MetricJSON:     metricJSON,
		AttemptCount:   1,
		LatencyMs:      latencyMs,
		StartedAt:      now,
		FinishedAt:     now,
	})
}

// recordGateAudit 记录一次人工审核决定（挂在运行的链路下，运行时间线才看得出「谁在什么时候点头」）。
// 审核是 HTTP 请求，本身已有根 span；这里的子 span 让运行的 trace_id 时间线保持完整。
func (s *WorkflowService) recordGateAudit(ctx context.Context, runDO *domain.WorkflowRunDO, stepName, statusCode, noteText string) {
	if s.auditRecorder == nil || runDO == nil {
		return
	}
	levelCode, httpStatus := domain.AuditLevelInfo, 200
	if statusCode == domain.WorkflowStepStatusRejected {
		levelCode, httpStatus = domain.AuditLevelWarn, 400
	}
	do := &domain.AuditLogDO{
		LogLevelCode:   levelCode,
		SpanTypeCode:   domain.AuditSpanTypeWorkflow,
		TraceID:        runDO.TraceID,
		ParentSpanCode: domain.AuditActionWorkflowRunCreate,
		UserULID:       pkg.ActorFromContext(ctx),
		ActionCode:     "workflow/gate/" + statusCode,
		ActionName:     "工作流人工审核",
		RequestPath:    "workflow://" + runDO.TemplateCode,
		RequestMethod:  "INTERNAL",
		TargetType:     "workflow_run",
		TargetULID:     runDO.RunULID,
		RespCode:       "GATE_" + strings.ToUpper(statusCode),
		RespMessage:    pkg.TruncateText(stepName+"："+noteText, 512),
		HTTPStatus:     httpStatus,
	}
	if err := s.auditRecorder.Record(ctx, do); err != nil {
		pkg.SLogger().Warnw("工作流审核审计写入失败", "run_ulid", runDO.RunULID, "error", err)
	}
}

// recordRunAudit 记录运行进入终态（完成/失败/取消），让运行链路有一个明确的收尾 span。
func (s *WorkflowService) recordRunAudit(ctx context.Context, runDO *domain.WorkflowRunDO, statusCode, messageText string) {
	if s.auditRecorder == nil || runDO == nil {
		return
	}
	levelCode, httpStatus := domain.AuditLevelInfo, 200
	switch statusCode {
	case domain.WorkflowRunStatusFailed:
		levelCode, httpStatus = domain.AuditLevelError, 500
	case domain.WorkflowRunStatusCanceled:
		levelCode, httpStatus = domain.AuditLevelWarn, 400
	}
	do := &domain.AuditLogDO{
		LogLevelCode:   levelCode,
		SpanTypeCode:   domain.AuditSpanTypeWorkflow,
		TraceID:        runDO.TraceID,
		ParentSpanCode: domain.AuditActionWorkflowRunCreate,
		UserULID:       runDO.UserULID,
		ActionCode:     "workflow/run/" + statusCode,
		ActionName:     "工作流运行收尾",
		RequestPath:    "workflow://" + runDO.TemplateCode,
		RequestMethod:  "INTERNAL",
		TargetType:     "workflow_run",
		TargetULID:     runDO.RunULID,
		RespCode:       "RUN_" + strings.ToUpper(statusCode),
		RespMessage:    pkg.TruncateText(messageText, 512),
		HTTPStatus:     httpStatus,
	}
	if err := s.auditRecorder.Record(ctx, do); err != nil {
		pkg.SLogger().Warnw("工作流运行收尾审计写入失败", "run_ulid", runDO.RunULID, "error", err)
	}
}

// recordStepAudit 为一步执行写审计（链路 ID 取运行的 trace_id，按步骤状态映射级别）。
func (s *WorkflowService) recordStepAudit(ctx context.Context, runULID, chapterULID string,
	stepDef domain.WorkflowStepDefDTO, statusCode string, latencyMs int64) {
	if s.auditRecorder == nil {
		return
	}
	runDO, err := s.workflowRepo.GetRun(ctx, runULID)
	if err != nil || runDO == nil {
		return
	}
	httpStatus, levelCode := stepAuditLevelOf(statusCode)
	do := &domain.AuditLogDO{
		LogLevelCode:   levelCode,
		SpanTypeCode:   domain.AuditSpanTypeWorkflow,
		TraceID:        runDO.TraceID,
		ParentSpanCode: domain.AuditActionWorkflowRunCreate,
		UserULID:       runDO.UserULID,
		ActionCode:     "workflow/step/" + stepDef.StepTypeCode,
		ActionName:     pkg.AuditActionLabel("workflow/step/" + stepDef.StepTypeCode),
		RequestPath:    "workflow://" + runDO.TemplateCode,
		RequestMethod:  "INTERNAL",
		TargetType:     "workflow_run",
		TargetULID:     runDO.RunULID,
		RespCode:       "STEP_" + strings.ToUpper(statusCode),
		RespMessage:    stepDef.StepName + "：" + statusCode,
		HTTPStatus:     httpStatus,
		LatencyMs:      latencyMs,
	}
	_ = s.auditRecorder.Record(ctx, do)
}

// stepAuditLevelOf 把步骤状态映射成审计级别与 HTTP 状态（仅失败与人工退回算异常）。
// skipped（体检达标跳过修订）是正常结果，记 warn 会让每次健康运行在审计页都顶着橙色级别。
func stepAuditLevelOf(statusCode string) (httpStatus int, levelCode string) {
	switch statusCode {
	case domain.WorkflowStepStatusFailed:
		return 500, domain.AuditLevelError
	case domain.WorkflowStepStatusRejected:
		return 400, domain.AuditLevelWarn
	default:
		return 200, domain.AuditLevelInfo
	}
}

// 工作流步骤的系统提示词：起草类产出会被原样写进章节，必须明确「只交正文」。
const (
	workflowBodyOnlySystemPrompt = "只输出可直接入库的正文。不要输出分析、清单、改动说明、标题、序号或任何过程文字。"
	workflowGenericSystemPrompt  = "严格按用户给定的格式与约束输出，不写多余说明。"
)

// generateText 流式累积一次模型输出（结构化长输出用非流式会被网关首包超时打断）。
// stepTypeCode 用于 AI 调用留痕（ai/workflow/<步骤类型>），空则按通用 AI 步骤记录。
func (s *WorkflowService) generateText(ctx context.Context, spec ai.ProviderSpec, systemText, promptText,
	stepTypeCode, traceID, runULID string) (string, error) {
	startedAt := time.Now()
	firstChunkAt := time.Time{}
	var usage ai.StreamUsage
	succeeded := false
	// 用量按「一次模型调用一条记录」在流结束时上报，避免只有次数没有 token 的伪统计。
	defer func() {
		latency := time.Since(startedAt)
		if !firstChunkAt.IsZero() {
			latency = time.Since(firstChunkAt)
		}
		if succeeded {
			s.aiClient.RecordStreamUsage(spec, usage.TokenUsage(), latency)
			return
		}
		s.aiClient.RecordStreamFailure(spec, usage.TokenUsage(), latency)
	}()
	stream, err := s.aiClient.Stream(ctx, spec, []*schema.Message{
		{Role: schema.System, Content: systemText},
		{Role: schema.User, Content: promptText},
	})
	if err != nil {
		s.recordAICallAudit(ctx, spec, stepTypeCode, traceID, runULID, promptText, startedAt, "", err)
		return "", pkg.WrapBizError(domain.RespCodeAIProviderError, "模型调用失败", err)
	}
	defer stream.Close()
	var builder strings.Builder
	for {
		chunk, recvErr := stream.Recv()
		if recvErr != nil {
			if recvErr.Error() == "EOF" {
				break
			}
			if strings.Contains(recvErr.Error(), "EOF") {
				break
			}
			s.recordAICallAudit(ctx, spec, stepTypeCode, traceID, runULID, promptText, startedAt, builder.String(), recvErr)
			return "", pkg.WrapBizError(domain.RespCodeAIProviderError, "模型输出中断", recvErr)
		}
		if chunk == nil {
			continue
		}
		if firstChunkAt.IsZero() {
			firstChunkAt = time.Now()
		}
		usage.Add(chunk)
		builder.WriteString(chunk.Content)
	}
	replyText := strings.TrimSpace(builder.String())
	succeeded = true
	s.recordAICallAudit(ctx, spec, stepTypeCode, traceID, runULID, promptText, startedAt, replyText, nil)
	return replyText, nil
}

// recordAICallAudit 为一次工作流内的模型调用写 ai_call 审计（挂在创建运行的请求之下）。
func (s *WorkflowService) recordAICallAudit(ctx context.Context, spec ai.ProviderSpec, stepTypeCode,
	traceID, runULID, promptText string, startedAt time.Time, replyText string, callErr error) {
	actionCode := "ai/workflow/" + stepTypeCode
	recordAICall(ctx, s.auditRecorder, aiCallMeta{
		ActionCode:     actionCode,
		ActionName:     pkg.AuditActionLabel(actionCode),
		ParentSpanCode: domain.AuditActionWorkflowRunCreate,
		TraceID:        traceID,
		TargetType:     "workflow_run",
		TargetULID:     runULID,
		ModelName:      spec.ChatModelName,
		PromptChars:    utf8.RuneCountInString(promptText),
	}, time.Since(startedAt),
		estimateTokenCount(utf8.RuneCountInString(promptText), utf8.RuneCountInString(replyText)), callErr)
}

// pendingHookText 取项目内未回收的伏笔摘要（限制条数与长度，避免上下文膨胀）。
func (s *WorkflowService) pendingHookText(ctx context.Context, projectULID string) (string, error) {
	loreList, err := s.continuityRepo.ListHooksByProject(ctx, projectULID, "")
	if err != nil {
		return "", err
	}
	if len(loreList) == 0 {
		return "", nil
	}
	var builder strings.Builder
	builder.WriteString("【待回收伏笔】\n")
	count := 0
	for i := range loreList {
		if loreList[i].HookStatusCode == domain.HookStatusCodeResolved ||
			loreList[i].HookStatusCode == domain.HookStatusCodeAbandoned {
			continue
		}
		builder.WriteString("- " + truncateRunes(loreList[i].DescriptionText, 120) + "\n")
		count++
		if count >= 5 {
			break
		}
	}
	if count == 0 {
		return "", nil
	}
	return builder.String(), nil
}

// saveChapterSummary 写入章节摘要（已存在则更新，状态置 pending 待人工核验）。
func (s *WorkflowService) saveChapterSummary(ctx context.Context, chapter *domain.ChapterDO, summaryText string) error {
	existing, err := s.continuityRepo.GetSummaryByChapter(ctx, chapter.ProjectULID, chapter.ChapterULID)
	if err != nil {
		return err
	}
	if existing != nil {
		return s.continuityRepo.UpdateSummary(ctx, existing.SummaryULID, map[string]any{
			"summary_text":        summaryText,
			"summary_status_code": domain.SummaryStatusCodePending,
		})
	}
	return s.continuityRepo.CreateSummary(ctx, &domain.ChapterSummaryDO{
		SummaryULID:       pkg.NewULID(domain.IDPrefixChapterSummary),
		ProjectULID:       chapter.ProjectULID,
		ChapterULID:       chapter.ChapterULID,
		SummaryText:       summaryText,
		SummaryStatusCode: domain.SummaryStatusCodePending,
	})
}

// condenseChapterText 收敛进入提示词的正文长度（保留首尾，避免超出模型上下文）。
func condenseChapterText(contentText string) string {
	trimmed := strings.TrimSpace(contentText)
	runes := []rune(trimmed)
	if len(runes) <= workflowChapterMaxRunes {
		return trimmed
	}
	head := workflowChapterMaxRunes * 3 / 5
	tail := workflowChapterMaxRunes * 2 / 5
	return string(runes[:head]) + "\n\n…（中间省略 " + intToText(int64(len(runes)-head-tail)) + " 字）…\n\n" +
		string(runes[len(runes)-tail:])
}

// parseReviewVerdict 解析复审 JSON：取结论、分数与问题清单文本；非 JSON 时按纯文本处理。
func parseReviewVerdict(replyText string) (string, int, string) {
	trimmed := strings.TrimSpace(replyText)
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		var payload struct {
			VerdictCode string `json:"verdict_code"`
			ScoreValue  int    `json:"score_value"`
			IssueList   []struct {
				LevelCode string `json:"level_code"`
				IssueText string `json:"issue_text"`
				QuoteText string `json:"quote_text"`
				FixText   string `json:"fix_text"`
			} `json:"issue_list"`
		}
		if err := json.Unmarshal([]byte(trimmed[start:end+1]), &payload); err == nil {
			var builder strings.Builder
			builder.WriteString("复审结论：" + payload.VerdictCode + "，得分 " + intToText(int64(payload.ScoreValue)) + "\n")
			for _, issue := range payload.IssueList {
				builder.WriteString("- [" + issue.LevelCode + "] " + issue.IssueText)
				if issue.QuoteText != "" {
					builder.WriteString("（原句：" + issue.QuoteText + "）")
				}
				if issue.FixText != "" {
					builder.WriteString(" 改法：" + issue.FixText)
				}
				builder.WriteString("\n")
			}
			return payload.VerdictCode, payload.ScoreValue, strings.TrimSpace(builder.String())
		}
	}
	return "revise", 0, trimmed
}

// verdictLabelOf 体检结论中文标签（工作流步骤输出用）。
func verdictLabelOf(verdictCode string) string {
	switch verdictCode {
	case check.VerdictPass:
		return "可发"
	case check.VerdictBlock:
		return "不可发"
	default:
		return "需修"
	}
}

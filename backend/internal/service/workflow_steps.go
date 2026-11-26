// 本文件实现工作流步骤的收尾动作：摘要落账、定稿入库、审核门禁、完成/失败通知与模型调用。

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

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
	summaryText, err := s.generateText(stepCtx, spec, workflowGenericSystemPrompt, rendered)
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
	if err := s.recordStep(ctx, runDO, domain.WorkflowStepTypeHumanReview, stepDef.StepName,
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
	httpStatus := 200
	levelCode := domain.AuditLevelInfo
	switch statusCode {
	case domain.WorkflowStepStatusFailed:
		httpStatus, levelCode = 500, domain.AuditLevelError
	case domain.WorkflowStepStatusSkipped, domain.WorkflowStepStatusRejected:
		httpStatus, levelCode = 400, domain.AuditLevelWarn
	}
	do := &domain.AuditLogDO{
		LogLevelCode:  levelCode,
		TraceID:       runDO.TraceID,
		UserULID:      runDO.UserULID,
		ActionCode:    "workflow/step/" + stepDef.StepTypeCode,
		RequestPath:   "workflow://" + runDO.TemplateCode,
		RequestMethod: "INTERNAL",
		TargetType:    "workflow_run",
		TargetULID:    runDO.RunULID,
		RespCode:      "STEP_" + strings.ToUpper(statusCode),
		RespMessage:   stepDef.StepName + "：" + statusCode,
		HTTPStatus:    httpStatus,
		LatencyMs:     latencyMs,
	}
	_ = s.auditRecorder.Record(ctx, do)
}

// 工作流步骤的系统提示词：起草类产出会被原样写进章节，必须明确「只交正文」。
const (
	workflowBodyOnlySystemPrompt = "只输出可直接入库的正文。不要输出分析、清单、改动说明、标题、序号或任何过程文字。"
	workflowGenericSystemPrompt  = "严格按用户给定的格式与约束输出，不写多余说明。"
)

// generateText 流式累积一次模型输出（结构化长输出用非流式会被网关首包超时打断）。
func (s *WorkflowService) generateText(ctx context.Context, spec ai.ProviderSpec, systemText, promptText string) (string, error) {
	stream, err := s.aiClient.Stream(ctx, spec, []*schema.Message{
		{Role: schema.System, Content: systemText},
		{Role: schema.User, Content: promptText},
	})
	if err != nil {
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
			return "", pkg.WrapBizError(domain.RespCodeAIProviderError, "模型输出中断", recvErr)
		}
		if chunk != nil {
			builder.WriteString(chunk.Content)
		}
	}
	return strings.TrimSpace(builder.String()), nil
}

// chapterContextText 装配章节上下文：项目设定、角色卡、前章摘要与大纲要求。
func (s *WorkflowService) chapterContextText(ctx context.Context, chapter *domain.ChapterDO,
	params domain.WorkflowRunParamsDTO) (string, error) {
	project, err := s.projectRepo.GetByULID(ctx, chapter.ProjectULID)
	if err != nil {
		return "", err
	}
	var builder strings.Builder
	if project != nil {
		if project.SynopsisText != "" {
			builder.WriteString("【项目简介】\n" + project.SynopsisText + "\n\n")
		}
		if project.StyleText != "" {
			builder.WriteString("【文风要求】\n" + project.StyleText + "\n\n")
		}
	}
	builder.WriteString("【本章】第 " + intToText(int64(chapter.ChapterOrder)) + " 章 " + chapter.ChapterName + "\n")
	if chapter.SummaryText != "" {
		builder.WriteString("章节梗概：" + chapter.SummaryText + "\n")
	}
	summaries, err := s.continuityRepo.ListSummariesByProject(ctx, chapter.ProjectULID, "", "")
	if err == nil && len(summaries) > 0 {
		builder.WriteString("\n【前情摘要】\n")
		appended := 0
		for i := range summaries {
			if summaries[i].ChapterULID == chapter.ChapterULID {
				continue
			}
			builder.WriteString("- " + truncateRunes(summaries[i].SummaryText, 300) + "\n")
			appended++
			if appended >= 5 {
				break
			}
		}
	}
	hookText, err := s.pendingHookText(ctx, chapter.ProjectULID)
	if err == nil && hookText != "" {
		builder.WriteString("\n" + hookText)
	}
	return strings.TrimSpace(builder.String()), nil
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

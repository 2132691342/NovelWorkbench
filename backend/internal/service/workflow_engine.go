// 本文件实现工作流执行引擎：按模板步骤逐章执行，AI 出稿、确定性体检、人工审核门禁。
// 单进程内以 goroutine 调度，同一运行同时只跑一个执行者；步骤边界响应暂停/取消。

package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/ai/check"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
)

// 执行引擎规模约束。
const (
	workflowStepTimeout     = 10 * time.Minute // 单个 AI 步骤超时
	workflowRunMinTimeout   = 60 * time.Minute // 单次运行最低总时长
	workflowRunPerChapter   = 20 * time.Minute // 每章追加的运行时长（推理模型单章可能跑十几分钟）
	workflowReviewMaxTokens = 4000             // 复审输出上限
	workflowDraftMaxTokens  = 12000            // 起草/改写输出上限（整章）
)

// workflowRunner 工作流执行器：管理运行中的 goroutine，保证同一运行不并发执行。
type workflowRunner struct {
	svc      *WorkflowService
	mu       sync.Mutex
	activeSet map[string]bool // 正在执行的运行 ULID
}

// newWorkflowRunner 构造执行器。
func newWorkflowRunner(svc *WorkflowService) *workflowRunner {
	return &workflowRunner{svc: svc, activeSet: make(map[string]bool, 4)}
}

// start 启动一次运行（已在进行中则忽略，避免重复执行）。
func (r *workflowRunner) start(runULID string) {
	r.mu.Lock()
	if r.activeSet[runULID] {
		r.mu.Unlock()
		return
	}
	r.activeSet[runULID] = true
	r.mu.Unlock()
	go func() {
		defer func() {
			r.mu.Lock()
			delete(r.activeSet, runULID)
			r.mu.Unlock()
		}()
		r.svc.executeRun(runULID)
	}()
}

// resume 从审核门禁继续执行（与 start 同一入口，语义更明确）。
func (r *workflowRunner) resume(runULID string) {
	r.start(runULID)
}

// RecoverInterrupted 启动时恢复中断的运行：把 running 状态退回 queued 并重新调度。
func (r *workflowRunner) RecoverInterrupted(ctx context.Context) {
	dos, err := r.svc.workflowRepo.ListRunsByStatus(ctx, []string{domain.WorkflowRunStatusRunning})
	if err != nil {
		pkg.SLogger().Warnw("工作流中断恢复查询失败", "error", err)
		return
	}
	for i := range dos {
		if err := r.svc.workflowRepo.UpdateRun(ctx, dos[i].RunULID, map[string]any{
			"run_status_code": domain.WorkflowRunStatusQueued,
		}); err != nil {
			pkg.SLogger().Warnw("工作流中断恢复更新失败", "run_ulid", dos[i].RunULID, "error", err)
			continue
		}
		pkg.SLogger().Infow("工作流中断任务已重新入队", "run_ulid", dos[i].RunULID)
		r.start(dos[i].RunULID)
	}
}

// executeRun 执行一次运行：逐章推进，直到完成、失败、取消或停在审核门禁。
func (s *WorkflowService) executeRun(runULID string) {
	runDO, err := s.workflowRepo.GetRun(context.Background(), runULID)
	if err != nil || runDO == nil {
		pkg.SLogger().Warnw("工作流运行读取失败", "run_ulid", runULID, "error", err)
		return
	}
	// 运行总时长随章节数放大：推理模型单章可能跑十几分钟，固定上限会让长任务必然中断。
	totalTimeout := workflowRunMinTimeout + time.Duration(len(decodeStringList(runDO.ChapterULIDListJSON)))*workflowRunPerChapter
	ctx, cancel := context.WithTimeout(context.Background(), totalTimeout)
	defer cancel()
	if runDO.RunStatusCode != domain.WorkflowRunStatusQueued {
		return // 已被取消或正在别处执行
	}
	template, err := s.workflowRepo.GetTemplateByCode(ctx, runDO.TemplateCode)
	if err != nil || template == nil {
		s.failRun(ctx, runDO, "工作流模板不存在")
		return
	}
	stepList := unmarshalWorkflowSteps(template.StepSchemaJSON)
	if err := s.workflowRepo.UpdateRun(ctx, runDO.RunULID, map[string]any{
		"run_status_code": domain.WorkflowRunStatusRunning,
		"started_at":      time.Now(),
	}); err != nil {
		return
	}
	params := unmarshalWorkflowParams(runDO.ParamsJSON)
	chapterULIDList := decodeStringList(runDO.ChapterULIDListJSON)
	for chapterIndex := runDO.CurrentIndex; chapterIndex < len(chapterULIDList); chapterIndex++ {
		if s.stopRequested(ctx, runDO.RunULID) {
			return
		}
		outcome := s.runChapter(ctx, runDO.RunULID, chapterULIDList[chapterIndex], chapterIndex, stepList, params)
		switch outcome {
		case chapterOutcomeWaitingReview:
			return // 停在门禁，等 Approve/Reject 再继续
		case chapterOutcomeCanceled, chapterOutcomePaused:
			return
		case chapterOutcomeFailed:
			return
		}
	}
	s.finishRun(ctx, runDO.RunULID)
}

// chapterOutcome 单章执行结果。
type chapterOutcome int

// 单章执行结果取值。
const (
	chapterOutcomeDone          chapterOutcome = iota // 本章完成，继续下一章
	chapterOutcomeWaitingReview                       // 停在人工审核
	chapterOutcomePaused                              // 已暂停
	chapterOutcomeCanceled                            // 已取消
	chapterOutcomeFailed                              // 失败
)

// runChapter 执行一章的全部步骤（从 current_step_index 开始，支持审核后继续）。
func (s *WorkflowService) runChapter(ctx context.Context, runULID, chapterULID string,
	chapterIndex int, stepList []domain.WorkflowStepDefDTO, params domain.WorkflowRunParamsDTO) chapterOutcome {
	runDO, err := s.workflowRepo.GetRun(ctx, runULID)
	if err != nil || runDO == nil {
		return chapterOutcomeFailed
	}
	chapter, err := s.chapterRepo.GetByULID(ctx, chapterULID)
	if err != nil || chapter == nil {
		s.failRun(ctx, runDO, "目标章节不存在: "+chapterULID)
		return chapterOutcomeFailed
	}
	if err := s.workflowRepo.UpdateRun(ctx, runULID, map[string]any{
		"current_index": chapterIndex,
	}); err != nil {
		return chapterOutcomeFailed
	}
	startStepIndex := 0
	if chapterIndex == runDO.CurrentIndex {
		startStepIndex = runDO.CurrentStepIndex
	}
	checkPassFlag := false
	for stepIndex := startStepIndex; stepIndex < len(stepList); stepIndex++ {
		if s.stopRequested(ctx, runULID) {
			return s.handleStop(ctx, runULID)
		}
		stepDef := stepList[stepIndex]
		if stepDef.StepTypeCode == domain.WorkflowStepTypeHumanReview {
			if params.ReviewGateFlag == nil || !*params.ReviewGateFlag {
				continue // 未开启人工审核：门禁步骤直接跳过
			}
			if err := s.enterReviewGate(ctx, runULID, stepIndex, stepDef); err != nil {
				s.failRun(ctx, runDO, err.Error())
				return chapterOutcomeFailed
			}
			return chapterOutcomeWaitingReview
		}
		if stepDef.SkipWhenPass && checkPassFlag {
			if err := s.recordStepOf(ctx, runULID, chapterULID, stepIndex, stepDef,
				domain.WorkflowStepStatusSkipped, "体检已达标，跳过本步", "", 0); err != nil {
				return chapterOutcomeFailed
			}
			continue
		}
		result, stepErr := s.executeStep(ctx, runULID, chapter, stepIndex, stepDef, params)
		if stepErr != nil {
			s.failRun(ctx, runDO, stepDef.StepName+" 失败: "+stepErr.Error())
			return chapterOutcomeFailed
		}
		if stepDef.StepTypeCode == domain.WorkflowStepTypeCheck {
			checkPassFlag = result.CheckScore >= params.GateScore
			// 体检不达标时继续走修订步骤；达标时修订步骤会被 SkipWhenPass 跳过
		}
		if err := s.workflowRepo.UpdateRun(ctx, runULID, map[string]any{
			"current_step_index": stepIndex + 1,
		}); err != nil {
			return chapterOutcomeFailed
		}
	}
	// 本章步骤全部完成：进入下一章
	finishedCount := runDO.FinishedChapterCount
	if chapterIndex >= finishedCount {
		finishedCount = chapterIndex + 1
	}
	if err := s.workflowRepo.UpdateRun(ctx, runULID, map[string]any{
		"finished_chapter_count": finishedCount,
		"current_index":          chapterIndex + 1,
		"current_step_index":     0,
		"candidate_text":         "",
		"feedback_text":          "",
	}); err != nil {
		return chapterOutcomeFailed
	}
	return chapterOutcomeDone
}

// workflowStepResult 单步执行结果（体检分与复审结论供后续步骤与详情使用）。
type workflowStepResult struct {
	CheckScore   int
	VerdictCode  string
	OutputText   string
}

// executeStep 按步骤类型执行一步，成功后落步骤记录。
func (s *WorkflowService) executeStep(ctx context.Context, runULID string, chapter *domain.ChapterDO,
	stepIndex int, stepDef domain.WorkflowStepDefDTO, params domain.WorkflowRunParamsDTO) (workflowStepResult, error) {
	startedAt := time.Now()
	result := workflowStepResult{}
	var outputText, metricJSON string
	var err error
	switch stepDef.StepTypeCode {
	case domain.WorkflowStepTypeCheck:
		result, outputText, metricJSON, err = s.stepCheck(ctx, runULID, chapter)
	case domain.WorkflowStepTypeReview:
		result, outputText, metricJSON, err = s.stepReview(ctx, runULID, chapter, stepDef, params)
	case domain.WorkflowStepTypeSummary:
		result, outputText, metricJSON, err = s.stepSummary(ctx, runULID, chapter, stepDef, params)
	case domain.WorkflowStepTypeApply:
		result, outputText, metricJSON, err = s.stepApply(ctx, runULID)
	default:
		// draft / revise：AI 出正文并写回候选正文
		result, outputText, metricJSON, err = s.stepDraft(ctx, runULID, chapter, stepDef, params)
	}
	latencyMs := time.Since(startedAt).Milliseconds()
	if err != nil {
		_ = s.recordStepOf(ctx, runULID, chapter.ChapterULID, stepIndex, stepDef,
			domain.WorkflowStepStatusFailed, pkg.TruncateText(err.Error(), 500), "", latencyMs)
		return result, err
	}
	if err := s.recordStepOf(ctx, runULID, chapter.ChapterULID, stepIndex, stepDef,
		domain.WorkflowStepStatusSuccess, outputText, metricJSON, latencyMs); err != nil {
		return result, err
	}
	return result, nil
}

// stepDraft 起草/修订：取提示词 → 装配上下文 → 模型生成 → 写候选正文。
func (s *WorkflowService) stepDraft(ctx context.Context, runULID string, chapter *domain.ChapterDO,
	stepDef domain.WorkflowStepDefDTO, params domain.WorkflowRunParamsDTO) (workflowStepResult, string, string, error) {
	runDO, err := s.workflowRepo.GetRun(ctx, runULID)
	if err != nil || runDO == nil {
		return workflowStepResult{}, "", "", pkg.NewBizError(domain.RespCodeStorageError, "运行实例读取失败")
	}
	promptCode := firstNonEmpty(stepDef.PromptCode, params.DraftPromptCode, "chapter")
	sourceText := runDO.CandidateText
	if strings.TrimSpace(sourceText) == "" {
		sourceText = chapter.ContentText
	}
	promptText, err := s.promptSvc.PromptTextOf(ctx, chapter.ProjectULID, promptCode)
	if err != nil {
		return workflowStepResult{}, "", "", err
	}
	contextText, err := s.chapterContextText(ctx, chapter, params)
	if err != nil {
		return workflowStepResult{}, "", "", err
	}
	userInput := fmt.Sprintf("本章目标 %d 字左右。", params.TargetWordCount)
	if params.ExtraRequirementText != "" {
		userInput += "\n额外要求：" + params.ExtraRequirementText
	}
	if runDO.FeedbackText != "" {
		userInput += "\n审核退回意见（必须解决）：" + runDO.FeedbackText
	}
	rendered := ai.RenderPrompt(promptText, map[string]string{
		"user_input":      userInput,
		"chapter_content": condenseChapterText(sourceText),
		"context":         contextText,
		"prompt_code":     promptCode,
	})
	spec, err := s.providerSvc.ChatSpecOf(ctx, runDO.UserULID, params.ModelULID)
	if err != nil {
		return workflowStepResult{}, "", "", err
	}
	if spec.MaxTokens < workflowDraftMaxTokens {
		spec.MaxTokens = workflowDraftMaxTokens
	}
	stepCtx, cancel := context.WithTimeout(ctx, workflowStepTimeout)
	defer cancel()
	replyText, err := s.generateText(stepCtx, spec, workflowBodyOnlySystemPrompt, rendered)
	if err != nil {
		return workflowStepResult{}, "", "", err
	}
	if strings.TrimSpace(replyText) == "" {
		return workflowStepResult{}, "", "", pkg.NewBizError(domain.RespCodeAIProviderError, "模型没有返回正文")
	}
	if err := s.workflowRepo.UpdateRun(ctx, runULID, map[string]any{
		"candidate_text": replyText,
	}); err != nil {
		return workflowStepResult{}, "", "", err
	}
	metricJSON := fmt.Sprintf(`{"word_count":%d}`, pkg.CountWords(replyText))
	return workflowStepResult{OutputText: replyText}, pkg.TruncateText(replyText, 2000), metricJSON, nil
}

// stepCheck 量化体检：对候选正文（无候选时用库内正文）跑确定性门禁。
func (s *WorkflowService) stepCheck(ctx context.Context, runULID string, chapter *domain.ChapterDO) (workflowStepResult, string, string, error) {
	text := chapter.ContentText
	if runDO, err := s.workflowRepo.GetRun(ctx, runULID); err == nil && runDO != nil && strings.TrimSpace(runDO.CandidateText) != "" {
		text = runDO.CandidateText
	}
	report := check.Analyze(text)
	metricJSON := fmt.Sprintf(`{"score_value":%d,"verdict_code":"%s","word_count":%d,"issue_count":%d}`,
		report.ScoreValue, report.VerdictCode, report.WordCount, len(report.IssueList))
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("综合分 %d，结论 %s，字数 %d。\n", report.ScoreValue, verdictLabelOf(report.VerdictCode), report.WordCount))
	for _, issue := range report.IssueList {
		builder.WriteString("- [" + issue.LevelCode + "] " + issue.TitleText + "：" + issue.DetailText + "\n")
	}
	return workflowStepResult{CheckScore: report.ScoreValue, VerdictCode: report.VerdictCode,
		OutputText: builder.String()}, builder.String(), metricJSON, nil
}

// stepReview AI 复审：输出严格 JSON，解析失败时降级为纯文本结论。
func (s *WorkflowService) stepReview(ctx context.Context, runULID string, chapter *domain.ChapterDO,
	stepDef domain.WorkflowStepDefDTO, params domain.WorkflowRunParamsDTO) (workflowStepResult, string, string, error) {
	runDO, err := s.workflowRepo.GetRun(ctx, runULID)
	if err != nil || runDO == nil {
		return workflowStepResult{}, "", "", pkg.NewBizError(domain.RespCodeStorageError, "运行实例读取失败")
	}
	text := runDO.CandidateText
	if strings.TrimSpace(text) == "" {
		text = chapter.ContentText
	}
	promptCode := firstNonEmpty(stepDef.PromptCode, params.ReviewPromptCode, "review")
	promptText, err := s.promptSvc.PromptTextOf(ctx, chapter.ProjectULID, promptCode)
	if err != nil {
		return workflowStepResult{}, "", "", err
	}
	contextText, err := s.chapterContextText(ctx, chapter, params)
	if err != nil {
		return workflowStepResult{}, "", "", err
	}
	rendered := ai.RenderPrompt(promptText, map[string]string{
		"chapter_content": condenseChapterText(text),
		"context":         contextText,
		"user_input":      "",
		"prompt_code":     promptCode,
	})
	spec, err := s.providerSvc.ChatSpecOf(ctx, runDO.UserULID, params.ModelULID)
	if err != nil {
		return workflowStepResult{}, "", "", err
	}
	if spec.MaxTokens < workflowReviewMaxTokens {
		spec.MaxTokens = workflowReviewMaxTokens
	}
	stepCtx, cancel := context.WithTimeout(ctx, workflowStepTimeout)
	defer cancel()
	replyText, err := s.generateText(stepCtx, spec, workflowGenericSystemPrompt, rendered)
	if err != nil {
		return workflowStepResult{}, "", "", err
	}
	verdictCode, scoreValue, summaryText := parseReviewVerdict(replyText)
	metricJSON := fmt.Sprintf(`{"verdict_code":"%s","score_value":%d}`, verdictCode, scoreValue)
	return workflowStepResult{VerdictCode: verdictCode, CheckScore: scoreValue,
		OutputText: summaryText}, pkg.TruncateText(summaryText, 4000), metricJSON, nil
}


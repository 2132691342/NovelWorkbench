// 本文件实现工作流的运行控制：人工审核门禁（通过/退回）、暂停/继续/取消/重试与中断恢复。

package service

import (
	"context"
	"strings"
	"time"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
)


// Approve 人工审核通过：写入章节正文 → 继续下一章。
func (s *WorkflowService) Approve(ctx context.Context, userULID string, req *domain.WorkflowRunApproveREQ) (domain.WorkflowRunVO, error) {
	runDO, err := s.mustOwnRun(ctx, userULID, req.RunULID)
	if err != nil {
		return domain.WorkflowRunVO{}, err
	}
	if runDO.RunStatusCode != domain.WorkflowRunStatusWaitingReview {
		return domain.WorkflowRunVO{}, pkg.NewBizError(domain.RespCodeBizFailed, "该运行当前不在待审核状态")
	}
	if err := s.applyCandidate(ctx, runDO); err != nil {
		return domain.WorkflowRunVO{}, err
	}
	noteText := strings.TrimSpace(req.NoteText)
	if noteText == "" {
		noteText = "审核通过"
	}
	if err := s.recordStep(ctx, runDO, domain.WorkflowStepTypeHumanReview, "人工审核",
		domain.WorkflowStepStatusSuccess, noteText, "", 0); err != nil {
		return domain.WorkflowRunVO{}, err
	}
	// 门禁本身算已完成：步骤下标必须前移，否则恢复执行会重新卡在同一个门禁上；
	// 状态必须回到 queued，否则执行器看到 waiting_review 会直接退出。
	if err := s.workflowRepo.UpdateRun(ctx, runDO.RunULID, map[string]any{
		"run_status_code":    domain.WorkflowRunStatusQueued,
		"current_step_index": runDO.CurrentStepIndex + 1,
		"candidate_text":     "",
	}); err != nil {
		return domain.WorkflowRunVO{}, err
	}
	s.runner.resume(runDO.RunULID)
	return s.runVO(ctx, runDO.RunULID, "")
}

// Reject 人工审核退回：带意见重跑当前章。
func (s *WorkflowService) Reject(ctx context.Context, userULID string, req *domain.WorkflowRunRejectREQ) (domain.WorkflowRunVO, error) {
	runDO, err := s.mustOwnRun(ctx, userULID, req.RunULID)
	if err != nil {
		return domain.WorkflowRunVO{}, err
	}
	if runDO.RunStatusCode != domain.WorkflowRunStatusWaitingReview {
		return domain.WorkflowRunVO{}, pkg.NewBizError(domain.RespCodeBizFailed, "该运行当前不在待审核状态")
	}
	feedbackText := strings.TrimSpace(req.FeedbackText)
	if err := s.recordStep(ctx, runDO, domain.WorkflowStepTypeHumanReview, "人工审核",
		domain.WorkflowStepStatusRejected, feedbackText, "", 0); err != nil {
		return domain.WorkflowRunVO{}, err
	}
	// 退回即回到本章第一步重跑，并把意见注入起草提示词
	if err := s.workflowRepo.UpdateRun(ctx, runDO.RunULID, map[string]any{
		"run_status_code":    domain.WorkflowRunStatusQueued,
		"current_step_index": 0,
		"feedback_text":      feedbackText,
		"candidate_text":     "",
		"error_message":      "",
	}); err != nil {
		return domain.WorkflowRunVO{}, err
	}
	s.runner.start(runDO.RunULID)
	return s.runVO(ctx, runDO.RunULID, "")
}

// Pause 请求暂停（执行器在步骤边界响应）。
func (s *WorkflowService) Pause(ctx context.Context, userULID string, req *domain.WorkflowRunIDREQ) (domain.WorkflowRunVO, error) {
	runDO, err := s.mustOwnRun(ctx, userULID, req.RunULID)
	if err != nil {
		return domain.WorkflowRunVO{}, err
	}
	if !isWorkflowRunActive(runDO.RunStatusCode) {
		return domain.WorkflowRunVO{}, pkg.NewBizError(domain.RespCodeBizFailed, "该运行已结束，无法暂停")
	}
	if err := s.workflowRepo.UpdateRun(ctx, runDO.RunULID, map[string]any{
		"pause_requested_flag": true,
	}); err != nil {
		return domain.WorkflowRunVO{}, err
	}
	return s.runVO(ctx, runDO.RunULID, "")
}

// Resume 继续已暂停的运行。
func (s *WorkflowService) Resume(ctx context.Context, userULID string, req *domain.WorkflowRunIDREQ) (domain.WorkflowRunVO, error) {
	runDO, err := s.mustOwnRun(ctx, userULID, req.RunULID)
	if err != nil {
		return domain.WorkflowRunVO{}, err
	}
	if runDO.RunStatusCode != domain.WorkflowRunStatusPaused {
		return domain.WorkflowRunVO{}, pkg.NewBizError(domain.RespCodeBizFailed, "只有已暂停的运行可以继续")
	}
	if err := s.workflowRepo.UpdateRun(ctx, runDO.RunULID, map[string]any{
		"run_status_code":      domain.WorkflowRunStatusQueued,
		"pause_requested_flag": false,
	}); err != nil {
		return domain.WorkflowRunVO{}, err
	}
	s.runner.start(runDO.RunULID)
	return s.runVO(ctx, runDO.RunULID, "")
}

// Cancel 取消运行（不可恢复）。
func (s *WorkflowService) Cancel(ctx context.Context, userULID string, req *domain.WorkflowRunIDREQ) (domain.WorkflowRunVO, error) {
	runDO, err := s.mustOwnRun(ctx, userULID, req.RunULID)
	if err != nil {
		return domain.WorkflowRunVO{}, err
	}
	if runDO.RunStatusCode == domain.WorkflowRunStatusSucceeded || runDO.RunStatusCode == domain.WorkflowRunStatusCanceled {
		return s.runVO(ctx, runDO.RunULID, "")
	}
	if err := s.workflowRepo.UpdateRun(ctx, runDO.RunULID, map[string]any{
		"run_status_code":       domain.WorkflowRunStatusCanceled,
		"cancel_requested_flag": true,
		"finished_at":           time.Now(),
	}); err != nil {
		return domain.WorkflowRunVO{}, err
	}
	return s.runVO(ctx, runDO.RunULID, "")
}

// Retry 重试失败的运行（从当前章第一步重跑）。
func (s *WorkflowService) Retry(ctx context.Context, userULID string, req *domain.WorkflowRunIDREQ) (domain.WorkflowRunVO, error) {
	runDO, err := s.mustOwnRun(ctx, userULID, req.RunULID)
	if err != nil {
		return domain.WorkflowRunVO{}, err
	}
	if runDO.RunStatusCode != domain.WorkflowRunStatusFailed {
		return domain.WorkflowRunVO{}, pkg.NewBizError(domain.RespCodeBizFailed, "只有失败的运行可以重试")
	}
	if err := s.workflowRepo.UpdateRun(ctx, runDO.RunULID, map[string]any{
		"run_status_code":    domain.WorkflowRunStatusQueued,
		"current_step_index": 0,
		"candidate_text":     "",
		"error_message":      "",
		"cancel_requested_flag": false,
		"pause_requested_flag":  false,
	}); err != nil {
		return domain.WorkflowRunVO{}, err
	}
	s.runner.start(runDO.RunULID)
	return s.runVO(ctx, runDO.RunULID, "")
}


// RecoverInterrupted 启动时恢复中断的运行：running 状态退回 queued 并重新调度。
func (s *WorkflowService) RecoverInterrupted(ctx context.Context) {
	s.runner.RecoverInterrupted(ctx)
}

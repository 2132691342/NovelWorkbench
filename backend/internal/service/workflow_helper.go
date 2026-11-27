// 本文件实现工作流的视图组装与通用小工具（参数解码、状态判定、章节定位）。

package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
)

// recordStep 追加一条步骤记录（失败只返回错误，不静默吞掉）。
// 步骤下标显式传入：跳过的步骤不推进 current_step_index，用运行上的下标会写错行。
func (s *WorkflowService) recordStep(ctx context.Context, runDO *domain.WorkflowRunDO, stepIndex int,
	stepTypeCode, stepName, statusCode, outputText, metricJSON string, latencyMs int64) error {
	now := time.Now()
	return s.workflowRepo.CreateStep(ctx, &domain.WorkflowStepDO{
		StepULID:       pkg.NewULID(domain.IDPrefixWorkflowStep),
		RunULID:        runDO.RunULID,
		ChapterULID:    currentChapterULIDOfRun(runDO),
		StepIndex:      stepIndex,
		StepTypeCode:   stepTypeCode,
		StepName:       stepName,
		StepStatusCode: statusCode,
		OutputText:     pkg.TruncateText(outputText, 20000),
		MetricJSON:     metricJSON,
		AttemptCount:   1,
		LatencyMs:      latencyMs,
		StartedAt:      now,
		FinishedAt:     now,
	})
}

// 门禁收口动作：命中 running 行就原地更新，找不到才回退插入。
const (
	gateStepActionUpdate = "update" // 原地更新为终态
	gateStepActionInsert = "insert" // 历史数据/异常路径的回退插入
)

// gateStepCloseAction 纯函数决定门禁收口方式（便于单测覆盖两条分支）。
func gateStepCloseAction(gateStep *domain.WorkflowStepDO) string {
	if gateStep == nil {
		return gateStepActionInsert
	}
	return gateStepActionUpdate
}

// closeGateStep 把门禁那行原地更新为终态，而不是再插一行。
// 找不到 running 门禁行（历史数据/异常路径）时回退插入，保证审核不会因此报错。
func (s *WorkflowService) closeGateStep(ctx context.Context, runDO *domain.WorkflowRunDO, stepIndex int,
	stepTypeCode, stepName, statusCode, outputText string) error {
	chapterULID := currentChapterULIDOfRun(runDO)
	gateStep, err := s.workflowRepo.FindRunningGateStep(ctx, runDO.RunULID, chapterULID, stepIndex)
	if err != nil {
		return err
	}
	if gateStepCloseAction(gateStep) == gateStepActionInsert {
		return s.recordStep(ctx, runDO, stepIndex, stepTypeCode, stepName, statusCode, outputText, "", 0)
	}
	now := time.Now()
	return s.workflowRepo.UpdateStep(ctx, gateStep.StepULID, map[string]any{
		"step_status_code": statusCode,
		"output_text":      pkg.TruncateText(outputText, 20000),
		"latency_ms":       now.Sub(gateStep.StartedAt).Milliseconds(),
		"finished_at":      now,
	})
}

// isWorkflowRunActive 判断运行是否仍在进行中（可暂停）。
func isWorkflowRunActive(statusCode string) bool {
	return statusCode == domain.WorkflowRunStatusQueued || statusCode == domain.WorkflowRunStatusRunning ||
		statusCode == domain.WorkflowRunStatusWaitingReview
}

// currentChapterULIDOfRun 取运行当前目标章节 ULID（越界返回空串）。
func currentChapterULIDOfRun(runDO *domain.WorkflowRunDO) string {
	chapterULIDList := decodeStringList(runDO.ChapterULIDListJSON)
	if runDO.CurrentIndex < 0 || runDO.CurrentIndex >= len(chapterULIDList) {
		return ""
	}
	return chapterULIDList[runDO.CurrentIndex]
}

// projectNameOf 安全取项目名（项目已删时返回空串）。
func projectNameOf(project *domain.ProjectDO) string {
	if project == nil {
		return ""
	}
	return project.ProjectName
}

// toWorkflowTemplateVO 组装模板视图。
func toWorkflowTemplateVO(do *domain.WorkflowTemplateDO) domain.WorkflowTemplateVO {
	return domain.WorkflowTemplateVO{
		TemplateULID:    do.TemplateULID,
		TemplateCode:    do.TemplateCode,
		TemplateName:    do.TemplateName,
		DescriptionText: do.DescriptionText,
		StepList:        unmarshalWorkflowSteps(do.StepSchemaJSON),
		BuiltinFlag:     do.BuiltinFlag,
		EnabledFlag:     do.EnabledFlag,
		SortOrder:       do.SortOrder,
	}
}

// toWorkflowRunVO 组装运行视图（章节名由调用方回填）。
func toWorkflowRunVO(do *domain.WorkflowRunDO, projectName, chapterName string) domain.WorkflowRunVO {
	params := unmarshalWorkflowParams(do.ParamsJSON)
	return domain.WorkflowRunVO{
		RunULID:              do.RunULID,
		ProjectULID:          do.ProjectULID,
		ProjectName:          projectName,
		TemplateCode:         do.TemplateCode,
		TemplateName:         do.TemplateName,
		RunStatusCode:        do.RunStatusCode,
		CurrentIndex:         do.CurrentIndex,
		TotalChapterCount:    do.TotalChapterCount,
		FinishedChapterCount: do.FinishedChapterCount,
		CurrentChapterULID:   currentChapterULIDOfRun(do),
		CurrentChapterName:   chapterName,
		TraceID:              do.TraceID,
		ErrorMessage:         do.ErrorMessage,
		FeedbackText:         do.FeedbackText,
		TargetWordCount:      params.TargetWordCount,
		ReviewGateFlag:       params.ReviewGateFlag != nil && *params.ReviewGateFlag,
		CreatedAt:            pkg.FormatTime(do.CreatedAt),
		StartedAt:            pkg.FormatTime(do.StartedAt),
		FinishedAt:           pkg.FormatTime(do.FinishedAt),
	}
}

// toWorkflowStepVO 组装步骤视图。
func toWorkflowStepVO(do *domain.WorkflowStepDO, chapterNameMap map[string]string) domain.WorkflowStepVO {
	return domain.WorkflowStepVO{
		StepULID:       do.StepULID,
		ChapterULID:    do.ChapterULID,
		ChapterName:    chapterNameMap[do.ChapterULID],
		StepIndex:      do.StepIndex,
		StepTypeCode:   do.StepTypeCode,
		StepName:       do.StepName,
		StepStatusCode: do.StepStatusCode,
		PromptCode:     do.PromptCode,
		OutputText:     do.OutputText,
		MetricJSON:     do.MetricJSON,
		AttemptCount:   do.AttemptCount,
		LatencyMs:      do.LatencyMs,
		ErrorMessage:   do.ErrorMessage,
		StartedAt:      pkg.FormatTime(do.StartedAt),
		FinishedAt:     pkg.FormatTime(do.FinishedAt),
	}
}

// unmarshalWorkflowParams 反序列化运行参数（损坏数据按默认参数处理）。
func unmarshalWorkflowParams(paramsJSON string) domain.WorkflowRunParamsDTO {
	defaultParams := domain.WorkflowRunParamsDTO{
		TargetWordCount: workflowDefaultTargetWords,
		GateScore:       workflowDefaultGateScore,
	}
	if paramsJSON == "" {
		return defaultParams
	}
	params := domain.WorkflowRunParamsDTO{}
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return defaultParams
	}
	return params
}

// jsonStringValue 从 JSON 对象取字符串字段（解析失败返回空串）。
func jsonStringValue(jsonText, key string) string {
	if jsonText == "" {
		return ""
	}
	valueMap := make(map[string]any, 8)
	if err := json.Unmarshal([]byte(jsonText), &valueMap); err != nil {
		return ""
	}
	value, _ := valueMap[key].(string)
	return value
}

// jsonIntValue 从 JSON 对象取数值字段（解析失败返回 0）。
func jsonIntValue(jsonText, key string) int64 {
	if jsonText == "" {
		return 0
	}
	valueMap := make(map[string]any, 8)
	if err := json.Unmarshal([]byte(jsonText), &valueMap); err != nil {
		return 0
	}
	switch typed := valueMap[key].(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	default:
		return 0
	}
}

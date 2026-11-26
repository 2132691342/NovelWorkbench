// 本文件定义内置工作流模板与步骤编排：作者选模板、填参数，AI 按步骤跑完，人只在审核点点头。

package service

import (
	"encoding/json"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// workflowTemplateSeed 内置模板定义（步骤定义以代码为准，启动期同步）。
type workflowTemplateSeed struct {
	TemplateCode    string
	TemplateName    string
	DescriptionText string
	SortOrder       int
	StepList        []domain.WorkflowStepDefDTO
}

// workflowTemplateSeedList 内置模板清单：三条覆盖作者 90% 的重复劳动。
var workflowTemplateSeedList = []workflowTemplateSeed{
	{
		TemplateCode:    domain.WorkflowTemplateChapterDraft,
		TemplateName:    "逐章成稿",
		DescriptionText: "按章节要求写正文 → 体检 → 不达标自动修订 → AI 复审 → 人工审核 → 定稿入库",
		SortOrder:       10,
		StepList: []domain.WorkflowStepDefDTO{
			{StepTypeCode: domain.WorkflowStepTypeDraft, StepName: "写正文", PromptCode: "chapter", MaxAttempts: 2},
			{StepTypeCode: domain.WorkflowStepTypeCheck, StepName: "量化体检"},
			{StepTypeCode: domain.WorkflowStepTypeRevise, StepName: "按问题修订", PromptCode: "polish", MaxAttempts: 2, SkipWhenPass: true},
			{StepTypeCode: domain.WorkflowStepTypeReview, StepName: "AI 复审", PromptCode: "review", MaxAttempts: 1},
			{StepTypeCode: domain.WorkflowStepTypeHumanReview, StepName: "人工审核", HumanGateFlag: true},
			{StepTypeCode: domain.WorkflowStepTypeApply, StepName: "定稿入库"},
			{StepTypeCode: domain.WorkflowStepTypeSummary, StepName: "生成摘要", PromptCode: "summary", MaxAttempts: 1},
		},
	},
	{
		TemplateCode:    domain.WorkflowTemplateChapterPolish,
		TemplateName:    "存量打磨",
		DescriptionText: "对已有章节润色 → 体检 → 不达标再修订 → 人工审核 → 定稿入库",
		SortOrder:       20,
		StepList: []domain.WorkflowStepDefDTO{
			{StepTypeCode: domain.WorkflowStepTypeDraft, StepName: "润色改写", PromptCode: "polish", MaxAttempts: 2},
			{StepTypeCode: domain.WorkflowStepTypeCheck, StepName: "量化体检"},
			{StepTypeCode: domain.WorkflowStepTypeRevise, StepName: "按问题修订", PromptCode: "polish", MaxAttempts: 1, SkipWhenPass: true},
			{StepTypeCode: domain.WorkflowStepTypeHumanReview, StepName: "人工审核", HumanGateFlag: true},
			{StepTypeCode: domain.WorkflowStepTypeApply, StepName: "定稿入库"},
		},
	},
	{
		TemplateCode:    domain.WorkflowTemplateChapterDeai,
		TemplateName:    "批量去 AI 味",
		DescriptionText: "对已有章节逐章去 AI 味 → 体检 → 人工抽检 → 定稿入库，适合入库前的批量清洗",
		SortOrder:       30,
		StepList: []domain.WorkflowStepDefDTO{
			{StepTypeCode: domain.WorkflowStepTypeDraft, StepName: "去 AI 味改写", PromptCode: "deai", MaxAttempts: 2},
			{StepTypeCode: domain.WorkflowStepTypeCheck, StepName: "量化体检"},
			{StepTypeCode: domain.WorkflowStepTypeHumanReview, StepName: "人工抽检", HumanGateFlag: true},
			{StepTypeCode: domain.WorkflowStepTypeApply, StepName: "定稿入库"},
		},
	},
}

// 工作流执行默认参数。
const (
	workflowDefaultTargetWords = 2500 // 每章目标字数
	workflowDefaultGateScore   = 80   // 体检通过分
	workflowDefaultMaxAttempts = 2    // AI 步骤默认尝试次数
	workflowChapterMaxRunes    = 6000 // 章节正文进入提示词的上限（超出保留首尾）
)

// marshalWorkflowSteps 序列化步骤定义（空列表存 "[]"）。
func marshalWorkflowSteps(stepList []domain.WorkflowStepDefDTO) string {
	if len(stepList) == 0 {
		return "[]"
	}
	data, err := json.Marshal(stepList)
	if err != nil {
		return "[]"
	}
	return string(data)
}

// unmarshalWorkflowSteps 反序列化步骤定义（损坏数据按空列表处理，运行时报「模板无步骤」）。
func unmarshalWorkflowSteps(stepSchemaJSON string) []domain.WorkflowStepDefDTO {
	stepList := make([]domain.WorkflowStepDefDTO, 0, 8)
	if stepSchemaJSON == "" {
		return stepList
	}
	if err := json.Unmarshal([]byte(stepSchemaJSON), &stepList); err != nil {
		return []domain.WorkflowStepDefDTO{}
	}
	return stepList
}

// workflowSeedByCode 按编码取内置模板定义（未知编码返回 nil）。
func workflowSeedByCode(templateCode string) *workflowTemplateSeed {
	for i := range workflowTemplateSeedList {
		if workflowTemplateSeedList[i].TemplateCode == templateCode {
			return &workflowTemplateSeedList[i]
		}
	}
	return nil
}

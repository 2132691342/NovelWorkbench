// 本文件实现「一键开跑」：作者只选项目，参数取自项目上的自动化默认值，
// 模板按章节现状自动判断（有空白章 → 逐章成稿；只有成稿 → 存量打磨）。
// 这是产品主路径：AI 干活、人只在审核点点头，因此这里不做阻塞式拦截，只把缺口列出来。

package service

import (
	"context"
	"strings"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
)

// QuickStart 用项目默认参数直接开跑一次自动化任务。
func (s *WorkflowService) QuickStart(ctx context.Context, userULID string,
	req *domain.WorkflowQuickStartREQ) (domain.WorkflowQuickStartVO, error) {
	project, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID)
	if err != nil {
		return domain.WorkflowQuickStartVO{}, err
	}
	templateCode, err := s.resolveQuickTemplate(ctx, project.ProjectULID, req)
	if err != nil {
		return domain.WorkflowQuickStartVO{}, err
	}
	params := s.quickStartParamsOf(project, req)
	runVO, err := s.CreateRun(ctx, userULID, &domain.WorkflowRunCreateREQ{
		ProjectULID:          project.ProjectULID,
		TemplateCode:         templateCode,
		WorkflowRunParamsDTO: params,
		AutoStartFlag:        true,
	})
	if err != nil {
		return domain.WorkflowQuickStartVO{}, err
	}
	vo := domain.WorkflowQuickStartVO{
		RunVO:        runVO,
		TemplateCode: templateCode,
		ChapterCount: runVO.TotalChapterCount,
		ReadyFlag:    true,
	}
	// 就绪度只做提示：缺素材仍允许开跑，但要让作者知道产出可能偏离设定
	if readiness, err := s.Readiness(ctx, userULID, project.ProjectULID); err == nil {
		vo.ReadyFlag = readiness.ReadyFlag
		vo.BlockingCount = readiness.BlockingCount
		for i := range readiness.ItemList {
			item := &readiness.ItemList[i]
			if item.ReadyFlag || !item.BlockingFlag {
				continue
			}
			vo.BlockingHintList = append(vo.BlockingHintList, item.ItemName+"："+item.HintText)
			if len(vo.BlockingHintList) >= 3 {
				break
			}
		}
	}
	return vo, nil
}

// quickStartParamsOf 用项目默认值组装运行参数（请求里显式给了的字段覆盖默认）。
func (s *WorkflowService) quickStartParamsOf(project *domain.ProjectDO,
	req *domain.WorkflowQuickStartREQ) domain.WorkflowRunParamsDTO {
	params := domain.WorkflowRunParamsDTO{
		ChapterULIDList:      req.ChapterULIDList,
		NewChapterCount:      req.NewChapterCount,
		TargetWordCount:      firstPositive(req.TargetWordCount, project.AutoTargetWordCount),
		GateScore:            firstPositive(req.GateScore, project.AutoGateScore),
		ExtraRequirementText: mergeRequirementText(project.AutoRequirementText, req.ExtraRequirementText),
		ModelULID:            req.ModelULID,
	}
	reviewGateFlag := project.AutoReviewGateFlag
	if req.ReviewGateFlag != nil {
		reviewGateFlag = *req.ReviewGateFlag
	}
	params.ReviewGateFlag = &reviewGateFlag
	return params
}

// resolveQuickTemplate 决定这次跑哪个模板：显式指定优先，否则按章节现状判断。
// 有空白章就该写正文；全是成稿才轮到打磨——这个判断让作者不用懂模板差异。
func (s *WorkflowService) resolveQuickTemplate(ctx context.Context, projectULID string,
	req *domain.WorkflowQuickStartREQ) (string, error) {
	if strings.TrimSpace(req.TemplateCode) != "" {
		return req.TemplateCode, nil
	}
	chapters, err := s.chapterRepo.ListByProject(ctx, projectULID)
	if err != nil {
		return "", err
	}
	blankCount, writtenCount := 0, 0
	for i := range chapters {
		if strings.TrimSpace(chapters[i].ContentText) == "" {
			blankCount++
		} else {
			writtenCount++
		}
	}
	if len(req.ChapterULIDList) > 0 {
		// 手选了章节：以选中章节的现状判断，选中的全是成稿就走打磨
		blankCount, writtenCount = s.countChaptersByState(ctx, req.ChapterULIDList)
	}
	switch {
	case blankCount > 0 || req.NewChapterCount > 0:
		return domain.WorkflowTemplateChapterDraft, nil
	case writtenCount > 0:
		return domain.WorkflowTemplateChapterPolish, nil
	default:
		return "", pkg.NewBizError(domain.RespCodeBizFailed,
			"项目内还没有章节：请到创作规划加几章，或指定新建章节数")
	}
}

// countChaptersByState 统计指定章节里的空白章与成稿数（供模板选择）。
func (s *WorkflowService) countChaptersByState(ctx context.Context, chapterULIDList []string) (int, int) {
	chapters, err := s.chapterRepo.ListByULIDList(ctx, chapterULIDList)
	if err != nil {
		return 0, len(chapterULIDList)
	}
	blankCount, writtenCount := 0, 0
	for i := range chapters {
		if strings.TrimSpace(chapters[i].ContentText) == "" {
			blankCount++
		} else {
			writtenCount++
		}
	}
	return blankCount, writtenCount
}

// firstPositive 返回第一个大于零的值（0 表示「用默认」）。
func firstPositive(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

// mergeRequirementText 合并项目整体要求与本次追加要求（项目要求在前，作为长期约束）。
func mergeRequirementText(projectRequirement, extraRequirement string) string {
	projectText := strings.TrimSpace(projectRequirement)
	extraText := strings.TrimSpace(extraRequirement)
	switch {
	case projectText == "":
		return extraText
	case extraText == "":
		return projectText
	default:
		return projectText + "\n本次额外要求：" + extraText
	}
}

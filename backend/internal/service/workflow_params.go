// 本文件实现工作流的运行准备：参数收敛、目标章节解析与空白章节创建。

package service

import (
	"context"
	"strings"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
)

// normalizeWorkflowParams 收敛运行参数到合法区间（模型输出不可信，落库前统一钳制）。
func normalizeWorkflowParams(params *domain.WorkflowRunParamsDTO) error {
	if params.TargetWordCount <= 0 {
		params.TargetWordCount = workflowDefaultTargetWords
	}
	if params.TargetWordCount < 300 || params.TargetWordCount > 20000 {
		return pkg.NewBizError(domain.RespCodeInvalidParam, "每章目标字数须在 300-20000 之间")
	}
	if params.GateScore <= 0 {
		params.GateScore = workflowDefaultGateScore
	}
	if params.GateScore < 60 || params.GateScore > 100 {
		return pkg.NewBizError(domain.RespCodeInvalidParam, "体检通过分须在 60-100 之间")
	}
	if params.NewChapterCount < 0 || params.NewChapterCount > 50 {
		return pkg.NewBizError(domain.RespCodeInvalidParam, "新建章节数须在 0-50 之间")
	}
	if params.ReviewGateFlag == nil {
		enabledFlag := true // 默认开启人工审核：AI 出稿、人来点头
		params.ReviewGateFlag = &enabledFlag
	}
	params.ExtraRequirementText = pkg.TruncateText(strings.TrimSpace(params.ExtraRequirementText), 1000)
	return nil
}

// resolveChapterList 解析目标章节，规则按模板用途区分：
// 起草类模板取空白章（没有则按需新建）；打磨/去 AI 味类模板取已有正文的章节（没有则说明原因）。
// 这条区分是必要的：让「去 AI 味」默认指向空白章会得到空结果，让「逐章成稿」默认指向已有正文会覆盖作者已写好的内容。
func (s *WorkflowService) resolveChapterList(ctx context.Context, project *domain.ProjectDO,
	templateCode string, params *domain.WorkflowRunParamsDTO) ([]string, error) {
	chapterDOS, err := s.chapterRepo.ListByProject(ctx, project.ProjectULID)
	if err != nil {
		return nil, err
	}
	byULID := make(map[string]*domain.ChapterDO, len(chapterDOS))
	blankULIDList := make([]string, 0, len(chapterDOS))
	writtenULIDList := make([]string, 0, len(chapterDOS))
	for i := range chapterDOS {
		byULID[chapterDOS[i].ChapterULID] = &chapterDOS[i]
		if strings.TrimSpace(chapterDOS[i].ContentText) == "" {
			blankULIDList = append(blankULIDList, chapterDOS[i].ChapterULID)
		} else {
			writtenULIDList = append(writtenULIDList, chapterDOS[i].ChapterULID)
		}
	}
	if len(params.ChapterULIDList) > 0 {
		resolved := make([]string, 0, len(params.ChapterULIDList))
		for _, chapterULID := range params.ChapterULIDList {
			if _, ok := byULID[chapterULID]; !ok {
				return nil, pkg.NewBizError(domain.RespCodeInvalidParam, "章节不属于该项目: "+chapterULID)
			}
			resolved = append(resolved, chapterULID)
		}
		return resolved, nil
	}
	if templateNeedsWrittenChapter(templateCode) {
		if len(writtenULIDList) > 0 {
			return writtenULIDList, nil
		}
		return nil, pkg.NewBizError(domain.RespCodeBizFailed,
			"项目内还没有写好的章节：本模板处理已有正文，请先写章节或改用「逐章成稿」")
	}
	if len(blankULIDList) > 0 {
		return blankULIDList, nil
	}
	if params.NewChapterCount <= 0 {
		return nil, pkg.NewBizError(domain.RespCodeBizFailed,
			"项目内没有空白章节：请选择已有章节，或指定新建章节数")
	}
	return s.createBlankChapters(ctx, project.ProjectULID, params.NewChapterCount)
}

// templateNeedsWrittenChapter 判断模板是否处理「已有正文」的章节（纯函数）。
func templateNeedsWrittenChapter(templateCode string) bool {
	switch templateCode {
	case domain.WorkflowTemplateChapterPolish, domain.WorkflowTemplateChapterDeai:
		return true
	default:
		return false
	}
}

// createBlankChapters 新建若干空白章节（挂到最后一卷；无卷时自动建一卷）。
func (s *WorkflowService) createBlankChapters(ctx context.Context, projectULID string, count int) ([]string, error) {
	volumeDOS, err := s.volumeRepo.ListByProject(ctx, projectULID)
	if err != nil {
		return nil, err
	}
	var volumeULID string
	if len(volumeDOS) > 0 {
		volumeULID = volumeDOS[len(volumeDOS)-1].VolumeULID
	} else {
		volume := &domain.VolumeDO{
			VolumeULID:  pkg.NewULID(domain.IDPrefixVolume),
			ProjectULID: projectULID,
			VolumeName:  "第一卷",
			VolumeOrder: 1,
		}
		if err := s.volumeRepo.Create(ctx, volume); err != nil {
			return nil, err
		}
		volumeULID = volume.VolumeULID
	}
	maxOrder, err := s.chapterRepo.MaxOrder(ctx, projectULID)
	if err != nil {
		return nil, err
	}
	created := make([]string, 0, count)
	for i := 0; i < count; i++ {
		chapter := &domain.ChapterDO{
			ChapterULID:   pkg.NewULID(domain.IDPrefixChapter),
			ProjectULID:   projectULID,
			VolumeULID:    volumeULID,
			ChapterName:   "第 " + intToText(int64(maxOrder+i+1)) + " 章",
			ChapterOrder:  maxOrder + i + 1,
			ChapterStatus: "draft",
		}
		if err := s.chapterRepo.Create(ctx, chapter); err != nil {
			return nil, err
		}
		created = append(created, chapter.ChapterULID)
	}
	return created, nil
}

// ListRuns 分页查询运行实例（回填项目名与当前章节名）。
func (s *WorkflowService) ListRuns(ctx context.Context, userULID string, req *domain.WorkflowRunListREQ) (domain.PageDTO[domain.WorkflowRunVO], error) {
	req.Normalize()
	if req.ProjectULID != "" {
		if _, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID); err != nil {
			return domain.PageDTO[domain.WorkflowRunVO]{}, err
		}
	}
	dos, total, err := s.workflowRepo.ListRuns(ctx, req)
	if err != nil {
		return domain.PageDTO[domain.WorkflowRunVO]{}, err
	}
	projectNameMap, err := s.projectNameMap(ctx, dos)
	if err != nil {
		return domain.PageDTO[domain.WorkflowRunVO]{}, err
	}
	chapterNameMap, err := s.chapterNameMap(ctx, dos)
	if err != nil {
		return domain.PageDTO[domain.WorkflowRunVO]{}, err
	}
	vos := make([]domain.WorkflowRunVO, 0, len(dos))
	for i := range dos {
		currentChapterULID := currentChapterULIDOfRun(&dos[i])
		vos = append(vos, toWorkflowRunVO(&dos[i], projectNameMap[dos[i].ProjectULID], chapterNameMap[currentChapterULID]))
	}
	return domain.NewPageDTO(total, req.PageREQ, vos), nil
}

// Detail 查询运行详情：状态、步骤时间线与当前章节正文（审核面板的数据源）。
func (s *WorkflowService) Detail(ctx context.Context, userULID string, runULID string) (domain.WorkflowRunDetailVO, error) {
	runDO, err := s.mustOwnRun(ctx, userULID, runULID)
	if err != nil {
		return domain.WorkflowRunDetailVO{}, err
	}
	project, err := s.projectRepo.GetByULID(ctx, runDO.ProjectULID)
	if err != nil {
		return domain.WorkflowRunDetailVO{}, err
	}
	stepDOS, err := s.workflowRepo.ListStepsByRun(ctx, runDO.RunULID)
	if err != nil {
		return domain.WorkflowRunDetailVO{}, err
	}
	chapterNameMap := make(map[string]string, 8)
	currentChapterULID := currentChapterULIDOfRun(runDO)
	detail := domain.WorkflowRunDetailVO{
		WorkflowRunVO: toWorkflowRunVO(runDO, projectNameOf(project), ""),
		StepList:      make([]domain.WorkflowStepVO, 0, len(stepDOS)),
	}
	if currentChapterULID != "" {
		chapter, err := s.chapterRepo.GetByULID(ctx, currentChapterULID)
		if err != nil {
			return domain.WorkflowRunDetailVO{}, err
		}
		if chapter != nil {
			chapterNameMap[chapter.ChapterULID] = chapter.ChapterName
			detail.CurrentChapterName = chapter.ChapterName
			detail.ChapterContentText = chapter.ContentText
			detail.ChapterWordCount = chapter.WordCount
		}
	}
	if strings.TrimSpace(detail.ChapterContentText) == "" {
		detail.ChapterContentText = runDO.CandidateText
		detail.ChapterWordCount = pkg.CountWords(runDO.CandidateText)
	}
	for i := range stepDOS {
		stepVO := toWorkflowStepVO(&stepDOS[i], chapterNameMap)
		detail.StepList = append(detail.StepList, stepVO)
		if stepDOS[i].ChapterULID == currentChapterULID {
			switch stepDOS[i].StepTypeCode {
			case domain.WorkflowStepTypeCheck:
				detail.CheckScore = int(jsonIntValue(stepDOS[i].MetricJSON, "score_value"))
				detail.CheckVerdictCode = jsonStringValue(stepDOS[i].MetricJSON, "verdict_code")
			case domain.WorkflowStepTypeReview:
				detail.ReviewVerdictCode = jsonStringValue(stepDOS[i].MetricJSON, "verdict_code")
				detail.ReviewOutputText = stepDOS[i].OutputText
			}
		}
	}
	return detail, nil
}

// Stats 统计各状态运行数（工作流中心顶部卡片）。
func (s *WorkflowService) Stats(ctx context.Context, projectULID string) (domain.WorkflowRunStatsVO, error) {
	countByStatus, err := s.workflowRepo.CountRunsByStatus(ctx, projectULID)
	if err != nil {
		return domain.WorkflowRunStatsVO{}, err
	}
	return domain.WorkflowRunStatsVO{
		RunningCount:       countByStatus[domain.WorkflowRunStatusRunning],
		WaitingReviewCount: countByStatus[domain.WorkflowRunStatusWaitingReview],
		FinishedCount:      countByStatus[domain.WorkflowRunStatusSucceeded],
		FailedCount:        countByStatus[domain.WorkflowRunStatusFailed],
	}, nil
}
// runVO 读取最新运行状态并组装视图（操作类接口的统一返回）。
func (s *WorkflowService) runVO(ctx context.Context, runULID string, chapterName string) (domain.WorkflowRunVO, error) {
	runDO, err := s.workflowRepo.GetRun(ctx, runULID)
	if err != nil {
		return domain.WorkflowRunVO{}, err
	}
	if runDO == nil {
		return domain.WorkflowRunVO{}, pkg.NewBizError(domain.RespCodeNotFound, "运行实例不存在")
	}
	project, err := s.projectRepo.GetByULID(ctx, runDO.ProjectULID)
	if err != nil {
		return domain.WorkflowRunVO{}, err
	}
	if chapterName == "" {
		chapterULID := currentChapterULIDOfRun(runDO)
		if chapterULID != "" {
			if chapter, err := s.chapterRepo.GetByULID(ctx, chapterULID); err == nil && chapter != nil {
				chapterName = chapter.ChapterName
			}
		}
	}
	return toWorkflowRunVO(runDO, projectNameOf(project), chapterName), nil
}

// applyCandidate 把待入库正文写回章节（含快照留档与字数账目）。
func (s *WorkflowService) applyCandidate(ctx context.Context, runDO *domain.WorkflowRunDO) error {
	candidateText := strings.TrimSpace(runDO.CandidateText)
	if candidateText == "" {
		return pkg.NewBizError(domain.RespCodeBizFailed, "没有待入库的正文")
	}
	chapterULID := currentChapterULIDOfRun(runDO)
	if chapterULID == "" {
		return pkg.NewBizError(domain.RespCodeBizFailed, "运行没有目标章节")
	}
	chapter, err := s.chapterRepo.GetByULID(ctx, chapterULID)
	if err != nil {
		return err
	}
	if chapter == nil {
		return pkg.NewBizError(domain.RespCodeNotFound, "目标章节已不存在")
	}
	if strings.TrimSpace(chapter.ContentText) != "" {
		// 覆盖已有正文前留档，作者可随时回滚
		if err := s.chapterRepo.CreateSnapshot(ctx, &domain.ChapterSnapshotDO{
			SnapshotULID: pkg.NewULID(domain.IDPrefixChapterSnapshot),
			ChapterULID:  chapter.ChapterULID,
			ContentText:  chapter.ContentText,
			SnapshotType: "manual",
			ChangeNote:   "工作流定稿前留档",
		}); err != nil {
			return err
		}
	}
	newWordCount := pkg.CountWords(candidateText)
	if err := s.chapterRepo.Update(ctx, chapter.ChapterULID, map[string]any{
		"content_text":   candidateText,
		"word_count":     newWordCount,
		"chapter_status": "revised",
	}); err != nil {
		return err
	}
	if delta := newWordCount - chapter.WordCount; delta != 0 {
		if err := applyWordDelta(ctx, s.projectRepo, chapter.ProjectULID, delta); err != nil {
			return err
		}
	}
	return nil
}

// mustOwnRun 加载运行实例并校验归属当前用户。
func (s *WorkflowService) mustOwnRun(ctx context.Context, userULID string, runULID string) (*domain.WorkflowRunDO, error) {
	runDO, err := s.workflowRepo.GetRun(ctx, runULID)
	if err != nil {
		return nil, err
	}
	if runDO == nil {
		return nil, pkg.NewBizError(domain.RespCodeNotFound, "运行实例不存在")
	}
	if runDO.UserULID != userULID {
		return nil, pkg.NewBizError(domain.RespCodeForbidden, "无权操作该运行实例")
	}
	return runDO, nil
}

// projectNameMap 批量取项目名（列表回填，避免 N+1）。
func (s *WorkflowService) projectNameMap(ctx context.Context, dos []domain.WorkflowRunDO) (map[string]string, error) {
	seen := make(map[string]bool, len(dos))
	projectULIDList := make([]string, 0, len(dos))
	for i := range dos {
		if seen[dos[i].ProjectULID] {
			continue
		}
		seen[dos[i].ProjectULID] = true
		projectULIDList = append(projectULIDList, dos[i].ProjectULID)
	}
	nameMap := make(map[string]string, len(projectULIDList))
	if len(projectULIDList) == 0 {
		return nameMap, nil
	}
	projects, err := s.projectRepo.ListByULIDList(ctx, projectULIDList)
	if err != nil {
		return nil, err
	}
	for i := range projects {
		nameMap[projects[i].ProjectULID] = projects[i].ProjectName
	}
	return nameMap, nil
}

// chapterNameMap 批量取当前章节名（列表回填）。
func (s *WorkflowService) chapterNameMap(ctx context.Context, dos []domain.WorkflowRunDO) (map[string]string, error) {
	seen := make(map[string]bool, len(dos))
	chapterULIDList := make([]string, 0, len(dos))
	for i := range dos {
		chapterULID := currentChapterULIDOfRun(&dos[i])
		if chapterULID == "" || seen[chapterULID] {
			continue
		}
		seen[chapterULID] = true
		chapterULIDList = append(chapterULIDList, chapterULID)
	}
	nameMap := make(map[string]string, len(chapterULIDList))
	if len(chapterULIDList) == 0 {
		return nameMap, nil
	}
	chapters, err := s.chapterRepo.ListByULIDList(ctx, chapterULIDList)
	if err != nil {
		return nil, err
	}
	for i := range chapters {
		nameMap[chapters[i].ChapterULID] = chapters[i].ChapterName
	}
	return nameMap, nil
}


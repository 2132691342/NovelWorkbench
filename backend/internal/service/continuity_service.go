// 本文件实现连续性看板业务：时间事实复核、伏笔钩子跟踪与章节摘要管理。

package service

import (
	"context"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// ContinuityService 连续性看板业务：时间事实复核、伏笔钩子跟踪、章节摘要管理。
type ContinuityService struct {
	continuityRepo *repo.ContinuityRepo // 连续性账本仓储
	projectRepo    *repo.ProjectRepo    // 项目仓储（归属校验）
}

// NewContinuityService 构造连续性看板服务。
func NewContinuityService(continuityRepo *repo.ContinuityRepo, projectRepo *repo.ProjectRepo) *ContinuityService {
	return &ContinuityService{continuityRepo: continuityRepo, projectRepo: projectRepo}
}

// isValidFactStatusCode 判断时间事实状态码是否合法（空值视为合法，由调用方补默认值）。
func isValidFactStatusCode(code string) bool {
	switch code {
	case "", domain.FactStatusCodeCandidate, domain.FactStatusCodeConfirmed,
		domain.FactStatusCodeSuperseded, domain.FactStatusCodeStale:
		return true
	default:
		return false
	}
}

// isValidHookStatusCode 判断伏笔钩子状态码是否合法。
func isValidHookStatusCode(code string) bool {
	switch code {
	case domain.HookStatusCodePlanned, domain.HookStatusCodePlanted, domain.HookStatusCodeEchoed,
		domain.HookStatusCodeResolved, domain.HookStatusCodeAbandoned:
		return true
	default:
		return false
	}
}

// isValidImportanceCode 判断重要程度编码是否合法（空值合法，补默认 medium）。
func isValidImportanceCode(code string) bool {
	switch code {
	case "", domain.ImportanceCodeHigh, domain.ImportanceCodeMedium, domain.ImportanceCodeLow:
		return true
	default:
		return false
	}
}

// isValidSummaryStatusCode 判断章节摘要状态码是否合法（空值合法，补默认 pending）。
func isValidSummaryStatusCode(code string) bool {
	switch code {
	case "", domain.SummaryStatusCodePending, domain.SummaryStatusCodeVerified, domain.SummaryStatusCodeStale:
		return true
	default:
		return false
	}
}

// SaveFact 新建/更新时间事实（有 fact_ulid 为更新）。
func (s *ContinuityService) SaveFact(ctx context.Context, userULID string, req *domain.TemporalFactSaveREQ) (domain.TemporalFactVO, error) {
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID); err != nil {
		return domain.TemporalFactVO{}, err
	}
	if !isValidFactStatusCode(req.FactStatusCode) {
		return domain.TemporalFactVO{}, pkg.NewBizError(domain.RespCodeInvalidParam, "无效的事实状态编码")
	}
	if req.FactULID == "" {
		statusCode := req.FactStatusCode
		if statusCode == "" {
			statusCode = domain.FactStatusCodeCandidate
		}
		do := &domain.TemporalFactDO{
			FactULID:              pkg.NewULID(domain.IDPrefixTemporalFact),
			ProjectULID:           req.ProjectULID,
			SubjectText:           req.SubjectText,
			PredicateCode:         req.PredicateCode,
			ObjectText:            req.ObjectText,
			FactStatusCode:        statusCode,
			ValidFromChapterULID:  req.ValidFromChapterULID,
			ValidUntilChapterULID: req.ValidUntilChapterULID,
			SourceChapterULID:     req.SourceChapterULID,
			SourceQuoteText:       req.SourceQuoteText,
			NoteText:              req.NoteText,
		}
		if err := s.continuityRepo.CreateFact(ctx, do); err != nil {
			return domain.TemporalFactVO{}, err
		}
		return toTemporalFactVO(do), nil
	}
	existing, err := s.continuityRepo.GetFactByULID(ctx, req.FactULID)
	if err != nil {
		return domain.TemporalFactVO{}, err
	}
	if existing == nil {
		return domain.TemporalFactVO{}, pkg.NewBizError(domain.RespCodeNotFound, "时间事实不存在")
	}
	if existing.ProjectULID != req.ProjectULID {
		return domain.TemporalFactVO{}, pkg.NewBizError(domain.RespCodeInvalidParam, "时间事实不属于该项目")
	}
	statusCode := req.FactStatusCode
	if statusCode == "" {
		statusCode = existing.FactStatusCode
	}
	fields := map[string]any{
		"subject_text":             req.SubjectText,
		"predicate_code":           req.PredicateCode,
		"object_text":              req.ObjectText,
		"fact_status_code":         statusCode,
		"valid_from_chapter_ulid":  req.ValidFromChapterULID,
		"valid_until_chapter_ulid": req.ValidUntilChapterULID,
		"source_chapter_ulid":      req.SourceChapterULID,
		"source_quote_text":        req.SourceQuoteText,
		"note_text":                req.NoteText,
	}
	if err := s.continuityRepo.UpdateFact(ctx, req.FactULID, fields); err != nil {
		return domain.TemporalFactVO{}, err
	}
	updated, err := s.continuityRepo.GetFactByULID(ctx, req.FactULID)
	if err != nil {
		return domain.TemporalFactVO{}, err
	}
	return toTemporalFactVO(updated), nil
}

// ReviewFact 复核时间事实（确认/取代/标记过期，不可退回候选）。
func (s *ContinuityService) ReviewFact(ctx context.Context, userULID string, req *domain.TemporalFactReviewREQ) (domain.TemporalFactVO, error) {
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID); err != nil {
		return domain.TemporalFactVO{}, err
	}
	switch req.FactStatusCode {
	case domain.FactStatusCodeConfirmed, domain.FactStatusCodeSuperseded, domain.FactStatusCodeStale:
	default:
		return domain.TemporalFactVO{}, pkg.NewBizError(domain.RespCodeInvalidParam, "无效的复核状态编码")
	}
	existing, err := s.continuityRepo.GetFactByULID(ctx, req.FactULID)
	if err != nil {
		return domain.TemporalFactVO{}, err
	}
	if existing == nil {
		return domain.TemporalFactVO{}, pkg.NewBizError(domain.RespCodeNotFound, "时间事实不存在")
	}
	if existing.ProjectULID != req.ProjectULID {
		return domain.TemporalFactVO{}, pkg.NewBizError(domain.RespCodeInvalidParam, "时间事实不属于该项目")
	}
	fields := map[string]any{"fact_status_code": req.FactStatusCode}
	if req.NoteText != "" {
		fields["note_text"] = req.NoteText
	}
	if err := s.continuityRepo.UpdateFact(ctx, req.FactULID, fields); err != nil {
		return domain.TemporalFactVO{}, err
	}
	updated, err := s.continuityRepo.GetFactByULID(ctx, req.FactULID)
	if err != nil {
		return domain.TemporalFactVO{}, err
	}
	return toTemporalFactVO(updated), nil
}

// ListFacts 查询项目时间事实列表（状态/章节过滤）。
func (s *ContinuityService) ListFacts(ctx context.Context, userULID string, req *domain.TemporalFactListREQ) ([]domain.TemporalFactVO, error) {
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID); err != nil {
		return nil, err
	}
	if !isValidFactStatusCode(req.FactStatusCode) {
		return nil, pkg.NewBizError(domain.RespCodeInvalidParam, "无效的事实状态编码")
	}
	dos, err := s.continuityRepo.ListFactsByProject(ctx, req.ProjectULID, req.FactStatusCode, req.ChapterULID)
	if err != nil {
		return nil, err
	}
	vos := make([]domain.TemporalFactVO, 0, len(dos))
	for i := range dos {
		vos = append(vos, toTemporalFactVO(&dos[i]))
	}
	return vos, nil
}

// SaveHook 新建/更新伏笔钩子（状态随显式保存推进）。
func (s *ContinuityService) SaveHook(ctx context.Context, userULID string, req *domain.ForeshadowHookSaveREQ) (domain.ForeshadowHookVO, error) {
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID); err != nil {
		return domain.ForeshadowHookVO{}, err
	}
	if !isValidHookStatusCode(req.HookStatusCode) {
		return domain.ForeshadowHookVO{}, pkg.NewBizError(domain.RespCodeInvalidParam, "无效的伏笔状态编码")
	}
	if !isValidImportanceCode(req.ImportanceCode) {
		return domain.ForeshadowHookVO{}, pkg.NewBizError(domain.RespCodeInvalidParam, "无效的重要程度编码")
	}
	importanceCode := req.ImportanceCode
	if importanceCode == "" {
		importanceCode = domain.ImportanceCodeMedium
	}
	if req.HookULID == "" {
		do := &domain.ForeshadowHookDO{
			HookULID:           pkg.NewULID(domain.IDPrefixForeshadowHook),
			ProjectULID:        req.ProjectULID,
			HookStatusCode:     req.HookStatusCode,
			DescriptionText:    req.DescriptionText,
			ExpectedPayoffText: req.ExpectedPayoffText,
			SetupChapterULID:   req.SetupChapterULID,
			PayoffChapterULID:  req.PayoffChapterULID,
			ImportanceCode:     importanceCode,
			NoteText:           req.NoteText,
		}
		if err := s.continuityRepo.CreateHook(ctx, do); err != nil {
			return domain.ForeshadowHookVO{}, err
		}
		return toForeshadowHookVO(do), nil
	}
	existing, err := s.continuityRepo.GetHookByULID(ctx, req.HookULID)
	if err != nil {
		return domain.ForeshadowHookVO{}, err
	}
	if existing == nil {
		return domain.ForeshadowHookVO{}, pkg.NewBizError(domain.RespCodeNotFound, "伏笔钩子不存在")
	}
	if existing.ProjectULID != req.ProjectULID {
		return domain.ForeshadowHookVO{}, pkg.NewBizError(domain.RespCodeInvalidParam, "伏笔钩子不属于该项目")
	}
	fields := map[string]any{
		"hook_status_code":     req.HookStatusCode,
		"description_text":     req.DescriptionText,
		"expected_payoff_text": req.ExpectedPayoffText,
		"setup_chapter_ulid":   req.SetupChapterULID,
		"payoff_chapter_ulid":  req.PayoffChapterULID,
		"importance_code":      importanceCode,
		"note_text":            req.NoteText,
	}
	if err := s.continuityRepo.UpdateHook(ctx, req.HookULID, fields); err != nil {
		return domain.ForeshadowHookVO{}, err
	}
	updated, err := s.continuityRepo.GetHookByULID(ctx, req.HookULID)
	if err != nil {
		return domain.ForeshadowHookVO{}, err
	}
	return toForeshadowHookVO(updated), nil
}

// ListHooks 查询项目伏笔钩子列表（状态过滤）。
func (s *ContinuityService) ListHooks(ctx context.Context, userULID string, req *domain.ForeshadowHookListREQ) ([]domain.ForeshadowHookVO, error) {
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID); err != nil {
		return nil, err
	}
	if !isValidHookStatusCode(req.HookStatusCode) && req.HookStatusCode != "" {
		return nil, pkg.NewBizError(domain.RespCodeInvalidParam, "无效的伏笔状态编码")
	}
	dos, err := s.continuityRepo.ListHooksByProject(ctx, req.ProjectULID, req.HookStatusCode)
	if err != nil {
		return nil, err
	}
	vos := make([]domain.ForeshadowHookVO, 0, len(dos))
	for i := range dos {
		vos = append(vos, toForeshadowHookVO(&dos[i]))
	}
	return vos, nil
}

// SaveSummary 新建/更新章节摘要（同章节重复保存时更新既有记录）。
func (s *ContinuityService) SaveSummary(ctx context.Context, userULID string, req *domain.ChapterSummarySaveREQ) (domain.ChapterSummaryVO, error) {
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID); err != nil {
		return domain.ChapterSummaryVO{}, err
	}
	if !isValidSummaryStatusCode(req.SummaryStatusCode) {
		return domain.ChapterSummaryVO{}, pkg.NewBizError(domain.RespCodeInvalidParam, "无效的摘要状态编码")
	}
	statusCode := req.SummaryStatusCode
	if statusCode == "" {
		statusCode = domain.SummaryStatusCodePending
	}
	fields := map[string]any{
		"summary_text":        req.SummaryText,
		"source_hash_text":    req.SourceHashText,
		"summary_status_code": statusCode,
	}
	if req.SummaryULID != "" {
		existing, err := s.continuityRepo.GetSummaryByULID(ctx, req.SummaryULID)
		if err != nil {
			return domain.ChapterSummaryVO{}, err
		}
		if existing == nil {
			return domain.ChapterSummaryVO{}, pkg.NewBizError(domain.RespCodeNotFound, "章节摘要不存在")
		}
		if existing.ProjectULID != req.ProjectULID {
			return domain.ChapterSummaryVO{}, pkg.NewBizError(domain.RespCodeInvalidParam, "章节摘要不属于该项目")
		}
		fields["chapter_ulid"] = req.ChapterULID
		if err := s.continuityRepo.UpdateSummary(ctx, req.SummaryULID, fields); err != nil {
			return domain.ChapterSummaryVO{}, err
		}
		updated, err := s.continuityRepo.GetSummaryByULID(ctx, req.SummaryULID)
		if err != nil {
			return domain.ChapterSummaryVO{}, err
		}
		return toChapterSummaryVO(updated), nil
	}
	existing, err := s.continuityRepo.GetSummaryByChapter(ctx, req.ProjectULID, req.ChapterULID)
	if err != nil {
		return domain.ChapterSummaryVO{}, err
	}
	if existing != nil {
		if err := s.continuityRepo.UpdateSummary(ctx, existing.SummaryULID, fields); err != nil {
			return domain.ChapterSummaryVO{}, err
		}
		updated, err := s.continuityRepo.GetSummaryByULID(ctx, existing.SummaryULID)
		if err != nil {
			return domain.ChapterSummaryVO{}, err
		}
		return toChapterSummaryVO(updated), nil
	}
	do := &domain.ChapterSummaryDO{
		SummaryULID:       pkg.NewULID(domain.IDPrefixChapterSummary),
		ProjectULID:       req.ProjectULID,
		ChapterULID:       req.ChapterULID,
		SummaryText:       req.SummaryText,
		SourceHashText:    req.SourceHashText,
		SummaryStatusCode: statusCode,
	}
	if err := s.continuityRepo.CreateSummary(ctx, do); err != nil {
		return domain.ChapterSummaryVO{}, err
	}
	return toChapterSummaryVO(do), nil
}

// ListSummaries 查询项目章节摘要列表（状态/章节过滤）。
func (s *ContinuityService) ListSummaries(ctx context.Context, userULID string, req *domain.ChapterSummaryListREQ) ([]domain.ChapterSummaryVO, error) {
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID); err != nil {
		return nil, err
	}
	if !isValidSummaryStatusCode(req.SummaryStatusCode) {
		return nil, pkg.NewBizError(domain.RespCodeInvalidParam, "无效的摘要状态编码")
	}
	dos, err := s.continuityRepo.ListSummariesByProject(ctx, req.ProjectULID, req.SummaryStatusCode, req.ChapterULID)
	if err != nil {
		return nil, err
	}
	vos := make([]domain.ChapterSummaryVO, 0, len(dos))
	for i := range dos {
		vos = append(vos, toChapterSummaryVO(&dos[i]))
	}
	return vos, nil
}

// toTemporalFactVO 组装时间事实视图对象。
func toTemporalFactVO(do *domain.TemporalFactDO) domain.TemporalFactVO {
	return domain.TemporalFactVO{
		FactULID:              do.FactULID,
		ProjectULID:           do.ProjectULID,
		SubjectText:           do.SubjectText,
		PredicateCode:         do.PredicateCode,
		ObjectText:            do.ObjectText,
		FactStatusCode:        do.FactStatusCode,
		ValidFromChapterULID:  do.ValidFromChapterULID,
		ValidUntilChapterULID: do.ValidUntilChapterULID,
		SourceChapterULID:     do.SourceChapterULID,
		SourceQuoteText:       do.SourceQuoteText,
		NoteText:              do.NoteText,
		UpdatedAtMs:           do.UpdatedAt.UnixMilli(),
	}
}

// toForeshadowHookVO 组装伏笔钩子视图对象。
func toForeshadowHookVO(do *domain.ForeshadowHookDO) domain.ForeshadowHookVO {
	return domain.ForeshadowHookVO{
		HookULID:           do.HookULID,
		ProjectULID:        do.ProjectULID,
		HookStatusCode:     do.HookStatusCode,
		DescriptionText:    do.DescriptionText,
		ExpectedPayoffText: do.ExpectedPayoffText,
		SetupChapterULID:   do.SetupChapterULID,
		PayoffChapterULID:  do.PayoffChapterULID,
		ImportanceCode:     do.ImportanceCode,
		NoteText:           do.NoteText,
		UpdatedAtMs:        do.UpdatedAt.UnixMilli(),
	}
}

// toChapterSummaryVO 组装章节摘要视图对象。
func toChapterSummaryVO(do *domain.ChapterSummaryDO) domain.ChapterSummaryVO {
	return domain.ChapterSummaryVO{
		SummaryULID:       do.SummaryULID,
		ProjectULID:       do.ProjectULID,
		ChapterULID:       do.ChapterULID,
		SummaryText:       do.SummaryText,
		SourceHashText:    do.SourceHashText,
		SummaryStatusCode: do.SummaryStatusCode,
		UpdatedAtMs:       do.UpdatedAt.UnixMilli(),
	}
}

// 本文件实现章节业务：章节 CRUD、快照留档/回滚与字数账目维护。

package service

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai/check"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// ChapterService 章节业务：章节 CRUD、快照留档/回滚与字数账目维护。
type ChapterService struct {
	chapterRepo *repo.ChapterRepo // 章节仓储
	volumeRepo  *repo.VolumeRepo  // 卷仓储
	projectRepo *repo.ProjectRepo // 项目仓储（归属校验）
	db          *gorm.DB          // 事务入口
}

// NewChapterService 构造章节服务。
func NewChapterService(chapterRepo *repo.ChapterRepo, volumeRepo *repo.VolumeRepo, projectRepo *repo.ProjectRepo, db *gorm.DB) *ChapterService {
	return &ChapterService{chapterRepo: chapterRepo, volumeRepo: volumeRepo, projectRepo: projectRepo, db: db}
}

// Create 创建章节（排序号取项目内最大值 +1，状态默认 draft）。
func (s *ChapterService) Create(ctx context.Context, userULID string, req *domain.ChapterCreateREQ) (domain.ChapterDetailVO, error) {
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, req.ProjectULID); err != nil {
		return domain.ChapterDetailVO{}, err
	}
	volume, err := s.volumeRepo.GetByULID(ctx, req.VolumeULID)
	if err != nil {
		return domain.ChapterDetailVO{}, err
	}
	if volume == nil {
		return domain.ChapterDetailVO{}, pkg.NewBizError(domain.RespCodeNotFound, "卷不存在")
	}
	if volume.ProjectULID != req.ProjectULID {
		return domain.ChapterDetailVO{}, pkg.NewBizError(domain.RespCodeInvalidParam, "卷不属于该项目")
	}
	maxOrder, err := s.chapterRepo.MaxOrder(ctx, req.ProjectULID)
	if err != nil {
		return domain.ChapterDetailVO{}, err
	}
	do := &domain.ChapterDO{
		ChapterULID:   pkg.NewULID(domain.IDPrefixChapter),
		ProjectULID:   req.ProjectULID,
		VolumeULID:    req.VolumeULID,
		ChapterName:   req.ChapterName,
		ChapterOrder:  maxOrder + 1,
		ChapterStatus: "draft",
	}
	if err := s.chapterRepo.Create(ctx, do); err != nil {
		return domain.ChapterDetailVO{}, err
	}
	return toChapterDetailVO(do), nil
}

// Update 更新章节（仅覆盖非空字段；正文更新时按字数差维护项目字数账目）。
func (s *ChapterService) Update(ctx context.Context, userULID string, req *domain.ChapterUpdateREQ) (domain.ChapterDetailVO, error) {
	var result domain.ChapterDetailVO
	err := repo.WithTx(s.db, func(tx *gorm.DB) error {
		txChapter := repo.NewChapterRepo(tx)
		txVolume := repo.NewVolumeRepo(tx)
		txProject := repo.NewProjectRepo(tx)
		chapter, err := txChapter.GetByULID(ctx, req.ChapterULID)
		if err != nil {
			return err
		}
		if chapter == nil {
			return pkg.NewBizError(domain.RespCodeNotFound, "章节不存在")
		}
		if _, err := mustOwnProject(ctx, txProject, userULID, chapter.ProjectULID); err != nil {
			return err
		}
		fields := map[string]any{}
		if req.ChapterName != "" {
			fields["chapter_name"] = req.ChapterName
		}
		if req.SummaryText != nil {
			fields["summary_text"] = *req.SummaryText
		}
		if req.ChapterStatus != "" {
			fields["chapter_status"] = req.ChapterStatus
		}
		if req.VolumeULID != "" && req.VolumeULID != chapter.VolumeULID {
			volume, err := txVolume.GetByULID(ctx, req.VolumeULID)
			if err != nil {
				return err
			}
			if volume == nil {
				return pkg.NewBizError(domain.RespCodeNotFound, "卷不存在")
			}
			if volume.ProjectULID != chapter.ProjectULID {
				return pkg.NewBizError(domain.RespCodeInvalidParam, "卷不属于该项目")
			}
			fields["volume_ulid"] = req.VolumeULID
		}
		if req.ContentText != nil {
			newCount := pkg.CountWords(*req.ContentText)
			fields["content_text"] = *req.ContentText
			fields["word_count"] = newCount
			if delta := newCount - chapter.WordCount; delta != 0 {
				if err := applyWordDelta(ctx, txProject, chapter.ProjectULID, delta); err != nil {
					return err
				}
			}
		}
		if len(fields) > 0 {
			if err := txChapter.Update(ctx, req.ChapterULID, fields); err != nil {
				return err
			}
		}
		updated, err := txChapter.GetByULID(ctx, req.ChapterULID)
		if err != nil {
			return err
		}
		if updated == nil {
			return pkg.NewBizError(domain.RespCodeNotFound, "章节不存在")
		}
		result = toChapterDetailVO(updated)
		return nil
	})
	if err != nil {
		return domain.ChapterDetailVO{}, err
	}
	return result, nil
}

// Delete 删除章节及其快照，并回补项目字数差。
func (s *ChapterService) Delete(ctx context.Context, userULID string, req *domain.ChapterDeleteREQ) error {
	return repo.WithTx(s.db, func(tx *gorm.DB) error {
		txChapter := repo.NewChapterRepo(tx)
		txProject := repo.NewProjectRepo(tx)
		chapter, err := txChapter.GetByULID(ctx, req.ChapterULID)
		if err != nil {
			return err
		}
		if chapter == nil {
			return nil // 幂等：目标已不存在，视为删除成功
		}
		if _, err := mustOwnProject(ctx, txProject, userULID, chapter.ProjectULID); err != nil {
			return err
		}
		if err := txChapter.DeleteSnapshotsByChapter(ctx, req.ChapterULID); err != nil {
			return err
		}
		if err := txChapter.Delete(ctx, req.ChapterULID); err != nil {
			return err
		}
		if chapter.WordCount != 0 {
			if err := applyWordDelta(ctx, txProject, chapter.ProjectULID, -chapter.WordCount); err != nil {
				return err
			}
		}
		return nil
	})
}

// Detail 查询章节详情（含正文，编辑器载入）。
func (s *ChapterService) Detail(ctx context.Context, userULID string, req *domain.ChapterContentREQ) (domain.ChapterDetailVO, error) {
	chapter, err := s.mustOwnChapter(ctx, userULID, req.ChapterULID)
	if err != nil {
		return domain.ChapterDetailVO{}, err
	}
	return toChapterDetailVO(chapter), nil
}

// SnapshotCreate 以当前正文生成人工快照。
func (s *ChapterService) SnapshotCreate(ctx context.Context, userULID string, req *domain.ChapterSnapshotCreateREQ) (domain.ChapterSnapshotVO, error) {
	chapter, err := s.mustOwnChapter(ctx, userULID, req.ChapterULID)
	if err != nil {
		return domain.ChapterSnapshotVO{}, err
	}
	do := &domain.ChapterSnapshotDO{
		SnapshotULID: pkg.NewULID(domain.IDPrefixChapterSnapshot),
		ChapterULID:  chapter.ChapterULID,
		ContentText:  chapter.ContentText,
		SnapshotType: "manual",
		ChangeNote:   req.ChangeNote,
	}
	if err := s.chapterRepo.CreateSnapshot(ctx, do); err != nil {
		return domain.ChapterSnapshotVO{}, err
	}
	return toSnapshotVO(do), nil
}

// SnapshotList 按生成时间倒序返回章节快照列表。
func (s *ChapterService) SnapshotList(ctx context.Context, userULID string, req *domain.ChapterSnapshotListREQ) ([]domain.ChapterSnapshotVO, error) {
	if _, err := s.mustOwnChapter(ctx, userULID, req.ChapterULID); err != nil {
		return nil, err
	}
	dos, err := s.chapterRepo.ListSnapshotsByChapter(ctx, req.ChapterULID)
	if err != nil {
		return nil, err
	}
	vos := make([]domain.ChapterSnapshotVO, 0, len(dos))
	for i := range dos {
		vos = append(vos, toSnapshotVO(&dos[i]))
	}
	return vos, nil
}

// SnapshotRestore 回滚正文到快照（回滚前自动留档当前正文，并维护字数账目）。
func (s *ChapterService) SnapshotRestore(ctx context.Context, userULID string, req *domain.ChapterSnapshotRestoreREQ) (domain.ChapterDetailVO, error) {
	var result domain.ChapterDetailVO
	err := repo.WithTx(s.db, func(tx *gorm.DB) error {
		txChapter := repo.NewChapterRepo(tx)
		txProject := repo.NewProjectRepo(tx)
		snapshot, err := txChapter.GetSnapshotByULID(ctx, req.SnapshotULID)
		if err != nil {
			return err
		}
		if snapshot == nil {
			return pkg.NewBizError(domain.RespCodeNotFound, "快照不存在")
		}
		chapter, err := txChapter.GetByULID(ctx, snapshot.ChapterULID)
		if err != nil {
			return err
		}
		if chapter == nil {
			return pkg.NewBizError(domain.RespCodeNotFound, "章节不存在")
		}
		if _, err := mustOwnProject(ctx, txProject, userULID, chapter.ProjectULID); err != nil {
			return err
		}
		backup := &domain.ChapterSnapshotDO{
			SnapshotULID: pkg.NewULID(domain.IDPrefixChapterSnapshot),
			ChapterULID:  chapter.ChapterULID,
			ContentText:  chapter.ContentText,
			SnapshotType: "manual",
			ChangeNote:   "restore 自动留档",
		}
		if err := txChapter.CreateSnapshot(ctx, backup); err != nil {
			return err
		}
		newCount := pkg.CountWords(snapshot.ContentText)
		fields := map[string]any{"content_text": snapshot.ContentText, "word_count": newCount}
		if err := txChapter.Update(ctx, chapter.ChapterULID, fields); err != nil {
			return err
		}
		if delta := newCount - chapter.WordCount; delta != 0 {
			if err := applyWordDelta(ctx, txProject, chapter.ProjectULID, delta); err != nil {
				return err
			}
		}
		updated, err := txChapter.GetByULID(ctx, chapter.ChapterULID)
		if err != nil {
			return err
		}
		if updated == nil {
			return pkg.NewBizError(domain.RespCodeNotFound, "章节不存在")
		}
		result = toChapterDetailVO(updated)
		return nil
	})
	if err != nil {
		return domain.ChapterDetailVO{}, err
	}
	return result, nil
}

// mustOwnChapter 加载章节并校验所属项目归属当前用户。
func (s *ChapterService) mustOwnChapter(ctx context.Context, userULID, chapterULID string) (*domain.ChapterDO, error) {
	chapter, err := s.chapterRepo.GetByULID(ctx, chapterULID)
	if err != nil {
		return nil, err
	}
	if chapter == nil {
		return nil, pkg.NewBizError(domain.RespCodeNotFound, "章节不存在")
	}
	if _, err := mustOwnProject(ctx, s.projectRepo, userULID, chapter.ProjectULID); err != nil {
		return nil, err
	}
	return chapter, nil
}

// applyWordDelta 按字数差调整项目累计字数并累加今日统计。
func applyWordDelta(ctx context.Context, projectRepo *repo.ProjectRepo, projectULID string, delta int64) error {
	if err := projectRepo.AdjustTotalWordCount(ctx, projectULID, delta); err != nil {
		return err
	}
	return projectRepo.UpsertWordDelta(ctx, projectULID, time.Now().Format("2006-01-02"), delta)
}

// toChapterBriefVO 组装章节摘要视图。
func toChapterBriefVO(do *domain.ChapterDO) domain.ChapterBriefVO {
	return domain.ChapterBriefVO{
		ChapterULID:   do.ChapterULID,
		ChapterName:   do.ChapterName,
		ChapterOrder:  do.ChapterOrder,
		WordCount:     do.WordCount,
		ChapterStatus: do.ChapterStatus,
	}
}

// toChapterDetailVO 组装章节详情视图。
func toChapterDetailVO(do *domain.ChapterDO) domain.ChapterDetailVO {
	return domain.ChapterDetailVO{
		ChapterBriefVO: toChapterBriefVO(do),
		ProjectULID:    do.ProjectULID,
		VolumeULID:     do.VolumeULID,
		ContentText:    do.ContentText,
		SummaryText:    do.SummaryText,
		UpdatedAt:      pkg.FormatTime(do.UpdatedAt),
	}
}

// toSnapshotVO 组装快照视图（字数按快照正文计算）。
func toSnapshotVO(do *domain.ChapterSnapshotDO) domain.ChapterSnapshotVO {
	return domain.ChapterSnapshotVO{
		SnapshotULID: do.SnapshotULID,
		ChapterULID:  do.ChapterULID,
		SnapshotType: do.SnapshotType,
		ChangeNote:   do.ChangeNote,
		WordCount:    pkg.CountWords(do.ContentText),
		CreatedAt:    pkg.FormatTime(do.CreatedAt),
	}
}

// Check 章节量化体检：正文优先用请求体，否则读库内章节正文（需项目归属）。
func (s *ChapterService) Check(ctx context.Context, userULID string, req *domain.ChapterCheckREQ) (domain.ChapterCheckVO, error) {
	contentText := req.ContentText
	if strings.TrimSpace(contentText) == "" {
		if req.ChapterULID == "" {
			return domain.ChapterCheckVO{}, pkg.NewBizError(domain.RespCodeInvalidParam, "请提供章节或正文")
		}
		chapter, err := s.mustOwnChapter(ctx, userULID, req.ChapterULID)
		if err != nil {
			return domain.ChapterCheckVO{}, err
		}
		contentText = chapter.ContentText
	} else if req.ChapterULID != "" {
		if _, err := s.mustOwnChapter(ctx, userULID, req.ChapterULID); err != nil {
			return domain.ChapterCheckVO{}, err
		}
	}
	report := check.Analyze(contentText)
	vo := domain.ChapterCheckVO{
		WordCount: report.WordCount, ParagraphCount: report.ParagraphCount,
		DialogueRatio: report.DialogueRatio, DashCount: report.DashCount,
		BannedWordCount: report.BannedWordCount, WarnWordCount: report.WarnWordCount,
		EmotionWordCount: report.EmotionWordCount, BareTagCount: report.BareTagCount,
		HookPresent: report.HookPresent, ScoreValue: report.ScoreValue, VerdictCode: report.VerdictCode,
		IssueList: make([]domain.ChapterCheckIssueVO, 0, len(report.IssueList)),
	}
	for _, issue := range report.IssueList {
		vo.IssueList = append(vo.IssueList, domain.ChapterCheckIssueVO{
			IssueCode: issue.IssueCode, LevelCode: issue.LevelCode, TitleText: issue.TitleText,
			DetailText: issue.DetailText, HitCount: issue.HitCount, SampleText: issue.SampleText,
		})
	}
	return vo, nil
}

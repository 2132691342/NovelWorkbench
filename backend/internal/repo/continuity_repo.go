// 本文件实现连续性看板（时间事实、伏笔钩子、章节摘要）的持久化仓储。

package repo

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// ContinuityRepo 连续性看板持久化仓储：时间事实、伏笔钩子、章节摘要。
type ContinuityRepo struct {
	db *gorm.DB // 事务入口
}

// NewContinuityRepo 构造连续性看板仓储。
func NewContinuityRepo(db *gorm.DB) *ContinuityRepo {
	return &ContinuityRepo{db: db}
}

// CreateFact 插入时间事实记录。
func (r *ContinuityRepo) CreateFact(ctx context.Context, do *domain.TemporalFactDO) error {
	if err := r.db.WithContext(ctx).Create(do).Error; err != nil {
		return fmt.Errorf("create temporal fact: %w", err)
	}
	return nil
}

// UpdateFact 按列名增量更新时间事实（map 更新防零值丢失）。
func (r *ContinuityRepo) UpdateFact(ctx context.Context, factULID string, fields map[string]any) error {
	result := r.db.WithContext(ctx).Model(&domain.TemporalFactDO{}).
		Where("fact_ulid = ?", factULID).Updates(fields)
	if result.Error != nil {
		return fmt.Errorf("update temporal fact: %w", result.Error)
	}
	return nil
}

// GetFactByULID 按 ULID 查询时间事实，不存在时返回 (nil, nil)。
func (r *ContinuityRepo) GetFactByULID(ctx context.Context, factULID string) (*domain.TemporalFactDO, error) {
	var do domain.TemporalFactDO
	err := r.db.WithContext(ctx).Where("fact_ulid = ?", factULID).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get temporal fact: %w", err)
	}
	return &do, nil
}

// ListFactsByProject 按项目查询时间事实（状态 + 章节过滤）。
func (r *ContinuityRepo) ListFactsByProject(ctx context.Context, projectULID, factStatusCode, chapterULID string) ([]domain.TemporalFactDO, error) {
	query := r.db.WithContext(ctx).Model(&domain.TemporalFactDO{}).
		Where("project_ulid = ?", projectULID)
	if factStatusCode != "" {
		query = query.Where("fact_status_code = ?", factStatusCode)
	}
	if chapterULID != "" {
		query = query.Where("(valid_from_chapter_ulid = ? OR valid_until_chapter_ulid = ? OR source_chapter_ulid = ?)",
			chapterULID, chapterULID, chapterULID)
	}
	var dos []domain.TemporalFactDO
	if err := query.Order("created_at DESC").Find(&dos).Error; err != nil {
		return nil, fmt.Errorf("list temporal facts: %w", err)
	}
	return dos, nil
}

// CreateHook 插入伏笔钩子记录。
func (r *ContinuityRepo) CreateHook(ctx context.Context, do *domain.ForeshadowHookDO) error {
	if err := r.db.WithContext(ctx).Create(do).Error; err != nil {
		return fmt.Errorf("create foreshadow hook: %w", err)
	}
	return nil
}

// UpdateHook 按列名增量更新伏笔钩子（map 更新防零值丢失）。
func (r *ContinuityRepo) UpdateHook(ctx context.Context, hookULID string, fields map[string]any) error {
	result := r.db.WithContext(ctx).Model(&domain.ForeshadowHookDO{}).
		Where("hook_ulid = ?", hookULID).Updates(fields)
	if result.Error != nil {
		return fmt.Errorf("update foreshadow hook: %w", result.Error)
	}
	return nil
}

// GetHookByULID 按 ULID 查询伏笔钩子，不存在时返回 (nil, nil)。
func (r *ContinuityRepo) GetHookByULID(ctx context.Context, hookULID string) (*domain.ForeshadowHookDO, error) {
	var do domain.ForeshadowHookDO
	err := r.db.WithContext(ctx).Where("hook_ulid = ?", hookULID).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get foreshadow hook: %w", err)
	}
	return &do, nil
}

// ListHooksByProject 按项目查询伏笔钩子（状态过滤）。
func (r *ContinuityRepo) ListHooksByProject(ctx context.Context, projectULID, hookStatusCode string) ([]domain.ForeshadowHookDO, error) {
	query := r.db.WithContext(ctx).Model(&domain.ForeshadowHookDO{}).
		Where("project_ulid = ?", projectULID)
	if hookStatusCode != "" {
		query = query.Where("hook_status_code = ?", hookStatusCode)
	}
	var dos []domain.ForeshadowHookDO
	if err := query.Order("created_at DESC").Find(&dos).Error; err != nil {
		return nil, fmt.Errorf("list foreshadow hooks: %w", err)
	}
	return dos, nil
}

// CreateSummary 插入章节摘要记录。
func (r *ContinuityRepo) CreateSummary(ctx context.Context, do *domain.ChapterSummaryDO) error {
	if err := r.db.WithContext(ctx).Create(do).Error; err != nil {
		return fmt.Errorf("create chapter summary: %w", err)
	}
	return nil
}

// UpdateSummary 按列名增量更新章节摘要（map 更新防零值丢失）。
func (r *ContinuityRepo) UpdateSummary(ctx context.Context, summaryULID string, fields map[string]any) error {
	result := r.db.WithContext(ctx).Model(&domain.ChapterSummaryDO{}).
		Where("summary_ulid = ?", summaryULID).Updates(fields)
	if result.Error != nil {
		return fmt.Errorf("update chapter summary: %w", result.Error)
	}
	return nil
}

// GetSummaryByULID 按 ULID 查询章节摘要，不存在时返回 (nil, nil)。
func (r *ContinuityRepo) GetSummaryByULID(ctx context.Context, summaryULID string) (*domain.ChapterSummaryDO, error) {
	var do domain.ChapterSummaryDO
	err := r.db.WithContext(ctx).Where("summary_ulid = ?", summaryULID).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get chapter summary: %w", err)
	}
	return &do, nil
}

// GetSummaryByChapter 按章节查询摘要（同一章节至多一条），不存在时返回 (nil, nil)。
func (r *ContinuityRepo) GetSummaryByChapter(ctx context.Context, projectULID, chapterULID string) (*domain.ChapterSummaryDO, error) {
	var do domain.ChapterSummaryDO
	err := r.db.WithContext(ctx).
		Where("project_ulid = ? AND chapter_ulid = ?", projectULID, chapterULID).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get chapter summary by chapter: %w", err)
	}
	return &do, nil
}

// ListSummariesByProject 按项目查询章节摘要（状态 + 章节过滤）。
func (r *ContinuityRepo) ListSummariesByProject(ctx context.Context, projectULID, summaryStatusCode, chapterULID string) ([]domain.ChapterSummaryDO, error) {
	query := r.db.WithContext(ctx).Model(&domain.ChapterSummaryDO{}).
		Where("project_ulid = ?", projectULID)
	if summaryStatusCode != "" {
		query = query.Where("summary_status_code = ?", summaryStatusCode)
	}
	if chapterULID != "" {
		query = query.Where("chapter_ulid = ?", chapterULID)
	}
	var dos []domain.ChapterSummaryDO
	if err := query.Order("created_at DESC").Find(&dos).Error; err != nil {
		return nil, fmt.Errorf("list chapter summaries: %w", err)
	}
	return dos, nil
}

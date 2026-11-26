// 本文件实现小说项目的持久化仓储。

package repo

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
)

// ProjectRepo 小说项目持久化仓储。
type ProjectRepo struct {
	db *gorm.DB // 事务入口
}

// NewProjectRepo 构造项目仓储。
func NewProjectRepo(db *gorm.DB) *ProjectRepo {
	return &ProjectRepo{db: db}
}

// Create 插入项目记录。
func (r *ProjectRepo) Create(ctx context.Context, do *domain.ProjectDO) error {
	if err := r.db.WithContext(ctx).Create(do).Error; err != nil {
		return fmt.Errorf("create project: %w", err)
	}
	return nil
}

// Update 按列名增量更新项目（map 更新防零值丢失）。
func (r *ProjectRepo) Update(ctx context.Context, projectULID string, fields map[string]any) error {
	result := r.db.WithContext(ctx).Model(&domain.ProjectDO{}).
		Where("project_ulid = ?", projectULID).Updates(fields)
	if result.Error != nil {
		return fmt.Errorf("update project: %w", result.Error)
	}
	return nil
}

// Delete 删除项目记录。
func (r *ProjectRepo) Delete(ctx context.Context, projectULID string) error {
	if err := r.db.WithContext(ctx).
		Where("project_ulid = ?", projectULID).Delete(&domain.ProjectDO{}).Error; err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	return nil
}

// GetByULID 按 ULID 查询项目，不存在时返回 (nil, nil)。
func (r *ProjectRepo) GetByULID(ctx context.Context, projectULID string) (*domain.ProjectDO, error) {
	var do domain.ProjectDO
	err := r.db.WithContext(ctx).Where("project_ulid = ?", projectULID).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get project: %w", err)
	}
	return &do, nil
}

// List 按用户分页查询项目（名称模糊 + 状态过滤，创建时间倒序），返回列表与总数。
func (r *ProjectRepo) List(ctx context.Context, userULID, keywordText, projectStatus string, pageNum, pageSize int) ([]domain.ProjectDO, int64, error) {
	query := r.db.WithContext(ctx).Model(&domain.ProjectDO{}).Where("user_ulid = ?", userULID)
	if keywordText != "" {
		query = query.Where("project_name ILIKE ?", "%"+keywordText+"%")
	}
	if projectStatus != "" {
		query = query.Where("project_status = ?", projectStatus)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count projects: %w", err)
	}
	var dos []domain.ProjectDO
	err := query.Order("created_at DESC").
		Offset((pageNum - 1) * pageSize).Limit(pageSize).
		Find(&dos).Error
	if err != nil {
		return nil, 0, fmt.Errorf("list projects: %w", err)
	}
	return dos, total, nil
}

// CountVolumesByProject 统计各项目卷数（按 project_ulid 分组）。
func (r *ProjectRepo) CountVolumesByProject(ctx context.Context, projectULIDs []string) (map[string]int64, error) {
	counts := make(map[string]int64, len(projectULIDs))
	if len(projectULIDs) == 0 {
		return counts, nil
	}
	var rows []struct {
		ProjectULID string
		Cnt         int64
	}
	err := r.db.WithContext(ctx).Model(&domain.VolumeDO{}).
		Select("project_ulid, COUNT(*) AS cnt").
		Where("project_ulid IN ?", projectULIDs).
		Group("project_ulid").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("count volumes: %w", err)
	}
	for _, row := range rows {
		counts[row.ProjectULID] = row.Cnt
	}
	return counts, nil
}

// CountChaptersByProject 统计各项目章节数（按 project_ulid 分组）。
func (r *ProjectRepo) CountChaptersByProject(ctx context.Context, projectULIDs []string) (map[string]int64, error) {
	counts := make(map[string]int64, len(projectULIDs))
	if len(projectULIDs) == 0 {
		return counts, nil
	}
	var rows []struct {
		ProjectULID string
		Cnt         int64
	}
	err := r.db.WithContext(ctx).Model(&domain.ChapterDO{}).
		Select("project_ulid, COUNT(*) AS cnt").
		Where("project_ulid IN ?", projectULIDs).
		Group("project_ulid").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("count chapters: %w", err)
	}
	for _, row := range rows {
		counts[row.ProjectULID] = row.Cnt
	}
	return counts, nil
}

// AdjustTotalWordCount 以表达式增减项目累计字数（防并发覆盖）。
func (r *ProjectRepo) AdjustTotalWordCount(ctx context.Context, projectULID string, delta int64) error {
	err := r.db.WithContext(ctx).Model(&domain.ProjectDO{}).
		Where("project_ulid = ?", projectULID).
		Update("total_word_count", gorm.Expr("total_word_count + ?", delta)).Error
	if err != nil {
		return fmt.Errorf("adjust project word count: %w", err)
	}
	return nil
}

// UpsertWordDelta 累加某日字数统计（存在则累加 word_delta，不存在则新建）。
func (r *ProjectRepo) UpsertWordDelta(ctx context.Context, projectULID, statDate string, delta int64) error {
	db := r.db.WithContext(ctx)
	var stat domain.WordCountStatDO
	err := db.Where("project_ulid = ? AND stat_date = ?", projectULID, statDate).First(&stat).Error
	switch {
	case err == nil:
		if err := db.Model(&domain.WordCountStatDO{}).
			Where("word_stat_ulid = ?", stat.WordStatULID).
			Update("word_delta", gorm.Expr("word_delta + ?", delta)).Error; err != nil {
			return fmt.Errorf("update word stat: %w", err)
		}
	case err == gorm.ErrRecordNotFound:
		do := &domain.WordCountStatDO{
			WordStatULID: pkg.NewULID(domain.IDPrefixWordStat),
			ProjectULID:  projectULID,
			StatDate:     statDate,
			WordDelta:    delta,
		}
		if err := db.Create(do).Error; err != nil {
			return fmt.Errorf("create word stat: %w", err)
		}
	default:
		return fmt.Errorf("get word stat: %w", err)
	}
	return nil
}

// DeleteWordStatsByProject 删除项目的全部每日字数统计。
func (r *ProjectRepo) DeleteWordStatsByProject(ctx context.Context, projectULID string) error {
	if err := r.db.WithContext(ctx).
		Where("project_ulid = ?", projectULID).Delete(&domain.WordCountStatDO{}).Error; err != nil {
		return fmt.Errorf("delete word stats: %w", err)
	}
	return nil
}

// ListByULIDList 批量按 ULID 查询项目（工作流列表回填项目名，防 N+1）。
func (r *ProjectRepo) ListByULIDList(ctx context.Context, projectULIDList []string) ([]domain.ProjectDO, error) {
	if len(projectULIDList) == 0 {
		return []domain.ProjectDO{}, nil
	}
	var dos []domain.ProjectDO
	if err := r.db.WithContext(ctx).Where("project_ulid IN ?", projectULIDList).Find(&dos).Error; err != nil {
		return nil, fmt.Errorf("list projects by ulid list: %w", err)
	}
	return dos, nil
}

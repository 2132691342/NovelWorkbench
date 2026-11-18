package repo

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// ChapterRepo 章节与章节快照持久化仓储。
type ChapterRepo struct {
	db *gorm.DB
}

// NewChapterRepo 构造章节仓储。
func NewChapterRepo(db *gorm.DB) *ChapterRepo {
	return &ChapterRepo{db: db}
}

// Create 插入章节记录。
func (r *ChapterRepo) Create(ctx context.Context, do *domain.ChapterDO) error {
	if err := r.db.WithContext(ctx).Create(do).Error; err != nil {
		return fmt.Errorf("create chapter: %w", err)
	}
	return nil
}

// Update 按列名增量更新章节（map 更新防零值丢失）。
func (r *ChapterRepo) Update(ctx context.Context, chapterULID string, fields map[string]any) error {
	result := r.db.WithContext(ctx).Model(&domain.ChapterDO{}).
		Where("chapter_ulid = ?", chapterULID).Updates(fields)
	if result.Error != nil {
		return fmt.Errorf("update chapter: %w", result.Error)
	}
	return nil
}

// Delete 删除章节记录。
func (r *ChapterRepo) Delete(ctx context.Context, chapterULID string) error {
	if err := r.db.WithContext(ctx).
		Where("chapter_ulid = ?", chapterULID).Delete(&domain.ChapterDO{}).Error; err != nil {
		return fmt.Errorf("delete chapter: %w", err)
	}
	return nil
}

// GetByULID 按 ULID 查询章节，不存在时返回 (nil, nil)。
func (r *ChapterRepo) GetByULID(ctx context.Context, chapterULID string) (*domain.ChapterDO, error) {
	var do domain.ChapterDO
	err := r.db.WithContext(ctx).Where("chapter_ulid = ?", chapterULID).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get chapter: %w", err)
	}
	return &do, nil
}

// ListByProject 按项目查询章节列表（排序号升序）。
func (r *ChapterRepo) ListByProject(ctx context.Context, projectULID string) ([]domain.ChapterDO, error) {
	var dos []domain.ChapterDO
	err := r.db.WithContext(ctx).Where("project_ulid = ?", projectULID).
		Order("chapter_order ASC").Find(&dos).Error
	if err != nil {
		return nil, fmt.Errorf("list chapters: %w", err)
	}
	return dos, nil
}

// MaxOrder 查询项目内最大章节排序号（无章节时返回 0）。
func (r *ChapterRepo) MaxOrder(ctx context.Context, projectULID string) (int, error) {
	var maxOrder int
	err := r.db.WithContext(ctx).Model(&domain.ChapterDO{}).
		Where("project_ulid = ?", projectULID).
		Select("COALESCE(MAX(chapter_order), 0)").Scan(&maxOrder).Error
	if err != nil {
		return 0, fmt.Errorf("max chapter order: %w", err)
	}
	return maxOrder, nil
}

// SumWordCountByVolume 统计卷内章节字数合计（卷删除时回补项目字数）。
func (r *ChapterRepo) SumWordCountByVolume(ctx context.Context, volumeULID string) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&domain.ChapterDO{}).
		Where("volume_ulid = ?", volumeULID).
		Select("COALESCE(SUM(word_count), 0)").Scan(&total).Error
	if err != nil {
		return 0, fmt.Errorf("sum chapter word count by volume: %w", err)
	}
	return total, nil
}

// DeleteByVolume 删除卷下全部章节。
func (r *ChapterRepo) DeleteByVolume(ctx context.Context, volumeULID string) error {
	if err := r.db.WithContext(ctx).
		Where("volume_ulid = ?", volumeULID).Delete(&domain.ChapterDO{}).Error; err != nil {
		return fmt.Errorf("delete chapters by volume: %w", err)
	}
	return nil
}

// DeleteSnapshotsByVolume 删除卷下全部章节快照（经章节子查询关联）。
func (r *ChapterRepo) DeleteSnapshotsByVolume(ctx context.Context, volumeULID string) error {
	subQuery := r.db.WithContext(ctx).Model(&domain.ChapterDO{}).
		Select("chapter_ulid").Where("volume_ulid = ?", volumeULID)
	if err := r.db.WithContext(ctx).
		Where("chapter_ulid IN (?)", subQuery).Delete(&domain.ChapterSnapshotDO{}).Error; err != nil {
		return fmt.Errorf("delete chapter snapshots by volume: %w", err)
	}
	return nil
}

// DeleteByProject 删除项目下全部章节。
func (r *ChapterRepo) DeleteByProject(ctx context.Context, projectULID string) error {
	if err := r.db.WithContext(ctx).
		Where("project_ulid = ?", projectULID).Delete(&domain.ChapterDO{}).Error; err != nil {
		return fmt.Errorf("delete chapters by project: %w", err)
	}
	return nil
}

// CreateSnapshot 插入章节快照。
func (r *ChapterRepo) CreateSnapshot(ctx context.Context, do *domain.ChapterSnapshotDO) error {
	if err := r.db.WithContext(ctx).Create(do).Error; err != nil {
		return fmt.Errorf("create chapter snapshot: %w", err)
	}
	return nil
}

// GetSnapshotByULID 按 ULID 查询快照，不存在时返回 (nil, nil)。
func (r *ChapterRepo) GetSnapshotByULID(ctx context.Context, snapshotULID string) (*domain.ChapterSnapshotDO, error) {
	var do domain.ChapterSnapshotDO
	err := r.db.WithContext(ctx).Where("snapshot_ulid = ?", snapshotULID).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get chapter snapshot: %w", err)
	}
	return &do, nil
}

// ListSnapshotsByChapter 按章节查询快照列表（生成时间倒序）。
func (r *ChapterRepo) ListSnapshotsByChapter(ctx context.Context, chapterULID string) ([]domain.ChapterSnapshotDO, error) {
	var dos []domain.ChapterSnapshotDO
	err := r.db.WithContext(ctx).Where("chapter_ulid = ?", chapterULID).
		Order("created_at DESC").Find(&dos).Error
	if err != nil {
		return nil, fmt.Errorf("list chapter snapshots: %w", err)
	}
	return dos, nil
}

// DeleteSnapshotsByChapter 删除章节下全部快照。
func (r *ChapterRepo) DeleteSnapshotsByChapter(ctx context.Context, chapterULID string) error {
	if err := r.db.WithContext(ctx).
		Where("chapter_ulid = ?", chapterULID).Delete(&domain.ChapterSnapshotDO{}).Error; err != nil {
		return fmt.Errorf("delete chapter snapshots: %w", err)
	}
	return nil
}

// DeleteSnapshotsByProject 删除项目下全部章节快照（经章节子查询关联）。
func (r *ChapterRepo) DeleteSnapshotsByProject(ctx context.Context, projectULID string) error {
	subQuery := r.db.WithContext(ctx).Model(&domain.ChapterDO{}).
		Select("chapter_ulid").Where("project_ulid = ?", projectULID)
	if err := r.db.WithContext(ctx).
		Where("chapter_ulid IN (?)", subQuery).Delete(&domain.ChapterSnapshotDO{}).Error; err != nil {
		return fmt.Errorf("delete chapter snapshots by project: %w", err)
	}
	return nil
}

// 本文件实现大纲节点与场景卡的持久化仓储。

package repo

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// OutlineRepo 创作规划仓储：大纲节点树与详细场景卡片持久化。
// 所有查询均以 project_ulid 限定，避免越项目访问。
type OutlineRepo struct {
	db *gorm.DB // 事务入口
}

// NewOutlineRepo 构造创作规划仓储。
func NewOutlineRepo(db *gorm.DB) *OutlineRepo {
	return &OutlineRepo{db: db}
}

// CreateNode 插入大纲节点。
func (r *OutlineRepo) CreateNode(ctx context.Context, do *domain.OutlineNodeDO) error {
	if err := r.db.WithContext(ctx).Create(do).Error; err != nil {
		return fmt.Errorf("create outline node: %w", err)
	}
	return nil
}

// UpdateNode 按节点 ULID 与项目 ULID 增量更新大纲节点（map 更新保留零值字段）。
func (r *OutlineRepo) UpdateNode(ctx context.Context, projectULID, outlineULID string, fields map[string]any) error {
	result := r.db.WithContext(ctx).Model(&domain.OutlineNodeDO{}).
		Where("project_ulid = ? AND outline_ulid = ?", projectULID, outlineULID).Updates(fields)
	if result.Error != nil {
		return fmt.Errorf("update outline node: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteNode 按节点 ULID 与项目 ULID 删除大纲节点。
func (r *OutlineRepo) DeleteNode(ctx context.Context, projectULID, outlineULID string) error {
	if err := r.db.WithContext(ctx).
		Where("project_ulid = ? AND outline_ulid = ?", projectULID, outlineULID).
		Delete(&domain.OutlineNodeDO{}).Error; err != nil {
		return fmt.Errorf("delete outline node: %w", err)
	}
	return nil
}

// GetNodeByULID 按项目与节点 ULID 查询大纲节点，不存在时返回 (nil, nil)。
func (r *OutlineRepo) GetNodeByULID(ctx context.Context, projectULID, outlineULID string) (*domain.OutlineNodeDO, error) {
	var do domain.OutlineNodeDO
	err := r.db.WithContext(ctx).
		Where("project_ulid = ? AND outline_ulid = ?", projectULID, outlineULID).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get outline node: %w", err)
	}
	return &do, nil
}

// ListNodesByProject 查询项目下全部大纲节点（排序号升序）。
func (r *OutlineRepo) ListNodesByProject(ctx context.Context, projectULID string) ([]domain.OutlineNodeDO, error) {
	var dos []domain.OutlineNodeDO
	err := r.db.WithContext(ctx).Where("project_ulid = ?", projectULID).
		Order("sort_order ASC, outline_ulid ASC").Find(&dos).Error
	if err != nil {
		return nil, fmt.Errorf("list outline nodes: %w", err)
	}
	return dos, nil
}

// CountNodesByParent 统计项目内指定父节点下的子节点数量（空串表示根节点）。
func (r *OutlineRepo) CountNodesByParent(ctx context.Context, projectULID, parentOutlineULID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.OutlineNodeDO{}).
		Where("project_ulid = ? AND parent_outline_ulid = ?", projectULID, parentOutlineULID).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("count outline child nodes: %w", err)
	}
	return count, nil
}

// MaxNodeOrder 查询项目内指定父节点下的最大排序号（无节点时返回 0）。
func (r *OutlineRepo) MaxNodeOrder(ctx context.Context, projectULID, parentOutlineULID string) (int, error) {
	var maxOrder int
	err := r.db.WithContext(ctx).Model(&domain.OutlineNodeDO{}).
		Where("project_ulid = ? AND parent_outline_ulid = ?", projectULID, parentOutlineULID).
		Select("COALESCE(MAX(sort_order), 0)").Scan(&maxOrder).Error
	if err != nil {
		return 0, fmt.Errorf("max outline node order: %w", err)
	}
	return maxOrder, nil
}

// CreateScene 插入详细场景卡片。
func (r *OutlineRepo) CreateScene(ctx context.Context, do *domain.DetailedSceneDO) error {
	if err := r.db.WithContext(ctx).Create(do).Error; err != nil {
		return fmt.Errorf("create detailed scene: %w", err)
	}
	return nil
}

// UpdateScene 按场景 ULID 与项目 ULID 增量更新场景卡片。
func (r *OutlineRepo) UpdateScene(ctx context.Context, projectULID, sceneULID string, fields map[string]any) error {
	result := r.db.WithContext(ctx).Model(&domain.DetailedSceneDO{}).
		Where("project_ulid = ? AND scene_ulid = ?", projectULID, sceneULID).Updates(fields)
	if result.Error != nil {
		return fmt.Errorf("update detailed scene: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteScene 按场景 ULID 与项目 ULID 删除场景卡片。
func (r *OutlineRepo) DeleteScene(ctx context.Context, projectULID, sceneULID string) error {
	if err := r.db.WithContext(ctx).
		Where("project_ulid = ? AND scene_ulid = ?", projectULID, sceneULID).
		Delete(&domain.DetailedSceneDO{}).Error; err != nil {
		return fmt.Errorf("delete detailed scene: %w", err)
	}
	return nil
}

// GetSceneByULID 按项目与场景 ULID 查询场景卡片，不存在时返回 (nil, nil)。
func (r *OutlineRepo) GetSceneByULID(ctx context.Context, projectULID, sceneULID string) (*domain.DetailedSceneDO, error) {
	var do domain.DetailedSceneDO
	err := r.db.WithContext(ctx).
		Where("project_ulid = ? AND scene_ulid = ?", projectULID, sceneULID).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get detailed scene: %w", err)
	}
	return &do, nil
}

// ListScenes 查询项目下场景列表；outlineULID 非空时按节点过滤，排序号升序。
func (r *OutlineRepo) ListScenes(ctx context.Context, projectULID, outlineULID string) ([]domain.DetailedSceneDO, error) {
	query := r.db.WithContext(ctx).Where("project_ulid = ?", projectULID)
	if outlineULID != "" {
		query = query.Where("outline_ulid = ?", outlineULID)
	}
	var dos []domain.DetailedSceneDO
	err := query.Order("sort_order ASC, scene_ulid ASC").Find(&dos).Error
	if err != nil {
		return nil, fmt.Errorf("list detailed scenes: %w", err)
	}
	return dos, nil
}

// MaxSceneOrder 查询项目内指定节点下的最大场景排序号（无场景时返回 0）。
func (r *OutlineRepo) MaxSceneOrder(ctx context.Context, projectULID, outlineULID string) (int, error) {
	var maxOrder int
	err := r.db.WithContext(ctx).Model(&domain.DetailedSceneDO{}).
		Where("project_ulid = ? AND outline_ulid = ?", projectULID, outlineULID).
		Select("COALESCE(MAX(sort_order), 0)").Scan(&maxOrder).Error
	if err != nil {
		return 0, fmt.Errorf("max detailed scene order: %w", err)
	}
	return maxOrder, nil
}

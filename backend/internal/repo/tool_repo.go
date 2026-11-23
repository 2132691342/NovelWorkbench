// 本文件实现工具持久化仓储。

package repo

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// ToolRepo 工具持久化仓储。
type ToolRepo struct {
	db *gorm.DB // 事务入口
}

// NewToolRepo 构造工具仓储。
func NewToolRepo(db *gorm.DB) *ToolRepo {
	return &ToolRepo{db: db}
}

// Create 插入工具记录。
func (r *ToolRepo) Create(ctx context.Context, do *domain.ToolDO) error {
	if err := r.db.WithContext(ctx).Create(do).Error; err != nil {
		return fmt.Errorf("create tool: %w", err)
	}
	return nil
}

// Update 按列名增量更新工具（map 更新防零值丢失）。
func (r *ToolRepo) Update(ctx context.Context, toolULID string, fields map[string]any) error {
	result := r.db.WithContext(ctx).Model(&domain.ToolDO{}).
		Where("tool_ulid = ?", toolULID).Updates(fields)
	if result.Error != nil {
		return fmt.Errorf("update tool: %w", result.Error)
	}
	return nil
}

// Delete 删除工具记录。
func (r *ToolRepo) Delete(ctx context.Context, toolULID string) error {
	if err := r.db.WithContext(ctx).
		Where("tool_ulid = ?", toolULID).Delete(&domain.ToolDO{}).Error; err != nil {
		return fmt.Errorf("delete tool: %w", err)
	}
	return nil
}

// GetByULID 按 ULID 查询工具，不存在时返回 (nil, nil)。
func (r *ToolRepo) GetByULID(ctx context.Context, toolULID string) (*domain.ToolDO, error) {
	var do domain.ToolDO
	err := r.db.WithContext(ctx).Where("tool_ulid = ?", toolULID).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get tool: %w", err)
	}
	return &do, nil
}

// GetByCode 按编码查询工具，不存在时返回 (nil, nil)。
func (r *ToolRepo) GetByCode(ctx context.Context, toolCode string) (*domain.ToolDO, error) {
	var do domain.ToolDO
	err := r.db.WithContext(ctx).Where("tool_code = ?", toolCode).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get tool by code: %w", err)
	}
	return &do, nil
}

// List 按条件查询工具（管理端与对话期自动触发共用）。
func (r *ToolRepo) List(ctx context.Context, req *domain.ToolListREQ) ([]domain.ToolDO, error) {
	query := r.db.WithContext(ctx).Model(&domain.ToolDO{})
	if req.EnabledOnly {
		query = query.Where("enabled_flag = ?", true)
	}
	if keyword := strings.TrimSpace(req.KeywordText); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("tool_name ILIKE ? OR tool_code ILIKE ?", like, like)
	}
	var dos []domain.ToolDO
	if err := query.Order("sort_order ASC, created_at ASC").Find(&dos).Error; err != nil {
		return nil, fmt.Errorf("list tools: %w", err)
	}
	return dos, nil
}

// ListBuiltin 查询全部内置工具（播种校验用）。
func (r *ToolRepo) ListBuiltin(ctx context.Context) ([]domain.ToolDO, error) {
	var dos []domain.ToolDO
	if err := r.db.WithContext(ctx).Where("builtin_flag = ?", true).Find(&dos).Error; err != nil {
		return nil, fmt.Errorf("list builtin tools: %w", err)
	}
	return dos, nil
}

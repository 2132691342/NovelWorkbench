// 本文件实现菜单（权限登记处）的持久化仓储。

package repo

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// MenuRepo 菜单持久化仓储。
type MenuRepo struct {
	db *gorm.DB // 事务入口
}

// NewMenuRepo 构造菜单仓储。
func NewMenuRepo(db *gorm.DB) *MenuRepo {
	return &MenuRepo{db: db}
}

// Create 插入菜单记录。
func (r *MenuRepo) Create(ctx context.Context, do *domain.MenuDO) error {
	if err := r.db.WithContext(ctx).Create(do).Error; err != nil {
		return fmt.Errorf("create menu: %w", err)
	}
	return nil
}

// Update 按列名增量更新菜单（map 更新防零值丢失）。
func (r *MenuRepo) Update(ctx context.Context, menuULID string, fields map[string]any) error {
	result := r.db.WithContext(ctx).Model(&domain.MenuDO{}).
		Where("menu_ulid = ?", menuULID).Updates(fields)
	if result.Error != nil {
		return fmt.Errorf("update menu: %w", result.Error)
	}
	return nil
}

// Delete 按 ULID 删除菜单。
func (r *MenuRepo) Delete(ctx context.Context, menuULID string) error {
	if err := r.db.WithContext(ctx).
		Where("menu_ulid = ?", menuULID).Delete(&domain.MenuDO{}).Error; err != nil {
		return fmt.Errorf("delete menu: %w", err)
	}
	return nil
}

// GetByULID 按 ULID 查询菜单，不存在时返回 (nil, nil)。
func (r *MenuRepo) GetByULID(ctx context.Context, menuULID string) (*domain.MenuDO, error) {
	var do domain.MenuDO
	err := r.db.WithContext(ctx).Where("menu_ulid = ?", menuULID).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get menu: %w", err)
	}
	return &do, nil
}

// GetByCode 按菜单编码查询，不存在时返回 (nil, nil)。
func (r *MenuRepo) GetByCode(ctx context.Context, menuCode string) (*domain.MenuDO, error) {
	var do domain.MenuDO
	err := r.db.WithContext(ctx).Where("menu_code = ?", menuCode).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get menu by code: %w", err)
	}
	return &do, nil
}

// ListAll 查询全部菜单（按同级排序号升序）。
func (r *MenuRepo) ListAll(ctx context.Context) ([]domain.MenuDO, error) {
	var dos []domain.MenuDO
	if err := r.db.WithContext(ctx).Order("sort_order ASC, created_at ASC").Find(&dos).Error; err != nil {
		return nil, fmt.Errorf("list menus: %w", err)
	}
	return dos, nil
}

// ListEnabled 查询启用中的菜单（导航树构建用）。
func (r *MenuRepo) ListEnabled(ctx context.Context) ([]domain.MenuDO, error) {
	var dos []domain.MenuDO
	if err := r.db.WithContext(ctx).Where("enabled_flag = ?", true).
		Order("sort_order ASC, created_at ASC").Find(&dos).Error; err != nil {
		return nil, fmt.Errorf("list enabled menus: %w", err)
	}
	return dos, nil
}

// List 按条件查询菜单（管理端）。
func (r *MenuRepo) List(ctx context.Context, req *domain.MenuListREQ) ([]domain.MenuDO, error) {
	query := r.db.WithContext(ctx).Model(&domain.MenuDO{})
	if req.MenuTypeCode != "" {
		query = query.Where("menu_type_code = ?", req.MenuTypeCode)
	}
	if req.ScopeCode != "" {
		query = query.Where("scope_code = ?", req.ScopeCode)
	}
	if keyword := strings.TrimSpace(req.KeywordText); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("menu_name ILIKE ? OR menu_code ILIKE ? OR perm_code ILIKE ?", like, like, like)
	}
	var dos []domain.MenuDO
	if err := query.Order("sort_order ASC, created_at ASC").Find(&dos).Error; err != nil {
		return nil, fmt.Errorf("list menus by filter: %w", err)
	}
	return dos, nil
}

// CountChildren 统计直接下级数量（删除前校验）。
func (r *MenuRepo) CountChildren(ctx context.Context, menuULID string) (int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&domain.MenuDO{}).
		Where("parent_ulid = ?", menuULID).Count(&total).Error; err != nil {
		return 0, fmt.Errorf("count menu children: %w", err)
	}
	return total, nil
}

// CountByPermCode 统计使用某权限码的菜单行数（权限码唯一性校验）。
func (r *MenuRepo) CountByPermCode(ctx context.Context, permCode, excludeMenuULID string) (int64, error) {
	var total int64
	query := r.db.WithContext(ctx).Model(&domain.MenuDO{}).Where("perm_code = ?", permCode)
	if excludeMenuULID != "" {
		query = query.Where("menu_ulid <> ?", excludeMenuULID)
	}
	if err := query.Count(&total).Error; err != nil {
		return 0, fmt.Errorf("count menus by perm code: %w", err)
	}
	return total, nil
}

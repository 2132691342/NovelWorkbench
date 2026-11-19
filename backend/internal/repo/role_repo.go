// 本文件实现角色与用户角色关联的持久化仓储。

package repo

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
)

// RoleRepo 角色与用户-角色关联持久化仓储。
type RoleRepo struct {
	db *gorm.DB // 事务入口
}

// NewRoleRepo 构造角色仓储。
func NewRoleRepo(db *gorm.DB) *RoleRepo {
	return &RoleRepo{db: db}
}

// Create 插入角色记录。
func (r *RoleRepo) Create(ctx context.Context, do *domain.RoleDO) error {
	if err := r.db.WithContext(ctx).Create(do).Error; err != nil {
		return fmt.Errorf("create role: %w", err)
	}
	return nil
}

// GetByULID 按 ULID 查询角色，不存在时返回 (nil, nil)。
func (r *RoleRepo) GetByULID(ctx context.Context, roleULID string) (*domain.RoleDO, error) {
	var do domain.RoleDO
	err := r.db.WithContext(ctx).Where("role_ulid = ?", roleULID).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get role by ulid: %w", err)
	}
	return &do, nil
}

// GetByCode 按角色编码查询角色，不存在时返回 (nil, nil)。
func (r *RoleRepo) GetByCode(ctx context.Context, roleCode string) (*domain.RoleDO, error) {
	var do domain.RoleDO
	err := r.db.WithContext(ctx).Where("role_code = ?", roleCode).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get role by code: %w", err)
	}
	return &do, nil
}

// ListPage 分页查询角色（按创建时间倒序），返回列表与总条数。
func (r *RoleRepo) ListPage(ctx context.Context, pageNum int, pageSize int) ([]domain.RoleDO, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&domain.RoleDO{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count roles: %w", err)
	}
	var dos []domain.RoleDO
	if err := r.db.WithContext(ctx).
		Order("created_at DESC").Offset((pageNum - 1) * pageSize).Limit(pageSize).
		Find(&dos).Error; err != nil {
		return nil, 0, fmt.Errorf("list roles: %w", err)
	}
	return dos, total, nil
}

// ListAll 查询全部角色（播种/校验用）。
func (r *RoleRepo) ListAll(ctx context.Context) ([]domain.RoleDO, error) {
	var dos []domain.RoleDO
	if err := r.db.WithContext(ctx).Order("role_code ASC").Find(&dos).Error; err != nil {
		return nil, fmt.Errorf("list all roles: %w", err)
	}
	return dos, nil
}

// Update 更新角色（整体覆盖可变字段）。
func (r *RoleRepo) Update(ctx context.Context, do *domain.RoleDO) error {
	updates := map[string]any{
		"role_code":       do.RoleCode,
		"role_name":       do.RoleName,
		"remark_text":     do.RemarkText,
		"perm_codes_json": do.PermCodesJSON,
	}
	if err := r.db.WithContext(ctx).Model(&domain.RoleDO{}).
		Where("role_ulid = ?", do.RoleULID).Updates(updates).Error; err != nil {
		return fmt.Errorf("update role: %w", err)
	}
	return nil
}

// Delete 按 ULID 删除角色。
func (r *RoleRepo) Delete(ctx context.Context, roleULID string) error {
	if err := r.db.WithContext(ctx).Where("role_ulid = ?", roleULID).Delete(&domain.RoleDO{}).Error; err != nil {
		return fmt.Errorf("delete role: %w", err)
	}
	return nil
}

// ListRoleCodesByUserULID 查询用户的角色编码列表（join user_roles）。
func (r *RoleRepo) ListRoleCodesByUserULID(ctx context.Context, userULID string) ([]string, error) {
	var roleCodes []string
	err := r.db.WithContext(ctx).Model(&domain.RoleDO{}).
		Joins("JOIN user_roles ON user_roles.role_ulid = roles.role_ulid").
		Where("user_roles.user_ulid = ?", userULID).
		Pluck("roles.role_code", &roleCodes).Error
	if err != nil {
		return nil, fmt.Errorf("list role codes by user: %w", err)
	}
	return roleCodes, nil
}

// ListRolesByUserULID 查询用户的角色记录列表（权限判定用）。
func (r *RoleRepo) ListRolesByUserULID(ctx context.Context, userULID string) ([]domain.RoleDO, error) {
	var dos []domain.RoleDO
	err := r.db.WithContext(ctx).
		Joins("JOIN user_roles ON user_roles.role_ulid = roles.role_ulid").
		Where("user_roles.user_ulid = ?", userULID).Find(&dos).Error
	if err != nil {
		return nil, fmt.Errorf("list roles by user: %w", err)
	}
	return dos, nil
}

// ReplaceUserRoles 整体覆盖用户的角色关联（先删后插）。
func (r *RoleRepo) ReplaceUserRoles(ctx context.Context, userULID string, roleULIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_ulid = ?", userULID).Delete(&domain.UserRoleDO{}).Error; err != nil {
			return fmt.Errorf("clear user roles: %w", err)
		}
		for _, roleULID := range roleULIDs {
			link := &domain.UserRoleDO{
				UserRoleULID: pkg.NewULID(domain.IDPrefixUserRole),
				UserULID:     userULID,
				RoleULID:     roleULID,
			}
			if err := tx.Create(link).Error; err != nil {
				return fmt.Errorf("insert user role: %w", err)
			}
		}
		return nil
	})
}

// DeleteUserRoles 按用户删除全部角色关联。
func (r *RoleRepo) DeleteUserRoles(ctx context.Context, userULID string) error {
	if err := r.db.WithContext(ctx).Where("user_ulid = ?", userULID).Delete(&domain.UserRoleDO{}).Error; err != nil {
		return fmt.Errorf("delete user roles: %w", err)
	}
	return nil
}

// CountUsersByRoleULID 统计角色下的用户数（删除前校验）。
func (r *RoleRepo) CountUsersByRoleULID(ctx context.Context, roleULID string) (int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&domain.UserRoleDO{}).
		Where("role_ulid = ?", roleULID).Count(&total).Error; err != nil {
		return 0, fmt.Errorf("count users by role: %w", err)
	}
	return total, nil
}

// ListLinksByUserULIDs 批量查询用户-角色关联（用户列表补角色，防 N+1）。
func (r *RoleRepo) ListLinksByUserULIDs(ctx context.Context, userULIDs []string) ([]domain.UserRoleDO, error) {
	var links []domain.UserRoleDO
	if err := r.db.WithContext(ctx).
		Where("user_ulid IN ?", userULIDs).Find(&links).Error; err != nil {
		return nil, fmt.Errorf("list user role links: %w", err)
	}
	return links, nil
}

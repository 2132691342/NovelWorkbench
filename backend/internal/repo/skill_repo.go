// 本文件实现技能与技能参考资料的持久化仓储。

package repo

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// SkillRepo 技能与技能参考资料持久化仓储。
type SkillRepo struct {
	db *gorm.DB // 事务入口
}

// NewSkillRepo 构造技能仓储。
func NewSkillRepo(db *gorm.DB) *SkillRepo {
	return &SkillRepo{db: db}
}

// Create 插入技能记录。
func (r *SkillRepo) Create(ctx context.Context, do *domain.SkillDO) error {
	if err := r.db.WithContext(ctx).Create(do).Error; err != nil {
		return fmt.Errorf("create skill: %w", err)
	}
	return nil
}

// Update 按列名增量更新技能（map 更新防零值丢失）。
func (r *SkillRepo) Update(ctx context.Context, skillULID string, fields map[string]any) error {
	result := r.db.WithContext(ctx).Model(&domain.SkillDO{}).
		Where("skill_ulid = ?", skillULID).Updates(fields)
	if result.Error != nil {
		return fmt.Errorf("update skill: %w", result.Error)
	}
	return nil
}

// Delete 删除技能记录。
func (r *SkillRepo) Delete(ctx context.Context, skillULID string) error {
	if err := r.db.WithContext(ctx).
		Where("skill_ulid = ?", skillULID).Delete(&domain.SkillDO{}).Error; err != nil {
		return fmt.Errorf("delete skill: %w", err)
	}
	return nil
}

// GetByULID 按 ULID 查询技能，不存在时返回 (nil, nil)。
func (r *SkillRepo) GetByULID(ctx context.Context, skillULID string) (*domain.SkillDO, error) {
	var do domain.SkillDO
	err := r.db.WithContext(ctx).Where("skill_ulid = ?", skillULID).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get skill: %w", err)
	}
	return &do, nil
}

// GetByCode 按 (作用域, 项目, 编码) 查询技能，不存在时返回 (nil, nil)。
func (r *SkillRepo) GetByCode(ctx context.Context, scopeCode, projectULID, skillCode string) (*domain.SkillDO, error) {
	var do domain.SkillDO
	err := r.db.WithContext(ctx).
		Where("scope_code = ? AND project_ulid = ? AND skill_code = ?", scopeCode, projectULID, skillCode).
		First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get skill by code: %w", err)
	}
	return &do, nil
}

// List 按条件查询技能：项目不为空时返回「该项目技能 + 全局技能」。
func (r *SkillRepo) List(ctx context.Context, req *domain.SkillListREQ) ([]domain.SkillDO, error) {
	query := r.db.WithContext(ctx).Model(&domain.SkillDO{})
	switch {
	case req.ProjectULID != "":
		query = query.Where("(scope_code = ? AND project_ulid = ?) OR scope_code = ?",
			domain.SkillScopeProject, req.ProjectULID, domain.SkillScopeGlobal)
	case req.ScopeCode != "":
		query = query.Where("scope_code = ?", req.ScopeCode)
	}
	if req.CategoryCode != "" {
		query = query.Where("category_code = ?", req.CategoryCode)
	}
	if req.EnabledOnly {
		query = query.Where("enabled_flag = ?", true)
	}
	if keyword := strings.TrimSpace(req.KeywordText); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("skill_name ILIKE ? OR skill_code ILIKE ?", like, like)
	}
	var dos []domain.SkillDO
	if err := query.Order("scope_code DESC, sort_order ASC, created_at ASC").Find(&dos).Error; err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}
	return dos, nil
}

// ListBuiltinGlobal 查询全部内置全局技能（播种校验用）。
func (r *SkillRepo) ListBuiltinGlobal(ctx context.Context) ([]domain.SkillDO, error) {
	var dos []domain.SkillDO
	if err := r.db.WithContext(ctx).
		Where("scope_code = ? AND builtin_flag = ?", domain.SkillScopeGlobal, true).
		Find(&dos).Error; err != nil {
		return nil, fmt.Errorf("list builtin skills: %w", err)
	}
	return dos, nil
}

// CountReferences 统计技能参考资料条数，返回 技能ULID → 条数。
func (r *SkillRepo) CountReferences(ctx context.Context, skillULIDs []string) (map[string]int, error) {
	result := make(map[string]int, len(skillULIDs))
	if len(skillULIDs) == 0 {
		return result, nil
	}
	type countRow struct {
		SkillULID string
		Total     int
	}
	var rows []countRow
	if err := r.db.WithContext(ctx).Model(&domain.SkillReferenceDO{}).
		Select("skill_ulid, count(*) AS total").
		Where("skill_ulid IN ?", skillULIDs).
		Group("skill_ulid").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("count skill references: %w", err)
	}
	for _, row := range rows {
		result[row.SkillULID] = row.Total
	}
	return result, nil
}

// CreateReference 插入技能参考资料。
func (r *SkillRepo) CreateReference(ctx context.Context, do *domain.SkillReferenceDO) error {
	if err := r.db.WithContext(ctx).Create(do).Error; err != nil {
		return fmt.Errorf("create skill reference: %w", err)
	}
	return nil
}

// UpdateReference 按列名增量更新参考资料。
func (r *SkillRepo) UpdateReference(ctx context.Context, refULID string, fields map[string]any) error {
	result := r.db.WithContext(ctx).Model(&domain.SkillReferenceDO{}).
		Where("ref_ulid = ?", refULID).Updates(fields)
	if result.Error != nil {
		return fmt.Errorf("update skill reference: %w", result.Error)
	}
	return nil
}

// DeleteReference 删除参考资料。
func (r *SkillRepo) DeleteReference(ctx context.Context, refULID string) error {
	if err := r.db.WithContext(ctx).
		Where("ref_ulid = ?", refULID).Delete(&domain.SkillReferenceDO{}).Error; err != nil {
		return fmt.Errorf("delete skill reference: %w", err)
	}
	return nil
}

// DeleteReferencesBySkill 删除某技能的全部参考资料（技能删除级联）。
func (r *SkillRepo) DeleteReferencesBySkill(ctx context.Context, skillULID string) error {
	if err := r.db.WithContext(ctx).
		Where("skill_ulid = ?", skillULID).Delete(&domain.SkillReferenceDO{}).Error; err != nil {
		return fmt.Errorf("delete skill references by skill: %w", err)
	}
	return nil
}

// GetReference 按 ULID 查询参考资料，不存在时返回 (nil, nil)。
func (r *SkillRepo) GetReference(ctx context.Context, refULID string) (*domain.SkillReferenceDO, error) {
	var do domain.SkillReferenceDO
	err := r.db.WithContext(ctx).Where("ref_ulid = ?", refULID).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get skill reference: %w", err)
	}
	return &do, nil
}

// ListReferences 查询技能的参考资料（按排序号升序）。
func (r *SkillRepo) ListReferences(ctx context.Context, skillULID string) ([]domain.SkillReferenceDO, error) {
	var dos []domain.SkillReferenceDO
	if err := r.db.WithContext(ctx).Where("skill_ulid = ?", skillULID).
		Order("sort_order ASC, created_at ASC").Find(&dos).Error; err != nil {
		return nil, fmt.Errorf("list skill references: %w", err)
	}
	return dos, nil
}

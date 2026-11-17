package repo

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// CharacterRepo 角色卡与角色关系边持久化仓储。
type CharacterRepo struct {
	db *gorm.DB
}

// NewCharacterRepo 构造角色仓储。
func NewCharacterRepo(db *gorm.DB) *CharacterRepo {
	return &CharacterRepo{db: db}
}

// Create 插入角色记录。
func (r *CharacterRepo) Create(ctx context.Context, do *domain.CharacterDO) error {
	if err := r.db.WithContext(ctx).Create(do).Error; err != nil {
		return fmt.Errorf("create character: %w", err)
	}
	return nil
}

// Update 按列名增量更新角色（map 更新防零值丢失）。
func (r *CharacterRepo) Update(ctx context.Context, characterULID string, fields map[string]any) error {
	result := r.db.WithContext(ctx).Model(&domain.CharacterDO{}).
		Where("character_ulid = ?", characterULID).Updates(fields)
	if result.Error != nil {
		return fmt.Errorf("update character: %w", result.Error)
	}
	return nil
}

// Delete 删除角色记录。
func (r *CharacterRepo) Delete(ctx context.Context, characterULID string) error {
	if err := r.db.WithContext(ctx).
		Where("character_ulid = ?", characterULID).Delete(&domain.CharacterDO{}).Error; err != nil {
		return fmt.Errorf("delete character: %w", err)
	}
	return nil
}

// GetByULID 按 ULID 查询角色，不存在时返回 (nil, nil)。
func (r *CharacterRepo) GetByULID(ctx context.Context, characterULID string) (*domain.CharacterDO, error) {
	var do domain.CharacterDO
	err := r.db.WithContext(ctx).Where("character_ulid = ?", characterULID).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get character: %w", err)
	}
	return &do, nil
}

// ListByProject 按项目查询角色（类型过滤 + 姓名/别名模糊，创建时间升序）。
func (r *CharacterRepo) ListByProject(ctx context.Context, projectULID, roleTypeCode, keywordText string) ([]domain.CharacterDO, error) {
	query := r.db.WithContext(ctx).Model(&domain.CharacterDO{}).Where("project_ulid = ?", projectULID)
	if roleTypeCode != "" {
		query = query.Where("role_type_code = ?", roleTypeCode)
	}
	if keywordText != "" {
		pattern := "%" + keywordText + "%"
		query = query.Where("character_name ILIKE ? OR alias_text ILIKE ?", pattern, pattern)
	}
	var dos []domain.CharacterDO
	if err := query.Order("created_at ASC").Find(&dos).Error; err != nil {
		return nil, fmt.Errorf("list characters: %w", err)
	}
	return dos, nil
}

// CreateRelation 插入角色关系边。
func (r *CharacterRepo) CreateRelation(ctx context.Context, do *domain.CharacterRelationDO) error {
	if err := r.db.WithContext(ctx).Create(do).Error; err != nil {
		return fmt.Errorf("create character relation: %w", err)
	}
	return nil
}

// DeleteRelation 删除角色关系边。
func (r *CharacterRepo) DeleteRelation(ctx context.Context, relationULID string) error {
	if err := r.db.WithContext(ctx).
		Where("relation_ulid = ?", relationULID).Delete(&domain.CharacterRelationDO{}).Error; err != nil {
		return fmt.Errorf("delete character relation: %w", err)
	}
	return nil
}

// GetRelationByULID 按 ULID 查询关系边，不存在时返回 (nil, nil)。
func (r *CharacterRepo) GetRelationByULID(ctx context.Context, relationULID string) (*domain.CharacterRelationDO, error) {
	var do domain.CharacterRelationDO
	err := r.db.WithContext(ctx).Where("relation_ulid = ?", relationULID).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get character relation: %w", err)
	}
	return &do, nil
}

// ListRelationsByProject 查询项目的全部角色关系边。
func (r *CharacterRepo) ListRelationsByProject(ctx context.Context, projectULID string) ([]domain.CharacterRelationDO, error) {
	var dos []domain.CharacterRelationDO
	err := r.db.WithContext(ctx).Model(&domain.CharacterRelationDO{}).
		Where("project_ulid = ?", projectULID).Order("created_at ASC").Find(&dos).Error
	if err != nil {
		return nil, fmt.Errorf("list character relations: %w", err)
	}
	return dos, nil
}

// DeleteRelationsByCharacter 删除涉及某角色的全部关系边（起点或终点）。
func (r *CharacterRepo) DeleteRelationsByCharacter(ctx context.Context, characterULID string) error {
	err := r.db.WithContext(ctx).
		Where("from_character_ulid = ? OR to_character_ulid = ?", characterULID, characterULID).
		Delete(&domain.CharacterRelationDO{}).Error
	if err != nil {
		return fmt.Errorf("delete character relations: %w", err)
	}
	return nil
}

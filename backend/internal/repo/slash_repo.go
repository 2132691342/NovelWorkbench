package repo

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// SlashRepo 斜杠命令（技能资产）持久化仓储。
type SlashRepo struct {
	db *gorm.DB
}

// NewSlashRepo 构造斜杠命令仓储。
func NewSlashRepo(db *gorm.DB) *SlashRepo {
	return &SlashRepo{db: db}
}

// Create 插入斜杠命令记录。
func (r *SlashRepo) Create(ctx context.Context, do *domain.SlashCommandDO) error {
	if err := r.db.WithContext(ctx).Create(do).Error; err != nil {
		return fmt.Errorf("create slash command: %w", err)
	}
	return nil
}

// Update 按列名增量更新斜杠命令（map 更新防零值丢失）。
func (r *SlashRepo) Update(ctx context.Context, commandULID string, fields map[string]any) error {
	result := r.db.WithContext(ctx).Model(&domain.SlashCommandDO{}).
		Where("command_ulid = ?", commandULID).Updates(fields)
	if result.Error != nil {
		return fmt.Errorf("update slash command: %w", result.Error)
	}
	return nil
}

// Delete 删除斜杠命令记录。
func (r *SlashRepo) Delete(ctx context.Context, commandULID string) error {
	if err := r.db.WithContext(ctx).
		Where("command_ulid = ?", commandULID).Delete(&domain.SlashCommandDO{}).Error; err != nil {
		return fmt.Errorf("delete slash command: %w", err)
	}
	return nil
}

// GetByULID 按 ULID 查询斜杠命令，不存在时返回 (nil, nil)。
func (r *SlashRepo) GetByULID(ctx context.Context, commandULID string) (*domain.SlashCommandDO, error) {
	var do domain.SlashCommandDO
	err := r.db.WithContext(ctx).Where("command_ulid = ?", commandULID).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get slash command: %w", err)
	}
	return &do, nil
}

// GetByCode 按命令编码查询斜杠命令，不存在时返回 (nil, nil)。
func (r *SlashRepo) GetByCode(ctx context.Context, commandCode string) (*domain.SlashCommandDO, error) {
	var do domain.SlashCommandDO
	err := r.db.WithContext(ctx).Where("command_code = ?", commandCode).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get slash command by code: %w", err)
	}
	return &do, nil
}

// List 查询斜杠命令列表（enabledOnly 为真仅返回启用命令，按排序号升序）。
func (r *SlashRepo) List(ctx context.Context, enabledOnly bool) ([]domain.SlashCommandDO, error) {
	query := r.db.WithContext(ctx).Model(&domain.SlashCommandDO{})
	if enabledOnly {
		query = query.Where("enabled_flag = ?", true)
	}
	var dos []domain.SlashCommandDO
	if err := query.Order("sort_order ASC, created_at ASC").Find(&dos).Error; err != nil {
		return nil, fmt.Errorf("list slash commands: %w", err)
	}
	return dos, nil
}

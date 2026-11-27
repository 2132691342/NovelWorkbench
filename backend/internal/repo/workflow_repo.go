// 本文件实现工作流模板、运行实例与步骤记录的持久化仓储。

package repo

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// WorkflowRepo 工作流持久化仓储：模板、运行实例与步骤时间线。
type WorkflowRepo struct {
	db *gorm.DB // 事务入口
}

// NewWorkflowRepo 构造工作流仓储。
func NewWorkflowRepo(db *gorm.DB) *WorkflowRepo {
	return &WorkflowRepo{db: db}
}

// CreateTemplate 插入模板记录。
func (r *WorkflowRepo) CreateTemplate(ctx context.Context, do *domain.WorkflowTemplateDO) error {
	if err := r.db.WithContext(ctx).Create(do).Error; err != nil {
		return fmt.Errorf("create workflow template: %w", err)
	}
	return nil
}

// UpdateTemplate 按列名增量更新模板（map 更新防零值丢失）。
func (r *WorkflowRepo) UpdateTemplate(ctx context.Context, templateULID string, fields map[string]any) error {
	result := r.db.WithContext(ctx).Model(&domain.WorkflowTemplateDO{}).
		Where("template_ulid = ?", templateULID).Updates(fields)
	if result.Error != nil {
		return fmt.Errorf("update workflow template: %w", result.Error)
	}
	return nil
}

// GetTemplateByCode 按编码查询模板，不存在时返回 (nil, nil)。
func (r *WorkflowRepo) GetTemplateByCode(ctx context.Context, templateCode string) (*domain.WorkflowTemplateDO, error) {
	var do domain.WorkflowTemplateDO
	err := r.db.WithContext(ctx).Where("template_code = ?", templateCode).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get workflow template: %w", err)
	}
	return &do, nil
}

// ListTemplates 查询启用的模板（排序号升序）。
func (r *WorkflowRepo) ListTemplates(ctx context.Context) ([]domain.WorkflowTemplateDO, error) {
	var dos []domain.WorkflowTemplateDO
	if err := r.db.WithContext(ctx).Where("enabled_flag = ?", true).
		Order("sort_order ASC").Find(&dos).Error; err != nil {
		return nil, fmt.Errorf("list workflow templates: %w", err)
	}
	return dos, nil
}

// CreateRun 插入运行实例。
func (r *WorkflowRepo) CreateRun(ctx context.Context, do *domain.WorkflowRunDO) error {
	if err := r.db.WithContext(ctx).Create(do).Error; err != nil {
		return fmt.Errorf("create workflow run: %w", err)
	}
	return nil
}

// UpdateRun 按列名增量更新运行实例（map 更新防零值丢失）。
func (r *WorkflowRepo) UpdateRun(ctx context.Context, runULID string, fields map[string]any) error {
	result := r.db.WithContext(ctx).Model(&domain.WorkflowRunDO{}).
		Where("run_ulid = ?", runULID).Updates(fields)
	if result.Error != nil {
		return fmt.Errorf("update workflow run: %w", result.Error)
	}
	return nil
}

// GetRun 按 ULID 查询运行实例，不存在时返回 (nil, nil)。
func (r *WorkflowRepo) GetRun(ctx context.Context, runULID string) (*domain.WorkflowRunDO, error) {
	var do domain.WorkflowRunDO
	err := r.db.WithContext(ctx).Where("run_ulid = ?", runULID).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get workflow run: %w", err)
	}
	return &do, nil
}

// ListRuns 分页查询运行实例（创建时间倒序）。
func (r *WorkflowRepo) ListRuns(ctx context.Context, req *domain.WorkflowRunListREQ) ([]domain.WorkflowRunDO, int64, error) {
	query := r.db.WithContext(ctx).Model(&domain.WorkflowRunDO{})
	if req.ProjectULID != "" {
		query = query.Where("project_ulid = ?", req.ProjectULID)
	}
	if req.RunStatusCode != "" {
		query = query.Where("run_status_code = ?", req.RunStatusCode)
	}
	if req.KeywordText != "" {
		like := "%" + req.KeywordText + "%"
		query = query.Where("run_ulid LIKE ? OR template_name LIKE ?", like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count workflow runs: %w", err)
	}
	var dos []domain.WorkflowRunDO
	if err := query.Order("created_at DESC").
		Offset((req.PageNum - 1) * req.PageSize).Limit(req.PageSize).
		Find(&dos).Error; err != nil {
		return nil, 0, fmt.Errorf("list workflow runs: %w", err)
	}
	return dos, total, nil
}

// ListRunsByStatus 查询指定状态的运行实例（执行器启动时恢复中断任务）。
func (r *WorkflowRepo) ListRunsByStatus(ctx context.Context, statusCodeList []string) ([]domain.WorkflowRunDO, error) {
	var dos []domain.WorkflowRunDO
	if err := r.db.WithContext(ctx).Where("run_status_code IN ?", statusCodeList).
		Order("created_at ASC").Find(&dos).Error; err != nil {
		return nil, fmt.Errorf("list workflow runs by status: %w", err)
	}
	return dos, nil
}

// ListPendingReviewRuns 查询当前用户待审核的运行（更新时间倒序，Dashboard 收件箱用）。
func (r *WorkflowRepo) ListPendingReviewRuns(ctx context.Context, userULID string, limit int) ([]domain.WorkflowRunDO, error) {
	if limit <= 0 {
		limit = 10
	}
	var dos []domain.WorkflowRunDO
	if err := r.db.WithContext(ctx).
		Where("user_ulid = ? AND run_status_code = ?", userULID, domain.WorkflowRunStatusWaitingReview).
		Order("updated_at DESC").Limit(limit).Find(&dos).Error; err != nil {
		return nil, fmt.Errorf("list pending review workflow runs: %w", err)
	}
	return dos, nil
}

// CountRunsByStatus 统计各状态运行数（工作流概览卡片）。
func (r *WorkflowRepo) CountRunsByStatus(ctx context.Context, projectULID string) (map[string]int64, error) {
	type statusCount struct {
		RunStatusCode string `gorm:"column:run_status_code"`
		Total         int64  `gorm:"column:total"`
	}
	query := r.db.WithContext(ctx).Model(&domain.WorkflowRunDO{})
	if projectULID != "" {
		query = query.Where("project_ulid = ?", projectULID)
	}
	var rows []statusCount
	if err := query.Select("run_status_code, COUNT(*) AS total").
		Group("run_status_code").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("count workflow runs by status: %w", err)
	}
	result := make(map[string]int64, len(rows))
	for i := range rows {
		result[rows[i].RunStatusCode] = rows[i].Total
	}
	return result, nil
}

// DeleteRunsByProject 删除项目下的全部运行与其步骤（项目级联删除用）。
// 步骤表没有外键约束，必须显式先删，否则删完项目会留下指向已删章节的孤儿步骤。
func (r *WorkflowRepo) DeleteRunsByProject(ctx context.Context, projectULID string) error {
	var runULIDList []string
	if err := r.db.WithContext(ctx).Model(&domain.WorkflowRunDO{}).
		Where("project_ulid = ?", projectULID).Pluck("run_ulid", &runULIDList).Error; err != nil {
		return fmt.Errorf("list workflow runs by project: %w", err)
	}
	if len(runULIDList) > 0 {
		if err := r.db.WithContext(ctx).Where("run_ulid IN ?", runULIDList).
			Delete(&domain.WorkflowStepDO{}).Error; err != nil {
			return fmt.Errorf("delete workflow steps by project: %w", err)
		}
	}
	if err := r.db.WithContext(ctx).Where("project_ulid = ?", projectULID).
		Delete(&domain.WorkflowRunDO{}).Error; err != nil {
		return fmt.Errorf("delete workflow runs: %w", err)
	}
	return nil
}

// CreateStep 插入步骤记录。
func (r *WorkflowRepo) CreateStep(ctx context.Context, do *domain.WorkflowStepDO) error {
	if err := r.db.WithContext(ctx).Create(do).Error; err != nil {
		return fmt.Errorf("create workflow step: %w", err)
	}
	return nil
}

// UpdateStep 按列名增量更新步骤记录。
func (r *WorkflowRepo) UpdateStep(ctx context.Context, stepULID string, fields map[string]any) error {
	result := r.db.WithContext(ctx).Model(&domain.WorkflowStepDO{}).
		Where("step_ulid = ?", stepULID).Updates(fields)
	if result.Error != nil {
		return fmt.Errorf("update workflow step: %w", result.Error)
	}
	return nil
}

// FindRunningGateStep 查当前章门禁上那条 running 的门禁行（进入门禁时写入，审核时原地更新）。
// 找不到返回 (nil, nil)：历史数据或异常路径下调用方回退为插入一行。
func (r *WorkflowRepo) FindRunningGateStep(ctx context.Context, runULID, chapterULID string,
	stepIndex int) (*domain.WorkflowStepDO, error) {
	var do domain.WorkflowStepDO
	result := r.db.WithContext(ctx).
		Where("run_ulid = ? AND chapter_ulid = ? AND step_index = ? AND step_type_code = ? AND step_status_code = ?",
			runULID, chapterULID, stepIndex, domain.WorkflowStepTypeHumanReview, domain.WorkflowStepStatusRunning).
		Order("created_at DESC").Limit(1).Find(&do)
	if result.Error != nil {
		return nil, fmt.Errorf("find running workflow gate step: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &do, nil
}

// ListStepsByRun 查询一次运行的全部步骤（按创建顺序，即执行时间线）。
func (r *WorkflowRepo) ListStepsByRun(ctx context.Context, runULID string) ([]domain.WorkflowStepDO, error) {
	var dos []domain.WorkflowStepDO
	if err := r.db.WithContext(ctx).Where("run_ulid = ?", runULID).
		Order("created_at ASC").Find(&dos).Error; err != nil {
		return nil, fmt.Errorf("list workflow steps: %w", err)
	}
	return dos, nil
}

// DeleteStepsByRun 删除一次运行的全部步骤（重跑前清理）。
func (r *WorkflowRepo) DeleteStepsByRun(ctx context.Context, runULID string) error {
	if err := r.db.WithContext(ctx).Where("run_ulid = ?", runULID).
		Delete(&domain.WorkflowStepDO{}).Error; err != nil {
		return fmt.Errorf("delete workflow steps: %w", err)
	}
	return nil
}

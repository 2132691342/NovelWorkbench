package repo

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// ModelUsageRepo 模型用量统计持久化仓储（按日 × 供应商 × 模型 × 调用类型聚合）。
type ModelUsageRepo struct {
	db *gorm.DB
}

// NewModelUsageRepo 构造用量统计仓储。
func NewModelUsageRepo(db *gorm.DB) *ModelUsageRepo {
	return &ModelUsageRepo{db: db}
}

// UpsertIncrement 累加一条统计（唯一维度冲突时累加计数与 token）。
func (r *ModelUsageRepo) UpsertIncrement(ctx context.Context, do *domain.ModelUsageStatDO) error {
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "stat_date"}, {Name: "provider_ulid"}, {Name: "model_name"}, {Name: "call_type_code"},
		},
		DoUpdates: clause.Assignments(map[string]any{
			"call_count":        gorm.Expr("model_usage_stats.call_count + ?", do.CallCount),
			"success_count":     gorm.Expr("model_usage_stats.success_count + ?", do.SuccessCount),
			"fail_count":        gorm.Expr("model_usage_stats.fail_count + ?", do.FailCount),
			"prompt_tokens":     gorm.Expr("model_usage_stats.prompt_tokens + ?", do.PromptTokens),
			"completion_tokens": gorm.Expr("model_usage_stats.completion_tokens + ?", do.CompletionTokens),
			"total_tokens":      gorm.Expr("model_usage_stats.total_tokens + ?", do.TotalTokens),
			"latency_ms_sum":    gorm.Expr("model_usage_stats.latency_ms_sum + ?", do.LatencyMsSum),
			"provider_name":     do.ProviderName,
		}),
	}).Create(do).Error
	if err != nil {
		return fmt.Errorf("upsert model usage stat: %w", err)
	}
	return nil
}

// AggregateDaily 按日聚合区间用量（升序，缺日由 service 补零）。
func (r *ModelUsageRepo) AggregateDaily(ctx context.Context, fromDate, toDate string) ([]domain.ModelUsageDailyVO, error) {
	var rows []domain.ModelUsageDailyVO
	err := r.db.WithContext(ctx).Model(&domain.ModelUsageStatDO{}).
		Select(`stat_date,
			COALESCE(SUM(call_count), 0) AS call_count,
			COALESCE(SUM(fail_count), 0) AS fail_count,
			COALESCE(SUM(total_tokens), 0) AS total_tokens,
			COALESCE(SUM(completion_tokens), 0) AS completion_tokens,
			(COALESCE(SUM(latency_ms_sum), 0) / GREATEST(COALESCE(SUM(call_count), 0), 1))::bigint AS avg_latency_ms`).
		Where("stat_date BETWEEN ? AND ?", fromDate, toDate).
		Group("stat_date").Order("stat_date ASC").Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("aggregate daily usage: %w", err)
	}
	return rows, nil
}

// AggregateByProvider 按供应商聚合区间用量（调用次数倒序）。
func (r *ModelUsageRepo) AggregateByProvider(ctx context.Context, fromDate, toDate string) ([]domain.ModelUsageGroupVO, error) {
	return r.aggregateByColumn(ctx, fromDate, toDate, "provider_name", "provider_name")
}

// AggregateByModel 按模型聚合区间用量（调用次数倒序）。
func (r *ModelUsageRepo) AggregateByModel(ctx context.Context, fromDate, toDate string) ([]domain.ModelUsageGroupVO, error) {
	return r.aggregateByColumn(ctx, fromDate, toDate, "model_name", "model_name")
}

// aggregateByColumn 以指定列分组聚合（column 由调用方固定，不接受外部输入）。
func (r *ModelUsageRepo) aggregateByColumn(ctx context.Context, fromDate, toDate, column, alias string) ([]domain.ModelUsageGroupVO, error) {
	var rows []domain.ModelUsageGroupVO
	err := r.db.WithContext(ctx).Model(&domain.ModelUsageStatDO{}).
		Select(fmt.Sprintf(`%s AS group_name,
			COALESCE(SUM(call_count), 0) AS call_count,
			COALESCE(SUM(fail_count), 0) AS fail_count,
			COALESCE(SUM(total_tokens), 0) AS total_tokens,
			(COALESCE(SUM(latency_ms_sum), 0) / GREATEST(COALESCE(SUM(call_count), 0), 1))::bigint AS avg_latency_ms,
			(COALESCE(SUM(success_count), 0)::float8 / GREATEST(COALESCE(SUM(call_count), 0), 1)) AS success_rate_value`, alias)).
		Where("stat_date BETWEEN ? AND ?", fromDate, toDate).
		Group(column).Order("call_count DESC").Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("aggregate usage by %s: %w", column, err)
	}
	return rows, nil
}

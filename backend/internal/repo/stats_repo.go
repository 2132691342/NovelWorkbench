// 本文件实现数据总览的统计查询。

package repo

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// StatsRepo 数据总览统计查询仓储（word_count_stats 联 projects 做归属过滤）。
type StatsRepo struct {
	db *gorm.DB // 事务入口
}

// NewStatsRepo 构造统计仓储。
func NewStatsRepo(db *gorm.DB) *StatsRepo {
	return &StatsRepo{db: db}
}

// CountProjects 统计用户项目数（projectULID 非空时仅统计该项目）。
func (r *StatsRepo) CountProjects(ctx context.Context, userULID, projectULID string) (int64, error) {
	query := r.db.WithContext(ctx).Model(&domain.ProjectDO{}).Where("user_ulid = ?", userULID)
	if projectULID != "" {
		query = query.Where("project_ulid = ?", projectULID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count projects: %w", err)
	}
	return count, nil
}

// SumTotalWordCount 统计项目累计字数之和（projectULID 非空时仅统计该项目）。
func (r *StatsRepo) SumTotalWordCount(ctx context.Context, userULID, projectULID string) (int64, error) {
	query := r.db.WithContext(ctx).Model(&domain.ProjectDO{}).
		Where("user_ulid = ?", userULID)
	if projectULID != "" {
		query = query.Where("project_ulid = ?", projectULID)
	}
	var total int64
	if err := query.Select("COALESCE(SUM(total_word_count), 0)").Scan(&total).Error; err != nil {
		return 0, fmt.Errorf("sum total word count: %w", err)
	}
	return total, nil
}

// SumWordDeltaByDate 统计某日人工净增字数之和。
func (r *StatsRepo) SumWordDeltaByDate(ctx context.Context, userULID, projectULID, statDate string) (int64, error) {
	query := r.statsQuery(ctx, userULID, projectULID).Where("w.stat_date = ?", statDate)
	var total int64
	if err := query.Select("COALESCE(SUM(w.word_delta), 0)").Scan(&total).Error; err != nil {
		return 0, fmt.Errorf("sum word delta: %w", err)
	}
	return total, nil
}

// SumAIWordTotal 统计 AI 生成累计字数。
func (r *StatsRepo) SumAIWordTotal(ctx context.Context, userULID, projectULID string) (int64, error) {
	query := r.statsQuery(ctx, userULID, projectULID)
	var total int64
	if err := query.Select("COALESCE(SUM(w.ai_word_delta), 0)").Scan(&total).Error; err != nil {
		return 0, fmt.Errorf("sum ai word total: %w", err)
	}
	return total, nil
}

// SumDailyBetween 按日聚合区间内字数增量（升序；仅返回有数据的日期，缺日由 service 补零）。
func (r *StatsRepo) SumDailyBetween(ctx context.Context, userULID, projectULID, fromDate, toDate string) ([]domain.WordCountStatDO, error) {
	query := r.statsQuery(ctx, userULID, projectULID).
		Where("w.stat_date BETWEEN ? AND ?", fromDate, toDate)
	var rows []domain.WordCountStatDO
	err := query.Select("w.stat_date AS stat_date, COALESCE(SUM(w.word_delta), 0) AS word_delta, COALESCE(SUM(w.ai_word_delta), 0) AS ai_word_delta").
		Group("w.stat_date").Order("w.stat_date ASC").Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("sum daily word delta: %w", err)
	}
	return rows, nil
}

// statsQuery 构造 word_count_stats 联 projects 的归属过滤查询。
func (r *StatsRepo) statsQuery(ctx context.Context, userULID, projectULID string) *gorm.DB {
	query := r.db.WithContext(ctx).
		Table("word_count_stats w").
		Joins("JOIN projects p ON p.project_ulid = w.project_ulid").
		Where("p.user_ulid = ?", userULID)
	if projectULID != "" {
		query = query.Where("w.project_ulid = ?", projectULID)
	}
	return query
}

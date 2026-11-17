package service

import (
	"context"
	"time"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// trendDayCount 总览趋势覆盖的天数（含今日）。
const trendDayCount = 30

// StatsService 数据总览业务：项目/字数/AI 字数汇总与近 30 天趋势。
type StatsService struct {
	statsRepo   *repo.StatsRepo
	projectRepo *repo.ProjectRepo
}

// NewStatsService 构造数据总览服务。
func NewStatsService(statsRepo *repo.StatsRepo, projectRepo *repo.ProjectRepo) *StatsService {
	return &StatsService{statsRepo: statsRepo, projectRepo: projectRepo}
}

// Overview 查询数据总览（project_ulid 空为全局；趋势缺日期补零）。
func (s *StatsService) Overview(ctx context.Context, userULID string, req *domain.StatsOverviewREQ) (domain.StatsOverviewVO, error) {
	projectULID := req.ProjectULID
	if projectULID != "" {
		if _, err := mustOwnProject(ctx, s.projectRepo, userULID, projectULID); err != nil {
			return domain.StatsOverviewVO{}, err
		}
	}
	projectCount, err := s.statsRepo.CountProjects(ctx, userULID, projectULID)
	if err != nil {
		return domain.StatsOverviewVO{}, err
	}
	totalWordCount, err := s.statsRepo.SumTotalWordCount(ctx, userULID, projectULID)
	if err != nil {
		return domain.StatsOverviewVO{}, err
	}
	now := time.Now()
	todayWordDelta, err := s.statsRepo.SumWordDeltaByDate(ctx, userULID, projectULID, now.Format("2006-01-02"))
	if err != nil {
		return domain.StatsOverviewVO{}, err
	}
	aiWordTotal, err := s.statsRepo.SumAIWordTotal(ctx, userULID, projectULID)
	if err != nil {
		return domain.StatsOverviewVO{}, err
	}
	trendList, err := s.trendList(ctx, userULID, projectULID, now)
	if err != nil {
		return domain.StatsOverviewVO{}, err
	}
	return domain.StatsOverviewVO{
		ProjectCount:   projectCount,
		TotalWordCount: totalWordCount,
		TodayWordDelta: todayWordDelta,
		AIWordTotal:    aiWordTotal,
		TrendList:      trendList,
	}, nil
}

// trendList 组装近 30 天趋势（含今日，缺日期补零）。
func (s *StatsService) trendList(ctx context.Context, userULID, projectULID string, now time.Time) ([]domain.WordStatVO, error) {
	fromDate := now.AddDate(0, 0, -(trendDayCount - 1)).Format("2006-01-02")
	toDate := now.Format("2006-01-02")
	rows, err := s.statsRepo.SumDailyBetween(ctx, userULID, projectULID, fromDate, toDate)
	if err != nil {
		return nil, err
	}
	deltaByDate := make(map[string]domain.WordCountStatDO, len(rows))
	for i := range rows {
		deltaByDate[rows[i].StatDate] = rows[i]
	}
	trendList := make([]domain.WordStatVO, 0, trendDayCount)
	for day := trendDayCount - 1; day >= 0; day-- {
		dateText := now.AddDate(0, 0, -day).Format("2006-01-02")
		stat := deltaByDate[dateText]
		trendList = append(trendList, domain.WordStatVO{
			StatDate:    dateText,
			WordDelta:   stat.WordDelta,
			AIWordDelta: stat.AIWordDelta,
		})
	}
	return trendList, nil
}

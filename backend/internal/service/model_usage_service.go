// 本文件实现模型用量业务：内存缓冲累加、定期落库与区间聚合查询。

package service

import (
	"context"
	"sync"
	"time"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// usageFlushInterval 用量缓冲落库间隔（降低高频小请求的写放大）。
const usageFlushInterval = 5 * time.Second

// ModelUsageService 模型用量业务：内存缓冲累加 → 定期落库 → 区间聚合查询。
type ModelUsageService struct {
	usageRepo *repo.ModelUsageRepo                // 用量仓储
	mu        sync.Mutex                          // 缓冲互斥锁
	pending   map[string]*domain.ModelUsageStatDO // 维度键 → 待落库累加器
}

// NewModelUsageService 构造用量服务并启动后台落库协程。
func NewModelUsageService(usageRepo *repo.ModelUsageRepo, ctx context.Context) *ModelUsageService {
	service := &ModelUsageService{usageRepo: usageRepo, pending: make(map[string]*domain.ModelUsageStatDO)}
	go service.flushLoop(ctx)
	return service
}

// RecordUsage 写入内存缓冲（并发安全，供 ai.UsageRecorder 调用）。
func (s *ModelUsageService) RecordUsage(_ context.Context, record ai.UsageRecord) {
	statDate := time.Now().Format("2006-01-02")
	key := statDate + "|" + record.ProviderULID + "|" + record.ModelName + "|" + record.CallTypeCode
	s.mu.Lock()
	defer s.mu.Unlock()
	accumulator, ok := s.pending[key]
	if !ok {
		accumulator = &domain.ModelUsageStatDO{
			UsageStatULID: pkg.NewULID(domain.IDPrefixModelUsage),
			StatDate:      statDate,
			ProviderULID:  record.ProviderULID,
			ProviderName:  record.ProviderName,
			ModelName:     record.ModelName,
			CallTypeCode:  record.CallTypeCode,
		}
		s.pending[key] = accumulator
	}
	accumulator.CallCount++
	if record.SuccessFlag {
		accumulator.SuccessCount++
	} else {
		accumulator.FailCount++
	}
	accumulator.PromptTokens += int64(record.PromptTokens)
	accumulator.CompletionTokens += int64(record.CompletionTokens)
	accumulator.TotalTokens += int64(record.PromptTokens + record.CompletionTokens)
	accumulator.LatencyMsSum += record.LatencyMs
}

// flushLoop 周期性把缓冲写入数据库（ctx 取消时做最后一次落库）。
func (s *ModelUsageService) flushLoop(ctx context.Context) {
	ticker := time.NewTicker(usageFlushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.Flush(context.Background())
			return
		case <-ticker.C:
			s.Flush(ctx)
		}
	}
}

// Flush 将内存缓冲写入数据库（失败仅告警，缓冲丢弃避免无限增长）。
func (s *ModelUsageService) Flush(ctx context.Context) {
	s.mu.Lock()
	batch := s.pending
	s.pending = make(map[string]*domain.ModelUsageStatDO)
	s.mu.Unlock()
	for key, accumulator := range batch {
		if err := s.usageRepo.UpsertIncrement(ctx, accumulator); err != nil {
			pkg.SLogger().Warnw("模型用量落库失败", "dimension_key", key, "error", err)
		}
	}
}

// Overview 查询区间用量总览（缺日补零，保证图表连续）。
func (s *ModelUsageService) Overview(ctx context.Context, req *domain.ModelUsageQueryREQ) (domain.ModelUsageOverviewVO, error) {
	toDate, fromDate := resolveUsageRange(req.FromDate, req.ToDate)
	s.Flush(ctx) // 查询前落库，避免刚发生的调用看不到
	dailyRows, err := s.usageRepo.AggregateDaily(ctx, fromDate, toDate)
	if err != nil {
		return domain.ModelUsageOverviewVO{}, pkg.WrapBizError(domain.RespCodeStorageError, "用量趋势读取失败", err)
	}
	providerRows, err := s.usageRepo.AggregateByProvider(ctx, fromDate, toDate)
	if err != nil {
		return domain.ModelUsageOverviewVO{}, pkg.WrapBizError(domain.RespCodeStorageError, "供应商用量读取失败", err)
	}
	modelRows, err := s.usageRepo.AggregateByModel(ctx, fromDate, toDate)
	if err != nil {
		return domain.ModelUsageOverviewVO{}, pkg.WrapBizError(domain.RespCodeStorageError, "模型用量读取失败", err)
	}
	vo := domain.ModelUsageOverviewVO{
		DailyList:    fillUsageDaily(dailyRows, fromDate, toDate),
		ProviderList: nonNilUsageGroups(providerRows),
		ModelList:    nonNilUsageGroups(modelRows),
	}
	for i := range vo.DailyList {
		vo.CallCountTotal += vo.DailyList[i].CallCount
		vo.FailCountTotal += vo.DailyList[i].FailCount
		vo.TotalTokensTotal += vo.DailyList[i].TotalTokens
	}
	if vo.CallCountTotal > 0 {
		vo.AvgLatencyMs = weightedAvgLatency(vo.DailyList)
		vo.SuccessRateValue = float64(vo.CallCountTotal-vo.FailCountTotal) / float64(vo.CallCountTotal)
	}
	return vo, nil
}

// weightedAvgLatency 按每日调用数加权求区间平均延迟（整数运算，避免浮点漂移）。
func weightedAvgLatency(dailyList []domain.ModelUsageDailyVO) int64 {
	var latencySum int64
	var callSum int64
	for i := range dailyList {
		latencySum += dailyList[i].AvgLatencyMs * dailyList[i].CallCount
		callSum += dailyList[i].CallCount
	}
	if callSum == 0 {
		return 0
	}
	return latencySum / callSum
}

// resolveUsageRange 解析查询区间（缺省近 14 天；非法输入回退默认，起止倒置自动纠正）。
func resolveUsageRange(fromDate, toDate string) (string, string) {
	now := time.Now()
	if toDate == "" || !isDateText(toDate) {
		toDate = now.Format("2006-01-02")
	}
	toTime, err := time.Parse("2006-01-02", toDate)
	if err != nil {
		toDate = now.Format("2006-01-02")
		toTime = now
	}
	if fromDate == "" || !isDateText(fromDate) {
		fromDate = toTime.AddDate(0, 0, -13).Format("2006-01-02") // 默认近 14 天
	}
	if fromDate > toDate {
		fromDate, toDate = toDate, fromDate
	}
	return toDate, fromDate
}

// isDateText 校验 YYYY-MM-DD 日期文本。
func isDateText(text string) bool {
	_, err := time.Parse("2006-01-02", text)
	return err == nil
}

// fillUsageDaily 补齐区间内无数据的日期（图表连续）。
func fillUsageDaily(rows []domain.ModelUsageDailyVO, fromDate, toDate string) []domain.ModelUsageDailyVO {
	byDate := make(map[string]domain.ModelUsageDailyVO, len(rows))
	for i := range rows {
		byDate[rows[i].StatDate] = rows[i]
	}
	fromTime, err := time.Parse("2006-01-02", fromDate)
	if err != nil {
		return rows
	}
	toTime, err := time.Parse("2006-01-02", toDate)
	if err != nil {
		return rows
	}
	filled := make([]domain.ModelUsageDailyVO, 0, 32)
	for current := fromTime; !current.After(toTime); current = current.AddDate(0, 0, 1) {
		dateText := current.Format("2006-01-02")
		if row, ok := byDate[dateText]; ok {
			filled = append(filled, row)
			continue
		}
		filled = append(filled, domain.ModelUsageDailyVO{StatDate: dateText})
	}
	return filled
}

// nonNilUsageGroups 保证列表不为 nil（前端直接遍历）。
func nonNilUsageGroups(rows []domain.ModelUsageGroupVO) []domain.ModelUsageGroupVO {
	if rows == nil {
		return []domain.ModelUsageGroupVO{}
	}
	return rows
}

// 本文件实现审计的链路视图查询：按 trace 聚合、分页返回链路摘要、
// 回带链路内的全部 span，以及区间统计（级别分布、模型调用数、耗时、慢链路）。
// Manticore 不支持带聚合子查询的 FROM，耗时统计在内存里对分组结果求平均。

package repo

import (
	"context"
	"fmt"
	"strings"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

func (r *AuditRepo) ListTracePage(ctx context.Context, req *domain.AuditTraceListREQ) ([]string, map[string]AuditTraceAgg, int64, error) {
	if r.manticore == nil {
		return nil, nil, 0, fmt.Errorf("manticore 未配置")
	}
	condition := auditConditionOf(traceListConditionOf(req))
	totalRows, err := r.manticore.Query(ctx,
		fmt.Sprintf("SELECT COUNT(DISTINCT `trace_id`) AS total FROM %s WHERE %s", r.table, condition))
	if err != nil {
		return nil, nil, 0, err
	}
	var total int64
	if len(totalRows) > 0 {
		total = int64Value(totalRows[0]["total"])
	}
	if total == 0 {
		return []string{}, map[string]AuditTraceAgg{}, 0, nil
	}
	sql := fmt.Sprintf("SELECT `trace_id`, COUNT(*) AS span_count, SUM(`error_span_flag`) AS error_count, "+
		"SUM(`ai_span_flag`) AS ai_count, SUM(`token_count`) AS token_sum, "+
		"MIN(`created_at_ts`) AS start_ts, MAX(`created_at_ts` + `latency_ms`) AS end_ts "+
		"FROM %s WHERE %s GROUP BY `trace_id` ORDER BY end_ts DESC LIMIT %d OFFSET %d",
		r.table, condition, req.PageSize, (req.PageNum-1)*req.PageSize)
	rows, err := r.manticore.Query(ctx, sql)
	if err != nil {
		return nil, nil, 0, err
	}
	traceIDList := make([]string, 0, len(rows))
	aggMap := make(map[string]AuditTraceAgg, len(rows))
	for i := range rows {
		traceID := stringValue(rows[i]["trace_id"])
		if traceID == "" {
			continue
		}
		traceIDList = append(traceIDList, traceID)
		aggMap[traceID] = AuditTraceAgg{
			SpanCount:   int(int64Value(rows[i]["span_count"])),
			ErrorCount:  int(int64Value(rows[i]["error_count"])),
			AICallCount: int(int64Value(rows[i]["ai_count"])),
			TokenCount:  int64Value(rows[i]["token_sum"]),
			StartTs:     int64Value(rows[i]["start_ts"]),
			EndTs:       int64Value(rows[i]["end_ts"]),
		}
	}
	return traceIDList, aggMap, total, nil
}

// AuditTraceAgg 链路聚合指标（分页查询阶段由 Manticore 算出）。
type AuditTraceAgg struct {
	SpanCount   int   // span 总数（按筛选条件计）
	ErrorCount  int   // error span 数
	AICallCount int   // 模型调用次数
	TokenCount  int64 // 模型消耗 token
	StartTs     int64 // 链路开始（毫秒）
	EndTs       int64 // 链路结束（毫秒）
}

// ListSpansByTraceIDs 批量取若干链路的全部 span（时间升序），供服务层做链路展示汇总。
func (r *AuditRepo) ListSpansByTraceIDs(ctx context.Context, traceIDList []string, limit int) ([]domain.AuditLogDO, error) {
	if r.manticore == nil {
		return nil, fmt.Errorf("manticore 未配置")
	}
	if len(traceIDList) == 0 {
		return []domain.AuditLogDO{}, nil
	}
	if limit <= 0 || limit > 2000 {
		limit = 600
	}
	quoted := make([]string, 0, len(traceIDList))
	for _, traceID := range traceIDList {
		quoted = append(quoted, "'"+escapeManticore(traceID)+"'")
	}
	sql := fmt.Sprintf("SELECT %s FROM %s WHERE `trace_id` IN (%s) ORDER BY `created_at_ts` ASC LIMIT %d",
		auditSelectColumns(), r.table, strings.Join(quoted, ","), limit)
	rows, err := r.manticore.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	dos := make([]domain.AuditLogDO, 0, len(rows))
	for i := range rows {
		dos = append(dos, auditDOOf(rows[i]))
	}
	return dos, nil
}

// AuditStats 统计区间内的链路与 span 指标（审计页顶部卡片）。
func (r *AuditRepo) AuditStats(ctx context.Context, req *domain.AuditTraceListREQ) (domain.AuditStatsVO, error) {
	if r.manticore == nil {
		return domain.AuditStatsVO{}, fmt.Errorf("manticore 未配置")
	}
	condition := auditConditionOf(traceListConditionOf(req))
	var stats domain.AuditStatsVO
	rows, err := r.manticore.Query(ctx, fmt.Sprintf("SELECT COUNT(DISTINCT `trace_id`) AS trace_count, "+
		"COUNT(*) AS span_count, SUM(`ai_span_flag`) AS ai_count, SUM(`token_count`) AS token_sum, "+
		"SUM(`error_span_flag`) AS error_span_count FROM %s WHERE %s", r.table, condition))
	if err != nil {
		return domain.AuditStatsVO{}, err
	}
	if len(rows) > 0 {
		stats.TraceCount = int64Value(rows[0]["trace_count"])
		stats.SpanCount = int64Value(rows[0]["span_count"])
		stats.AICallCount = int64Value(rows[0]["ai_count"])
		stats.TokenCount = int64Value(rows[0]["token_sum"])
	}
	// 含 error 的链路数：单独一次去重计数，避免把 warn 链路算进来
	errorRows, err := r.manticore.Query(ctx,
		fmt.Sprintf("SELECT COUNT(DISTINCT `trace_id`) AS total FROM %s WHERE %s AND `error_span_flag` = 1",
			r.table, condition))
	if err != nil {
		return domain.AuditStatsVO{}, err
	}
	if len(errorRows) > 0 {
		stats.ErrorTraceCount = int64Value(errorRows[0]["total"])
	}
	// 仅 warn 的链路 = 有 warn 且无 error
	warnRows, err := r.manticore.Query(ctx,
		fmt.Sprintf("SELECT COUNT(DISTINCT `trace_id`) AS total FROM %s WHERE %s AND `error_span_flag` = 0 AND `warn_span_flag` = 1",
			r.table, condition))
	if err != nil {
		return domain.AuditStatsVO{}, err
	}
	if len(warnRows) > 0 {
		stats.WarnTraceCount = int64Value(warnRows[0]["total"])
	}
	// 平均链路耗时按「首末 span 跨度」算：工作流这类入口请求秒回，真正耗时全在后台 span 里，
	// 只统计根 span 会让跑了几分钟的链路在概览里显示成个位数毫秒。
	// Manticore 不支持带聚合子查询的 FROM，这里直接复用与列表同一条 GROUP BY 语句再在内存里求平均。
	spanRows, err := r.manticore.Query(ctx,
		fmt.Sprintf("SELECT `trace_id`, MIN(`created_at_ts`) AS start_ts, MAX(`created_at_ts` + `latency_ms`) AS end_ts "+
			"FROM %s WHERE %s GROUP BY `trace_id` LIMIT %d", r.table, condition, auditStatsTraceLimit))
	if err != nil {
		return domain.AuditStatsVO{}, err
	}
	if len(spanRows) > 0 {
		var totalMs int64
		for i := range spanRows {
			totalMs += int64Value(spanRows[i]["end_ts"]) - int64Value(spanRows[i]["start_ts"])
		}
		stats.AvgLatencyMs = totalMs / int64(len(spanRows))
	}
	// 慢链路：链路级总耗时超过 5 秒（用 HAVING 聚合，Manticore 支持单个 HAVING 条件）
	slowRows, err := r.manticore.Query(ctx,
		fmt.Sprintf("SELECT `trace_id`, SUM(`latency_ms`) AS total_ms FROM %s WHERE %s "+
			"GROUP BY `trace_id` HAVING total_ms > %d LIMIT 1000", r.table, condition, auditSlowTraceMs))
	if err != nil {
		return domain.AuditStatsVO{}, err
	}
	stats.SlowTraceCount = int64(len(slowRows))
	return stats, nil
}

// 慢链路与耗时统计阈值：慢链路门槛 5 秒；平均耗时最多采样 5000 条链路（概览不需要全量）。
const (
	auditSlowTraceMs     = 5000
	auditStatsTraceLimit = 5000
)

// traceListConditionOf 链路列表的筛选条件（链路视图按根动作/用户/时间收敛）。
func traceListConditionOf(req *domain.AuditTraceListREQ) *auditCondition {
	return &auditCondition{
		KeywordText: req.KeywordText, ActionCode: req.ActionCode, LogLevelCode: req.LogLevelCode,
		SpanTypeCode: req.SpanTypeCode, UserULID: req.UserULID,
		TimeFrom: req.TimeFrom, TimeTo: req.TimeTo, DateFrom: req.DateFrom, DateTo: req.DateTo,
	}
}

// auditDOOf 把查询行映射为审计持久化对象（时间为毫秒时间戳还原）。

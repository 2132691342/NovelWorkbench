// 可观测性的链路回归：审计链路摘要汇总 → 步骤级别映射 → 模型用量缓冲累加。
// 三件事共同支撑「出问题时能定位到哪一步」：链路级别算错会让排障走向错误方向，
// 步骤级别误判会让每次健康运行都挂着橙色告警，用量记成两条会让统计出现伪数据。

package service

import (
	"context"
	"testing"
	"time"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// auditSpan 构造一条审计 span（测试数据用）。
func auditSpan(actionCode string, spanTypeCode, logLevelCode string, parentSpanCode string, latencyMs int64) domain.AuditLogDO {
	return domain.AuditLogDO{
		AuditULID:      "AUD_" + actionCode,
		ActionCode:     actionCode,
		SpanTypeCode:   spanTypeCode,
		LogLevelCode:   logLevelCode,
		ParentSpanCode: parentSpanCode,
		LatencyMs:      latencyMs,
		HTTPStatus:     200,
		BaseDO:         domain.BaseDO{CreatedAt: time.UnixMilli(1795796396962)},
	}
}

// TestAuditTraceSummaryChain 覆盖链路摘要的汇总规则。
func TestAuditTraceSummaryChain(t *testing.T) {
	// 环节1：级别取最严重 span（error 压过 warn 压过 info），根 span 提供状态，最慢一步可定位。
	spanList := []domain.AuditLogDO{
		auditSpan("workflow/run/create", domain.AuditSpanTypeHTTP, domain.AuditLevelInfo, "", 12),
		auditSpan("workflow/step/draft", domain.AuditSpanTypeWorkflow, domain.AuditLevelInfo, "workflow/run/create", 900),
		auditSpan("ai/workflow/draft", domain.AuditSpanTypeAICall, domain.AuditLevelWarn, "workflow/run/create", 880),
		auditSpan("workflow/step/apply", domain.AuditSpanTypeWorkflow, domain.AuditLevelError, "workflow/run/create", 5),
	}
	vo := toAuditTraceSummaryVO("TRC_TEST", spanList, repo.AuditTraceAgg{
		SpanCount: 4, ErrorCount: 1, AICallCount: 1, TokenCount: 1200,
		StartTs: 1795796396962, EndTs: 1795796397974,
	})
	if vo.LogLevelCode != domain.AuditLevelError {
		t.Fatalf("链路级别应为 error，got %s", vo.LogLevelCode)
	}
	if vo.RootActionCode != "workflow/run/create" {
		t.Fatalf("根动作取错：%s", vo.RootActionCode)
	}
	// 链路耗时取「根 span 耗时」与「首末 span 跨度」的较大者：入口请求秒回、
	// 真正耗时在后台 span 里时，只有时间跨度为真（1012ms > 12ms）。
	if vo.TotalLatencyMs != 1012 {
		t.Fatalf("总耗时应取首末 span 跨度，got %d", vo.TotalLatencyMs)
	}
	if vo.SlowestSpanMs != 900 {
		t.Fatalf("最慢 span 应为 900ms，got %d", vo.SlowestSpanMs)
	}
	if vo.SlowestActionName == "" {
		t.Fatal("最慢 span 缺少可读动作名")
	}
	if vo.AICallCount != 1 || vo.TokenCount != 1200 || vo.SpanCount != 4 {
		t.Fatalf("聚合指标透传错误：%+v", vo)
	}

	// 环节2：没有根 span 时用时间区间兜底总耗时（历史行可能缺根）。
	fallbackVO := toAuditTraceSummaryVO("TRC_TEST", []domain.AuditLogDO{
		auditSpan("workflow/step/check", domain.AuditSpanTypeWorkflow, domain.AuditLevelInfo, "workflow/run/create", 30),
	}, repo.AuditTraceAgg{SpanCount: 1, StartTs: 1000, EndTs: 3500})
	if fallbackVO.TotalLatencyMs != 2500 {
		t.Fatalf("无根 span 时应按区间兜底，got %d", fallbackVO.TotalLatencyMs)
	}
	if fallbackVO.LogLevelCode != domain.AuditLevelInfo {
		t.Fatalf("无异常 span 时应为 info，got %s", fallbackVO.LogLevelCode)
	}

	// 环节3：步骤级别映射。只有失败与人工退回才是异常；
	// skipped（体检达标跳过修订）是正常结果，记 warn 会让每次健康运行都顶着橙色级别。
	levelCases := []struct {
		statusCode string
		wantLevel  string
		wantStatus int
	}{
		{domain.WorkflowStepStatusSuccess, domain.AuditLevelInfo, 200},
		{domain.WorkflowStepStatusSkipped, domain.AuditLevelInfo, 200},
		{domain.WorkflowStepStatusRejected, domain.AuditLevelWarn, 400},
		{domain.WorkflowStepStatusFailed, domain.AuditLevelError, 500},
	}
	for _, testCase := range levelCases {
		httpStatus, levelCode := stepAuditLevelOf(testCase.statusCode)
		if levelCode != testCase.wantLevel || httpStatus != testCase.wantStatus {
			t.Fatalf("%s: got (%s,%d)，want (%s,%d)", testCase.statusCode,
				levelCode, httpStatus, testCase.wantLevel, testCase.wantStatus)
		}
	}
}

// TestModelUsageBufferChain 覆盖用量缓冲的累加与区间平均延迟。
func TestModelUsageBufferChain(t *testing.T) {
	// 环节1：累加口径。未知用量单独计数，token 只累加真实值，调用次数与 token 落在同一条上。
	usageService := &ModelUsageService{pending: make(map[string]*domain.ModelUsageStatDO)}
	specRecord := func(success, known bool, promptTokens, completionTokens int) ai.UsageRecord {
		return ai.UsageRecord{
			ProviderULID: "PRV_1", ProviderName: "测试供应商", ModelName: "m1", CallTypeCode: ai.CallTypeChat,
			SuccessFlag: success, UsageKnownFlag: known,
			PromptTokens: promptTokens, CompletionTokens: completionTokens, LatencyMs: 100,
		}
	}
	usageService.RecordUsage(context.Background(), specRecord(true, true, 2214, 14))
	usageService.RecordUsage(context.Background(), specRecord(false, false, 0, 0))
	usageService.RecordUsage(context.Background(), specRecord(true, true, 100, 20))

	var accumulator *domain.ModelUsageStatDO
	for _, item := range usageService.pending {
		accumulator = item
	}
	if accumulator == nil {
		t.Fatal("缓冲未产生累加器")
	}
	if accumulator.CallCount != 3 || accumulator.SuccessCount != 2 || accumulator.FailCount != 1 {
		t.Fatalf("调用次数累加错误：call=%d success=%d fail=%d",
			accumulator.CallCount, accumulator.SuccessCount, accumulator.FailCount)
	}
	if accumulator.UnknownUsageCount != 1 {
		t.Fatalf("未知用量次数错误：%d", accumulator.UnknownUsageCount)
	}
	if accumulator.PromptTokens != 2314 || accumulator.CompletionTokens != 34 || accumulator.TotalTokens != 2348 {
		t.Fatalf("token 累加错误：prompt=%d completion=%d total=%d",
			accumulator.PromptTokens, accumulator.CompletionTokens, accumulator.TotalTokens)
	}
	if accumulator.LatencyMsSum != 300 {
		t.Fatalf("耗时累加错误：%d", accumulator.LatencyMsSum)
	}

	// 环节2：区间平均延迟按每日调用数加权（0 调用日不参与）。
	dailyList := []domain.ModelUsageDailyVO{
		{StatDate: "2026-11-25", CallCount: 2, AvgLatencyMs: 1000},
		{StatDate: "2026-11-26", CallCount: 0, AvgLatencyMs: 9999},
		{StatDate: "2026-11-27", CallCount: 6, AvgLatencyMs: 2000},
	}
	if got := weightedAvgLatency(dailyList); got != 1750 {
		t.Fatalf("加权平均延迟错误：%d", got)
	}
	if got := weightedAvgLatency(nil); got != 0 {
		t.Fatalf("无调用时应返回 0，实际 %d", got)
	}
}

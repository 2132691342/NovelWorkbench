// 本文件实现审计日志业务：写入（Manticore）、记录视图、链路视图（一次请求的完整调用链）、
// 详情、链路时间线与留存清理（审计 Manticore + 登录日志 PostgreSQL 一并清理）。

package service

import (
	"context"
	"time"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// 审计留存默认值与清理时刻配置键。
const (
	auditRetentionDefaultDays    = 30                     // 默认留存 30 天
	auditPurgeDefaultHour        = 3                      // 默认每日清理时刻（本地时区 03:00，业务低峰）
	settingKeyAuditRetentionDays = "audit_retention_days" // 应用配置键：审计留存天数
	settingKeyAuditPurgeHour     = "audit_purge_hour"     // 应用配置键：每日清理时刻（0–23）
	auditTraceSpanLimit          = 600                    // 一页链路最多带回的 span 数
)

// AuditService 审计日志业务：落库、查询与留存清理。
type AuditService struct {
	auditRepo     *repo.AuditRepo    // 审计仓储（Manticore）
	userRepo      *repo.UserRepo     // 用户仓储（补全操作人登录名）
	loginLogRepo  *repo.LoginLogRepo // 登录日志仓储（留存清理一并处理）
	settingRepo   *repo.SettingRepo  // 应用配置（留存天数与清理时刻可调）
	lastPurgedAt  time.Time          // 上次清理时间（进程内状态，供审计页摘要展示）
	lastPurgedSum purgeSummary       // 上次清理的删除量（审计 + 登录日志）
}

// purgeSummary 一次清理删除的条数。
type purgeSummary struct {
	AuditCount    int // 删除的审计条数
	LoginLogCount int // 删除的登录日志条数
}

// NewAuditService 构造审计日志服务（登录日志仓储可为 nil，兼容只跑审计的测试装配）。
func NewAuditService(auditRepo *repo.AuditRepo, userRepo *repo.UserRepo, settingRepo *repo.SettingRepo,
	loginLogRepo *repo.LoginLogRepo) *AuditService {
	return &AuditService{auditRepo: auditRepo, userRepo: userRepo, settingRepo: settingRepo,
		loginLogRepo: loginLogRepo}
}

// EnsureSchema 确保审计全文表存在并补齐 span 相关新列（启动期调用）。
func (s *AuditService) EnsureSchema(ctx context.Context) error {
	return s.auditRepo.EnsureSchema(ctx)
}

// Record 落库一条审计记录（自动补全动作中文名与操作人登录名；pkg.AuditRecorder 实现）。
func (s *AuditService) Record(ctx context.Context, do *domain.AuditLogDO) error {
	do.AuditULID = pkg.NewULID(domain.IDPrefixAudit)
	if do.SpanTypeCode == "" {
		do.SpanTypeCode = domain.AuditSpanTypeHTTP // 历史调用方（中间件）默认按入口请求归类
	}
	if do.ActionName == "" {
		do.ActionName = pkg.AuditActionNameOf(do.ActionCode)
	}
	if do.UserULID != "" && do.UserName == "" {
		userDO, err := s.userRepo.GetByULID(ctx, do.UserULID)
		if err == nil && userDO != nil {
			do.UserName = userDO.UserName
		}
	}
	return s.auditRepo.Create(ctx, do)
}

// ListPage 分页查询审计记录（记录视图：一行一个 span）。
func (s *AuditService) ListPage(ctx context.Context, req *domain.AuditListREQ) (domain.PageDTO[domain.AuditVO], error) {
	req.Normalize()
	dos, total, err := s.auditRepo.ListPage(ctx, req)
	if err != nil {
		return domain.PageDTO[domain.AuditVO]{}, pkg.WrapBizError(domain.RespCodeStorageError, "审计查询失败", err)
	}
	vos := make([]domain.AuditVO, 0, len(dos))
	for i := range dos {
		vos = append(vos, toAuditVO(&dos[i]))
	}
	return domain.NewPageDTO(total, req.PageREQ, vos), nil
}

// ListTracePage 分页查询链路（链路视图：一行一次入口请求，附带它下面的全部后台 span 指标）。
func (s *AuditService) ListTracePage(ctx context.Context, req *domain.AuditTraceListREQ) (domain.PageDTO[domain.AuditTraceSummaryVO], error) {
	req.Normalize()
	traceIDList, aggMap, total, err := s.auditRepo.ListTracePage(ctx, req)
	if err != nil {
		return domain.PageDTO[domain.AuditTraceSummaryVO]{}, pkg.WrapBizError(domain.RespCodeStorageError, "链路查询失败", err)
	}
	dos, err := s.auditRepo.ListSpansByTraceIDs(ctx, traceIDList, auditTraceSpanLimit)
	if err != nil {
		return domain.PageDTO[domain.AuditTraceSummaryVO]{}, pkg.WrapBizError(domain.RespCodeStorageError, "链路详情读取失败", err)
	}
	spanMap := make(map[string][]domain.AuditLogDO, len(traceIDList))
	for i := range dos {
		spanMap[dos[i].TraceID] = append(spanMap[dos[i].TraceID], dos[i])
	}
	vos := make([]domain.AuditTraceSummaryVO, 0, len(traceIDList))
	for _, traceID := range traceIDList {
		vos = append(vos, toAuditTraceSummaryVO(traceID, spanMap[traceID], aggMap[traceID]))
	}
	return domain.NewPageDTO(total, req.PageREQ, vos), nil
}

// Stats 返回区间内的审计概览（顶部卡片）。
func (s *AuditService) Stats(ctx context.Context, req *domain.AuditTraceListREQ) (domain.AuditStatsVO, error) {
	req.Normalize()
	stats, err := s.auditRepo.AuditStats(ctx, req)
	if err != nil {
		return domain.AuditStatsVO{}, pkg.WrapBizError(domain.RespCodeStorageError, "审计统计失败", err)
	}
	return stats, nil
}

// Detail 查询单条审计详情（含请求体与响应体原文）。
func (s *AuditService) Detail(ctx context.Context, req *domain.AuditDetailREQ) (domain.AuditDetailVO, error) {
	do, err := s.auditRepo.GetByULID(ctx, req.AuditULID)
	if err != nil {
		return domain.AuditDetailVO{}, pkg.WrapBizError(domain.RespCodeStorageError, "审计详情读取失败", err)
	}
	if do == nil {
		return domain.AuditDetailVO{}, pkg.NewBizError(domain.RespCodeNotFound, "审计记录不存在")
	}
	return toAuditDetailVO(do), nil
}

// Trace 按链路 ID 拉取完整调用时间线（同一 trace_id 内的全部 span）。
func (s *AuditService) Trace(ctx context.Context, req *domain.AuditTraceREQ) (domain.AuditTraceVO, error) {
	dos, err := s.auditRepo.ListByTrace(ctx, req.TraceID, 200)
	if err != nil {
		return domain.AuditTraceVO{}, pkg.WrapBizError(domain.RespCodeStorageError, "链路时间线读取失败", err)
	}
	vos := make([]domain.AuditDetailVO, 0, len(dos))
	for i := range dos {
		vos = append(vos, toAuditDetailVO(&dos[i]))
	}
	return domain.AuditTraceVO{TraceID: req.TraceID, Total: len(vos), ItemList: vos}, nil
}

// ActionOptions 返回动作码筛选项（按条数倒序，未登记动作名时回落动作码）。
func (s *AuditService) ActionOptions(ctx context.Context) ([]domain.AuditActionOptionVO, error) {
	options, err := s.auditRepo.ListActionOptions(ctx, 80)
	if err != nil {
		return nil, pkg.WrapBizError(domain.RespCodeStorageError, "动作码汇总失败", err)
	}
	for i := range options {
		if options[i].ActionName == "" {
			options[i].ActionName = pkg.AuditActionLabel(options[i].ActionCode)
		}
	}
	return options, nil
}

// Retention 返回留存策略现状（天数、清理时刻、总条数、上次与下次清理时间）。
func (s *AuditService) Retention(ctx context.Context) domain.AuditRetentionVO {
	vo := s.retentionVO(ctx)
	if total, err := s.auditRepo.CountAll(ctx); err == nil {
		vo.TotalCount = total
	}
	if s.loginLogRepo != nil {
		if total, err := s.loginLogRepo.CountAll(ctx); err == nil {
			vo.LoginLogTotalCount = total
		}
	}
	return vo
}

// Purge 立即执行一次留存清理（审计 + 登录日志），并写一条 system span 审计。
func (s *AuditService) Purge(ctx context.Context) (domain.AuditRetentionVO, error) {
	retentionDays := s.retentionDays(ctx)
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	purged, err := s.auditRepo.DeleteBefore(ctx, cutoff)
	if err != nil {
		return domain.AuditRetentionVO{}, pkg.WrapBizError(domain.RespCodeStorageError, "审计清理失败", err)
	}
	// 登录日志与审计同一留存窗口：安全事件的两份痕迹不能一个有、一个没
	loginLogPurged := 0
	if s.loginLogRepo != nil {
		loginLogPurged, err = s.loginLogRepo.DeleteBefore(ctx, cutoff)
		if err != nil {
			pkg.SLogger().Warnw("登录日志留存清理失败", "error", err)
		}
	}
	s.lastPurgedAt = time.Now()
	s.lastPurgedSum = purgeSummary{AuditCount: purged, LoginLogCount: loginLogPurged}
	total, _ := s.auditRepo.CountAll(ctx)
	pkg.SLogger().Infow("审计留存清理完成", "retention_days", retentionDays,
		"purged_count", purged, "login_log_purged_count", loginLogPurged, "total_count", total)
	s.recordPurgeAudit(ctx, retentionDays, purged, loginLogPurged, total)
	vo := s.retentionVO(ctx)
	vo.TotalCount = total
	vo.PurgedCount = purged
	vo.LoginLogPurgedCount = loginLogPurged
	if s.loginLogRepo != nil {
		if loginTotal, err := s.loginLogRepo.CountAll(ctx); err == nil {
			vo.LoginLogTotalCount = loginTotal
		}
	}
	return vo, nil
}

// recordPurgeAudit 记录一次清理动作（system span，链路 ID 取发起请求的链路或新生成）。
func (s *AuditService) recordPurgeAudit(ctx context.Context, retentionDays, purged, loginLogPurged int, remaining int64) {
	traceID := pkg.RequestIDFromContext(ctx)
	if traceID == "" {
		traceID = pkg.NewULID(domain.IDPrefixTrace)
	}
	do := &domain.AuditLogDO{
		LogLevelCode:  domain.AuditLevelInfo,
		SpanTypeCode:  domain.AuditSpanTypeSystem,
		TraceID:       traceID,
		UserULID:      pkg.ActorFromContext(ctx),
		ActionCode:    domain.AuditActionSystemPurge,
		ActionName:    "清理审计日志",
		RequestPath:   "system://audit/purge",
		RequestMethod: "INTERNAL",
		RespCode:      domain.RespCodeSuccess,
		RespMessage: "留存 " + intToText(int64(retentionDays)) + " 天，删除审计 " +
			intToText(int64(purged)) + " 条、登录日志 " + intToText(int64(loginLogPurged)) +
			" 条，剩余审计 " + intToText(remaining) + " 条",
		HTTPStatus: 200,
	}
	if err := s.Record(ctx, do); err != nil {
		pkg.SLogger().Warnw("审计清理动作写入失败", "error", err)
	}
}

// retentionVO 组装留存视图（下次清理时刻按当前配置真实计算）。
func (s *AuditService) retentionVO(ctx context.Context) domain.AuditRetentionVO {
	purgeHour := s.purgeHour(ctx)
	return domain.AuditRetentionVO{
		RetentionDays:       s.retentionDays(ctx),
		PurgeHour:           purgeHour,
		LastPurgedAt:        pkg.FormatTime(s.lastPurgedAt),
		NextPurgeAt:         pkg.FormatTime(nextPurgeTime(time.Now(), purgeHour)),
		PurgedCount:         s.lastPurgedSum.AuditCount,
		LoginLogPurgedCount: s.lastPurgedSum.LoginLogCount,
	}
}

// retentionDays 读取留存天数配置（缺省 30，非法值回落默认）。
func (s *AuditService) retentionDays(ctx context.Context) int {
	days, ok := s.settingInt(ctx, settingKeyAuditRetentionDays)
	if !ok || days <= 0 || days > 365 {
		return auditRetentionDefaultDays
	}
	return days
}

// purgeHour 读取每日清理时刻配置（缺省 3，非法值回落默认）。
func (s *AuditService) purgeHour(ctx context.Context) int {
	hour, ok := s.settingInt(ctx, settingKeyAuditPurgeHour)
	if !ok || hour < 0 || hour > 23 {
		return auditPurgeDefaultHour
	}
	return hour
}

// settingInt 读取整数型配置项（无仓储或读取失败返回 false）。
func (s *AuditService) settingInt(ctx context.Context, settingKey string) (int, bool) {
	if s.settingRepo == nil {
		return 0, false
	}
	item, err := s.settingRepo.GetByKey(ctx, settingKey)
	if err != nil || item == nil {
		return 0, false
	}
	value, ok := parseIntText(item.SettingValue)
	return value, ok
}

// RunRetentionJob 阻塞运行每日审计清理任务，直到 ctx 取消（优雅退出）。
// 启动时先清一次（补上停机期间漏掉的清理），之后每天在配置时刻执行。
func (s *AuditService) RunRetentionJob(ctx context.Context) {
	s.purgeOnce(ctx)
	for {
		nextRun := nextPurgeTime(time.Now(), s.purgeHour(ctx))
		timer := time.NewTimer(time.Until(nextRun))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			s.purgeOnce(ctx)
		}
	}
}

// purgeOnce 执行一次清理，失败只告警不中断任务。
func (s *AuditService) purgeOnce(ctx context.Context) {
	if _, err := s.Purge(ctx); err != nil {
		pkg.SLogger().Warnw("审计留存清理失败", "error", err)
	}
}

// nextPurgeTime 计算下一次清理时刻（本地时区，按配置小时）。
func nextPurgeTime(now time.Time, purgeHour int) time.Time {
	next := time.Date(now.Year(), now.Month(), now.Day(), purgeHour, 0, 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

// toAuditVO 组装审计视图对象（历史行缺级别/动作名/span 类型时补齐，保证列表可读）。
func toAuditVO(do *domain.AuditLogDO) domain.AuditVO {
	levelCode := do.LogLevelCode
	if levelCode == "" && do.HTTPStatus > 0 {
		levelCode = pkg.AuditLevelOf(do.HTTPStatus)
	}
	spanTypeCode := do.SpanTypeCode
	if spanTypeCode == "" {
		spanTypeCode = domain.AuditSpanTypeHTTP
	}
	actionName := do.ActionName
	if actionName == "" {
		actionName = pkg.AuditActionLabel(do.ActionCode)
	}
	return domain.AuditVO{
		AuditULID:      do.AuditULID,
		LogLevelCode:   levelCode,
		SpanTypeCode:   spanTypeCode,
		TraceID:        do.TraceID,
		ParentSpanCode: do.ParentSpanCode,
		UserULID:       do.UserULID,
		UserName:       do.UserName,
		ActionCode:     do.ActionCode,
		ActionName:     actionName,
		ModelName:      do.ModelName,
		TokenCount:     do.TokenCount,
		RequestPath:    do.RequestPath,
		RequestMethod:  do.RequestMethod,
		TargetType:     do.TargetType,
		TargetULID:     do.TargetULID,
		RespCode:       do.RespCode,
		RespMessage:    do.RespMessage,
		HTTPStatus:     do.HTTPStatus,
		LatencyMs:      do.LatencyMs,
		IPAddr:         do.IPAddr,
		UserAgentText:  do.UserAgentText,
		CreatedAt:      pkg.FormatTime(do.CreatedAt),
	}
}

// toAuditDetailVO 组装审计详情（在列表视图上补请求/响应体）。
func toAuditDetailVO(do *domain.AuditLogDO) domain.AuditDetailVO {
	return domain.AuditDetailVO{
		AuditVO:     toAuditVO(do),
		RequestBody: do.RequestBody,
		RespBody:    do.RespBody,
	}
}

// toAuditTraceSummaryVO 把一条链路的 span 列表汇总成链路摘要。
// 根 span 提供状态与动作；子 span 提供最慢一步与模型消耗（这部分只能逐行算，Manticore 不支持 MAX(string)）。
func toAuditTraceSummaryVO(traceID string, spanList []domain.AuditLogDO, agg repo.AuditTraceAgg) domain.AuditTraceSummaryVO {
	vo := domain.AuditTraceSummaryVO{
		TraceID:      traceID,
		SpanCount:    agg.SpanCount,
		AICallCount:  agg.AICallCount,
		TokenCount:   agg.TokenCount,
		LogLevelCode: domain.AuditLevelInfo,
		StartedAt:    pkg.FormatTime(time.UnixMilli(agg.StartTs)),
		FinishedAt:   pkg.FormatTime(time.UnixMilli(agg.EndTs)),
	}
	var rootSpan *domain.AuditLogDO
	slowestMs := int64(-1)
	for i := range spanList {
		span := &spanList[i]
		if span.ParentSpanCode == "" && rootSpan == nil {
			rootSpan = span
		}
		if span.LogLevelCode == domain.AuditLevelError {
			vo.LogLevelCode = domain.AuditLevelError
		} else if span.LogLevelCode == domain.AuditLevelWarn && vo.LogLevelCode != domain.AuditLevelError {
			vo.LogLevelCode = domain.AuditLevelWarn
		}
		if span.LatencyMs > slowestMs {
			slowestMs = span.LatencyMs
			vo.SlowestSpanMs = span.LatencyMs
			vo.SlowestActionName = auditActionLabelOf(span)
		}
	}
	if rootSpan != nil {
		vo.UserULID = rootSpan.UserULID
		vo.UserName = rootSpan.UserName
		vo.RootActionCode = rootSpan.ActionCode
		vo.RootActionName = auditActionLabelOf(rootSpan)
		vo.RootRequestPath = rootSpan.RequestPath
		vo.HTTPStatus = rootSpan.HTTPStatus
		vo.RespCode = rootSpan.RespCode
		vo.RespMessage = rootSpan.RespMessage
		vo.TotalLatencyMs = rootSpan.LatencyMs
	}
	// 链路耗时取「根 span 耗时」与「首末 span 的时间跨度」中的较大者：
	// 工作流这类入口请求会立刻返回，真正耗时全在后台上报的 span 里，
	// 只报根 span 会让一条跑了十几分钟的链路显示成 26ms。
	if spanMs := agg.EndTs - agg.StartTs; spanMs > vo.TotalLatencyMs {
		vo.TotalLatencyMs = spanMs
	}
	return vo
}

// auditActionLabelOf 取 span 的展示动作名（历史行缺名称时回落映射表，再回落动作码）。
func auditActionLabelOf(do *domain.AuditLogDO) string {
	if do.ActionName != "" {
		return do.ActionName
	}
	return pkg.AuditActionLabel(do.ActionCode)
}

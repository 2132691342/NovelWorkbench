// 本文件实现审计日志业务：写入（Manticore）、分页查询、详情、链路时间线与留存清理。

package service

import (
	"context"
	"time"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// 审计留存默认值与清理时刻。
const (
	auditRetentionDefaultDays = 30               // 默认留存 30 天
	auditPurgeHourOfDay       = 3                // 每日清理时刻（本地时区 03:00，业务低峰）
	settingKeyAuditRetentionDays = "audit_retention_days" // 应用配置键：审计留存天数
)

// AuditService 审计日志业务：落库、查询与留存清理。
type AuditService struct {
	auditRepo *repo.AuditRepo  // 审计仓储（Manticore）
	userRepo  *repo.UserRepo   // 用户仓储（补全操作人登录名）
	settingRepo *repo.SettingRepo // 应用配置（留存天数可调）
	lastPurgedAt time.Time     // 上次清理时间（进程内状态，供设置页展示）
}

// NewAuditService 构造审计日志服务。
func NewAuditService(auditRepo *repo.AuditRepo, userRepo *repo.UserRepo, settingRepo *repo.SettingRepo) *AuditService {
	return &AuditService{auditRepo: auditRepo, userRepo: userRepo, settingRepo: settingRepo}
}

// EnsureSchema 确保审计全文表存在（启动期调用）。
func (s *AuditService) EnsureSchema(ctx context.Context) error {
	return s.auditRepo.EnsureSchema(ctx)
}

// Record 落库一条审计记录（自动补全操作用户登录名；pkg.AuditRecorder 实现）。
func (s *AuditService) Record(ctx context.Context, do *domain.AuditLogDO) error {
	do.AuditULID = pkg.NewULID(domain.IDPrefixAudit)
	if do.UserULID != "" && do.UserName == "" {
		userDO, err := s.userRepo.GetByULID(ctx, do.UserULID)
		if err == nil && userDO != nil {
			do.UserName = userDO.UserName
		}
	}
	return s.auditRepo.Create(ctx, do)
}

// ListPage 分页查询审计日志。
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

// Detail 查询单条审计详情（含请求体与响应体原文）。
func (s *AuditService) Detail(ctx context.Context, req *domain.AuditDetailREQ) (domain.AuditDetailVO, error) {
	do, err := s.auditRepo.GetByULID(ctx, req.AuditULID)
	if err != nil {
		return domain.AuditDetailVO{}, pkg.WrapBizError(domain.RespCodeStorageError, "审计详情读取失败", err)
	}
	if do == nil {
		return domain.AuditDetailVO{}, pkg.NewBizError(domain.RespCodeNotFound, "审计记录不存在")
	}
	return domain.AuditDetailVO{AuditVO: toAuditVO(do), RequestBody: do.RequestBody, RespBody: do.RespBody}, nil
}

// Trace 按链路 ID 拉取完整调用时间线（一次请求内的全部审计行）。
func (s *AuditService) Trace(ctx context.Context, req *domain.AuditTraceREQ) (domain.AuditTraceVO, error) {
	dos, err := s.auditRepo.ListByTrace(ctx, req.TraceID, 200)
	if err != nil {
		return domain.AuditTraceVO{}, pkg.WrapBizError(domain.RespCodeStorageError, "链路时间线读取失败", err)
	}
	vos := make([]domain.AuditVO, 0, len(dos))
	for i := range dos {
		vos = append(vos, toAuditVO(&dos[i]))
	}
	return domain.AuditTraceVO{TraceID: req.TraceID, Total: len(vos), ItemList: vos}, nil
}

// ActionOptions 返回动作码筛选项（按条数倒序）。
func (s *AuditService) ActionOptions(ctx context.Context) ([]domain.AuditActionOptionVO, error) {
	options, err := s.auditRepo.ListActionOptions(ctx, 60)
	if err != nil {
		return nil, pkg.WrapBizError(domain.RespCodeStorageError, "动作码汇总失败", err)
	}
	return options, nil
}

// Retention 返回留存策略现状（天数、总条数、上次清理时间）。
func (s *AuditService) Retention(ctx context.Context) domain.AuditRetentionVO {
	vo := domain.AuditRetentionVO{
		RetentionDays:   s.retentionDays(ctx),
		NextPurgeAtText: "每日 03:00 自动清理",
	}
	if total, err := s.auditRepo.CountAll(ctx); err == nil {
		vo.TotalCount = total
	}
	vo.LastPurgedAt = pkg.FormatTime(s.lastPurgedAt)
	return vo
}

// Purge 立即执行一次留存清理，返回删除条数（手动触发与管理端展示共用）。
func (s *AuditService) Purge(ctx context.Context) (domain.AuditRetentionVO, error) {
	retentionDays := s.retentionDays(ctx)
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	purged, err := s.auditRepo.DeleteBefore(ctx, cutoff)
	if err != nil {
		return domain.AuditRetentionVO{}, pkg.WrapBizError(domain.RespCodeStorageError, "审计清理失败", err)
	}
	s.lastPurgedAt = time.Now()
	total, _ := s.auditRepo.CountAll(ctx)
	pkg.SLogger().Infow("审计留存清理完成", "retention_days", retentionDays, "purged_count", purged, "total_count", total)
	return domain.AuditRetentionVO{
		RetentionDays:   retentionDays,
		TotalCount:      total,
		PurgedCount:     purged,
		LastPurgedAt:    pkg.FormatTime(s.lastPurgedAt),
		NextPurgeAtText: "每日 03:00 自动清理",
	}, nil
}

// retentionDays 读取留存天数配置（缺省 30，非法值回落默认）。
func (s *AuditService) retentionDays(ctx context.Context) int {
	if s.settingRepo == nil {
		return auditRetentionDefaultDays
	}
	item, err := s.settingRepo.GetByKey(ctx, settingKeyAuditRetentionDays)
	if err != nil || item == nil {
		return auditRetentionDefaultDays
	}
	days, ok := parseIntText(item.SettingValue)
	if !ok || days <= 0 {
		return auditRetentionDefaultDays
	}
	return days
}

// RunRetentionJob 阻塞运行每日审计清理任务，直到 ctx 取消（优雅退出）。
// 启动时先清一次（补上停机期间漏掉的清理），之后每天在固定时刻执行。
func (s *AuditService) RunRetentionJob(ctx context.Context) {
	s.purgeOnce(ctx)
	for {
		nextRun := nextPurgeTime(time.Now())
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

// nextPurgeTime 计算下一次清理时刻（本地时区固定 03:00）。
func nextPurgeTime(now time.Time) time.Time {
	next := time.Date(now.Year(), now.Month(), now.Day(), auditPurgeHourOfDay, 0, 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

// toAuditVO 组装审计视图对象（历史行缺级别时按 HTTP 状态推导，保证列表着色一致）。
func toAuditVO(do *domain.AuditLogDO) domain.AuditVO {
	levelCode := do.LogLevelCode
	if levelCode == "" && do.HTTPStatus > 0 {
		levelCode = pkg.AuditLevelOf(do.HTTPStatus)
	}
	return domain.AuditVO{
		AuditULID:     do.AuditULID,
		LogLevelCode:  levelCode,
		TraceID:       do.TraceID,
		UserULID:      do.UserULID,
		UserName:      do.UserName,
		ActionCode:    do.ActionCode,
		RequestPath:   do.RequestPath,
		RequestMethod: do.RequestMethod,
		TargetType:    do.TargetType,
		TargetULID:    do.TargetULID,
		RespCode:      do.RespCode,
		RespMessage:   do.RespMessage,
		HTTPStatus:    do.HTTPStatus,
		LatencyMs:     do.LatencyMs,
		IPAddr:        do.IPAddr,
		UserAgentText: do.UserAgentText,
		CreatedAt:     pkg.FormatTime(do.CreatedAt),
	}
}

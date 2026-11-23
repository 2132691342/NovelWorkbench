// 本文件实现审计日志业务：变更记录落库、分页查询与详情读取。

package service

import (
	"context"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// AuditService 审计日志业务：变更记录落库与分页查询。
type AuditService struct {
	auditRepo *repo.AuditRepo // 审计仓储
	userRepo  *repo.UserRepo  // 用户仓储
}

// NewAuditService 构造审计日志服务。
func NewAuditService(auditRepo *repo.AuditRepo, userRepo *repo.UserRepo) *AuditService {
	return &AuditService{auditRepo: auditRepo, userRepo: userRepo}
}

// Record 落库一条审计记录（自动补全操作用户登录名；pkg.AuditRecorder 实现）。
func (s *AuditService) Record(ctx context.Context, do *domain.AuditLogDO) error {
	do.AuditULID = pkg.NewULID(domain.IDPrefixAudit)
	if do.UserULID != "" && do.UserName == "" {
		userDO, err := s.userRepo.GetByULID(ctx, do.UserULID)
		if err != nil {
			return err
		}
		if userDO != nil {
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
		return domain.PageDTO[domain.AuditVO]{}, err
	}
	vos := make([]domain.AuditVO, 0, len(dos))
	for i := range dos {
		vos = append(vos, toAuditVO(&dos[i]))
	}
	return domain.NewPageDTO(total, req.PageREQ, vos), nil
}

// Detail 查询单条审计详情（含请求体原文）。
func (s *AuditService) Detail(ctx context.Context, req *domain.AuditDetailREQ) (domain.AuditDetailVO, error) {
	do, err := s.auditRepo.GetByULID(ctx, req.AuditULID)
	if err != nil {
		return domain.AuditDetailVO{}, err
	}
	if do == nil {
		return domain.AuditDetailVO{}, pkg.NewBizError(domain.RespCodeNotFound, "审计记录不存在")
	}
	return domain.AuditDetailVO{AuditVO: toAuditVO(do), RequestBody: do.RequestBody}, nil
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

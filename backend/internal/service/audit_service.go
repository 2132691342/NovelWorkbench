// 本文件实现审计日志业务：变更记录落库与分页查询。

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
		vos = append(vos, domain.AuditVO{
			AuditULID:   dos[i].AuditULID,
			UserULID:    dos[i].UserULID,
			UserName:    dos[i].UserName,
			ActionCode:  dos[i].ActionCode,
			TargetType:  dos[i].TargetType,
			TargetULID:  dos[i].TargetULID,
			DetailText:  dos[i].DetailText,
			IPAddr:      dos[i].IPAddr,
			CreatedAtMs: dos[i].CreatedAt.UnixMilli(),
		})
	}
	return domain.NewPageDTO(total, req.PageREQ, vos), nil
}

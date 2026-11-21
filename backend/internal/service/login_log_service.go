// 本文件实现登录日志业务：登录结果留痕与分页查询。

package service

import (
	"context"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// LoginLogService 登录日志业务。
type LoginLogService struct {
	loginLogRepo *repo.LoginLogRepo // 登录日志仓储
}

// NewLoginLogService 构造登录日志服务。
func NewLoginLogService(loginLogRepo *repo.LoginLogRepo) *LoginLogService {
	return &LoginLogService{loginLogRepo: loginLogRepo}
}

// Record 记录一次登录结果（失败只告警，不阻断登录链路）。
func (s *LoginLogService) Record(ctx context.Context, accountText string, userULID string, resultCode string, failReasonText string, meta domain.LoginMetaDTO) {
	do := &domain.LoginLogDO{
		LoginLogULID:     pkg.NewULID(domain.IDPrefixLoginLog),
		UserULID:         userULID,
		LoginAccountText: pkg.TruncateText(accountText, 128),
		LoginResultCode:  resultCode,
		FailReasonText:   pkg.TruncateText(failReasonText, 256),
		IPAddr:           pkg.TruncateText(meta.IPAddr, 64),
		UserAgentText:    pkg.TruncateText(meta.UserAgentText, 512),
	}
	if err := s.loginLogRepo.Create(ctx, do); err != nil {
		pkg.SLogger().Warnw("登录日志落库失败", "account", accountText, "error", err)
	}
}

// ListPage 分页查询登录日志。
func (s *LoginLogService) ListPage(ctx context.Context, req *domain.LoginLogListREQ) (domain.PageDTO[domain.LoginLogVO], error) {
	req.Normalize()
	dos, total, err := s.loginLogRepo.ListPage(ctx, req)
	if err != nil {
		return domain.PageDTO[domain.LoginLogVO]{}, err
	}
	vos := make([]domain.LoginLogVO, 0, len(dos))
	for i := range dos {
		vos = append(vos, domain.LoginLogVO{
			LoginLogULID:     dos[i].LoginLogULID,
			UserULID:         dos[i].UserULID,
			LoginAccountText: dos[i].LoginAccountText,
			LoginResultCode:  dos[i].LoginResultCode,
			FailReasonText:   dos[i].FailReasonText,
			IPAddr:           dos[i].IPAddr,
			UserAgentText:    dos[i].UserAgentText,
			CreatedAt:        pkg.FormatTime(dos[i].CreatedAt),
		})
	}
	return domain.NewPageDTO(total, req.PageREQ, vos), nil
}

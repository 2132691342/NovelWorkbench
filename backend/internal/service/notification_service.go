// 本文件实现站内通知业务：创建、分页查询、未读统计、标记已读与删除。

package service

import (
	"context"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// NotificationService 站内通知业务。
type NotificationService struct {
	notificationRepo *repo.NotificationRepo // 通知仓储
}

// NewNotificationService 构造通知服务。
func NewNotificationService(notificationRepo *repo.NotificationRepo) *NotificationService {
	return &NotificationService{notificationRepo: notificationRepo}
}

// Notify 创建一条通知（业务侧触发；标题与正文超长时截断）。
func (s *NotificationService) Notify(ctx context.Context, createDTO domain.NotificationCreateDTO) error {
	if createDTO.UserULID == "" {
		return nil // 无接收人（如项目已删除）时静默跳过
	}
	do := &domain.NotificationDO{
		NotificationULID:      pkg.NewULID(domain.IDPrefixNotification),
		UserULID:              createDTO.UserULID,
		NotificationTypeCode:  createDTO.NotificationTypeCode,
		NotificationTitleText: pkg.TruncateText(createDTO.NotificationTitleText, 128),
		NotificationBodyText:  pkg.TruncateText(createDTO.NotificationBodyText, 512),
		ReadFlag:              false,
		LinkURL:               pkg.TruncateText(createDTO.LinkURL, 256),
		RefTypeCode:           createDTO.RefTypeCode,
		RefULID:               createDTO.RefULID,
	}
	if do.NotificationTypeCode == "" {
		do.NotificationTypeCode = domain.NotificationTypeSystem
	}
	return s.notificationRepo.Create(ctx, do)
}

// ListPage 分页查询当前用户的通知。
func (s *NotificationService) ListPage(ctx context.Context, userULID string, req *domain.NotificationListREQ) (domain.PageDTO[domain.NotificationVO], error) {
	req.Normalize()
	if req.ReadStatusCode == "" {
		req.ReadStatusCode = domain.NotificationReadStatusAll
	}
	dos, total, err := s.notificationRepo.ListPage(ctx, userULID, req)
	if err != nil {
		return domain.PageDTO[domain.NotificationVO]{}, err
	}
	vos := make([]domain.NotificationVO, 0, len(dos))
	for i := range dos {
		vos = append(vos, toNotificationVO(&dos[i]))
	}
	return domain.NewPageDTO(total, req.PageREQ, vos), nil
}

// UnreadCount 查询当前用户未读通知数。
func (s *NotificationService) UnreadCount(ctx context.Context, userULID string) (domain.NotificationUnreadCountVO, error) {
	total, err := s.notificationRepo.CountUnread(ctx, userULID)
	if err != nil {
		return domain.NotificationUnreadCountVO{}, err
	}
	return domain.NotificationUnreadCountVO{UnreadCount: total}, nil
}

// MarkRead 批量标记本人通知已读。
func (s *NotificationService) MarkRead(ctx context.Context, userULID string, req *domain.NotificationReadREQ) error {
	return s.notificationRepo.MarkRead(ctx, userULID, req.NotificationULIDList)
}

// MarkAllRead 标记本人全部通知已读。
func (s *NotificationService) MarkAllRead(ctx context.Context, userULID string) error {
	return s.notificationRepo.MarkAllRead(ctx, userULID)
}

// Delete 删除本人单条通知。
func (s *NotificationService) Delete(ctx context.Context, userULID string, req *domain.NotificationDeleteREQ) error {
	return s.notificationRepo.Delete(ctx, userULID, req.NotificationULID)
}

// DeleteByUser 清理某用户全部通知（账号删除时调用）。
func (s *NotificationService) DeleteByUser(ctx context.Context, userULID string) error {
	return s.notificationRepo.DeleteByUser(ctx, userULID)
}

// toNotificationVO 组装通知视图对象。
func toNotificationVO(do *domain.NotificationDO) domain.NotificationVO {
	return domain.NotificationVO{
		NotificationULID:      do.NotificationULID,
		NotificationTypeCode:  do.NotificationTypeCode,
		NotificationTitleText: do.NotificationTitleText,
		NotificationBodyText:  do.NotificationBodyText,
		ReadFlag:              do.ReadFlag,
		LinkURL:               do.LinkURL,
		RefTypeCode:           do.RefTypeCode,
		RefULID:               do.RefULID,
		CreatedAt:             pkg.FormatTime(do.CreatedAt),
	}
}

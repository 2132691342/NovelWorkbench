// 本文件实现站内通知的持久化仓储。

package repo

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// NotificationRepo 站内通知持久化仓储。
type NotificationRepo struct {
	db *gorm.DB // 事务入口
}

// NewNotificationRepo 构造通知仓储。
func NewNotificationRepo(db *gorm.DB) *NotificationRepo {
	return &NotificationRepo{db: db}
}

// Create 插入通知记录。
func (r *NotificationRepo) Create(ctx context.Context, do *domain.NotificationDO) error {
	if err := r.db.WithContext(ctx).Create(do).Error; err != nil {
		return fmt.Errorf("create notification: %w", err)
	}
	return nil
}

// ListPage 分页查询某用户的通知（按时间倒序）。
func (r *NotificationRepo) ListPage(ctx context.Context, userULID string, req *domain.NotificationListREQ) ([]domain.NotificationDO, int64, error) {
	query := r.db.WithContext(ctx).Model(&domain.NotificationDO{}).Where("user_ulid = ?", userULID)
	switch req.ReadStatusCode {
	case domain.NotificationReadStatusUnread:
		query = query.Where("read_flag = ?", false)
	case domain.NotificationReadStatusRead:
		query = query.Where("read_flag = ?", true)
	}
	if req.NotificationTypeCode != "" {
		query = query.Where("notification_type_code = ?", req.NotificationTypeCode)
	}
	if req.KeywordText != "" {
		like := "%" + req.KeywordText + "%"
		query = query.Where("notification_title_text LIKE ? OR notification_body_text LIKE ?", like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count notifications: %w", err)
	}
	var dos []domain.NotificationDO
	if err := query.Order("created_at DESC").
		Offset((req.PageNum - 1) * req.PageSize).Limit(req.PageSize).
		Find(&dos).Error; err != nil {
		return nil, 0, fmt.Errorf("list notifications: %w", err)
	}
	return dos, total, nil
}

// CountUnread 统计某用户未读通知数。
func (r *NotificationRepo) CountUnread(ctx context.Context, userULID string) (int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&domain.NotificationDO{}).
		Where("user_ulid = ? AND read_flag = ?", userULID, false).
		Count(&total).Error; err != nil {
		return 0, fmt.Errorf("count unread notifications: %w", err)
	}
	return total, nil
}

// MarkRead 批量标记已读（仅作用于本人通知）。
func (r *NotificationRepo) MarkRead(ctx context.Context, userULID string, notificationULIDList []string) error {
	if len(notificationULIDList) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Model(&domain.NotificationDO{}).
		Where("user_ulid = ? AND notification_ulid IN ?", userULID, notificationULIDList).
		Update("read_flag", true).Error; err != nil {
		return fmt.Errorf("mark notifications read: %w", err)
	}
	return nil
}

// MarkAllRead 标记某用户全部通知已读。
func (r *NotificationRepo) MarkAllRead(ctx context.Context, userULID string) error {
	if err := r.db.WithContext(ctx).Model(&domain.NotificationDO{}).
		Where("user_ulid = ? AND read_flag = ?", userULID, false).
		Update("read_flag", true).Error; err != nil {
		return fmt.Errorf("mark all notifications read: %w", err)
	}
	return nil
}

// DeleteByUser 删除某用户全部通知（用户删除时清理，避免孤儿数据）。
func (r *NotificationRepo) DeleteByUser(ctx context.Context, userULID string) error {
	if err := r.db.WithContext(ctx).Where("user_ulid = ?", userULID).Delete(&domain.NotificationDO{}).Error; err != nil {
		return fmt.Errorf("delete notifications by user: %w", err)
	}
	return nil
}

// Delete 删除本人单条通知。
func (r *NotificationRepo) Delete(ctx context.Context, userULID string, notificationULID string) error {
	if err := r.db.WithContext(ctx).
		Where("user_ulid = ? AND notification_ulid = ?", userULID, notificationULID).
		Delete(&domain.NotificationDO{}).Error; err != nil {
		return fmt.Errorf("delete notification: %w", err)
	}
	return nil
}

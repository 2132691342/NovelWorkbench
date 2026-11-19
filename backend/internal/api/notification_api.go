// 本文件实现站内通知模块的 HTTP 处理器：列表、未读数、已读与删除。

package api

import (
	"github.com/gin-gonic/gin"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/service"
)

// NotificationAPI 站内通知模块 HTTP 处理器。
type NotificationAPI struct {
	svc *service.NotificationService
}

// NewNotificationAPI 构造通知处理器。
func NewNotificationAPI(s *service.NotificationService) *NotificationAPI {
	return &NotificationAPI{svc: s}
}

// List 分页查询本人通知（POST /notification/list）。
func (a *NotificationAPI) List(c *gin.Context) {
	var req domain.NotificationListREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "notification.list", bindErr(err))
		return
	}
	page, err := a.svc.ListPage(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req)
	if err != nil {
		respondErr(c, "notification.list", err)
		return
	}
	pkg.Ok(c, page)
}

// UnreadCount 查询本人未读通知数（GET /notification/unread_count）。
func (a *NotificationAPI) UnreadCount(c *gin.Context) {
	vo, err := a.svc.UnreadCount(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID))
	if err != nil {
		respondErr(c, "notification.unread_count", err)
		return
	}
	pkg.Ok(c, vo)
}

// Read 批量标记本人通知已读（POST /notification/read）。
func (a *NotificationAPI) Read(c *gin.Context) {
	var req domain.NotificationReadREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "notification.read", bindErr(err))
		return
	}
	if err := a.svc.MarkRead(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req); err != nil {
		respondErr(c, "notification.read", err)
		return
	}
	pkg.Ok(c, nil)
}

// ReadAll 标记本人全部通知已读（POST /notification/read_all）。
func (a *NotificationAPI) ReadAll(c *gin.Context) {
	if err := a.svc.MarkAllRead(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID)); err != nil {
		respondErr(c, "notification.read_all", err)
		return
	}
	pkg.Ok(c, nil)
}

// Delete 删除本人单条通知（POST /notification/delete）。
func (a *NotificationAPI) Delete(c *gin.Context) {
	var req domain.NotificationDeleteREQ
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErr(c, "notification.delete", bindErr(err))
		return
	}
	if err := a.svc.Delete(c.Request.Context(), c.GetString(pkg.ContextKeyUserULID), &req); err != nil {
		respondErr(c, "notification.delete", err)
		return
	}
	pkg.Ok(c, nil)
}

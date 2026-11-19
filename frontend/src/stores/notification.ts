// 通知状态：未读数与最近通知（顶栏铃铛与通知中心共用一份事实）。

import { defineStore } from 'pinia'
import {
  fetchUnreadCount,
  listNotifications,
  markAllNotificationsRead,
  markNotificationsRead,
  type NWNotificationVo,
} from '@/api/notification'

export const useNotificationStore = defineStore('notification', {
  state: () => ({
    unread_count: 0,
    latest_list: [] as NWNotificationVo[],
    loading_flag: false,
  }),
  actions: {
    /** 刷新未读数（轮询失败静默，不打断业务操作） */
    async refreshUnread(): Promise<void> {
      try {
        const unreadVo = await fetchUnreadCount({ silent_error: true })
        this.unread_count = unreadVo.unread_count ?? 0
      } catch {
        // 轮询失败保持旧值
      }
    },
    /** 刷新最近 5 条通知（顶栏浮层） */
    async refreshLatest(): Promise<void> {
      try {
        const pageData = await listNotifications(
          { page_num: 1, page_size: 5 },
          { silent_error: true },
        )
        this.latest_list = pageData.items ?? []
      } catch {
        // 轮询失败保持旧值
      }
    },
    /** 同时刷新未读数与最近通知 */
    async refreshAll(): Promise<void> {
      if (this.loading_flag) {
        return
      }
      this.loading_flag = true
      try {
        await Promise.all([this.refreshUnread(), this.refreshLatest()])
      } finally {
        this.loading_flag = false
      }
    },
    /** 标记单条已读（同时更新顶栏与列表缓存） */
    async markRead(notificationULID: string): Promise<void> {
      const target = this.latest_list.find(
        (notificationItem) => notificationItem.notification_ulid === notificationULID,
      )
      if (target?.read_flag) {
        return
      }
      await markNotificationsRead([notificationULID])
      if (target) {
        target.read_flag = true
        this.unread_count = Math.max(0, this.unread_count - 1)
      }
    },
    /** 全部标记已读 */
    async markAllRead(): Promise<void> {
      await markAllNotificationsRead()
      this.unread_count = 0
      for (const notificationItem of this.latest_list) {
        notificationItem.read_flag = true
      }
    },
    /** 清空本地通知状态（登出时） */
    reset(): void {
      this.unread_count = 0
      this.latest_list = []
      this.loading_flag = false
    },
  },
})

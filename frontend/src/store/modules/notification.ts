// 通知状态：未读数与最近通知。
import { defineStore } from 'pinia'
import { listNotifications, fetchUnreadCount } from '@/api/notification'
import { store } from '../index'

export interface NotificationState {
  unreadCount: number
  latestTitle: string
}

/** 通知状态：顶栏铃铛的未读数与最近一条提示（轮询与操作后刷新共用）。 */
export const useNotificationStore = defineStore('notification', {
  state: (): NotificationState => ({
    unreadCount: 0,
    latestTitle: ''
  }),
  actions: {
    async refreshUnreadCount() {
      try {
        const result = await fetchUnreadCount()
        this.unreadCount = result.unread_count ?? 0
      } catch {
        // 未读数失败静默：不打扰写作
      }
    },
    async refreshLatest() {
      try {
        const page = await listNotifications({ page_num: 1, page_size: 1 })
        this.latestTitle = page.items[0]?.notification_title_text ?? ''
      } catch {
        this.latestTitle = ''
      }
    }
  }
})

export const useNotificationStoreWithOut = () => useNotificationStore(store)

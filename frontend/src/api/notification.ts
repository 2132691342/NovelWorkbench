// 站内通知接口封装（列表、未读数、已读与删除；字段对齐后端 notification 契约）
import { get, post, type NWRequestConfig } from '@/api/http'
import type { NWPageDto } from './contracts'

/** 通知视图对象（字段对齐后端 NotificationVO） */
export interface NWNotificationVo {
  notification_ulid: string
  notification_type_code: string
  notification_title_text: string
  notification_body_text: string
  read_flag: boolean
  link_url: string
  ref_type_code: string
  ref_ulid: string
  created_at_ms: number
}

/** 通知分页查询请求（字段对齐后端 NotificationListREQ） */
export interface NWNotificationListReqVo {
  page_num: number
  page_size: number
  read_status_code?: 'all' | 'unread' | 'read'
  notification_type_code?: string
  keyword_text?: string
}

/** 未读数视图（字段对齐后端 NotificationUnreadCountVO） */
export interface NWNotificationUnreadCountVo {
  unread_count: number
}

/** 通知分页列表 */
export function listNotifications(
  req: NWNotificationListReqVo,
  config?: NWRequestConfig,
): Promise<NWPageDto<NWNotificationVo>> {
  return post<NWPageDto<NWNotificationVo>>('/notification/list', req, config)
}

/** 未读通知数 */
export function fetchUnreadCount(config?: NWRequestConfig): Promise<NWNotificationUnreadCountVo> {
  return get<NWNotificationUnreadCountVo>('/notification/unread_count', config)
}

/** 批量标记已读 */
export function markNotificationsRead(notificationUlidList: string[]): Promise<null> {
  return post<null>('/notification/read', { notification_ulid_list: notificationUlidList })
}

/** 全部标记已读 */
export function markAllNotificationsRead(): Promise<null> {
  return post<null>('/notification/read_all')
}

/** 删除单条通知 */
export function deleteNotification(notificationUlid: string): Promise<null> {
  return post<null>('/notification/delete', { notification_ulid: notificationUlid })
}

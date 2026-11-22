// 本地存储读写：访问令牌（记住我 → localStorage，否则 sessionStorage）与用户信息校验。
import type { NWUserVo } from '@/api/contracts'

const TOKEN_KEY = 'nw_access_token'
const USER_KEY = 'nw_user_vo'

/** 读取访问令牌（会话级优先，其次持久级；SSE 与请求头共用同一来源） */
export function readToken(): string {
  return sessionStorage.getItem(TOKEN_KEY) ?? localStorage.getItem(TOKEN_KEY) ?? ''
}

/** 写入访问令牌：remember=true 持久化，false 只保留到关闭标签页 */
export function writeToken(token: string, remember = true): void {
  if (remember) {
    localStorage.setItem(TOKEN_KEY, token)
    sessionStorage.removeItem(TOKEN_KEY)
    return
  }
  sessionStorage.setItem(TOKEN_KEY, token)
  localStorage.removeItem(TOKEN_KEY)
}

/** 清除访问令牌（两级存储同时清理，避免残留） */
export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY)
  sessionStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(USER_KEY)
}

/** 校验持久化的用户信息结构（外部输入先校验再消费） */
export function isUserVO(value: unknown): value is NWUserVo {
  if (typeof value !== 'object' || value === null) return false
  const record = value as Record<string, unknown>
  return typeof record.user_ulid === 'string' && typeof record.user_name === 'string'
}

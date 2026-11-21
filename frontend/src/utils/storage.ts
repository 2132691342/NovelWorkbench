// 本地存储读写：访问令牌与项目选择的持久化。
import type { NWUserVo } from '@/api/contracts'

const TOKEN_KEY = 'nw_access_token'

/** 读取访问令牌（SSE 与请求头共用同一来源） */
export function readToken(): string {
  return localStorage.getItem(TOKEN_KEY) ?? ''
}

/** 写入访问令牌 */
export function writeToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token)
}

/** 清除访问令牌 */
export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY)
}

/** 校验持久化的用户信息结构（外部输入先校验再消费） */
export function isUserVO(value: unknown): value is NWUserVo {
  if (typeof value !== 'object' || value === null) return false
  const record = value as Record<string, unknown>
  return typeof record.user_ulid === 'string' && typeof record.user_name === 'string'
}

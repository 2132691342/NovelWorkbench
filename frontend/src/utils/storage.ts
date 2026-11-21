// 本地存储封装：访问令牌、用户信息、语言与项目选中态的读写。

import type { NWUserVo } from '@/api/contracts'

// 本地存储键名（token、用户信息、界面偏好）
export const NW_TOKEN_KEY = 'nw_access_token'
export const NW_USER_VO_KEY = 'nw_user_vo'
export const NW_LOCALE_KEY = 'nw_locale'
export const NW_SIDEBAR_COLLAPSED_KEY = 'nw_sidebar_collapsed'
export const NW_THEME_KEY = 'nw_theme'

// 支持的语言编码
export type NWLocaleCode = 'zh-CN' | 'en-US'

// 主题编码（system 跟随系统偏好）
export type NWThemeCode = 'light' | 'dark' | 'system'

/** 读取登录 token */
export function readToken(): string {
  return localStorage.getItem(NW_TOKEN_KEY) ?? ''
}

/** 写入登录 token */
export function writeToken(token: string): void {
  localStorage.setItem(NW_TOKEN_KEY, token)
}

/** 清除登录 token */
export function clearToken(): void {
  localStorage.removeItem(NW_TOKEN_KEY)
}

/** 校验持久化的用户信息结构（外部输入需先校验再消费） */
function isUserVo(value: unknown): value is NWUserVo {
  return (
    typeof value === 'object' &&
    value !== null &&
    'user_ulid' in value &&
    typeof value.user_ulid === 'string' &&
    'user_name' in value &&
    typeof value.user_name === 'string' &&
    'user_email' in value &&
    typeof value.user_email === 'string' &&
    'avatar_url' in value &&
    typeof value.avatar_url === 'string' &&
    'created_at' in value &&
    typeof value.created_at === 'string'
  )
}

/** 读取持久化的用户信息，结构非法时返回 null */
export function readUserVo(): NWUserVo | null {
  const rawText = localStorage.getItem(NW_USER_VO_KEY)
  if (!rawText) {
    return null
  }
  try {
    const parsed: unknown = JSON.parse(rawText)
    return isUserVo(parsed) ? parsed : null
  } catch {
    return null
  }
}

/** 写入用户信息（JSON 序列化持久化） */
export function writeUserVo(userVo: NWUserVo): void {
  localStorage.setItem(NW_USER_VO_KEY, JSON.stringify(userVo))
}

/** 清除用户信息 */
export function clearUserVo(): void {
  localStorage.removeItem(NW_USER_VO_KEY)
}

/** 读取界面语言，非法值回落到 zh-CN */
export function readLocale(): NWLocaleCode {
  const savedValue = localStorage.getItem(NW_LOCALE_KEY)
  if (savedValue === 'zh-CN' || savedValue === 'en-US') {
    return savedValue
  }
  return 'zh-CN'
}

/** 写入界面语言 */
export function writeLocale(localeCode: NWLocaleCode): void {
  localStorage.setItem(NW_LOCALE_KEY, localeCode)
}

/** 读取侧边栏折叠状态 */
export function readSidebarCollapsed(): boolean {
  return localStorage.getItem(NW_SIDEBAR_COLLAPSED_KEY) === '1'
}

/** 写入侧边栏折叠状态 */
export function writeSidebarCollapsed(collapsed: boolean): void {
  localStorage.setItem(NW_SIDEBAR_COLLAPSED_KEY, collapsed ? '1' : '0')
}

/** 读取主题偏好（非法值回落到亮色） */
export function readTheme(): NWThemeCode {
  const storedTheme = localStorage.getItem(NW_THEME_KEY)
  if (storedTheme === 'dark' || storedTheme === 'system') {
    return storedTheme
  }
  return 'light'
}

/** 写入主题偏好 */
export function writeTheme(themeCode: NWThemeCode): void {
  localStorage.setItem(NW_THEME_KEY, themeCode)
}

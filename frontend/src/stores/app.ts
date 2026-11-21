// 应用状态：界面语言、侧栏折叠与主题，均持久化到 localStorage。

import { defineStore } from 'pinia'
import { applyLocale } from '@/i18n'
import {
  type NWThemeCode,
  readLocale,
  readSidebarCollapsed,
  readTheme,
  writeSidebarCollapsed,
  writeTheme,
  type NWLocaleCode,
} from '@/utils/storage'
export type { NWThemeCode }

/** 系统暗色偏好媒体查询 */
const darkMediaQuery =
  typeof window !== 'undefined' ? window.matchMedia('(prefers-color-scheme: dark)') : null

/** 计算实际生效主题（system 时读系统偏好） */
function resolveThemeCode(themeCode: NWThemeCode): 'light' | 'dark' {
  if (themeCode === 'system') {
    return darkMediaQuery?.matches ? 'dark' : 'light'
  }
  return themeCode
}

/** 把主题写到 <html data-theme>，Element Plus 暗色变量据此生效 */
function applyTheme(themeCode: NWThemeCode): 'light' | 'dark' {
  const resolvedTheme = resolveThemeCode(themeCode)
  document.documentElement.setAttribute('data-theme', resolvedTheme)
  document.documentElement.classList.toggle('dark', resolvedTheme === 'dark')
  return resolvedTheme
}

export const useAppStore = defineStore('app', {
  state: () => ({
    locale_code: readLocale(),
    sidebar_collapsed: readSidebarCollapsed(),
    theme_code: readTheme(),
    resolved_theme_code: resolveThemeCode(readTheme()),
  }),
  actions: {
    /** 切换侧边栏折叠状态并持久化 */
    toggleSidebar(): void {
      this.sidebar_collapsed = !this.sidebar_collapsed
      writeSidebarCollapsed(this.sidebar_collapsed)
    },
    /** 切换界面语言（同步 vue-i18n 并持久化） */
    setLocale(localeCode: NWLocaleCode): void {
      this.locale_code = localeCode
      applyLocale(localeCode)
    },
    /** 设置侧边栏折叠状态（快捷键入口复用） */
    setSidebarCollapsed(collapsedFlag: boolean): void {
      this.sidebar_collapsed = collapsedFlag
      writeSidebarCollapsed(collapsedFlag)
    },
    /** 切换主题并持久化 */
    setTheme(themeCode: NWThemeCode): void {
      this.theme_code = themeCode
      this.resolved_theme_code = applyTheme(themeCode)
      writeTheme(themeCode)
    },
    /** 初始化主题（应用启动时调用一次） */
    initTheme(): void {
      this.resolved_theme_code = applyTheme(this.theme_code)
      darkMediaQuery?.addEventListener('change', () => {
        if (this.theme_code === 'system') {
          this.resolved_theme_code = applyTheme('system')
        }
      })
    },
  },
})

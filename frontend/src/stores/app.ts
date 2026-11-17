import { defineStore } from 'pinia'
import { applyLocale } from '@/i18n'
import {
  readLocale,
  readSidebarCollapsed,
  writeSidebarCollapsed,
  type NWLocaleCode,
} from '@/utils/storage'

export const useAppStore = defineStore('app', {
  state: () => ({
    locale_code: readLocale(),
    sidebar_collapsed: readSidebarCollapsed(),
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
  },
})

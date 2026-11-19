// 国际化实例：装配 zh-CN / en-US 语言包与本地化切换逻辑。

import { createI18n } from 'vue-i18n'
import zhCN from './locales/zh-CN'
import enUS from './locales/en-US'
import { readLocale, writeLocale, type NWLocaleCode } from '@/utils/storage'

/** 语言选项（供界面切换使用） */
export const NW_LOCALE_OPTIONS: Array<{ locale_code: NWLocaleCode; locale_label: string }> = [
  { locale_code: 'zh-CN', locale_label: '简体中文' },
  { locale_code: 'en-US', locale_label: 'English' },
]

const i18n = createI18n({
  legacy: false,
  locale: readLocale(),
  fallbackLocale: 'zh-CN',
  messages: {
    'zh-CN': zhCN,
    'en-US': enUS,
  },
})

/** 切换界面语言（同时持久化） */
export function applyLocale(localeCode: NWLocaleCode): void {
  i18n.global.locale.value = localeCode
  writeLocale(localeCode)
}

export type { NWLocaleCode }
export default i18n

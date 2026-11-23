// 语言注册表：登记可用语言与 Element Plus 语言包。
import zhCNMessages from '@/locales/zh-CN'
import zhCNElement from 'element-plus/es/locale/lang/zh-cn'

/**
 * 语言注册表：当前只交付简体中文。
 * 业务文案来自数据库（中文），界面壳文案在 locales/zh-CN.ts；
 * 新增语言时在此登记，并把缺失键补齐后再开放切换入口。
 */
export const localeRegistry = {
  'zh-CN': {
    label: '简体中文',
    messages: zhCNMessages,
    elementLocale: zhCNElement
  }
} as const

export type LocaleCode = keyof typeof localeRegistry

export const DEFAULT_LOCALE: LocaleCode = 'zh-CN'

export const isLocale = (value: unknown): value is LocaleCode =>
  typeof value === 'string' && value in localeRegistry

export const localeOptions = (Object.keys(localeRegistry) as LocaleCode[]).map((value) => ({
  value,
  label: localeRegistry[value].label
}))

export const localeMessages = Object.fromEntries(
  (Object.keys(localeRegistry) as LocaleCode[]).map((locale) => [
    locale,
    localeRegistry[locale].messages
  ])
)

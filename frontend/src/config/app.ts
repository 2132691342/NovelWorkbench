// 应用级常量：名称、默认主题与分页尺寸等。
export const appConfig = {
  title: import.meta.env.VITE_APP_TITLE,
  ui: {
    breadcrumb: true,
    breadcrumbIcon: true,
    hamburger: true,
    theme: true,
    locale: true,
    tagsViewIcon: true,
    footer: true,
    uniqueOpened: false
  }
} as const

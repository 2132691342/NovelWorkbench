/** 是否为外部链接（导航项直接 window.open） */
export const isUrl = (path: string) => /^(?:https?:|mailto:|tel:)/.test(path)

/** 拼接父子路由路径，消除重复斜杠 */
export const pathResolve = (parentPath: string, path: string) => {
  if (isUrl(path) || path.startsWith('/')) return path
  return `${parentPath}/${path}`.replace(/\/+/g, '/')
}

/** 业务外壳组件（常量路由的父级） */
export const Layout = () => import('@/layout/Layout.vue')

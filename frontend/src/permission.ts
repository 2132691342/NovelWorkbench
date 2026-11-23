// 全局路由守卫：登录态、后端导航 → 动态路由注册、页面权限码兜底与项目作用域路径补全。

import type { RouteRecordRaw } from 'vue-router'
import NProgress from 'nprogress'
import router from './router'
import { NO_REDIRECT_WHITE_LIST } from '@/constants'
import { useAppStoreWithOut } from '@/store/modules/app'
import { usePermissionStoreWithOut } from '@/store/modules/permission'
import { useUserStoreWithOut } from '@/store/modules/user'
import { useProjectStoreWithOut } from '@/store/modules/project'
import 'nprogress/nprogress.css'

NProgress.configure({ showSpinner: false })

/**
 * 动态路由注册：后端导航树 → 路由记录 → router.addRoute。
 * 已注册过返回 false；本次新注册返回 true（调用方需重新导航一次，否则首帧会落到 Fallback）。
 */
export const ensureDynamicRoutes = async (): Promise<boolean> => {
  const permissionStore = usePermissionStoreWithOut()
  if (permissionStore.isAddRouters) return false
  const userStore = useUserStoreWithOut()
  const nav = await userStore.loadNav()
  const routeList = permissionStore.generateRoutes(nav.menu_tree_list ?? [])
  routeList.forEach((route) => {
    router.addRoute('Root', route as unknown as RouteRecordRaw)
  })
  permissionStore.setIsAddRouters(true)
  return true
}

/** 项目作用域页面路径仍带 :project_ulid 占位时，替换为当前项目（否则原样放行，避免自我重定向） */
const resolveProjectScope = async (to: { path: string; meta: Record<string, unknown> }) => {
  if (!to.meta.projectScope || !to.path.includes(':project_ulid')) return null
  const projectStore = useProjectStoreWithOut()
  if (projectStore.projectOptions.length === 0) await projectStore.loadProjectOptions()
  const projectUlid = projectStore.currentProjectUlid || projectStore.projectOptions[0]?.project_ulid
  if (!projectUlid) return null
  return projectUlid
}

export const setupPermission = async () => {
  const userStore = useUserStoreWithOut()

  if (userStore.isAuthenticated) {
    try {
      await ensureDynamicRoutes()
    } catch {
      userStore.clearSession()
    }
  }

  router.beforeEach(async (to) => {
    NProgress.start()
    useAppStoreWithOut().pageLoading = true

    if (!userStore.isAuthenticated) {
      if (NO_REDIRECT_WHITE_LIST.includes(to.path)) return true
      return { path: '/login', query: { redirect: to.fullPath } }
    }

    if (to.path === '/login') return { path: '/' }

    try {
      const routesAdded = await ensureDynamicRoutes()
      if (routesAdded) return { path: to.fullPath, replace: true }
      if (userStore.userInfo === undefined) await userStore.fetchProfile()
    } catch {
      userStore.clearSession()
      return { path: '/login', query: { redirect: to.fullPath } }
    }

    // 项目作用域页面：路径里的占位符在导航层已替换，这里兜住直接输 URL 的情况
    const projectUlid = await resolveProjectScope(to as never)
    if (projectUlid) {
      const replaced = to.fullPath.replace('/:project_ulid', `/${projectUlid}`)
      return { path: replaced, replace: true }
    }

    const permCode = String(to.meta.permCode ?? '')
    if (permCode && !userStore.hasPerm(permCode)) {
      const permissionStore = usePermissionStoreWithOut()
      const firstPage = permissionStore.addRouters
        .flatMap((route) => [route, ...(route.children ?? [])])
        .find((route) => route.meta && !route.meta.hidden && route.component)
      return { path: firstPage?.path ?? '/404', replace: true }
    }

    if (to.name === 'Fallback') return { path: '/404', replace: true }
    return true
  })

  router.afterEach(() => {
    NProgress.done()
    useAppStoreWithOut().pageLoading = false
  })
}

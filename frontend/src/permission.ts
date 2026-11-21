// 全局路由守卫：登录态、导航加载与页面权限码校验。
import NProgress from 'nprogress'
import router from './router'
import { NO_REDIRECT_WHITE_LIST } from '@/constants'
import { useAppStoreWithOut } from '@/store/modules/app'
import { useUserStoreWithOut } from '@/store/modules/user'
import { useMenuStoreWithOut } from '@/store/modules/menu'
import 'nprogress/nprogress.css'

NProgress.configure({ showSpinner: false })

/**
 * 页面权限校验：路由 meta.permCode 命中用户权限码即放行。
 * 菜单可见性由后端导航树决定，这里是直接输 URL 时的兜底门禁。
 */
const ensureNavLoaded = async () => {
  const menuStore = useMenuStoreWithOut()
  if (!menuStore.navLoaded) await menuStore.loadNav()
}

export const setupPermission = async () => {
  const userStore = useUserStoreWithOut()

  if (userStore.isAuthenticated) {
    try {
      await ensureNavLoaded()
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
      await ensureNavLoaded()
      if (userStore.userInfo === undefined) await userStore.fetchProfile()
    } catch {
      userStore.clearSession()
      return { path: '/login', query: { redirect: to.fullPath } }
    }

    const permCode = String(to.meta.permCode ?? '')
    if (permCode && !userStore.hasPerm(permCode)) return { path: '/', replace: true }

    return true
  })

  router.afterEach(() => {
    NProgress.done()
    useAppStoreWithOut().pageLoading = false
  })
}

// 路由表：静态声明全部页面组件，菜单只控制可见性。
import { createRouter, createWebHashHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import type { App } from 'vue'
import { Layout } from '@/utils/routerHelper'
import { NO_RESET_WHITE_LIST } from '@/constants'

/** 常量路由：登录、重定向、404 与整个业务壳（页面组件静态声明，后端菜单只控制可见性）。 */
export const constantRouterMap: AppRouteRecordRaw[] = [
  {
    path: '/login',
    component: () => import('@/views/Login/Login.vue'),
    name: 'Login',
    meta: { hidden: true, title: 'router.login', noTagsView: true }
  },
  {
    path: '/',
    component: Layout,
    redirect: '/dashboard',
    name: 'Root',
    meta: { hidden: true },
    children: [
      {
        path: 'dashboard',
        name: 'Dashboard',
        component: () => import('@/views/Dashboard/index.vue'),
        meta: { title: 'router.dashboard', icon: 'mdi:view-dashboard', affix: true }
      },
      {
        path: 'workspace/:project_ulid',
        name: 'Workspace',
        component: () => import('@/views/Workspace/index.vue'),
        meta: { title: 'router.workspace', icon: 'mdi:book-open-page-variant-outline' }
      },
      {
        path: 'outline/:project_ulid',
        name: 'Outline',
        component: () => import('@/views/Outline/index.vue'),
        meta: { title: 'router.outline', icon: 'mdi:file-tree' }
      },
      {
        path: 'continuity/:project_ulid',
        name: 'Continuity',
        component: () => import('@/views/Continuity/index.vue'),
        meta: { title: 'router.continuity', icon: 'mdi:shield-check-outline' }
      },
      {
        path: 'character/:project_ulid',
        name: 'Character',
        component: () => import('@/views/Character/index.vue'),
        meta: { title: 'router.character', icon: 'mdi:account-group' }
      },
      {
        path: 'lore/:project_ulid',
        name: 'Lore',
        component: () => import('@/views/Lore/index.vue'),
        meta: { title: 'router.lore', icon: 'mdi:earth' }
      },
      {
        path: 'knowledge/:project_ulid',
        name: 'Knowledge',
        component: () => import('@/views/Knowledge/index.vue'),
        meta: { title: 'router.knowledge', icon: 'mdi:database-outline' }
      },
      {
        path: 'notes/:project_ulid',
        name: 'Notes',
        component: () => import('@/views/Notes/index.vue'),
        meta: { title: 'router.notes', icon: 'mdi:notebook-outline' }
      },
      {
        path: 'models',
        name: 'Models',
        component: () => import('@/views/Models/index.vue'),
        meta: { title: 'router.models', icon: 'mdi:robot-outline', permCode: 'system:model:manage' }
      },
      {
        path: 'skills',
        name: 'Skills',
        component: () => import('@/views/Skills/index.vue'),
        meta: { title: 'router.skills', icon: 'mdi:magic-staff', permCode: 'skill:manage' }
      },
      {
        path: 'prompts',
        name: 'Prompts',
        component: () => import('@/views/Prompts/index.vue'),
        meta: { title: 'router.prompts', icon: 'mdi:script-text-outline', permCode: 'prompt:manage' }
      },
      {
        path: 'notifications',
        name: 'Notifications',
        component: () => import('@/views/Notifications/index.vue'),
        meta: { title: 'router.notifications', icon: 'mdi:bell-outline' }
      },
      {
        path: 'profile',
        name: 'Profile',
        component: () => import('@/views/Profile/index.vue'),
        meta: { title: 'router.profile', icon: 'mdi:account-circle-outline', hidden: true }
      },
      {
        path: 'admin/users',
        name: 'AdminUsers',
        component: () => import('@/views/Admin/Users.vue'),
        meta: {
          title: 'router.adminUsers',
          icon: 'mdi:account-multiple-outline',
          permCode: 'system:user:manage'
        }
      },
      {
        path: 'admin/roles',
        name: 'AdminRoles',
        component: () => import('@/views/Admin/Roles.vue'),
        meta: { title: 'router.adminRoles', icon: 'mdi:key-outline', permCode: 'system:role:manage' }
      },
      {
        path: 'admin/menus',
        name: 'AdminMenus',
        component: () => import('@/views/Admin/Menus.vue'),
        meta: { title: 'router.adminMenus', icon: 'mdi:menu', permCode: 'system:menu:manage' }
      },
      {
        path: 'admin/permissions',
        name: 'AdminPermissions',
        component: () => import('@/views/Admin/Permissions.vue'),
        meta: {
          title: 'router.adminPermissions',
          icon: 'mdi:lock-outline',
          permCode: 'system:perm:read'
        }
      },
      {
        path: 'admin/audit',
        name: 'AdminAudit',
        component: () => import('@/views/Admin/Audit.vue'),
        meta: { title: 'router.adminAudit', icon: 'mdi:history', permCode: 'system:audit:read' }
      },
      {
        path: 'admin/login-logs',
        name: 'AdminLoginLogs',
        component: () => import('@/views/Admin/LoginLogs.vue'),
        meta: {
          title: 'router.adminLoginLogs',
          icon: 'mdi:login-variant',
          permCode: 'system:loginlog:read'
        }
      },
      {
        path: 'admin/jobs',
        name: 'AdminJobs',
        component: () => import('@/views/Admin/Jobs.vue'),
        meta: { title: 'router.adminJobs', icon: 'mdi:progress-clock', permCode: 'system:job:manage' }
      },
      {
        path: 'admin/usage',
        name: 'AdminUsage',
        component: () => import('@/views/Admin/Usage.vue'),
        meta: {
          title: 'router.adminUsage',
          icon: 'mdi:chart-box-outline',
          permCode: 'system:usage:read'
        }
      },
      {
        path: 'admin/status',
        name: 'AdminStatus',
        component: () => import('@/views/Admin/Status.vue'),
        meta: { title: 'router.adminStatus', icon: 'mdi:pulse', permCode: 'system:status:read' }
      },
      {
        path: 'settings',
        name: 'Settings',
        component: () => import('@/views/Settings/index.vue'),
        meta: {
          title: 'router.settings',
          icon: 'mdi:cog-outline',
          permCode: 'system:setting:manage'
        }
      },
      {
        path: 'redirect/:path(.*)',
        name: 'Redirect',
        component: () => import('@/views/Redirect/Redirect.vue'),
        meta: { hidden: true, noTagsView: true, noCache: true }
      },
      {
        path: ':pathMatch(.*)*',
        name: 'Fallback',
        component: () => import('@/views/Error/404.vue'),
        meta: { hidden: true, noTagsView: true }
      }
    ]
  }
]

const router = createRouter({
  history: createWebHashHistory(),
  routes: constantRouterMap as RouteRecordRaw[],
  scrollBehavior: () => ({ left: 0, top: 0 })
})

/** 移除动态注册的路由（登出时调用，保留常量路由） */
export const resetRouter = (): void => {
  router.getRoutes().forEach((route) => {
    const { name } = route
    if (name && !NO_RESET_WHITE_LIST.includes(name as string) && router.hasRoute(name)) {
      router.removeRoute(name)
    }
  })
}

export const setupRouter = (app: App<Element>) => {
  app.use(router)
}

export default router

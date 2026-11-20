// 路由表与守卫：页面路由、登录守卫与导航权限初始化。

import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { readToken } from '@/utils/storage'
import { useMenuStore } from '@/stores/menu'

/** 免鉴权页面 */
const PUBLIC_PAGE_PATH_LIST = ['/login']

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'LoginView',
    component: () => import('@/views/login/index.vue'),
    meta: { public_page_flag: true },
  },
  {
    path: '/',
    component: () => import('@/layout/AdminLayout.vue'),
    children: [
      {
        path: '',
        name: 'DashboardView',
        component: () => import('@/views/dashboard/index.vue'),
        meta: { menu_code: 'dashboard' },
      },
      {
        path: 'settings',
        name: 'SettingsView',
        component: () => import('@/views/settings/index.vue'),
        meta: { menu_code: 'settings', perm_code: 'system:setting:manage' },
      },
      {
        path: 'skills',
        name: 'SkillAdminView',
        component: () => import('@/views/skills/index.vue'),
        meta: { menu_code: 'skills', perm_code: 'skill:manage' },
      },
      {
        path: 'prompts',
        name: 'PromptProfileView',
        component: () => import('@/views/prompts/index.vue'),
        meta: { menu_code: 'prompts', perm_code: 'prompt:manage' },
      },
      {
        path: 'models',
        name: 'ModelProviderView',
        component: () => import('@/views/models/index.vue'),
        meta: { menu_code: 'models' },
      },
      {
        path: 'profile',
        name: 'ProfileView',
        component: () => import('@/views/profile/index.vue'),
        meta: { menu_code: 'profile' },
      },
      {
        path: 'notifications',
        name: 'NotificationCenterView',
        component: () => import('@/views/notification/index.vue'),
        meta: { menu_code: 'notifications' },
      },
      {
        path: 'admin/users',
        name: 'AdminUsersView',
        component: () => import('@/views/admin/users.vue'),
        meta: { menu_code: 'admin_users', perm_code: 'system:user:manage' },
      },
      {
        path: 'admin/roles',
        name: 'AdminRolesView',
        component: () => import('@/views/admin/roles.vue'),
        meta: { menu_code: 'admin_roles', perm_code: 'system:role:manage' },
      },
      {
        path: 'admin/menus',
        name: 'AdminMenusView',
        component: () => import('@/views/admin/menus.vue'),
        meta: { menu_code: 'admin_menus', perm_code: 'system:menu:manage' },
      },
      {
        path: 'admin/permissions',
        name: 'AdminPermissionsView',
        component: () => import('@/views/admin/permissions.vue'),
        meta: { menu_code: 'admin_perms', perm_code: 'system:perm:read' },
      },
      {
        path: 'admin/audit',
        name: 'AdminAuditView',
        component: () => import('@/views/admin/audit.vue'),
        meta: { menu_code: 'admin_audit', perm_code: 'system:audit:read' },
      },
      {
        path: 'admin/login-logs',
        name: 'AdminLoginLogView',
        component: () => import('@/views/admin/login-logs.vue'),
        meta: { menu_code: 'admin_login_log', perm_code: 'system:loginlog:read' },
      },
      {
        path: 'admin/jobs',
        name: 'AdminJobView',
        component: () => import('@/views/admin/jobs.vue'),
        meta: { menu_code: 'admin_jobs', perm_code: 'system:job:manage' },
      },
      {
        path: 'admin/usage',
        name: 'AdminUsageView',
        component: () => import('@/views/admin/usage.vue'),
        meta: { menu_code: 'admin_usage', perm_code: 'system:usage:read' },
      },
      {
        path: 'admin/status',
        name: 'AdminStatusView',
        component: () => import('@/views/admin/status.vue'),
        meta: { menu_code: 'admin_status', perm_code: 'system:status:read' },
      },
      {
        path: 'workspace/:project_ulid',
        name: 'WorkspaceView',
        component: () => import('@/views/workspace/index.vue'),
        meta: { menu_code: 'workspace', project_scope_flag: true },
      },
      {
        path: 'knowledge/:project_ulid',
        name: 'KnowledgeView',
        component: () => import('@/views/knowledge/index.vue'),
        meta: { menu_code: 'knowledge', project_scope_flag: true },
      },
      {
        path: 'character/:project_ulid',
        name: 'CharacterView',
        component: () => import('@/views/character/index.vue'),
        meta: { menu_code: 'character', project_scope_flag: true },
      },
      {
        path: 'notes/:project_ulid',
        name: 'NotesView',
        component: () => import('@/views/notes/index.vue'),
        meta: { menu_code: 'notes', project_scope_flag: true },
      },
      {
        path: 'outline/:project_ulid',
        name: 'OutlineView',
        component: () => import('@/views/outline/index.vue'),
        meta: { menu_code: 'outline', project_scope_flag: true },
      },
      {
        path: 'continuity/:project_ulid',
        name: 'ContinuityView',
        component: () => import('@/views/continuity/index.vue'),
        meta: { menu_code: 'continuity', project_scope_flag: true },
      },
      {
        path: 'lore/:project_ulid',
        name: 'LoreView',
        component: () => import('@/views/lore/index.vue'),
        meta: { menu_code: 'lore', project_scope_flag: true },
      },
    ],
  },
  {
    path: '/:pathMatch(.*)*',
    redirect: '/',
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

/**
 * 路由守卫：未登录跳登录页；已登录访问登录页回首页；
 * 首次进入业务页时拉取导航（菜单 + 权限码），并按 meta.perm_code 校验页面权限。
 */
router.beforeEach(async (to) => {
  const token = readToken()
  if (PUBLIC_PAGE_PATH_LIST.includes(to.path)) {
    return token ? { path: '/' } : true
  }
  if (!token) {
    return { path: '/login' }
  }
  const menuStore = useMenuStore()
  if (!menuStore.nav_loaded) {
    await menuStore.loadNav()
  }
  const permCode = String(to.meta.perm_code ?? '')
  if (permCode && !menuStore.hasPerm(permCode)) {
    return { path: '/' }
  }
  return true
})

export default router

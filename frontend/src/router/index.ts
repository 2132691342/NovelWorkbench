// 路由表与守卫：定义页面路由与登录鉴权守卫。

import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { readToken } from '@/utils/storage'

// 免鉴权页面
const PUBLIC_PAGE_PATH_LIST = ['/login']

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'LoginView',
    component: () => import('@/views/login/index.vue'),
  },
  {
    path: '/',
    component: () => import('@/layout/AdminLayout.vue'),
    children: [
      {
        path: '',
        name: 'DashboardView',
        component: () => import('@/views/dashboard/index.vue'),
        meta: { menu_key: 'dashboard' },
      },
      {
        path: 'settings',
        name: 'SettingsView',
        component: () => import('@/views/settings/index.vue'),
        meta: { menu_key: 'settings' },
      },
      {
        path: 'slash-admin',
        name: 'SlashAdminView',
        component: () => import('@/views/slash-admin/index.vue'),
        meta: { menu_key: 'slashAdmin' },
      },
      {
        path: 'admin/users',
        name: 'AdminUsersView',
        component: () => import('@/views/admin/users.vue'),
        meta: { menu_key: 'adminUsers' },
      },
      {
        path: 'admin/roles',
        name: 'AdminRolesView',
        component: () => import('@/views/admin/roles.vue'),
        meta: { menu_key: 'adminRoles' },
      },
      {
        path: 'lore/:project_ulid',
        name: 'LoreView',
        component: () => import('@/views/lore/index.vue'),
        meta: { menu_key: 'lore' },
      },
      {
        path: 'models',
        name: 'ModelProviderView',
        component: () => import('@/views/models/index.vue'),
        meta: { menu_key: 'models' },
      },
      {
        path: 'admin/usage',
        name: 'AdminUsageView',
        component: () => import('@/views/admin/usage.vue'),
        meta: { menu_key: 'adminUsage' },
      },
      {
        path: 'admin/status',
        name: 'AdminStatusView',
        component: () => import('@/views/admin/status.vue'),
        meta: { menu_key: 'adminStatus' },
      },
      {
        path: 'admin/audit',
        name: 'AdminAuditView',
        component: () => import('@/views/admin/audit.vue'),
        meta: { menu_key: 'adminAudit' },
      },
      {
        path: 'workspace/:project_ulid',
        name: 'WorkspaceView',
        component: () => import('@/views/workspace/index.vue'),
        meta: { menu_key: 'workspace' },
      },
      {
        path: 'knowledge/:project_ulid',
        name: 'KnowledgeView',
        component: () => import('@/views/knowledge/index.vue'),
        meta: { menu_key: 'knowledge' },
      },
      {
        path: 'character/:project_ulid',
        name: 'CharacterView',
        component: () => import('@/views/character/index.vue'),
        meta: { menu_key: 'character' },
      },
      {
        path: 'notes/:project_ulid',
        name: 'NotesView',
        component: () => import('@/views/notes/index.vue'),
        meta: { menu_key: 'notes' },
      },
      {
        path: 'outline/:project_ulid',
        name: 'OutlineView',
        component: () => import('@/views/outline/index.vue'),
        meta: { menu_key: 'outline' },
      },
      {
        path: 'continuity/:project_ulid',
        name: 'ContinuityView',
        component: () => import('@/views/continuity/index.vue'),
        meta: { menu_key: 'continuity' },
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

// 路由守卫：无 token 跳登录页；已登录访问登录页重定向首页
router.beforeEach((to) => {
  const token = readToken()
  if (PUBLIC_PAGE_PATH_LIST.includes(to.path)) {
    if (token) {
      return { path: '/' }
    }
    return true
  }
  if (!token) {
    return { path: '/login' }
  }
  return true
})

export default router

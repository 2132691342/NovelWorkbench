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

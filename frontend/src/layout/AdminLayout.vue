<!--
  主框架：侧边栏（工作台导航：项目上下文 + 项目内页面 + 全局页 + 系统管理）+ 顶栏（项目切换/语言/用户）。
  项目上下文导航是本项目的主入口：任何项目内页面都能在任意项目中直接切换。
-->
<script setup lang="ts">
import { computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useProjectStore } from '@/stores/project'
import { NW_LOCALE_OPTIONS } from '@/i18n'
import NWButton from '@/components/NWButton.vue'

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()
const projectStore = useProjectStore()
const { t } = useI18n()

/** 项目内页面路径前缀 → 页面键（用于侧边栏高亮与切换） */
const NW_PROJECT_PAGE_LIST = [
  { menu_key: 'workspace', route_name: 'workspace', path_prefix: '/workspace/' },
  { menu_key: 'character', route_name: 'character', path_prefix: '/character/' },
  { menu_key: 'lore', route_name: 'lore', path_prefix: '/lore/' },
  { menu_key: 'knowledge', route_name: 'knowledge', path_prefix: '/knowledge/' },
  { menu_key: 'notes', route_name: 'notes', path_prefix: '/notes/' },
] as const

const activeMenuPath = computed(() => route.path)

/** 当前路由命中的项目内页面（非项目内页面为 null） */
const activeProjectPage = computed(() => {
  const matched = NW_PROJECT_PAGE_LIST.find((pageItem) =>
    route.path.startsWith(pageItem.path_prefix),
  )
  return matched ?? null
})

/** 路由中的项目 ULID（项目内页面优先于本地存储） */
const routeProjectUlid = computed(() => {
  if (!activeProjectPage.value) {
    return ''
  }
  return String(route.params.project_ulid ?? '')
})

/** 当前生效项目 ULID */
const effectiveProjectUlid = computed(
  () => routeProjectUlid.value || projectStore.current_project_ulid,
)

/** 是否系统管理员（角色编码包含 admin 时展示系统管理入口） */
const isAdminUser = computed(() => (authStore.user_vo?.role_codes ?? []).includes('admin'))

const avatarText = computed(() => {
  const userName = authStore.user_vo?.user_name ?? ''
  return userName.length > 0 ? userName.slice(0, 1) : '?'
})

const currentLocaleLabel = computed(() => {
  const matchedOption = NW_LOCALE_OPTIONS.find(
    (optionItem) => optionItem.locale_code === appStore.locale_code,
  )
  return matchedOption?.locale_label ?? NW_LOCALE_OPTIONS[0].locale_label
})

function handleMenuSelect(menuPath: string | number): void {
  void router.push(String(menuPath))
}

/** 切换项目：停留在同类页面（写作台/角色/设定/知识库/笔记），无同类页面则进写作台 */
function handleProjectCommand(projectUlid: string | number | object): void {
  const targetUlid = String(projectUlid)
  if (targetUlid === 'dashboard') {
    void router.push('/')
    return
  }
  projectStore.setCurrentProject(targetUlid)
  const pathSuffix = activeProjectPage.value ? activeProjectPage.value.path_prefix : '/workspace/'
  void router.push(`${pathSuffix}${targetUlid}`)
}

function handleLocaleCommand(command: string | number | object): void {
  const localeCode = String(command)
  if (localeCode === 'zh-CN' || localeCode === 'en-US') {
    appStore.setLocale(localeCode)
  }
}

async function handleUserCommand(command: string | number | object): Promise<void> {
  if (command !== 'logout') {
    return
  }
  await authStore.logout()
  void router.push('/login')
}

// 路由中的项目变化时同步到全局选中态（URL 是项目上下文的最高优先级来源）
watch(routeProjectUlid, (projectUlid) => {
  if (projectUlid && projectUlid !== projectStore.current_project_ulid) {
    projectStore.setCurrentProject(projectUlid)
  }
})

onMounted(() => {
  if (!projectStore.project_list.length) {
    void projectStore.loadProjectList()
  }
})
</script>

<template>
  <div class="admin-layout">
    <aside class="admin-layout__aside" :class="{ 'is-collapsed': appStore.sidebar_collapsed }">
      <div class="admin-layout__brand">
        <svg class="admin-layout__brand-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
          <rect x="3" y="3" width="18" height="18" rx="5" fill="var(--primary)" />
          <path
            d="M8 8v8M8 8h2.6a2.4 2.4 0 0 1 2.4 2.4V16M16 8v8"
            stroke="#ffffff"
            stroke-width="1.8"
            stroke-linecap="round"
          />
        </svg>
        <span v-show="!appStore.sidebar_collapsed" class="admin-layout__brand-text">
          {{ t('common.appName') }}
        </span>
      </div>

      <!-- 项目切换器：项目内页面的统一入口 -->
      <div v-show="!appStore.sidebar_collapsed" class="admin-layout__project">
        <el-dropdown trigger="click" class="admin-layout__project-dropdown" @command="handleProjectCommand">
          <div class="admin-layout__project-trigger">
            <span class="admin-layout__project-name">
              {{ projectStore.current_project_vo?.project_name || t('layout.noProject') }}
            </span>
            <svg class="admin-layout__project-arrow" viewBox="0 0 24 24" fill="none" aria-hidden="true">
              <path d="M6 9l6 6 6-6" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
            </svg>
          </div>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item
                v-for="projectItem in projectStore.project_list"
                :key="projectItem.project_ulid"
                :command="projectItem.project_ulid"
                :disabled="projectItem.project_ulid === effectiveProjectUlid"
              >
                {{ projectItem.project_name }}
                <span class="admin-layout__project-meta">{{ projectItem.total_word_count }} 字</span>
              </el-dropdown-item>
              <el-dropdown-item v-if="!projectStore.project_list.length" disabled>
                {{ t('layout.noProjectTip') }}
              </el-dropdown-item>
              <el-dropdown-item divided command="dashboard">
                {{ t('layout.manageProjects') }}
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>

      <el-menu
        class="admin-layout__menu"
        :collapse="appStore.sidebar_collapsed"
        :collapse-transition="false"
        :default-active="activeMenuPath"
        @select="handleMenuSelect"
      >
        <el-menu-item index="/">
          <svg class="admin-layout__menu-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <rect x="4" y="4" width="7" height="7" rx="1.5" stroke="currentColor" stroke-width="1.8" />
            <rect x="13" y="4" width="7" height="7" rx="1.5" stroke="currentColor" stroke-width="1.8" />
            <rect x="4" y="13" width="7" height="7" rx="1.5" stroke="currentColor" stroke-width="1.8" />
            <rect x="13" y="13" width="7" height="7" rx="1.5" stroke="currentColor" stroke-width="1.8" />
          </svg>
          <template #title>{{ t('layout.menu.dashboard') }}</template>
        </el-menu-item>

        <div v-show="!appStore.sidebar_collapsed" class="admin-layout__group-title">
          {{ t('layout.projectGroup') }}
        </div>

        <el-menu-item
          v-for="pageItem in NW_PROJECT_PAGE_LIST"
          :key="pageItem.menu_key"
          :index="
            effectiveProjectUlid ? `${pageItem.path_prefix}${effectiveProjectUlid}` : pageItem.path_prefix
          "
          :disabled="!effectiveProjectUlid"
        >
          <svg class="admin-layout__menu-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <path
              v-if="pageItem.menu_key === 'workspace'"
              d="M5 5h14v14H5zM5 9h14M9 5v14"
              stroke="currentColor"
              stroke-width="1.8"
              stroke-linejoin="round"
            />
            <path
              v-else-if="pageItem.menu_key === 'character'"
              d="M12 11a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7zM5 20c0-3.3 3.1-5.5 7-5.5s7 2.2 7 5.5"
              stroke="currentColor"
              stroke-width="1.8"
              stroke-linecap="round"
            />
            <path
              v-else-if="pageItem.menu_key === 'lore'"
              d="M5 4h9l5 5v11H5zM14 4v5h5M8.5 13h7M8.5 16.5h5"
              stroke="currentColor"
              stroke-width="1.8"
              stroke-linejoin="round"
              stroke-linecap="round"
            />
            <path
              v-else-if="pageItem.menu_key === 'knowledge'"
              d="M4 6.5c3-1.6 5.3-1.6 8 0 2.7-1.6 5-1.6 8 0v11c-3-1.6-5.3-1.6-8 0-2.7-1.6-5-1.6-8 0zM12 6.5v11"
              stroke="currentColor"
              stroke-width="1.8"
              stroke-linejoin="round"
            />
            <path
              v-else
              d="M6 3.5h9l3 3v14H6zM9 9h6M9 12.5h6M9 16h4"
              stroke="currentColor"
              stroke-width="1.8"
              stroke-linejoin="round"
              stroke-linecap="round"
            />
          </svg>
          <template #title>{{ t(`layout.menu.${pageItem.menu_key}`) }}</template>
        </el-menu-item>

        <div v-show="!appStore.sidebar_collapsed" class="admin-layout__group-title">
          {{ t('layout.systemGroup') }}
        </div>

        <el-menu-item index="/models">
          <svg class="admin-layout__menu-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <circle cx="12" cy="12" r="3" stroke="currentColor" stroke-width="1.8" />
            <path
              d="M12 3.5v2M12 18.5v2M3.5 12h2M18.5 12h2M6 6l1.4 1.4M16.6 16.6L18 18M18 6l-1.4 1.4M7.4 16.6L6 18"
              stroke="currentColor"
              stroke-width="1.8"
              stroke-linecap="round"
            />
          </svg>
          <template #title>{{ t('layout.menu.models') }}</template>
        </el-menu-item>

        <el-menu-item index="/slash-admin">
          <svg class="admin-layout__menu-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <path
              d="M9 5L5 19M19 5l-4 14M4 10h16"
              stroke="currentColor"
              stroke-width="1.8"
              stroke-linecap="round"
            />
          </svg>
          <template #title>{{ t('layout.menu.slashAdmin') }}</template>
        </el-menu-item>

        <el-menu-item index="/settings">
          <svg class="admin-layout__menu-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <path
              d="M4 7h10M18 7h2M4 12h4M12 12h8M4 17h12M20 17h0"
              stroke="currentColor"
              stroke-width="1.8"
              stroke-linecap="round"
            />
          </svg>
          <template #title>{{ t('layout.menu.settings') }}</template>
        </el-menu-item>

        <el-sub-menu v-if="isAdminUser" index="/admin">
          <template #title>
            <svg class="admin-layout__menu-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
              <path
                d="M12 3l7 3v5c0 4.5-2.9 8.3-7 10-4.1-1.7-7-5.5-7-10V6l7-3z"
                stroke="currentColor"
                stroke-width="1.8"
                stroke-linejoin="round"
              />
              <path
                d="M9 12l2 2 4-4"
                stroke="currentColor"
                stroke-width="1.8"
                stroke-linecap="round"
                stroke-linejoin="round"
              />
            </svg>
            <span>{{ t('layout.menu.systemAdmin') }}</span>
          </template>

          <el-menu-item index="/admin/users">
            <template #title>{{ t('layout.menu.adminUsers') }}</template>
          </el-menu-item>
          <el-menu-item index="/admin/roles">
            <template #title>{{ t('layout.menu.adminRoles') }}</template>
          </el-menu-item>
          <el-menu-item index="/admin/audit">
            <template #title>{{ t('layout.menu.adminAudit') }}</template>
          </el-menu-item>
          <el-menu-item index="/admin/usage">
            <template #title>{{ t('layout.menu.adminUsage') }}</template>
          </el-menu-item>
          <el-menu-item index="/admin/status">
            <template #title>{{ t('layout.menu.adminStatus') }}</template>
          </el-menu-item>
        </el-sub-menu>
      </el-menu>
    </aside>

    <div class="admin-layout__body">
      <header class="admin-layout__header">
        <NWButton
          type="text"
          :aria-label="t('layout.toggleSidebar')"
          @click="appStore.toggleSidebar()"
        >
          <svg class="admin-layout__header-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <path
              d="M4 6h16M4 12h16M4 18h16"
              stroke="currentColor"
              stroke-width="1.8"
              stroke-linecap="round"
            />
          </svg>
        </NWButton>

        <span v-if="projectStore.current_project_vo" class="admin-layout__breadcrumb">
          {{ projectStore.current_project_vo.project_name }}
          <template v-if="activeProjectPage">
            / {{ t(`layout.menu.${activeProjectPage.menu_key}`) }}
          </template>
        </span>

        <div class="admin-layout__header-right">
          <el-dropdown @command="handleLocaleCommand">
            <span class="admin-layout__header-link">
              {{ currentLocaleLabel }}
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item
                  v-for="localeOption in NW_LOCALE_OPTIONS"
                  :key="localeOption.locale_code"
                  :command="localeOption.locale_code"
                >
                  {{ localeOption.locale_label }}
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>

          <el-dropdown @command="handleUserCommand">
            <span class="admin-layout__user">
              <el-avatar :size="32" :src="authStore.user_vo?.avatar_url" class="admin-layout__avatar">
                {{ avatarText }}
              </el-avatar>
              <span class="admin-layout__user-name">
                {{ authStore.user_vo?.user_name || t('layout.guest') }}
              </span>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="logout">{{ t('common.logout') }}</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </header>

      <main class="admin-layout__content">
        <router-view v-slot="{ Component }">
          <transition name="fade-slide" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </main>
    </div>
  </div>
</template>

<style scoped>
.admin-layout {
  display: flex;
  height: 100vh;
  overflow: hidden;
  background-color: var(--bg-page);
}

.admin-layout__aside {
  display: flex;
  flex-direction: column;
  width: 220px;
  flex-shrink: 0;
  background-color: var(--bg-card);
  border-right: 1px solid var(--border-light);
  transition: width 0.2s ease;
}

.admin-layout__aside.is-collapsed {
  width: 64px;
}

.admin-layout__brand {
  display: flex;
  align-items: center;
  gap: 10px;
  height: 56px;
  padding: 0 16px;
  border-bottom: 1px solid var(--border-light);
  overflow: hidden;
}

.admin-layout__brand-icon {
  width: 28px;
  height: 28px;
  flex-shrink: 0;
}

.admin-layout__brand-text {
  font-size: 16px;
  font-weight: 600;
  white-space: nowrap;
  color: var(--text-main);
}

/* 项目切换器 */
.admin-layout__project {
  padding: 12px;
  border-bottom: 1px solid var(--border-light);
}

.admin-layout__project-dropdown {
  width: 100%;
}

.admin-layout__project-trigger {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  height: 34px;
  padding: 0 10px;
  border: 1px solid var(--border);
  border-radius: var(--radius-base);
  background-color: var(--bg-page);
  cursor: pointer;
  transition: all var(--transition-base);
}

.admin-layout__project-trigger:hover {
  border-color: var(--primary-hover);
  color: var(--primary);
  box-shadow: var(--shadow-button-hover);
}

.admin-layout__project-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
  font-weight: 500;
}

.admin-layout__project-arrow {
  width: 14px;
  height: 14px;
  flex-shrink: 0;
  color: var(--text-secondary);
}

.admin-layout__project-meta {
  margin-left: 8px;
  color: var(--text-placeholder);
  font-size: 12px;
}

.admin-layout__menu {
  flex: 1;
  border-right: none;
  overflow-y: auto;
}

.admin-layout__group-title {
  padding: 12px 16px 6px;
  color: var(--text-placeholder);
  font-size: 12px;
  letter-spacing: 0.04em;
}

.admin-layout__menu-icon {
  width: 18px;
  height: 18px;
  margin-right: 8px;
}

.admin-layout__body {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
}

.admin-layout__header {
  display: flex;
  align-items: center;
  gap: 12px;
  height: 56px;
  padding: 0 20px;
  background-color: var(--bg-card);
  border-bottom: 1px solid var(--border-light);
}

.admin-layout__header-icon {
  width: 20px;
  height: 20px;
}

.admin-layout__breadcrumb {
  color: var(--text-secondary);
  font-size: 13px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.admin-layout__header-right {
  display: flex;
  align-items: center;
  gap: 20px;
  margin-left: auto;
}

.admin-layout__header-link {
  color: var(--text-secondary);
  cursor: pointer;
  outline: none;
  transition: color 0.2s ease;
}

.admin-layout__header-link:hover {
  color: var(--primary);
}

.admin-layout__user {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  outline: none;
}

.admin-layout__avatar {
  background-color: var(--primary);
  color: var(--text-inverse);
  font-weight: 600;
}

.admin-layout__user-name {
  color: var(--text-main);
  font-size: 14px;
}

.admin-layout__content {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
}
</style>

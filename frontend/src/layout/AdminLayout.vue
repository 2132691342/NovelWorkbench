<!--
  主框架：左侧导航（后端驱动菜单树）+ 顶栏（页面标题/面包屑/搜索/通知/语言/用户）。
  交互约定：Ctrl/Cmd+K 打开快速跳转，Ctrl/Cmd+B 折叠侧栏；项目内菜单在未选项目时禁用。
-->
<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useMenuStore } from '@/stores/menu'
import { useProjectStore } from '@/stores/project'
import { NW_LOCALE_OPTIONS } from '@/i18n'
import type { NWMenuItemVo } from '@/api/contracts'
import NWIcon from '@/components/NWIcon.vue'
import CommandPalette from './CommandPalette.vue'
import NotificationBell from './NotificationBell.vue'

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()
const menuStore = useMenuStore()
const projectStore = useProjectStore()
const { t } = useI18n()

/** 快速跳转弹层状态 */
const commandVisible = ref(false)


/** 项目作用域路由判断：路径里带 project_ulid 参数即项目页 */
const routeProjectUlid = computed(() => String(route.params.project_ulid ?? ''))

/** 当前生效项目 ULID（URL 优先于本地选中态） */
const effectiveProjectUlid = computed(
  () => routeProjectUlid.value || projectStore.current_project_ulid,
)

/** 菜单路由解析：项目内菜单的 route_path 以 :project_ulid 结尾 */
function resolveMenuPath(menuItem: NWMenuItemVo): string {
  if (menuItem.scope_code === 'project') {
    return menuItem.route_path.replace(':project_ulid', effectiveProjectUlid.value)
  }
  return menuItem.route_path
}

/** 菜单是否可点击：页面类型且有路由，项目内菜单还要求已选项目 */
function isMenuDisabled(menuItem: NWMenuItemVo): boolean {
  if (menuItem.menu_type_code !== 'page' || !menuItem.route_path) {
    return true
  }
  return menuItem.scope_code === 'project' && !effectiveProjectUlid.value
}

/** 侧边栏渲染数据：过滤掉没有可见子项的分组 */
const sidebarMenuList = computed(() =>
  menuStore.menu_tree_list
    .map((menuItem) => ({
      ...menuItem,
      child_list: (menuItem.child_list ?? []).filter((childItem) => childItem.menu_type_code === 'page'),
    }))
    .filter((menuItem) => menuItem.menu_type_code === 'page' || menuItem.child_list.length > 0),
)

/** 扁平化页面菜单（搜索、快捷键与标题回退用） */
const flatPageMenuList = computed(() => {
  const result: NWMenuItemVo[] = []
  for (const menuItem of menuStore.menu_tree_list) {
    if (menuItem.menu_type_code === 'page') {
      result.push(menuItem)
      continue
    }
    for (const childItem of menuItem.child_list ?? []) {
      if (childItem.menu_type_code === 'page') {
        result.push(childItem)
      }
    }
  }
  return result
})

/** 当前路由命中的菜单项（标题与高亮用） */
const activeMenuItem = computed(() => {
  const currentPath = route.path
  const exactItem = flatPageMenuList.value.find((menuItem) => menuItem.route_path === currentPath)
  if (exactItem) {
    return exactItem
  }
  let matchedItem: NWMenuItemVo | null = null
  let matchedLength = 0
  for (const menuItem of flatPageMenuList.value) {
    const basePath = menuItem.route_path.replace(':project_ulid', '')
    if (!basePath || basePath === '/' || !basePath.endsWith('/') || !currentPath.startsWith(basePath)) {
      continue
    }
    if (basePath.length > matchedLength) {
      matchedItem = menuItem
      matchedLength = basePath.length
    }
  }
  return matchedItem
})


/** 面包屑：分组名 / 页面名 */
const breadcrumbText = computed(() => {
  if (!activeMenuItem.value) {
    return t('common.appName')
  }
  for (const menuItem of sidebarMenuList.value) {
    if (menuItem.menu_type_code === 'page' && menuItem.menu_code === activeMenuItem.value.menu_code) {
      return t('common.appName')
    }
    if ((menuItem.child_list ?? []).some((childItem) => childItem.menu_code === activeMenuItem.value?.menu_code)) {
      return `${menuItem.menu_name} / ${activeMenuItem.value.menu_name}`
    }
  }
  return activeMenuItem.value.menu_name
})

const avatarText = computed(() => {
  const userName = authStore.user_vo?.user_name ?? ''
  return userName.length > 0 ? userName.slice(0, 1).toUpperCase() : '?'
})

const currentLocaleLabel = computed(() => {
  const matchedOption = NW_LOCALE_OPTIONS.find(
    (optionItem) => optionItem.locale_code === appStore.locale_code,
  )
  return matchedOption?.locale_label ?? NW_LOCALE_OPTIONS[0].locale_label
})

/** 项目当前字数（侧栏项目卡片展示） */
const projectMetaText = computed(() => {
  const projectVo = projectStore.current_project_vo
  if (!projectVo) {
    return t('layout.noProjectTip')
  }
  return `${projectVo.total_word_count.toLocaleString()} ${t('layout.wordsUnit')} · ${projectVo.chapter_count} ${t('layout.chaptersUnit')}`
})

/** 点击菜单：项目内菜单在切换页面前同步选中项目 */
function handleMenuClick(menuItem: NWMenuItemVo): void {
  if (isMenuDisabled(menuItem)) {
    return
  }
  if (menuItem.scope_code === 'project') {
    projectStore.setCurrentProject(effectiveProjectUlid.value)
  }
  void router.push(resolveMenuPath(menuItem))
}

/** 切换项目：停留在同类页面，无同类页面则进写作台 */
function handleProjectCommand(projectUlid: string | number | object): void {
  const targetUlid = String(projectUlid)
  if (targetUlid === 'dashboard') {
    void router.push('/')
    return
  }
  projectStore.setCurrentProject(targetUlid)
  const suffix =
    activeMenuItem.value?.scope_code === 'project'
      ? activeMenuItem.value.route_path
      : '/workspace/:project_ulid'
  void router.push(suffix.replace(':project_ulid', targetUlid))
}

function handleLocaleCommand(command: string | number | object): void {
  const localeCode = String(command)
  if (localeCode === 'zh-CN' || localeCode === 'en-US') {
    appStore.setLocale(localeCode)
  }
}

async function handleUserCommand(command: string | number | object): Promise<void> {
  const commandCode = String(command)
  if (commandCode === 'profile') {
    void router.push('/profile')
    return
  }
  if (commandCode === 'notifications') {
    void router.push('/notifications')
    return
  }
  if (commandCode !== 'logout') {
    return
  }
  await authStore.logout()
  menuStore.resetNav()
  void router.push('/login')
}

/** 打开快速跳转 */
function openCommandPalette(): void {
  commandVisible.value = true
}

/** 快速跳转到选中菜单 */
function handleCommandSelect(menuItem: NWMenuItemVo): void {
  commandVisible.value = false
  handleMenuClick(menuItem)
}

/** 全局快捷键：⌘/Ctrl+K 快速跳转、⌘/Ctrl+B 折叠侧栏、Esc 关闭弹层 */
function handleGlobalKeydown(event: KeyboardEvent): void {
  const metaKey = event.metaKey || event.ctrlKey
  if (metaKey && event.key.toLowerCase() === 'k') {
    event.preventDefault()
    openCommandPalette()
    return
  }
  if (metaKey && event.key.toLowerCase() === 'b') {
    event.preventDefault()
    appStore.toggleSidebar()
    return
  }
  if (event.key === 'Escape' && commandVisible.value) {
    commandVisible.value = false
  }
}

// 路由中的项目变化时同步到全局选中态（URL 是项目上下文的最高优先级来源）
watch(
  [routeProjectUlid, () => projectStore.list_loading],
  ([projectUlid, listLoading]) => {
    if (!projectUlid) {
      return
    }
    if (projectUlid !== projectStore.current_project_ulid) {
      projectStore.setCurrentProject(projectUlid)
    }
    if (listLoading) {
      return
    }
    if (projectStore.hasProject(projectUlid)) {
      return
    }
    if (projectStore.project_list.length === 0) {
      void projectStore.loadProjectList().then(() => {
        if (!projectStore.hasProject(projectUlid)) {
          ElMessage.warning(t('layout.projectMissing'))
          void router.replace('/')
        }
      })
      return
    }
    ElMessage.warning(t('layout.projectMissing'))
    void router.replace('/')
  },
  { immediate: true },
)

onMounted(() => {
  if (!menuStore.nav_loaded) {
    void menuStore.loadNav()
  }
  if (!projectStore.project_list.length) {
    void projectStore.loadProjectList()
  }
  window.addEventListener('keydown', handleGlobalKeydown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleGlobalKeydown)
})
</script>

<template>
  <div class="admin-layout">
    <aside class="admin-layout__aside" :class="{ 'is-collapsed': appStore.sidebar_collapsed }">
      <div class="admin-layout__brand">
        <span class="admin-layout__brand-mark">N</span>
        <span v-show="!appStore.sidebar_collapsed" class="admin-layout__brand-text">
          <strong>{{ t('common.appName') }}</strong>
          <small>{{ t('layout.brandSubtitle') }}</small>
        </span>
      </div>

      <!-- 项目切换器：项目内页面的统一入口 -->
      <div v-show="!appStore.sidebar_collapsed" class="admin-layout__project">
        <el-dropdown trigger="click" class="admin-layout__project-dropdown" @command="handleProjectCommand">
          <div class="admin-layout__project-trigger">
            <span class="admin-layout__project-cover">
              {{ (projectStore.current_project_vo?.project_name || 'N').slice(0, 1) }}
            </span>
            <span class="admin-layout__project-text">
              <span class="admin-layout__project-name">
                {{ projectStore.current_project_vo?.project_name || t('layout.noProject') }}
              </span>
              <span class="admin-layout__project-meta">{{ projectMetaText }}</span>
            </span>
            <NWIcon icon_code="chevronDown" :size_px="14" class="admin-layout__project-arrow" />
          </div>
          <template #dropdown>
            <el-dropdown-menu class="admin-layout__project-menu">
              <div v-if="!projectStore.project_list.length" class="admin-layout__project-empty">
                {{ t('layout.noProjectTip') }}
              </div>
              <el-dropdown-item
                v-for="projectItem in projectStore.project_list"
                :key="projectItem.project_ulid"
                :command="projectItem.project_ulid"
                :disabled="projectItem.project_ulid === effectiveProjectUlid"
              >
                <span class="admin-layout__project-option">
                  <span class="admin-layout__project-option-name">{{ projectItem.project_name }}</span>
                  <span class="admin-layout__project-option-meta">
                    {{ projectItem.total_word_count.toLocaleString() }} 字
                  </span>
                </span>
              </el-dropdown-item>
              <el-dropdown-item divided command="dashboard">
                {{ t('layout.manageProjects') }}
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>

      <nav class="admin-layout__nav">
        <template v-for="menuItem in sidebarMenuList" :key="menuItem.menu_code">
          <!-- 单页菜单（仪表盘 / 系统设置） -->
          <button
            v-if="menuItem.menu_type_code === 'page'"
            type="button"
            class="admin-layout__nav-item"
            :class="{ 'is-active': activeMenuItem?.menu_code === menuItem.menu_code }"
            :title="appStore.sidebar_collapsed ? menuItem.menu_name : ''"
            @click="handleMenuClick(menuItem)"
          >
            <NWIcon :icon_code="menuItem.icon_code" :size_px="18" />
            <span v-show="!appStore.sidebar_collapsed" class="admin-layout__nav-text">
              {{ menuItem.menu_name }}
            </span>
            <span v-if="!appStore.sidebar_collapsed && activeMenuItem?.menu_code === menuItem.menu_code" class="admin-layout__nav-dot" />
          </button>

          <!-- 分组菜单 -->
          <div v-else class="admin-layout__nav-group">
            <div v-show="!appStore.sidebar_collapsed" class="admin-layout__nav-group-title">
              {{ menuItem.menu_name }}
            </div>
            <button
              v-for="childItem in menuItem.child_list"
              :key="childItem.menu_code"
              type="button"
              class="admin-layout__nav-item"
              :class="{
                'is-active': activeMenuItem?.menu_code === childItem.menu_code,
                'is-disabled': isMenuDisabled(childItem),
              }"
              :title="appStore.sidebar_collapsed ? childItem.menu_name : ''"
              @click="handleMenuClick(childItem)"
            >
              <NWIcon :icon_code="childItem.icon_code" :size_px="18" />
              <span v-show="!appStore.sidebar_collapsed" class="admin-layout__nav-text">
                {{ childItem.menu_name }}
              </span>
              <span
                v-if="!appStore.sidebar_collapsed && activeMenuItem?.menu_code === childItem.menu_code"
                class="admin-layout__nav-dot"
              />
            </button>
          </div>
        </template>
      </nav>

      <div class="admin-layout__aside-footer">
        <button
          v-if="!appStore.sidebar_collapsed"
          type="button"
          class="admin-layout__collapse-btn"
          @click="appStore.toggleSidebar()"
        >
          <NWIcon icon_code="panelLeft" :size_px="16" />
          <span>{{ t('layout.collapseSidebar') }}</span>
          <kbd>Ctrl B</kbd>
        </button>
        <button
          v-else
          type="button"
          class="admin-layout__collapse-btn is-compact"
          :title="t('layout.expandSidebar')"
          @click="appStore.toggleSidebar()"
        >
          <NWIcon icon_code="panelLeft" :size_px="16" />
        </button>
      </div>
    </aside>

    <div class="admin-layout__body">
      <header class="admin-layout__header">
        <button
          type="button"
          class="admin-layout__header-btn"
          :aria-label="t('layout.toggleSidebar')"
          :title="t('layout.toggleSidebar')"
          @click="appStore.toggleSidebar()"
        >
          <NWIcon icon_code="panelLeft" :size_px="19" />
        </button>

        <div class="admin-layout__heading">
          <span class="admin-layout__breadcrumb">{{ breadcrumbText }}</span>
        </div>

        <div class="admin-layout__header-right">
          <button type="button" class="admin-layout__search-btn" @click="openCommandPalette">
            <NWIcon icon_code="search" :size_px="16" />
            <span>{{ t('layout.quickJumpPlaceholder') }}</span>
            <kbd>Ctrl K</kbd>
          </button>

          <NotificationBell />

          <el-dropdown @command="handleLocaleCommand">
            <button type="button" class="admin-layout__header-btn is-text">
              {{ currentLocaleLabel }}
            </button>
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
              <span class="admin-layout__user-text">
                <span class="admin-layout__user-name">
                  {{ authStore.user_vo?.user_name || t('layout.guest') }}
                </span>
                <span class="admin-layout__user-role">
                  {{ authStore.user_vo?.user_email || t('layout.guestHint') }}
                </span>
              </span>
              <NWIcon icon_code="chevronDown" :size_px="13" />
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="profile">{{ t('layout.profile') }}</el-dropdown-item>
                <el-dropdown-item command="notifications">{{ t('notifications.title') }}</el-dropdown-item>
                <el-dropdown-item divided command="logout">{{ t('common.logout') }}</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </header>

      <main class="admin-layout__content" :class="{ 'is-workspace': route.name === 'WorkspaceView' }">
        <router-view v-slot="{ Component }">
          <transition name="fade-slide" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </main>
    </div>


    <CommandPalette
      v-model:visible="commandVisible"
      :menu_tree_list="menuStore.menu_tree_list"
      :is_disabled="isMenuDisabled"
      @select="handleCommandSelect"
    />
  </div>
</template>
<style scoped src="./AdminLayout.css"></style>
<style scoped src="./AdminLayout.responsive.css"></style>

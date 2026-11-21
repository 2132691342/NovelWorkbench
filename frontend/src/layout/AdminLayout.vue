<!--
  主框架：左侧导航（后端驱动菜单树）+ 顶栏（面包屑/搜索/主题/通知/语言/用户）+ 多页签 + 内容区。
  交互约定：Ctrl/Cmd+K 快速跳转，Ctrl/Cmd+B 折叠侧栏；项目内菜单在未选项目时禁用。
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
import { useTagsViewStore, buildTagView } from '@/stores/tagsView'
import { NW_LOCALE_OPTIONS } from '@/i18n'
import type { NWMenuItemVo } from '@/api/contracts'
import NWIcon from '@/components/NWIcon.vue'
import SidebarMenu from './SidebarMenu.vue'
import Breadcrumb from './Breadcrumb.vue'
import TagsView from './TagsView.vue'
import ThemeSwitch from './ThemeSwitch.vue'
import ContentWrap from './ContentWrap.vue'
import CommandPalette from './CommandPalette.vue'
import NotificationBell from './NotificationBell.vue'

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()
const menuStore = useMenuStore()
const projectStore = useProjectStore()
const tagsViewStore = useTagsViewStore()
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

/** 当前页标题：菜单名优先，登录页等无菜单页面回落到应用名 */
const activePageTitle = computed(() => activeMenuItem.value?.menu_name ?? t('common.appName'))

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

/** 写作台需要满屏，不套内容留白 */
const compactContentFlag = computed(() => route.name === 'WorkspaceView')

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
  tagsViewStore.resetTags()
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

// 路由变化时登记页签（标题取菜单名，保证与侧栏一致）
watch(
  [() => route.fullPath, activePageTitle],
  () => {
    const tagView = buildTagView(route, activePageTitle.value)
    if (tagView) {
      tagsViewStore.addTag(tagView)
    }
  },
  { immediate: true },
)

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

      <SidebarMenu
        :menu_tree_list="menuStore.menu_tree_list"
        :active_menu_code="activeMenuItem?.menu_code ?? ''"
        :collapsed_flag="appStore.sidebar_collapsed"
        :is_menu_disabled="isMenuDisabled"
        @select="handleMenuClick"
      />

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

        <Breadcrumb
          :menu_tree_list="menuStore.menu_tree_list"
          :active_menu_code="activeMenuItem?.menu_code ?? ''"
        />

        <div class="admin-layout__header-right">
          <button type="button" class="admin-layout__search-btn" @click="openCommandPalette">
            <NWIcon icon_code="search" :size_px="16" />
            <span>{{ t('layout.quickJumpPlaceholder') }}</span>
            <kbd>Ctrl K</kbd>
          </button>

          <ThemeSwitch />

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

      <TagsView />

      <main class="admin-layout__content" :class="{ 'is-workspace': compactContentFlag }">
        <ContentWrap :compact_flag="compactContentFlag">
          <router-view v-slot="{ Component }">
            <transition name="fade-slide" mode="out-in">
              <component :is="Component" />
            </transition>
          </router-view>
        </ContentWrap>
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

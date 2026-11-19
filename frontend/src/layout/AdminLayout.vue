<!--
  主框架：二级侧边栏（后端驱动导航树）+ 顶栏（面包屑 / 项目切换 / 语言 / 用户）。
  导航完全由 /menu/nav 下发：分组标题与页面项都由菜单表决定，前端不再硬编码菜单结构。
-->
<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useMenuStore } from '@/stores/menu'
import { useProjectStore } from '@/stores/project'
import { NW_LOCALE_OPTIONS } from '@/i18n'
import type { NWMenuItemVo } from '@/api/contracts'
import NWButton from '@/components/NWButton.vue'
import NWIcon from '@/components/NWIcon.vue'

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()
const menuStore = useMenuStore()
const projectStore = useProjectStore()
const { t } = useI18n()

/** 图标快速跳转抽屉（顶栏搜索入口） */
const commandVisible = ref(false)
const commandKeyword = ref('')

/** 项目作用域路由判断：路径里带 project_ulid 参数即项目页 */
const routeProjectUlid = computed(() => String(route.params.project_ulid ?? ''))

/** 当前生效项目 ULID（URL 优先于本地选中态） */
const effectiveProjectUlid = computed(
  () => routeProjectUlid.value || projectStore.current_project_ulid,
)

/**
 * 菜单路由解析：项目内菜单的 route_path 以 :project_ulid 结尾，
 * 未选中项目时禁用该项（与后端 scope_code=project 对应）。
 */
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
    .filter((menuItem) =>
      menuItem.menu_type_code === 'page' ? true : menuItem.child_list.length > 0,
    ),
)

/** 扁平化页面菜单（顶栏搜索与快捷键跳转用） */
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

/** 搜索命中的菜单项 */
const commandResultList = computed(() => {
  const keyword = commandKeyword.value.trim().toLowerCase()
  if (!keyword) {
    return flatPageMenuList.value
  }
  return flatPageMenuList.value.filter(
    (menuItem) =>
      menuItem.menu_name.toLowerCase().includes(keyword) ||
      menuItem.menu_code.toLowerCase().includes(keyword),
  )
})

/**
 * 当前路由命中的菜单项（面包屑与高亮用）：
 * 先精确匹配路由，再取最长前缀匹配（项目内路由的实际路径比模板长）。
 */
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
    if (!basePath || !basePath.endsWith('/') || !currentPath.startsWith(basePath)) {
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
    return ''
  }
  for (const menuItem of sidebarMenuList.value) {
    if (menuItem.menu_type_code === 'page' && menuItem.menu_code === activeMenuItem.value.menu_code) {
      return activeMenuItem.value.menu_name
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

/** 切换项目：停留在同类页面（写作台/角色/知识库…），无同类页面则进写作台 */
function handleProjectCommand(projectUlid: string | number | object): void {
  const targetUlid = String(projectUlid)
  projectStore.setCurrentProject(targetUlid)
  const suffix = activeMenuItem.value?.scope_code === 'project' ? activeMenuItem.value.route_path : '/workspace/:project_ulid'
  void router.push(suffix.replace(':project_ulid', targetUlid))
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
  menuStore.resetNav()
  void router.push('/login')
}

/** 打开快速跳转 */
function openCommandPalette(): void {
  commandKeyword.value = ''
  commandVisible.value = true
}

/** 快速跳转到选中菜单 */
function handleCommandSelect(menuItem: NWMenuItemVo): void {
  commandVisible.value = false
  handleMenuClick(menuItem)
}

// 路由中的项目变化时同步到全局选中态（URL 是项目上下文的最高优先级来源）；
// 列表里没有该项目时重新拉取，仍缺失则判定为已删除/无权访问并回到仪表盘
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
      // 列表尚未加载（或确实为空）：拉一次再判定，避免空列表误判
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
})
</script>

<template>
  <div class="admin-layout">
    <aside class="admin-layout__aside" :class="{ 'is-collapsed': appStore.sidebar_collapsed }">
      <div class="admin-layout__brand">
        <span class="admin-layout__brand-mark">N</span>
        <span v-show="!appStore.sidebar_collapsed" class="admin-layout__brand-text">
          {{ t('common.appName') }}
        </span>
      </div>

      <!-- 项目切换器：项目内页面的统一入口 -->
      <div v-show="!appStore.sidebar_collapsed" class="admin-layout__project">
        <el-dropdown trigger="click" class="admin-layout__project-dropdown" @command="handleProjectCommand">
          <div class="admin-layout__project-trigger">
            <NWIcon icon_code="book" :size_px="15" />
            <span class="admin-layout__project-name">
              {{ projectStore.current_project_vo?.project_name || t('layout.noProject') }}
            </span>
            <NWIcon icon_code="collapse" :size_px="14" class="admin-layout__project-arrow" />
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
              <el-dropdown-item divided command="dashboard" @click="router.push('/')">
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
            <NWIcon :icon_code="menuItem.icon_code" />
            <span v-show="!appStore.sidebar_collapsed" class="admin-layout__nav-text">
              {{ menuItem.menu_name }}
            </span>
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
              <NWIcon :icon_code="childItem.icon_code" />
              <span v-show="!appStore.sidebar_collapsed" class="admin-layout__nav-text">
                {{ childItem.menu_name }}
              </span>
            </button>
          </div>
        </template>
      </nav>
    </aside>

    <div class="admin-layout__body">
      <header class="admin-layout__header">
        <NWButton type="text" :aria-label="t('layout.toggleSidebar')" @click="appStore.toggleSidebar()">
          <NWIcon icon_code="collapse" :size_px="20" />
        </NWButton>

        <span class="admin-layout__breadcrumb">{{ breadcrumbText }}</span>

        <div class="admin-layout__header-right">
          <el-tooltip :content="t('layout.quickJump')" placement="bottom">
            <button type="button" class="admin-layout__header-btn" @click="openCommandPalette">
              <NWIcon icon_code="search" :size_px="18" />
            </button>
          </el-tooltip>

          <el-dropdown @command="handleLocaleCommand">
            <span class="admin-layout__header-link">{{ currentLocaleLabel }}</span>
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
              <el-avatar :size="30" :src="authStore.user_vo?.avatar_url" class="admin-layout__avatar">
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

    <!-- 快速跳转：菜单项搜索 -->
    <el-dialog v-model="commandVisible" :title="t('layout.quickJump')" width="440px" class="admin-layout__command-dialog">
      <el-input
        v-model="commandKeyword"
        :placeholder="t('layout.quickJumpPlaceholder')"
        clearable
        autofocus
      />
      <div class="admin-layout__command-list">
        <button
          v-for="menuItem in commandResultList"
          :key="menuItem.menu_code"
          type="button"
          class="admin-layout__command-item"
          :disabled="isMenuDisabled(menuItem)"
          @click="handleCommandSelect(menuItem)"
        >
          <NWIcon :icon_code="menuItem.icon_code" :size_px="16" />
          <span>{{ menuItem.menu_name }}</span>
        </button>
        <p v-if="!commandResultList.length" class="admin-layout__command-empty">
          {{ t('layout.quickJumpEmpty') }}
        </p>
      </div>
    </el-dialog>
  </div>
</template>
<style scoped src="./AdminLayout.css"></style>

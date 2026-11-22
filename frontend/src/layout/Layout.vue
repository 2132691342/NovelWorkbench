<!-- 布局外壳：sidebar / top / mixed / dual 四种布局，导航统一由后端菜单树驱动。 -->
<script setup lang="ts">
  import { computed, ref, watch } from 'vue'
  import { useRoute } from 'vue-router'
  import { ElScrollbar } from 'element-plus'
  import { useAppStore } from '@/store/modules/app'
  import { useProjectStore } from '@/store/modules/project'
  import Logo from '@/components/Logo/index.vue'
  import TagsView from '@/components/TagsView/index.vue'
  import Backtop from '@/components/Backtop/index.vue'
  import AppView from './components/AppView.vue'
  import ToolHeader from './components/ToolHeader.vue'
  import Menu from './components/Menu.vue'
  import { useNavMenu, type NavNode } from './useNavMenu'

  const appStore = useAppStore()
  const projectStore = useProjectStore()
  const route = useRoute()
  const { navNodes, activeKey } = useNavMenu()

  const layout = computed(() => (appStore.mobile ? 'sidebar' : appStore.layout))
  const collapsed = computed(() => appStore.collapse && !appStore.mobile)

  /** 当前页面所属的一级分组（mixed/dual 的二级菜单来源） */
  const routeGroupKey = computed(() => {
    const hit = (node: NavNode): boolean =>
      node.key === activeKey.value || node.children.some((child) => child.key === activeKey.value)
    return navNodes.value.find(hit)?.key ?? ''
  })

  /** dual 布局下用户手动选中的一级分组：默认跟随当前路由 */
  const pickedGroupKey = ref('')
  watch(routeGroupKey, (key) => {
    if (key) pickedGroupKey.value = key
  }, { immediate: true })

  const activeGroup = computed<NavNode | undefined>(() => {
    const key = pickedGroupKey.value || routeGroupKey.value
    const picked = navNodes.value.find((node) => node.key === key && node.children.length > 0)
    // 当前页是顶层页面（仪表盘/系统设置）时没有所属分组，退到首个有子项的分组，避免二级栏空白
    return picked ?? navNodes.value.find((node) => node.children.length > 0)
  })

  /** 一级导航：分组只切换二级菜单，页面直接跳转 */
  const primaryNodes = computed<NavNode[]>(() =>
    navNodes.value.map((node) =>
      node.children.length > 0 ? { ...node, path: '', disabled: false } : node
    )
  )

  const secondaryNodes = computed<NavNode[]>(() => activeGroup.value?.children ?? [])

  // 深链进入项目页时同步项目选中态，顶栏切换器与项目作用域菜单据此高亮
  watch(
    () => String(route.params.project_ulid ?? ''),
    (projectUlid) => {
      if (projectUlid) void projectStore.syncFromRoute(projectUlid)
    },
    { immediate: true }
  )
</script>

<template>
  <section class="nw-layout" :class="`nw-layout--${layout}`">
    <header class="nw-layout__topbar">
      <Logo :compact="layout === 'dual'" />
      <Menu
        v-if="layout === 'top' || layout === 'mixed'"
        mode="horizontal"
        :nodes="primaryNodes"
        @group-select="pickedGroupKey = $event"
      />
      <ToolHeader :class="{ 'is-inline': layout === 'top' || layout === 'mixed' }" />
    </header>

    <div class="nw-layout__body">
      <aside v-if="layout === 'sidebar'" :class="['nw-layout__sidebar', { 'is-collapsed': collapsed }]">
        <ElScrollbar>
          <Menu :collapsed="collapsed" />
        </ElScrollbar>
      </aside>

      <aside v-if="layout === 'dual'" class="nw-layout__rail">
        <ElScrollbar>
          <Menu
            mode="vertical"
            collapsed
            :nodes="primaryNodes"
            @group-select="pickedGroupKey = $event"
          />
        </ElScrollbar>
      </aside>

      <aside
        v-if="layout === 'mixed' || layout === 'dual'"
        :class="['nw-layout__secondary', { 'is-collapsed': collapsed && layout === 'mixed' }]"
      >
        <div v-if="layout === 'mixed' && !collapsed" class="nw-layout__secondary-title">
          {{ activeGroup?.label ?? '' }}
        </div>
        <ElScrollbar>
          <Menu mode="vertical" :nodes="secondaryNodes" />
        </ElScrollbar>
      </aside>

      <main class="nw-layout__workspace">
        <TagsView class="nw-layout__tags" />
        <ElScrollbar v-loading="appStore.pageLoading" class="nw-layout__scroll">
          <AppView />
        </ElScrollbar>
      </main>
    </div>

    <button
      v-if="appStore.mobile && !appStore.collapse"
      class="nw-layout__mask"
      type="button"
      aria-label="关闭导航菜单"
      @click="appStore.collapse = true"
    ></button>

    <Backtop />
  </section>
</template>

<style lang="less" scoped>
  .nw-layout {
    display: flex;
    flex-direction: column;
    width: 100%;
    height: 100%;
    overflow: hidden;
    background: var(--app-content-bg-color);

    &__topbar {
      z-index: 20;
      display: flex;
      flex: none;
      gap: 8px;
      align-items: center;
      height: var(--top-tool-height);
      background: var(--top-header-bg-color);
      border-bottom: 1px solid var(--layout-border-color);
      box-shadow: var(--layout-shadow);

      :deep(.nw-logo) {
        min-width: var(--left-menu-max-width);
      }

      /* 顶栏已承载主菜单时，工具区收窄：只隐藏面包屑与布局文字，项目切换仍保留 */
      :deep(.nw-tool-header.is-inline) {
        flex: none;

        .nw-tool-header__breadcrumb,
        .nw-tool-header__layout-label {
          display: none;
        }
      }
    }

    &__body {
      display: flex;
      flex: 1;
      min-height: 0;
    }

    &__sidebar {
      z-index: 30;
      display: flex;
      flex: none;
      flex-direction: column;
      width: var(--left-menu-max-width);
      min-height: 0;
      background: var(--left-menu-bg-color);
      border-right: 1px solid var(--layout-border-color);
      transition: width var(--transition-time-02);

      &.is-collapsed {
        width: var(--left-menu-min-width);
      }
    }

    &__secondary {
      z-index: 25;
      display: flex;
      flex: none;
      flex-direction: column;
      width: var(--left-menu-max-width);
      min-height: 0;
      background: var(--left-menu-bg-light-color);
      border-right: 1px solid var(--layout-border-color);
      transition: width var(--transition-time-02);

      &.is-collapsed {
        display: none;
      }
    }

    &__rail {
      z-index: 30;
      display: flex;
      flex: none;
      flex-direction: column;
      width: var(--left-menu-min-width);
      min-height: 0;
      background: var(--left-menu-bg-color);
      border-right: 1px solid var(--layout-border-color);

      :deep(.nw-menu__label) {
        display: none;
      }

      :deep(.el-sub-menu__icon-arrow) {
        display: none;
      }
    }

    &__secondary-title {
      display: flex;
      flex: none;
      align-items: center;
      height: 44px;
      padding: 0 18px;
      font-size: 12px;
      font-weight: 700;
      letter-spacing: 0.06em;
      color: var(--nw-text-muted);
      text-transform: uppercase;
      border-bottom: 1px solid var(--layout-border-color);
    }

    &__workspace {
      display: flex;
      flex: 1;
      flex-direction: column;
      min-width: 0;
      min-height: 0;
    }

    &__tags {
      flex: none;
      background: var(--top-header-bg-color);
      border-bottom: 1px solid var(--layout-border-color);
    }

    &__scroll {
      flex: 1;
      min-height: 0;
    }

    &__mask {
      position: fixed;
      inset: 0;
      z-index: 25;
      cursor: pointer;
      background: rgb(15 23 42 / 45%);
      border: 0;
    }

    /* dual：图标栏 + 二级菜单，图标栏不显示文字 */
    &--dual .nw-layout__sidebar {
      :deep(.nw-menu__label) {
        display: none;
      }

      :deep(.el-sub-menu__icon-arrow) {
        display: none;
      }
    }

    /* top/mixed：顶栏承载主菜单，内容区不再有左侧一级菜单 */
    &--top .nw-layout__workspace,
    &--mixed .nw-layout__workspace {
      min-width: 0;
    }
  }

  @media (width <= 767px) {
    .nw-layout__sidebar {
      position: fixed;
      inset: var(--top-tool-height) auto 0 0;
      z-index: 30;
      width: min(82vw, var(--left-menu-max-width));
      box-shadow: 20px 0 50px rgb(15 23 42 / 25%);

      &.is-collapsed {
        transform: translateX(-100%);
      }
    }

    .nw-layout__secondary {
      display: none;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .nw-layout__sidebar,
    .nw-layout__secondary {
      transition: none;
    }
  }
</style>

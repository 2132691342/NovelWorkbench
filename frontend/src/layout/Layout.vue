<!-- 布局外壳：顶栏、侧边栏与内容区，并同步项目上下文。 -->
<script setup lang="ts">
  import { computed, watch } from 'vue'
  import { useRoute } from 'vue-router'
  import { ElScrollbar } from 'element-plus'
  import { useAppStore } from '@/store/modules/app'
  import { useProjectStore } from '@/store/modules/project'
  import Logo from '@/components/Logo/index.vue'
  import TagsView from '@/components/TagsView/index.vue'
  import Backtop from '@/components/Backtop/index.vue'
  import AppView from './components/AppView.vue'
  import ToolHeader from './components/ToolHeader.vue'
  import NavMenu from './components/NavMenu.vue'

  const appStore = useAppStore()
  const projectStore = useProjectStore()
  const route = useRoute()

  const collapsed = computed(() => appStore.collapse && !appStore.mobile)

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
  <section class="nw-layout">
    <header class="nw-layout__topbar">
      <Logo />
      <ToolHeader />
    </header>

    <div class="nw-layout__body">
      <aside :class="['nw-layout__sidebar', { 'is-collapsed': collapsed }]">
        <NavMenu />
      </aside>

      <main class="nw-layout__workspace">
        <TagsView class="nw-layout__tags" />
        <ElScrollbar v-loading="appStore.pageLoading" class="nw-layout__scroll">
          <AppView />
        </ElScrollbar>
      </main>
    </div>

    <button
      v-if="appStore.mobile && !collapsed"
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
      align-items: center;
      height: var(--top-tool-height);
      background: var(--top-header-bg-color);
      border-bottom: 1px solid var(--layout-border-color);
      box-shadow: var(--layout-shadow);

      :deep(.v-logo) {
        min-width: var(--left-menu-max-width);
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
  }

  @media (prefers-reduced-motion: reduce) {
    .nw-layout__sidebar {
      transition: none;
    }
  }
</style>

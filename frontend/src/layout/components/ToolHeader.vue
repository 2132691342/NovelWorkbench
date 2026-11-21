<!-- 顶栏：折叠、刷新、主题切换与用户入口。 -->
<script setup lang="ts">
  import { ThemeSwitch } from '@novelworkbench/components'
  import Collapse from '@/components/Collapse/index.vue'
  import UserInfo from '@/components/UserInfo/index.vue'
  import Breadcrumb from '@/components/Breadcrumb/index.vue'
  import { useAppStore } from '@/store/modules/app'
  import ProjectSwitch from './ProjectSwitch.vue'
  import NotificationBell from './NotificationBell.vue'

  const appStore = useAppStore()
</script>

<template>
  <div class="nw-tool-header">
    <div class="nw-tool-header__left">
      <Collapse class="header-action" color="var(--top-header-text-color)" />
      <Breadcrumb class="nw-tool-header__breadcrumb" />
      <ProjectSwitch class="nw-tool-header__project" />
    </div>

    <div class="nw-tool-header__right">
      <NotificationBell class="header-action" />
      <ThemeSwitch v-model="appStore.isDark" class="header-action" />
      <UserInfo />
    </div>
  </div>
</template>

<style lang="less" scoped>
  .nw-tool-header {
    display: flex;
    flex: 1;
    align-items: center;
    justify-content: space-between;
    height: var(--top-tool-height);
    min-width: 0;

    &__left,
    &__right {
      display: flex;
      align-items: center;
      height: 100%;
      min-width: 0;
    }

    &__breadcrumb {
      overflow: hidden;
    }

    &__project {
      flex: none;
    }

    :deep(.el-switch.header-action) {
      height: 100%;
      padding: 0 10px;
      border-radius: 0;

      &:hover {
        background: var(--top-header-hover-color);
      }
    }
  }

  @media (width <= 1100px) {
    .nw-tool-header__breadcrumb {
      display: none;
    }
  }

  @media (width <= 860px) {
    .nw-tool-header__project {
      display: none;
    }
  }
</style>

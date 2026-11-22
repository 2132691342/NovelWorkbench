<!-- 顶栏：布局切换、折叠、面包屑、项目切换、通知、主题与用户入口。 -->
<script setup lang="ts">
  import { computed, ref } from 'vue'
  import { ThemeSwitch } from '@novelworkbench/components'
  import Collapse from '@/components/Collapse/index.vue'
  import UserInfo from '@/components/UserInfo/index.vue'
  import Breadcrumb from '@/components/Breadcrumb/index.vue'
  import { useAppStore, type LayoutMode } from '@/store/modules/app'
  import ProjectSwitch from './ProjectSwitch.vue'
  import NotificationBell from './NotificationBell.vue'

  const appStore = useAppStore()

  const layoutVisible = ref(false)

  const layoutOptions: { value: LayoutMode; label: string; icon: string }[] = [
    { value: 'sidebar', label: '侧边菜单', icon: 'mdi:view-dashboard' },
    { value: 'top', label: '顶部菜单', icon: 'mdi:menu' },
    { value: 'mixed', label: '混合菜单', icon: 'mdi:layers-outline' },
    { value: 'dual', label: '双列菜单', icon: 'mdi:file-tree' }
  ]

  const currentLayoutLabel = computed(
    () => layoutOptions.find((item) => item.value === appStore.layout)?.label ?? ''
  )

  const selectLayout = (layout: LayoutMode) => {
    appStore.setLayout(layout)
    layoutVisible.value = false
  }
</script>

<template>
  <div class="nw-tool-header">
    <div class="nw-tool-header__left">
      <Collapse class="header-action" color="var(--top-header-text-color)" />
      <Breadcrumb class="nw-tool-header__breadcrumb" />
      <ProjectSwitch class="nw-tool-header__project" />
    </div>

    <div class="nw-tool-header__right">
      <ElPopover
        v-model:visible="layoutVisible"
        placement="bottom-end"
        :width="200"
        trigger="click"
        popper-class="nw-layout-popover"
      >
        <template #reference>
          <button class="nw-tool-header__action" type="button" title="切换布局">
            <Icon icon="mdi:view-quilt-outline" :size="18" />
            <span class="nw-tool-header__layout-label">{{ currentLayoutLabel }}</span>
          </button>
        </template>
        <div class="nw-layout-list">
          <button
            v-for="item in layoutOptions"
            :key="item.value"
            type="button"
            :class="['nw-layout-list__item', { 'is-active': appStore.layout === item.value }]"
            @click="selectLayout(item.value)"
          >
            <Icon :icon="item.icon" :size="16" />
            <span>{{ item.label }}</span>
            <Icon v-if="appStore.layout === item.value" icon="mdi:check" :size="15" />
          </button>
        </div>
      </ElPopover>

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

    &__action {
      display: flex;
      gap: 6px;
      align-items: center;
      height: 100%;
      padding: 0 10px;
      color: var(--top-header-text-color);
      cursor: pointer;
      background: transparent;
      border: 0;
      transition:
        color 0.16s ease,
        background-color 0.16s ease;

      &:hover,
      &:focus-visible {
        color: var(--el-color-primary);
        background: var(--top-header-hover-color);
        outline: none;
      }
    }

    &__layout-label {
      font-size: 12px;
      white-space: nowrap;
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

  .nw-layout-list {
    display: flex;
    flex-direction: column;
    gap: 4px;

    &__item {
      display: flex;
      gap: 8px;
      align-items: center;
      width: 100%;
      height: 34px;
      padding: 0 10px;
      font-size: 13px;
      color: var(--el-text-color-regular);
      cursor: pointer;
      background: transparent;
      border: 1px solid transparent;
      border-radius: var(--nw-radius-md);
      transition:
        color 0.16s ease,
        background-color 0.16s ease,
        border-color 0.16s ease;

      span {
        flex: 1;
        text-align: left;
      }

      &:hover {
        color: var(--el-color-primary);
        background: var(--nw-primary-soft);
      }

      &.is-active {
        font-weight: 600;
        color: var(--el-color-primary);
        background: var(--nw-primary-soft);
        border-color: var(--el-color-primary-light-7);
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

    .nw-tool-header__layout-label {
      display: none;
    }
  }
</style>

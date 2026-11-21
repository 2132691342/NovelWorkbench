<!-- 侧边栏导航：渲染后端下发的菜单树与当前高亮。 -->
<script setup lang="ts">
  import { computed } from 'vue'
  import { useRouter } from 'vue-router'
  import { ElMessage } from 'element-plus'
  import { useAppStore } from '@/store/modules/app'
  import { useNavMenu, type NavNode } from '../useNavMenu'
  import { resolveMenuIcon } from '../menuIcons'

  const appStore = useAppStore()
  const router = useRouter()
  const { navNodes, activeKey } = useNavMenu()

  const collapsed = computed(() => appStore.collapse && !appStore.mobile)

  const open = (node: NavNode) => {
    if (node.disabled) {
      ElMessage.warning('请先在顶部选择一个项目')
      return
    }
    if (node.path) void router.push(node.path)
  }
</script>

<template>
  <nav :class="['nw-nav', { 'is-collapsed': collapsed }]">
    <template v-for="node in navNodes" :key="node.key">
      <div v-if="node.children.length > 0 && !collapsed" class="nw-nav__group">
        {{ node.label }}
      </div>

      <button
        v-for="entry in node.children.length > 0 ? node.children : [node]"
        :key="entry.key"
        type="button"
        :class="[
          'nw-nav__item',
          { 'is-active': entry.key === activeKey, 'is-disabled': entry.disabled }
        ]"
        :title="collapsed ? entry.label : ''"
        @click="open(entry)"
      >
        <Icon :icon="resolveMenuIcon(entry.icon)" :size="19" />
        <span v-if="!collapsed" class="nw-nav__label">{{ entry.label }}</span>
      </button>
    </template>
  </nav>
</template>

<style lang="less" scoped>
  .nw-nav {
    display: flex;
    flex: 1;
    flex-direction: column;
    gap: 2px;
    min-height: 0;
    padding: 10px 10px 20px;
    overflow-y: auto;

    &__group {
      padding: 14px 10px 6px;
      font-size: 11px;
      font-weight: 700;
      letter-spacing: 0.08em;
      color: var(--el-text-color-placeholder);
      text-transform: uppercase;
    }

    &__item {
      position: relative;
      display: flex;
      gap: 10px;
      align-items: center;
      width: 100%;
      height: 40px;
      padding: 0 12px;
      font-size: 13.5px;
      font-weight: 500;
      color: var(--left-menu-text-color);
      text-align: left;
      cursor: pointer;
      background: transparent;
      border: 0;
      border-radius: var(--nw-radius-md);
      transition:
        color 0.16s ease,
        background-color 0.16s ease,
        transform 0.16s cubic-bezier(0.34, 1.4, 0.64, 1);

      &:hover:not(.is-disabled) {
        color: var(--left-menu-text-active-color);
        background: var(--left-menu-bg-light-color);
        transform: translateX(2px);
      }

      &.is-active {
        font-weight: 600;
        color: var(--left-menu-text-active-color);
        background: var(--left-menu-bg-active-color);
      }

      &.is-active::before {
        position: absolute;
        top: 10px;
        bottom: 10px;
        left: 0;
        width: 3px;
        background: var(--el-color-primary);
        border-radius: 0 3px 3px 0;
        content: '';
      }

      &.is-disabled {
        cursor: not-allowed;
        opacity: 0.45;
      }

      &:focus-visible {
        outline: 2px solid var(--el-color-primary-light-5);
        outline-offset: -2px;
      }
    }

    &.is-collapsed {
      align-items: center;
      padding: 10px 6px;

      .nw-nav__item {
        justify-content: center;
        width: 44px;
        padding: 0;
      }
    }
  }
</style>

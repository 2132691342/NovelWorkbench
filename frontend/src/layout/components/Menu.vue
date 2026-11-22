<!-- 侧边/顶部导航：Element Plus ElMenu 渲染后端导航树，支持折叠与四布局复用。 -->
<script setup lang="ts">
  import { computed } from 'vue'
  import { useRouter } from 'vue-router'
  import { ElMessage } from 'element-plus'
  import { useAppStore } from '@/store/modules/app'
  import { useNavMenu, type NavNode } from '../useNavMenu'
  import { resolveMenuIcon } from '../menuIcons'

  const props = withDefaults(
    defineProps<{
      mode?: 'vertical' | 'horizontal' // 纵向（侧栏）或横向（顶栏）
      nodes?: NavNode[] // 指定渲染节点（混合布局的二级菜单用），缺省渲染完整树
      collapsed?: boolean // 是否折叠（仅纵向生效）
    }>(),
    { mode: 'vertical', nodes: undefined, collapsed: false }
  )

  const emit = defineEmits<{ 'group-select': [groupKey: string] }>()

  const router = useRouter()
  const appStore = useAppStore()
  const { navNodes, activeKey } = useNavMenu()

  const renderNodes = computed(() => props.nodes ?? navNodes.value)
  const isCollapsed = computed(
    () => props.mode === 'vertical' && (props.collapsed || (appStore.collapse && !appStore.mobile))
  )

  /** 菜单项 index：有路径用路径，未选项目时用编码占位（保证 key 唯一） */
  const itemIndex = (node: NavNode) => node.path || node.key

  /** 当前高亮：优先用最长匹配到的页面路径，避免项目 ULID 变化导致失焦 */
  const activeIndex = computed(() => {
    const flatten = (list: NavNode[]): NavNode[] =>
      list.flatMap((node) => (node.children.length > 0 ? flatten(node.children) : [node]))
    const hit = flatten(navNodes.value).find((node) => node.key === activeKey.value)
    return hit?.path || router.currentRoute.value.path
  })

  /** 默认展开包含当前页的分组；当前页不在分组内时展开首个分组，避免菜单看起来是空的 */
  const defaultOpeneds = computed(() => {
    const parent = navNodes.value.find((node) =>
      node.children.some((child) => child.key === activeKey.value)
    )
    if (parent) return [parent.key]
    const firstGroup = navNodes.value.find((node) => node.children.length > 0)
    return firstGroup ? [firstGroup.key] : []
  })

  /** 点击菜单项：禁用项提示先选项目，其余直接跳转 */
  const handleSelect = (index: string) => {
    if (!index) return
    const findNode = (list: NavNode[]): NavNode | undefined => {
      for (const node of list) {
        if (itemIndex(node) === index) return node
        const hit = findNode(node.children)
        if (hit) return hit
      }
      return undefined
    }
    const target = findNode(navNodes.value)
    // 一级分组：只切换二级菜单，不跳转
    if (target && target.children.length > 0) {
      emit('group-select', target.key)
      return
    }
    if (target?.disabled || !target?.path) {
      ElMessage.warning('请先在顶部选择一个项目')
      return
    }
    if (target.path !== router.currentRoute.value.path) void router.push(target.path)
  }
</script>

<template>
  <ElMenu
    :default-active="activeIndex"
    :default-openeds="defaultOpeneds"
    :mode="mode"
    :collapse="isCollapsed"
    :unique-opened="false"
    class="nw-menu"
    :class="[`nw-menu--${mode}`, { 'is-collapsed': isCollapsed }]"
    @select="handleSelect"
  >
    <template v-for="node in renderNodes" :key="node.key">
      <ElSubMenu v-if="node.children.length > 0" :index="node.key">
        <template #title>
          <Icon :icon="resolveMenuIcon(node.icon)" :size="18" />
          <span class="nw-menu__label">{{ node.label }}</span>
        </template>
        <ElMenuItem
          v-for="child in node.children"
          :key="child.key"
          :index="itemIndex(child)"
          :disabled="child.disabled"
        >
          <Icon :icon="resolveMenuIcon(child.icon)" :size="17" />
          <span class="nw-menu__label">{{ child.label }}</span>
        </ElMenuItem>
      </ElSubMenu>
      <ElMenuItem v-else :index="itemIndex(node)" :disabled="node.disabled">
        <Icon :icon="resolveMenuIcon(node.icon)" :size="18" />
        <span class="nw-menu__label">{{ node.label }}</span>
      </ElMenuItem>
    </template>
  </ElMenu>
</template>

<style lang="less" scoped>
  .nw-menu {
    --nw-menu-item-radius: var(--nw-radius-md);

    border: 0;
    background: transparent;

    :deep(.el-menu) {
      border: 0;
      background: transparent;
    }

    :deep(.el-sub-menu__title),
    :deep(.el-menu-item) {
      gap: 10px;
      height: 40px;
      margin: 2px 8px;
      padding: 0 12px !important;
      font-size: 13.5px;
      font-weight: 500;
      line-height: 40px;
      color: var(--left-menu-text-color);
      border-radius: var(--nw-menu-item-radius);
      transition:
        color 0.16s ease,
        background-color 0.16s ease,
        transform 0.16s cubic-bezier(0.34, 1.4, 0.64, 1);

      &:hover {
        color: var(--left-menu-text-active-color);
        background: var(--left-menu-bg-light-color);
      }
    }

    :deep(.el-menu-item.is-active) {
      font-weight: 600;
      color: var(--left-menu-text-active-color);
      background: var(--left-menu-bg-active-color);
    }

    :deep(.el-menu-item.is-disabled) {
      cursor: not-allowed;
      opacity: 0.45;
    }

    :deep(.el-sub-menu .el-menu) {
      background: transparent;
    }

    :deep(.el-sub-menu__icon-arrow) {
      right: 14px;
      color: var(--nw-text-muted);
    }

    &__label {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    &--vertical.is-collapsed {
      :deep(.el-sub-menu__title),
      :deep(.el-menu-item) {
        justify-content: center;
        padding: 0 !important;
        margin: 2px 8px;
      }

      :deep(.el-sub-menu__icon-arrow) {
        display: none;
      }
    }

    &--horizontal {
      flex: 1;
      min-width: 0;

      :deep(.el-menu--horizontal) {
        height: var(--top-tool-height);
        border: 0;
      }

      :deep(.el-sub-menu__title),
      :deep(.el-menu-item) {
        height: 36px;
        margin: 0 2px;
        line-height: 36px;
        color: var(--top-header-text-color);
        border: 0;
        border-radius: var(--nw-radius-md);
      }

      :deep(.el-menu-item.is-active),
      :deep(.el-sub-menu.is-active > .el-sub-menu__title) {
        color: var(--el-color-primary);
        background: var(--nw-primary-soft);
        border: 0;
      }
    }
  }
</style>

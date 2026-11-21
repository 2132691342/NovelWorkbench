<!-- 大纲树：展示层级结构并支持节点操作。 -->
<script setup lang="ts">
  import { computed, ref } from 'vue'
  import { ElEmpty } from 'element-plus'
  import type { NWOutlineNodeVo } from '@/api/outline'
  import { nodeStatusBadge, nodeStatusLabel, nodeTypeLabel } from './meta'
  import { flattenOutlineTree } from './tree'

  const props = withDefaults(
    defineProps<{
      nodeList: NWOutlineNodeVo[]
      selectedUlid?: string
    }>(),
    { selectedUlid: '' }
  )

  const emit = defineEmits<{
    select: [node: NWOutlineNodeVo]
    'create-child': [node: NWOutlineNodeVo]
    'edit-node': [node: NWOutlineNodeVo]
    'delete-node': [node: NWOutlineNodeVo]
  }>()

  /** 折叠状态：默认全部展开，仅记录被折叠的节点 */
  const collapsedMap = ref<Record<string, boolean>>({})

  const rowList = computed(() => flattenOutlineTree(props.nodeList, collapsedMap.value))

  /** 按节点类型给出层级图标 */
  const nodeIcon = (node: NWOutlineNodeVo): string => {
    if (node.node_type_code === 'volume') return 'mdi:folder-outline'
    if (node.node_type_code === 'arc') return 'mdi:layers-outline'
    if (node.node_type_code === 'storyBlock') return 'mdi:file-tree'
    return 'mdi:file-document-outline'
  }

  /** 切换子节点展开状态 */
  const toggleCollapse = (node: NWOutlineNodeVo) => {
    collapsedMap.value = {
      ...collapsedMap.value,
      [node.outline_ulid]: !collapsedMap.value[node.outline_ulid]
    }
  }
</script>

<template>
  <div class="outline-tree">
    <div
      v-for="row in rowList"
      :key="row.node.outline_ulid"
      class="outline-tree__row"
      :class="{ 'is-active': row.node.outline_ulid === selectedUlid }"
      :style="{ paddingLeft: `${8 + row.depth * 14}px` }"
      @click="emit('select', row.node)"
    >
      <button
        v-if="row.hasChildren"
        type="button"
        class="outline-tree__toggle"
        :title="collapsedMap[row.node.outline_ulid] ? '展开' : '折叠'"
        @click.stop="toggleCollapse(row.node)"
      >
        <Icon
          :icon="
            collapsedMap[row.node.outline_ulid]
              ? 'mdi:chevron-double-right'
              : 'mdi:chevron-double-left'
          "
          :size="11"
        />
      </button>
      <span v-else class="outline-tree__toggle is-leaf" />

      <Icon :icon="nodeIcon(row.node)" :size="14" class="outline-tree__icon" />
      <span class="outline-tree__title" :title="row.node.title_text">{{ row.node.title_text }}</span>
      <span class="outline-tree__badge">{{ nodeTypeLabel(row.node.node_type_code) }}</span>
      <span
        class="outline-tree__dot"
        :class="`outline-tree__dot--${nodeStatusBadge(row.node.node_status_code)}`"
        :title="nodeStatusLabel(row.node.node_status_code)"
      />

      <span class="outline-tree__actions">
        <button
          type="button"
          class="outline-tree__action"
          title="新建子节点"
          @click.stop="emit('create-child', row.node)"
        >
          <Icon icon="mdi:plus" :size="12" />
        </button>
        <button
          type="button"
          class="outline-tree__action"
          title="编辑节点"
          @click.stop="emit('edit-node', row.node)"
        >
          <Icon icon="mdi:pencil-outline" :size="12" />
        </button>
        <button
          type="button"
          class="outline-tree__action is-danger"
          title="删除节点"
          @click.stop="emit('delete-node', row.node)"
        >
          <Icon icon="mdi:delete-outline" :size="12" />
        </button>
      </span>
    </div>

    <ElEmpty v-if="rowList.length === 0" description="还没有大纲，先建一个卷" :image-size="70" />
  </div>
</template>

<style lang="less" scoped>
  .outline-tree {
    display: flex;
    flex-direction: column;
    gap: 1px;

    &__row {
      display: flex;
      gap: 6px;
      align-items: center;
      padding: 6px 8px;
      border-radius: var(--nw-radius-base);
      cursor: pointer;
      transition: background 0.15s ease;

      &:hover {
        background: var(--nw-surface-subtle);
      }

      &.is-active {
        background: var(--nw-primary-soft);
      }

      &.is-active .outline-tree__title {
        font-weight: 650;
        color: var(--el-color-primary);
      }
    }

    &__toggle {
      display: grid;
      flex: none;
      width: 14px;
      height: 14px;
      padding: 0;
      color: var(--nw-text-muted);
      cursor: pointer;
      background: transparent;
      border: none;
      place-items: center;

      &.is-leaf {
        cursor: default;
      }
    }

    &__icon {
      flex: none;
      color: var(--nw-text-secondary);
    }

    &__title {
      flex: 1;
      min-width: 0;
      overflow: hidden;
      font-size: 13px;
      color: var(--nw-ink);
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    &__badge {
      flex: none;
      padding: 0 5px;
      font-size: 10px;
      line-height: 16px;
      color: var(--nw-text-muted);
      background: var(--el-fill-color-light);
      border-radius: var(--nw-radius-sm);
    }

    &__dot {
      flex: none;
      width: 6px;
      height: 6px;
      background: var(--nw-text-muted);
      border-radius: 50%;

      &--success {
        background: var(--nw-success);
      }

      &--warning {
        background: var(--nw-warning);
      }

      &--neutral {
        background: var(--nw-text-muted);
      }
    }

    &__actions {
      display: none;
      flex: none;
      gap: 2px;
      align-items: center;
    }

    &__row:hover &__actions {
      display: flex;
    }

    &__action {
      display: grid;
      width: 18px;
      height: 18px;
      padding: 0;
      color: var(--nw-text-secondary);
      cursor: pointer;
      background: transparent;
      border: none;
      border-radius: var(--nw-radius-sm);
      place-items: center;

      &:hover {
        color: var(--el-color-primary);
        background: var(--el-fill-color-light);
      }

      &.is-danger:hover {
        color: var(--nw-danger);
      }
    }
  }
</style>

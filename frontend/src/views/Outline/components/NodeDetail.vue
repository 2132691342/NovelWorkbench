<!-- 大纲节点详情：展示节点属性与关联场景卡。 -->
<script setup lang="ts">
  import { ElButton, ElCard, ElEmpty } from 'element-plus'
  import type { NWDetailedSceneVo, NWOutlineNodeVo } from '@/api/outline'
  import { formatRelativeTime } from '@/utils/time'
  import { nodeStatusBadge, nodeStatusLabel, nodeTypeLabel, paceLabel } from './meta'

  defineProps<{
    node: NWOutlineNodeVo
    sceneList: NWDetailedSceneVo[]
    sceneLoading: boolean
  }>()

  const emit = defineEmits<{
    'create-child': []
    'edit-node': []
    'delete-node': []
    'create-scene': []
    'edit-scene': [scene: NWDetailedSceneVo]
    'delete-scene': [scene: NWDetailedSceneVo]
  }>()
</script>

<template>
  <div class="node-detail">
    <ElCard shadow="never" class="node-detail__card">
      <div class="node-detail__head">
        <div class="node-detail__title">
          <span class="nw-badge nw-badge--neutral">{{ nodeTypeLabel(node.node_type_code) }}</span>
          <h2 class="node-detail__name">{{ node.title_text }}</h2>
          <span
            class="nw-badge"
            :class="`nw-badge--${nodeStatusBadge(node.node_status_code)}`"
          >
            {{ nodeStatusLabel(node.node_status_code) }}
          </span>
        </div>
        <div class="node-detail__actions">
          <ElButton size="small" text @click="emit('create-child')">
            <Icon icon="mdi:plus" :size="13" />
            <span class="ml-4px">子节点</span>
          </ElButton>
          <ElButton size="small" text @click="emit('edit-node')">编辑</ElButton>
          <ElButton size="small" text type="danger" @click="emit('delete-node')">删除</ElButton>
        </div>
      </div>
      <p class="node-detail__summary">
        {{ node.summary_text || '还没有概要，点「编辑」补一段这一节要写什么。' }}
      </p>
      <p class="node-detail__updated">更新于 {{ formatRelativeTime(node.updated_at) }}</p>
    </ElCard>

    <ElCard shadow="never" class="node-detail__scenes">
      <div class="node-detail__scenes-head">
        <span class="node-detail__scenes-title">场景卡（{{ sceneList.length }}）</span>
        <ElButton type="primary" plain size="small" @click="emit('create-scene')">
          <Icon icon="mdi:plus" :size="13" />
          <span class="ml-4px">新建场景</span>
        </ElButton>
      </div>

      <div v-loading="sceneLoading" class="node-detail__scene-list">
        <div v-for="scene in sceneList" :key="scene.scene_ulid" class="scene-card">
          <div class="scene-card__head">
            <span class="scene-card__place">{{ scene.location_text || '未设定地点' }}</span>
            <span class="nw-badge nw-badge--neutral">{{ paceLabel(scene.pace_code) }}</span>
            <span class="scene-card__words">{{ scene.estimated_words }} 字</span>
            <div class="scene-card__spacer" />
            <ElButton size="small" text @click="emit('edit-scene', scene)">编辑</ElButton>
            <ElButton size="small" text type="danger" @click="emit('delete-scene', scene)">
              删除
            </ElButton>
          </div>
          <p class="scene-card__line">
            <span class="scene-card__label">冲突</span>
            {{ scene.conflict_text || '—' }}
          </p>
          <p class="scene-card__line">
            <span class="scene-card__label">目标</span>
            {{ scene.goal_text || '—' }}
          </p>
          <p class="scene-card__line">
            <span class="scene-card__label">结果</span>
            {{ scene.outcome_text || '—' }}
          </p>
        </div>

        <ElEmpty
          v-if="!sceneLoading && sceneList.length === 0"
          description="这个节点还没有场景卡，新建一张细化冲突与节奏"
          :image-size="70"
        />
      </div>
    </ElCard>
  </div>
</template>

<style lang="less" scoped>
  .node-detail {
    display: flex;
    flex-direction: column;
    gap: 16px;

    &__card {
      :deep(.el-card__body) {
        display: flex;
        flex-direction: column;
        gap: 10px;
        padding: 16px;
      }
    }

    &__head {
      display: flex;
      gap: 12px;
      align-items: flex-start;
      justify-content: space-between;
    }

    &__title {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      align-items: center;
      min-width: 0;
    }

    &__name {
      font-size: 17px;
      font-weight: 700;
      color: var(--nw-ink);
    }

    &__actions {
      display: flex;
      flex: none;
      gap: 2px;
      align-items: center;
    }

    &__summary {
      font-size: 13px;
      line-height: 1.8;
      color: var(--el-text-color-regular);
      white-space: pre-wrap;
    }

    &__updated {
      font-size: 11.5px;
      color: var(--nw-text-muted);
    }

    &__scenes {
      :deep(.el-card__body) {
        padding: 14px 16px 16px;
      }
    }

    &__scenes-head {
      display: flex;
      align-items: center;
      justify-content: space-between;
      margin-bottom: 12px;
    }

    &__scenes-title {
      font-size: 13px;
      font-weight: 650;
      color: var(--nw-ink);
    }

    &__scene-list {
      display: flex;
      flex-direction: column;
      gap: 10px;
    }
  }

  .scene-card {
    padding: 12px 14px;
    background: var(--nw-surface-subtle);
    border: 1px solid var(--nw-border-soft);
    border-radius: var(--nw-radius-md);

    &__head {
      display: flex;
      gap: 8px;
      align-items: center;
      margin-bottom: 8px;
    }

    &__place {
      font-size: 13px;
      font-weight: 650;
      color: var(--nw-ink);
    }

    &__words {
      font-size: 11.5px;
      color: var(--nw-text-muted);
    }

    &__spacer {
      flex: 1;
    }

    &__line {
      display: flex;
      gap: 8px;
      font-size: 12.5px;
      line-height: 1.7;
      color: var(--el-text-color-regular);
      word-break: break-word;
    }

    &__label {
      flex: none;
      width: 32px;
      font-size: 11.5px;
      color: var(--nw-text-muted);
    }
  }
</style>

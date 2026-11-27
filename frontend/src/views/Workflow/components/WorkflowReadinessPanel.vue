<!-- 就绪度清单：把「素材齐不齐」画成可勾选的列表，未就绪项直接给一句可执行建议 + 去补齐按钮。
     页面顶部与新建向导第 2 步共用同一个组件，避免两处判定口径不一致。 -->
<script setup lang="ts">
  import { computed } from 'vue'
  import type { NWWorkflowReadinessItemVo, NWWorkflowReadinessVo } from '@/api/workflow'

  const props = defineProps<{
    readiness: NWWorkflowReadinessVo | null
    loading?: boolean
    /** 紧凑模式：向导内嵌用，隐藏大分数，只留清单 */
    compact?: boolean
  }>()

  const emit = defineEmits<{
    (event: 'fix', route: string): void
  }>()

  /** 检查项图标：编码固定，图标只是视觉锚点 */
  const itemIconMap: Record<string, string> = {
    project_synopsis: 'mdi:text-box-outline',
    project_style: 'mdi:palette-outline',
    character: 'mdi:account-group',
    character_relation: 'mdi:account-switch-outline',
    lore: 'mdi:earth',
    outline: 'mdi:file-tree',
    chapter_outline: 'mdi:format-list-checks',
    prompt: 'mdi:script-text-outline',
    model: 'mdi:robot-outline'
  }

  const itemList = computed(() => props.readiness?.item_list ?? [])

  /** 未就绪项排前面：用户点开只想看「还缺什么」 */
  const sortedItemList = computed(() =>
    [...itemList.value].sort((left, right) => {
      if (left.ready_flag !== right.ready_flag) return left.ready_flag ? 1 : -1
      if (left.blocking_flag !== right.blocking_flag) return left.blocking_flag ? -1 : 1
      return 0
    })
  )

  const allReady = computed(() => props.readiness?.ready_flag === true)

  const scoreTone = computed(() => {
    const score = props.readiness?.score_value ?? 0
    if (score >= 100) return 'success'
    if (score >= 70) return 'warning'
    return 'danger'
  })

  const fix = (item: NWWorkflowReadinessItemVo) => {
    if (!item.fix_route) return
    emit('fix', item.fix_route)
  }
</script>

<template>
  <section v-loading="loading" :class="['wf-ready', { 'is-compact': compact }]">
    <div v-if="!compact" class="wf-ready__score">
      <ElProgress
        type="dashboard"
        :percentage="readiness?.score_value ?? 0"
        :width="96"
        :stroke-width="8"
        :color="scoreTone === 'success' ? 'var(--nw-success)' : scoreTone === 'warning' ? 'var(--nw-warning)' : 'var(--nw-danger)'"
      >
        <template #default="{ percentage }">
          <span class="wf-ready__score-value">{{ percentage }}</span>
          <span class="wf-ready__score-unit">就绪度</span>
        </template>
      </ElProgress>
      <div class="wf-ready__summary">
        <strong v-if="allReady" class="wf-ready__ok">
          <Icon icon="mdi:check-decagram" :size="16" />
          <span>素材齐备，可以开跑</span>
        </strong>
        <strong v-else class="wf-ready__todo">
          {{ readiness?.blocking_count ?? 0 }} 项必填未就绪
        </strong>
        <p class="nw-hint">
          {{ allReady ? '所有检查项已就绪，随时可以点「新建任务」。' : '补齐必填项再开跑，产出质量会明显更稳。' }}
        </p>
      </div>
    </div>

    <ul class="wf-ready__list">
      <li
        v-for="item in sortedItemList"
        :key="item.item_code"
        :class="['wf-ready__item', { 'is-ready': item.ready_flag, 'is-blocking': !item.ready_flag && item.blocking_flag }]"
      >
        <span class="wf-ready__icon">
          <Icon
            :icon="item.ready_flag ? 'mdi:check-circle' : itemIconMap[item.item_code] ?? 'mdi:circle-outline'"
            :size="14"
          />
        </span>
        <div class="wf-ready__body">
          <div class="wf-ready__head">
            <strong>{{ item.item_name }}</strong>
            <span class="wf-ready__value">{{ item.current_value }} / {{ item.target_value }}</span>
            <ElTag v-if="!item.ready_flag && item.blocking_flag" type="danger" size="small" effect="plain">
              必填
            </ElTag>
          </div>
          <p v-if="!item.ready_flag" class="wf-ready__hint">{{ item.hint_text }}</p>
        </div>
        <ElButton
          v-if="!item.ready_flag && item.fix_route"
          size="small"
          text
          type="primary"
          @click="fix(item)"
        >
          去补齐
        </ElButton>
      </li>
      <li v-if="sortedItemList.length === 0 && !loading" class="wf-ready__item wf-ready__item--empty">
        暂无检查结果
      </li>
    </ul>
  </section>
</template>

<style lang="less" scoped>
  .wf-ready {
    display: flex;
    gap: 18px;
    align-items: flex-start;
    padding: 14px 16px;
    background: var(--nw-surface);
    border: 1px solid var(--nw-border);
    border-radius: var(--nw-radius-lg);
    box-shadow: var(--nw-shadow-card);

    &.is-compact {
      display: block;
      padding: 0;
      background: transparent;
      border: none;
      box-shadow: none;
    }

    &__score {
      display: flex;
      flex: none;
      gap: 14px;
      align-items: center;
    }

    &__score-value {
      display: block;
      font-size: 22px;
      font-weight: 700;
      line-height: 1.1;
      color: var(--nw-ink);
      font-variant-numeric: tabular-nums;
    }

    &__score-unit {
      display: block;
      font-size: 11px;
      color: var(--el-text-color-secondary);
    }

    &__summary {
      display: flex;
      flex-direction: column;
      gap: 4px;
      max-width: 260px;
    }

    &__ok {
      display: flex;
      gap: 6px;
      align-items: center;
      font-size: 13px;
      color: var(--nw-success);
    }

    &__todo {
      font-size: 13px;
      color: var(--nw-warning);
    }

    &__list {
      display: grid;
      flex: 1;
      grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
      gap: 6px 14px;
      padding: 0;
      margin: 0;
      list-style: none;
    }

    &__item {
      display: flex;
      gap: 8px;
      align-items: flex-start;
      padding: 6px 8px;
      border-radius: var(--nw-radius-md);
      transition: background 0.18s ease;

      &:hover {
        background: var(--nw-surface-subtle);
      }

      &.is-blocking {
        background: rgb(239 68 68 / 5%);
      }

      &--empty {
        font-size: 12px;
        color: var(--el-text-color-secondary);
      }
    }

    &__icon {
      display: flex;
      flex: none;
      align-items: center;
      height: 18px;
      color: var(--el-text-color-placeholder);

      .is-ready & {
        color: var(--nw-success);
      }

      .is-blocking & {
        color: var(--nw-danger);
      }
    }

    &__body {
      flex: 1;
      min-width: 0;
    }

    &__head {
      display: flex;
      flex-wrap: wrap;
      gap: 6px;
      align-items: center;

      strong {
        font-size: 12.5px;
        font-weight: 600;
        color: var(--nw-ink);
      }
    }

    &__value {
      font-size: 11px;
      color: var(--el-text-color-placeholder);
      font-variant-numeric: tabular-nums;
    }

    &__hint {
      margin: 2px 0 0;
      font-size: 11.5px;
      line-height: 1.5;
      color: var(--el-text-color-secondary);
    }
  }
</style>

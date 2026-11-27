<!-- 步骤时间线：按章节分组，每步一行（图标按类型、状态、耗时、提示词编码、输出可折叠）。
     跨章混排看不出进度，因此以章节为分组单位；失败步骤就地显示原因。 -->
<script setup lang="ts">
  import { computed, ref } from 'vue'
  import type { NWWorkflowStepVo } from '@/api/workflow'
  import { formatTime } from '@/utils/time'

  const props = defineProps<{
    stepList: NWWorkflowStepVo[]
    currentChapterUlid: string
  }>()

  const stepTypeMeta: Record<string, { label: string; icon: string }> = {
    draft: { label: '出稿', icon: 'mdi:pen' },
    check: { label: '体检', icon: 'mdi:clipboard-check-outline' },
    revise: { label: '修订', icon: 'mdi:auto-fix' },
    review: { label: '复审', icon: 'mdi:magnify-scan' },
    human_review: { label: '人工审核', icon: 'mdi:account-check-outline' },
    summary: { label: '摘要', icon: 'mdi:text-box-outline' },
    apply: { label: '入库', icon: 'mdi:content-save-check-outline' }
  }

  const stepStatusMeta: Record<string, { label: string; type: 'success' | 'warning' | 'danger' | 'info' | 'primary' }> = {
    queued: { label: '待执行', type: 'info' },
    running: { label: '执行中', type: 'primary' },
    success: { label: '完成', type: 'success' },
    failed: { label: '失败', type: 'danger' },
    skipped: { label: '跳过', type: 'info' },
    rejected: { label: '已退回', type: 'warning' }
  }

  /** 折叠状态：按步骤 ULID 记，默认全收起（时间线只该展示骨架） */
  const expandedSet = ref<string[]>([])

  const toggleExpanded = (stepUlid: string) => {
    expandedSet.value = expandedSet.value.includes(stepUlid)
      ? expandedSet.value.filter((ulid) => ulid !== stepUlid)
      : [...expandedSet.value, stepUlid]
  }

  /** 按章节分组：保持步骤出现顺序（时间线语义），不按章节名排序 */
  const groupList = computed(() => {
    const groupMap = new Map<string, { chapterUlid: string; chapterName: string; stepList: NWWorkflowStepVo[] }>()
    for (const step of props.stepList) {
      const key = step.chapter_ulid || 'unknown'
      if (!groupMap.has(key)) {
        groupMap.set(key, {
          chapterUlid: step.chapter_ulid,
          chapterName: step.chapter_name || '未关联章节',
          stepList: []
        })
      }
      groupMap.get(key)!.stepList.push(step)
    }
    return [...groupMap.values()]
  })

  const latencyTextOf = (latencyMs: number) => {
    if (!latencyMs) return '—'
    if (latencyMs < 1000) return `${latencyMs} ms`
    return `${(latencyMs / 1000).toFixed(1)} s`
  }

  const hasOutput = (step: NWWorkflowStepVo) =>
    Boolean(step.output_text || step.metric_json || step.error_message)
</script>

<template>
  <div class="wf-timeline">
    <section
      v-for="group in groupList"
      :key="group.chapterUlid || group.chapterName"
      :class="['wf-timeline__group', { 'is-current': group.chapterUlid === currentChapterUlid }]"
    >
      <header class="wf-timeline__group-head">
        <Icon icon="mdi:book-open-page-variant-outline" :size="13" />
        <strong>{{ group.chapterName }}</strong>
        <span class="wf-timeline__count">{{ group.stepList.length }} 步</span>
        <ElTag v-if="group.chapterUlid === currentChapterUlid" size="small" effect="plain" type="primary">
          当前章
        </ElTag>
      </header>

      <ul class="wf-steps">
        <li v-for="step in group.stepList" :key="step.step_ulid" class="wf-step">
          <span :class="['wf-step__icon', `is-${step.step_status_code}`]">
            <Icon :icon="stepTypeMeta[step.step_type_code]?.icon ?? 'mdi:circle-small'" :size="13" />
          </span>
          <div class="wf-step__body">
            <div class="wf-step__head">
              <strong>{{ step.step_name }}</strong>
              <ElTag :type="stepStatusMeta[step.step_status_code]?.type ?? 'info'" size="small" effect="plain">
                {{ stepStatusMeta[step.step_status_code]?.label ?? step.step_status_code }}
              </ElTag>
              <ElTag v-if="step.prompt_code" size="small" effect="plain" type="info">
                /{{ step.prompt_code }}
              </ElTag>
              <span class="wf-step__latency">{{ latencyTextOf(step.latency_ms) }}</span>
            </div>

            <p v-if="step.error_message" class="wf-step__error">
              <Icon icon="mdi:alert-circle-outline" :size="12" />
              <span>{{ step.error_message }}</span>
            </p>

            <button
              v-if="hasOutput(step)"
              type="button"
              class="wf-step__toggle"
              @click="toggleExpanded(step.step_ulid)"
            >
              <Icon :icon="expandedSet.includes(step.step_ulid) ? 'mdi:chevron-up' : 'mdi:chevron-down'" :size="13" />
              <span>{{ expandedSet.includes(step.step_ulid) ? '收起输出' : '查看输出' }}</span>
            </button>

            <pre v-if="expandedSet.includes(step.step_ulid)" class="wf-step__output">{{ step.output_text || '（无文本输出）' }}</pre>

            <p class="wf-step__time">{{ formatTime(step.finished_at || step.started_at) }}</p>
          </div>
        </li>
      </ul>
    </section>

    <ElEmpty v-if="groupList.length === 0" description="暂无步骤记录" :image-size="60" />
  </div>
</template>

<style lang="less" scoped>
  .wf-timeline {
    display: flex;
    flex-direction: column;
    gap: 12px;

    &__group {
      padding: 10px 12px;
      background: var(--nw-surface);
      border: 1px solid var(--nw-border-soft);
      border-radius: var(--nw-radius-md);

      &.is-current {
        border-color: var(--el-color-primary);
        background: rgb(99 102 241 / 3%);
      }
    }

    &__group-head {
      display: flex;
      gap: 6px;
      align-items: center;
      margin-bottom: 8px;
      color: var(--el-text-color-secondary);

      strong {
        font-size: 12.5px;
        color: var(--nw-ink);
      }
    }

    &__count {
      font-size: 11px;
      color: var(--el-text-color-placeholder);
    }
  }

  .wf-steps {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 0;
    margin: 0;
    list-style: none;
  }

  .wf-step {
    display: flex;
    gap: 8px;

    &__icon {
      display: flex;
      flex: none;
      align-items: center;
      justify-content: center;
      width: 24px;
      height: 24px;
      color: var(--el-color-primary);
      background: var(--nw-primary-soft);
      border-radius: 50%;

      &.is-failed {
        color: var(--nw-danger);
        background: rgb(239 68 68 / 10%);
      }

      &.is-skipped,
      &.is-queued {
        color: var(--el-text-color-placeholder);
        background: var(--nw-surface-subtle);
      }

      &.is-rejected {
        color: var(--nw-warning);
        background: rgb(245 158 11 / 12%);
      }
    }

    &__body {
      flex: 1;
      min-width: 0;
      padding: 6px 8px;
      background: var(--nw-surface-subtle);
      border-radius: var(--nw-radius-sm);
    }

    &__head {
      display: flex;
      flex-wrap: wrap;
      gap: 6px;
      align-items: center;

      strong {
        font-size: 12px;
        color: var(--nw-ink);
      }
    }

    &__latency {
      margin-left: auto;
      font-size: 11px;
      color: var(--el-text-color-placeholder);
      font-variant-numeric: tabular-nums;
    }

    &__error {
      display: flex;
      gap: 4px;
      align-items: flex-start;
      margin: 4px 0 0;
      font-size: 11.5px;
      line-height: 1.5;
      color: var(--nw-danger);
    }

    &__toggle {
      display: inline-flex;
      gap: 3px;
      align-items: center;
      padding: 2px 0;
      margin-top: 4px;
      font-size: 11.5px;
      color: var(--el-color-primary);
      cursor: pointer;
      background: none;
      border: none;

      &:hover {
        text-decoration: underline;
      }
    }

    &__output {
      max-height: 220px;
      padding: 8px;
      margin: 4px 0 0;
      overflow: auto;
      font-family: inherit;
      font-size: 11.5px;
      line-height: 1.65;
      white-space: pre-wrap;
      word-break: break-word;
      background: var(--nw-surface);
      border: 1px solid var(--nw-border-soft);
      border-radius: var(--nw-radius-sm);
    }

    &__time {
      margin: 4px 0 0;
      font-size: 10.5px;
      color: var(--el-text-color-placeholder);
    }
  }
</style>

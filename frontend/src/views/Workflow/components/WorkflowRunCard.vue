<!-- 单条工作流运行卡片：状态、进度、当前章、耗时与「去审核」主按钮。
     待审核时整卡高亮并把主按钮换成「去审核」，因为那是此刻唯一该做的动作。 -->
<script setup lang="ts">
  import { computed } from 'vue'
  import type { NWWorkflowRunVo } from '@/api/workflow'
  import { formatTime } from '@/utils/time'

  const props = defineProps<{
    run: NWWorkflowRunVo
    acting: boolean
  }>()

  const emit = defineEmits<{
    (event: 'detail', run: NWWorkflowRunVo): void
    (event: 'control', run: NWWorkflowRunVo, action: 'pause' | 'resume' | 'retry'): void
  }>()

  const statusMeta: Record<string, { label: string; type: 'success' | 'warning' | 'danger' | 'info' | 'primary' }> = {
    queued: { label: '排队中', type: 'info' },
    running: { label: '执行中', type: 'primary' },
    waiting_review: { label: '待审核', type: 'warning' },
    succeeded: { label: '已完成', type: 'success' },
    failed: { label: '失败', type: 'danger' },
    canceled: { label: '已取消', type: 'info' },
    paused: { label: '已暂停', type: 'info' }
  }

  const waitingReview = computed(() => props.run.run_status_code === 'waiting_review')

  const progressOf = computed(() => {
    if (props.run.total_chapter_count === 0) return 0
    return Math.round((props.run.finished_chapter_count / props.run.total_chapter_count) * 100)
  })

  /** 耗时：跑完看总时长，还在跑看到此刻 */
  const elapsedText = computed(() => {
    const startTime = new Date(props.run.started_at || props.run.created_at).getTime()
    if (Number.isNaN(startTime)) return '—'
    const endTime = props.run.finished_at
      ? new Date(props.run.finished_at).getTime()
      : Date.now()
    const seconds = Math.max(0, Math.round((endTime - startTime) / 1000))
    if (seconds < 60) return `${seconds} 秒`
    const minutes = Math.round(seconds / 60)
    if (minutes < 60) return `${minutes} 分钟`
    return `${Math.floor(minutes / 60)} 小时 ${minutes % 60} 分`
  })

  const statusLabel = (code: string) => statusMeta[code]?.label ?? code
</script>

<template>
  <article
    :class="['wf-run', { 'is-review': waitingReview }]"
    @click="emit('detail', run)"
  >
    <header class="wf-run__head">
      <div class="wf-run__title">
        <strong>{{ run.template_name }}</strong>
        <ElTag :type="statusMeta[run.run_status_code]?.type ?? 'info'" size="small" effect="light">
          {{ statusLabel(run.run_status_code) }}
        </ElTag>
      </div>
      <span class="wf-run__time">{{ formatTime(run.created_at) }}</span>
    </header>

    <p class="wf-run__meta">
      {{ run.project_name }}
      <span class="wf-run__dot">·</span>
      {{ run.current_chapter_name || '—' }}
      <span class="wf-run__dot">·</span>
      目标 {{ run.target_word_count }} 字/章
    </p>

    <div class="wf-run__progress">
      <ElProgress
        :percentage="progressOf"
        :status="run.run_status_code === 'failed' ? 'exception' : undefined"
        :stroke-width="5"
        :show-text="false"
      />
      <span class="wf-run__progress-text">
        {{ run.finished_chapter_count }}/{{ run.total_chapter_count }} 章
      </span>
    </div>

    <p class="wf-run__foot-meta">
      <span>耗时 {{ elapsedText }}</span>
      <span v-if="!run.review_gate_flag" class="wf-run__dot">·</span>
      <span v-if="!run.review_gate_flag">免审核直通</span>
    </p>

    <p v-if="run.error_message" class="wf-run__error">{{ run.error_message }}</p>
    <p v-if="waitingReview" class="wf-run__review-hint">
      <Icon icon="mdi:account-check-outline" :size="13" />
      <span>当前章已出稿，等你点头才继续</span>
    </p>

    <footer class="wf-run__footer">
      <ElButton
        :type="waitingReview ? 'primary' : 'default'"
        :plain="!waitingReview"
        size="small"
        @click.stop="emit('detail', run)"
      >
        <Icon :icon="waitingReview ? 'mdi:account-check-outline' : 'mdi:eye-outline'" :size="14" />
        <span class="wf-run__btn-text">{{ waitingReview ? '去审核' : '详情' }}</span>
      </ElButton>
      <ElButton
        v-if="run.run_status_code === 'running'"
        size="small"
        text
        :loading="acting"
        @click.stop="emit('control', run, 'pause')"
      >
        暂停
      </ElButton>
      <ElButton
        v-if="run.run_status_code === 'paused'"
        size="small"
        text
        :loading="acting"
        @click.stop="emit('control', run, 'resume')"
      >
        继续
      </ElButton>
      <ElButton
        v-if="run.run_status_code === 'failed'"
        size="small"
        text
        type="primary"
        :loading="acting"
        @click.stop="emit('control', run, 'retry')"
      >
        重试
      </ElButton>
    </footer>
  </article>
</template>

<style lang="less" scoped>
  @import './workflowRunCard.less';
</style>

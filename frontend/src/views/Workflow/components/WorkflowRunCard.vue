<!-- 单条工作流运行卡片：状态、进度、章节信息与操作按钮。 -->
<script setup lang="ts">
  import type { NWWorkflowRunVo } from '@/api/workflow'
  import { formatTime } from '@/utils/time'

  defineProps<{
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

  const progressOf = (run: NWWorkflowRunVo) => {
    if (run.total_chapter_count === 0) return 0
    return Math.round((run.finished_chapter_count / run.total_chapter_count) * 100)
  }

  const statusLabel = (code: string) => statusMeta[code]?.label ?? code
</script>

<template>
  <article
    :class="['wf-run', { 'is-review': run.run_status_code === 'waiting_review' }]"
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
        :percentage="progressOf(run)"
        :status="run.run_status_code === 'failed' ? 'exception' : undefined"
        :stroke-width="5"
        :show-text="false"
      />
      <span class="wf-run__progress-text">
        {{ run.finished_chapter_count }}/{{ run.total_chapter_count }} 章
      </span>
    </div>

    <p v-if="run.error_message" class="wf-run__error">{{ run.error_message }}</p>

    <footer class="wf-run__footer">
      <ElButton size="small" text @click.stop="emit('detail', run)">
        {{ run.run_status_code === 'waiting_review' ? '去审核' : '详情' }}
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

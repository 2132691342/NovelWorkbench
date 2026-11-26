<!-- 运行详情抽屉：步骤时间线 + 待审核正文 + 通过/退回。人只看结论，不改稿。 -->
<script setup lang="ts">
  import { computed, ref, watch } from 'vue'
  import { ElMessage, ElMessageBox } from 'element-plus'
  import {
    approveWorkflowRun,
    cancelWorkflowRun,
    getWorkflowRunDetail,
    rejectWorkflowRun,
    type NWWorkflowRunDetailVo,
    type NWWorkflowStepVo
  } from '@/api/workflow'
  import { formatTime } from '@/utils/time'

  const props = defineProps<{
    visible: boolean
    runUlid: string
  }>()

  const emit = defineEmits<{
    'update:visible': [visible: boolean]
    changed: []
  }>()

  const loading = ref(false)
  const acting = ref(false)
  const detail = ref<NWWorkflowRunDetailVo | null>(null)
  const feedbackText = ref('')

  const statusMeta: Record<string, { label: string; type: 'success' | 'warning' | 'danger' | 'info' | 'primary' }> = {
    queued: { label: '排队中', type: 'info' },
    running: { label: '执行中', type: 'primary' },
    waiting_review: { label: '待审核', type: 'warning' },
    succeeded: { label: '已完成', type: 'success' },
    failed: { label: '失败', type: 'danger' },
    canceled: { label: '已取消', type: 'info' },
    paused: { label: '已暂停', type: 'info' }
  }

  const stepTypeIcon: Record<string, string> = {
    draft: 'mdi:pen',
    check: 'mdi:clipboard-check-outline',
    revise: 'mdi:auto-fix',
    review: 'mdi:magnify-scan',
    human_review: 'mdi:account-check-outline',
    summary: 'mdi:text-box-outline',
    apply: 'mdi:content-save-check-outline'
  }

  const stepStatusMeta: Record<string, { label: string; type: 'success' | 'warning' | 'danger' | 'info' | 'primary' }> = {
    queued: { label: '待执行', type: 'info' },
    running: { label: '执行中', type: 'primary' },
    success: { label: '完成', type: 'success' },
    failed: { label: '失败', type: 'danger' },
    skipped: { label: '跳过', type: 'info' },
    rejected: { label: '已退回', type: 'warning' }
  }

  const progressPercent = computed(() => {
    if (!detail.value || detail.value.total_chapter_count === 0) return 0
    return Math.round((detail.value.finished_chapter_count / detail.value.total_chapter_count) * 100)
  })

  const waitingReview = computed(() => detail.value?.run_status_code === 'waiting_review')

  /** 只展示当前章的步骤，跨章混排会看不出进度 */
  const currentChapterStepList = computed<NWWorkflowStepVo[]>(() => {
    if (!detail.value) return []
    const chapterUlid = detail.value.current_chapter_ulid
    if (!chapterUlid) return detail.value.step_list
    return detail.value.step_list.filter((step) => step.chapter_ulid === chapterUlid)
  })

  const load = async () => {
    if (!props.runUlid) return
    loading.value = true
    try {
      detail.value = await getWorkflowRunDetail(props.runUlid)
    } catch {
      detail.value = null
    } finally {
      loading.value = false
    }
  }

  watch(
    () => [props.visible, props.runUlid],
    ([visible]) => {
      if (visible) {
        feedbackText.value = ''
        void load()
      }
    }
  )

  const approve = async () => {
    acting.value = true
    try {
      await approveWorkflowRun(props.runUlid)
      ElMessage.success('已通过，继续下一章')
      emit('changed')
      await load()
    } finally {
      acting.value = false
    }
  }

  const reject = async () => {
    const feedback = feedbackText.value.trim()
    if (feedback.length < 2) {
      ElMessage.warning('请写下退回意见，AI 会据此重写')
      return
    }
    acting.value = true
    try {
      await rejectWorkflowRun(props.runUlid, feedback)
      ElMessage.success('已退回，正在按意见重写')
      feedbackText.value = ''
      emit('changed')
      await load()
    } finally {
      acting.value = false
    }
  }

  const confirmCancel = async () => {
    try {
      await ElMessageBox.confirm('取消后本次运行不再继续，已产出的正文保留。确定取消？', '取消运行', {
        type: 'warning'
      })
    } catch {
      return
    }
    await cancelWorkflowRun(props.runUlid)
    ElMessage.success('已取消')
    emit('changed')
    await load()
  }
</script>

<template>
  <ElDrawer
    :model-value="visible"
    title="运行详情"
    size="720px"
    destroy-on-close
    @update:model-value="emit('update:visible', $event)"
  >
    <div v-loading="loading" class="wf-detail">
      <template v-if="detail">
        <header class="wf-detail__head">
          <div>
            <h3 class="wf-detail__name">{{ detail.template_name }}</h3>
            <p class="wf-detail__meta">
              {{ detail.project_name }} · {{ detail.finished_chapter_count }}/{{ detail.total_chapter_count }} 章
              <span class="wf-detail__dot">·</span>
              目标 {{ detail.target_word_count }} 字/章
            </p>
          </div>
          <ElTag :type="statusMeta[detail.run_status_code]?.type ?? 'info'" effect="light">
            {{ statusMeta[detail.run_status_code]?.label ?? detail.run_status_code }}
          </ElTag>
        </header>

        <ElProgress
          :percentage="progressPercent"
          :status="detail.run_status_code === 'failed' ? 'exception' : undefined"
          :stroke-width="6"
        />

        <ElAlert
          v-if="detail.error_message"
          type="error"
          :closable="false"
          show-icon
          :title="detail.error_message"
        />

        <div class="wf-detail__stats">
          <div class="wf-detail__stat">
            <span>体检得分</span>
            <strong :class="detail.check_score >= 80 ? 'is-ok' : 'is-warn'">
              {{ detail.check_score || '—' }}
            </strong>
          </div>
          <div class="wf-detail__stat">
            <span>复审结论</span>
            <strong>{{ detail.review_verdict_code || '—' }}</strong>
          </div>
          <div class="wf-detail__stat">
            <span>当前章字数</span>
            <strong>{{ detail.chapter_word_count }}</strong>
          </div>
          <div class="wf-detail__stat">
            <span>链路 ID</span>
            <strong class="wf-detail__trace">{{ detail.trace_id || '—' }}</strong>
          </div>
        </div>

        <section v-if="waitingReview" class="wf-detail__review">
          <h4 class="wf-detail__title">
            <Icon icon="mdi:account-check-outline" :size="15" />
            <span>待你审核：{{ detail.current_chapter_name || '当前章节' }}</span>
          </h4>
          <pre class="wf-detail__content">{{ detail.chapter_content_text || '（暂无正文）' }}</pre>
          <ElInput
            v-model="feedbackText"
            type="textarea"
            :rows="3"
            maxlength="1024"
            show-word-limit
            placeholder="退回意见（不通过时必填）：例如「第三章节奏太慢，删掉赶路段，加强结尾钩子」"
          />
          <div class="wf-detail__actions">
            <ElButton type="success" :loading="acting" @click="approve()">
              <Icon icon="mdi:check" :size="15" />
              <span class="wf-detail__btn-text">通过并入库</span>
            </ElButton>
            <ElButton type="warning" :loading="acting" @click="reject()">
              <Icon icon="mdi:backup-restore" :size="15" />
              <span class="wf-detail__btn-text">退回重写</span>
            </ElButton>
          </div>
        </section>

        <section v-if="detail.review_output_text" class="wf-detail__review-text">
          <h4 class="wf-detail__title">
            <Icon icon="mdi:magnify-scan" :size="15" />
            <span>AI 复审结论</span>
          </h4>
          <pre class="wf-detail__content wf-detail__content--short">{{ detail.review_output_text }}</pre>
        </section>

        <section class="wf-detail__timeline">
          <h4 class="wf-detail__title">
            <Icon icon="mdi:timeline-clock-outline" :size="15" />
            <span>执行时间线</span>
          </h4>
          <ul class="wf-steps">
            <li v-for="step in currentChapterStepList" :key="step.step_ulid" class="wf-step">
              <span class="wf-step__icon">
                <Icon :icon="stepTypeIcon[step.step_type_code] ?? 'mdi:circle-small'" :size="14" />
              </span>
              <div class="wf-step__body">
                <div class="wf-step__head">
                  <strong>{{ step.step_name }}</strong>
                  <ElTag :type="stepStatusMeta[step.step_status_code]?.type ?? 'info'" size="small" effect="plain">
                    {{ stepStatusMeta[step.step_status_code]?.label ?? step.step_status_code }}
                  </ElTag>
                  <span class="wf-step__latency">{{ step.latency_ms }} ms</span>
                </div>
                <p v-if="step.output_text" class="wf-step__output">{{ step.output_text }}</p>
                <p v-if="step.error_message" class="wf-step__error">{{ step.error_message }}</p>
                <p class="wf-step__time">{{ formatTime(step.finished_at) }}</p>
              </div>
            </li>
            <li v-if="currentChapterStepList.length === 0" class="wf-step wf-step--empty">
              暂无步骤记录
            </li>
          </ul>
        </section>

        <div class="wf-detail__footer">
          <ElButton v-if="detail.run_status_code === 'running'" plain type="danger" @click="confirmCancel()">
            取消运行
          </ElButton>
          <span class="wf-detail__footer-hint">
            创建于 {{ formatTime(detail.created_at) }}
          </span>
        </div>
      </template>
      <ElEmpty v-else-if="!loading" description="未读取到运行详情" />
    </div>
  </ElDrawer>
</template>

<style lang="less" scoped>
  .wf-detail {
    display: flex;
    flex-direction: column;
    gap: 16px;

    &__head {
      display: flex;
      gap: 12px;
      align-items: flex-start;
      justify-content: space-between;
    }

    &__name {
      margin: 0;
      font-size: 16px;
      font-weight: 700;
      color: var(--nw-ink);
    }

    &__meta {
      margin: 4px 0 0;
      font-size: 12px;
      color: var(--el-text-color-secondary);
    }

    &__dot {
      margin: 0 4px;
    }

    &__stats {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(130px, 1fr));
      gap: 8px;
    }

    &__stat {
      display: flex;
      flex-direction: column;
      gap: 2px;
      padding: 8px 10px;
      background: var(--nw-surface-subtle);
      border: 1px solid var(--nw-border-soft);
      border-radius: var(--nw-radius-md);

      span {
        font-size: 11.5px;
        color: var(--el-text-color-secondary);
      }

      strong {
        font-size: 14px;
        font-weight: 600;
        color: var(--nw-ink);

        &.is-ok {
          color: var(--nw-success);
        }

        &.is-warn {
          color: var(--nw-warning);
        }
      }
    }

    &__trace {
      font-family: var(--nw-font-mono);
      font-size: 11px !important;
      word-break: break-all;
    }

    &__title {
      display: flex;
      gap: 6px;
      align-items: center;
      margin: 0 0 8px;
      font-size: 13px;
      font-weight: 700;
      color: var(--el-text-color-primary);
    }

    &__review,
    &__review-text,
    &__timeline {
      display: flex;
      flex-direction: column;
      gap: 10px;
    }

    &__content {
      max-height: 320px;
      padding: 12px;
      margin: 0;
      overflow: auto;
      font-family: inherit;
      font-size: 13px;
      line-height: 1.75;
      white-space: pre-wrap;
      word-break: break-word;
      background: var(--nw-surface-subtle);
      border: 1px solid var(--nw-border);
      border-radius: var(--nw-radius-md);

      &--short {
        max-height: 200px;
        font-size: 12.5px;
        color: var(--el-text-color-regular);
      }
    }

    &__actions {
      display: flex;
      gap: 8px;
    }

    &__btn-text {
      margin-left: 4px;
    }

    &__footer {
      display: flex;
      gap: 10px;
      align-items: center;
      justify-content: space-between;
      padding-top: 8px;
      border-top: 1px solid var(--nw-border-soft);
    }

    &__footer-hint {
      font-size: 11.5px;
      color: var(--el-text-color-placeholder);
    }
  }

  .wf-steps {
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 0;
    margin: 0;
    list-style: none;
  }

  .wf-step {
    display: flex;
    gap: 10px;

    &__icon {
      display: flex;
      flex: none;
      align-items: center;
      justify-content: center;
      width: 26px;
      height: 26px;
      color: var(--el-color-primary);
      background: var(--nw-primary-soft);
      border-radius: 50%;
    }

    &__body {
      flex: 1;
      min-width: 0;
      padding: 8px 10px;
      background: var(--nw-surface);
      border: 1px solid var(--nw-border-soft);
      border-radius: var(--nw-radius-md);
    }

    &__head {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      align-items: center;

      strong {
        font-size: 12.5px;
        color: var(--nw-ink);
      }
    }

    &__latency {
      margin-left: auto;
      font-size: 11px;
      color: var(--el-text-color-placeholder);
    }

    &__output {
      display: -webkit-box;
      margin: 6px 0 0;
      overflow: hidden;
      font-size: 12px;
      line-height: 1.6;
      color: var(--el-text-color-regular);
      -webkit-line-clamp: 4;
      -webkit-box-orient: vertical;
    }

    &__error {
      margin: 6px 0 0;
      font-size: 12px;
      color: var(--nw-danger);
    }

    &__time {
      margin: 4px 0 0;
      font-size: 11px;
      color: var(--el-text-color-placeholder);
    }

    &--empty {
      padding: 12px;
      font-size: 12px;
      color: var(--el-text-color-secondary);
    }
  }
</style>

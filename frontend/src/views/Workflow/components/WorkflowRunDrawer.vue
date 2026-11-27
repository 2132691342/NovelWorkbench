<!-- 运行详情抽屉：状态与进度 → 待审核正文（通过/退回）→ 步骤时间线 → 运行控制。
     抽屉只做「看一眼、点一下」，复杂列表拆到子组件，避免一个文件堆到上千行。 -->
<script setup lang="ts">
  import { computed, ref, watch } from 'vue'
  import { useRouter } from 'vue-router'
  import { ElMessage, ElMessageBox } from 'element-plus'
  import {
    cancelWorkflowRun,
    getWorkflowRunDetail,
    pauseWorkflowRun,
    resumeWorkflowRun,
    retryWorkflowRun,
    type NWWorkflowRunDetailVo
  } from '@/api/workflow'
  import { formatTime } from '@/utils/time'
  import WorkflowReviewPanel from './WorkflowReviewPanel.vue'
  import WorkflowStepTimeline from './WorkflowStepTimeline.vue'

  const props = defineProps<{
    visible: boolean
    runUlid: string
  }>()

  const emit = defineEmits<{
    'update:visible': [visible: boolean]
    changed: []
  }>()

  const router = useRouter()
  const loading = ref(false)
  const controlAction = ref<'' | 'pause' | 'resume' | 'cancel' | 'retry'>('')
  const detail = ref<NWWorkflowRunDetailVo | null>(null)

  const statusMeta: Record<string, { label: string; type: 'success' | 'warning' | 'danger' | 'info' | 'primary' }> = {
    queued: { label: '排队中', type: 'info' },
    running: { label: '执行中', type: 'primary' },
    waiting_review: { label: '待审核', type: 'warning' },
    succeeded: { label: '已完成', type: 'success' },
    failed: { label: '失败', type: 'danger' },
    canceled: { label: '已取消', type: 'info' },
    paused: { label: '已暂停', type: 'info' }
  }

  const progressPercent = computed(() => {
    if (!detail.value || detail.value.total_chapter_count === 0) return 0
    return Math.round((detail.value.finished_chapter_count / detail.value.total_chapter_count) * 100)
  })

  const waitingReview = computed(() => detail.value?.run_status_code === 'waiting_review')

  /** 失败步骤存在时给出「重试本运行」的快捷入口 */
  const failedStep = computed(() => detail.value?.step_list.find((step) => step.step_status_code === 'failed') ?? null)

  const elapsedText = computed(() => {
    if (!detail.value) return '—'
    const startTime = new Date(detail.value.started_at || detail.value.created_at).getTime()
    if (Number.isNaN(startTime)) return '—'
    const endTime = detail.value.finished_at ? new Date(detail.value.finished_at).getTime() : Date.now()
    const seconds = Math.max(0, Math.round((endTime - startTime) / 1000))
    if (seconds < 60) return `${seconds} 秒`
    const minutes = Math.round(seconds / 60)
    if (minutes < 60) return `${minutes} 分钟`
    return `${Math.floor(minutes / 60)} 小时 ${minutes % 60} 分`
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
      if (visible) void load()
    }
  )

  const copyText = async (textValue: string, label: string) => {
    if (!textValue) {
      ElMessage.warning(`${label}为空`)
      return
    }
    try {
      await navigator.clipboard.writeText(textValue)
      ElMessage.success(`已复制${label}`)
    } catch {
      ElMessage.warning('浏览器拒绝了剪贴板访问，请手动选中复制')
    }
  }

  const openTrace = () => {
    if (!detail.value?.trace_id) return
    void router.push({ path: '/admin/audit', query: { trace_id: detail.value.trace_id } })
  }

  /** 控制类操作统一走二次确认 + loading，避免误点 */
  const runControl = async (action: 'pause' | 'resume' | 'cancel' | 'retry') => {
    if (!detail.value) return
    const confirmMap: Record<typeof action, { title: string; message: string }> = {
      pause: { title: '暂停运行', message: '当前步骤跑完就停下，之后可以继续。确定暂停？' },
      resume: { title: '继续运行', message: '从上次停下的步骤继续执行，确定继续？' },
      cancel: { title: '取消运行', message: '取消后本次运行不再继续，已产出的正文保留。确定取消？' },
      retry: { title: '重试运行', message: '将从当前章第一步重跑，已有产出会被覆盖。确定重试？' }
    }
    try {
      await ElMessageBox.confirm(confirmMap[action].message, confirmMap[action].title, { type: 'warning' })
    } catch {
      return
    }
    controlAction.value = action
    try {
      const runUlid = detail.value.run_ulid
      if (action === 'pause') await pauseWorkflowRun(runUlid)
      if (action === 'resume') await resumeWorkflowRun(runUlid)
      if (action === 'cancel') await cancelWorkflowRun(runUlid)
      if (action === 'retry') await retryWorkflowRun(runUlid)
      const successMap = { pause: '已请求暂停', resume: '已继续执行', cancel: '已取消', retry: '已重新入队' }
      ElMessage.success(successMap[action])
      emit('changed')
      await load()
    } finally {
      controlAction.value = ''
    }
  }

  const handleReviewChanged = async () => {
    emit('changed')
    await load()
  }
</script>

<template>
  <ElDrawer
    :model-value="visible"
    title="运行详情"
    size="760px"
    destroy-on-close
    class="wf-drawer"
    @update:model-value="emit('update:visible', $event)"
  >
    <div v-loading="loading" class="wf-detail">
      <template v-if="detail">
        <header class="wf-detail__head">
          <div class="wf-detail__title-block">
            <h3 class="wf-detail__name">{{ detail.template_name }}</h3>
            <p class="wf-detail__meta">
              {{ detail.project_name }}
              <span class="wf-detail__dot">·</span>
              {{ detail.finished_chapter_count }}/{{ detail.total_chapter_count }} 章
              <span class="wf-detail__dot">·</span>
              {{ detail.current_chapter_name || '—' }}
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

        <div class="wf-detail__id-row">
          <button type="button" class="wf-detail__id" @click="copyText(detail.run_ulid, '运行 ID')">
            <span class="wf-detail__id-label">运行</span>
            <code>{{ detail.run_ulid }}</code>
            <Icon icon="mdi:content-copy" :size="12" />
          </button>
          <div v-if="detail.trace_id" class="wf-detail__id is-trace">
            <span class="wf-detail__id-label">链路</span>
            <code>{{ detail.trace_id }}</code>
            <button type="button" class="wf-detail__id-action" title="复制链路 ID" @click="copyText(detail.trace_id, '链路 ID')">
              <Icon icon="mdi:content-copy" :size="12" />
            </button>
            <button type="button" class="wf-detail__id-action" title="查看审计链路" @click="openTrace()">
              <Icon icon="mdi:open-in-new" :size="12" />
            </button>
          </div>
        </div>

        <div class="wf-detail__stats">
          <div class="wf-detail__stat">
            <span>耗时</span>
            <strong>{{ elapsedText }}</strong>
          </div>
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
            <span>目标字数</span>
            <strong>{{ detail.target_word_count }}</strong>
          </div>
        </div>

        <ElAlert
          v-if="detail.error_message"
          type="error"
          :closable="false"
          show-icon
          :title="detail.error_message"
        >
          <template v-if="failedStep" #default>
            <span class="wf-detail__failed-step">失败步骤：{{ failedStep.step_name }}（{{ failedStep.error_message }}）</span>
          </template>
        </ElAlert>

        <WorkflowReviewPanel v-if="waitingReview" :detail="detail" @changed="handleReviewChanged()" />

        <section v-if="detail.review_output_text" class="wf-detail__section">
          <h4 class="wf-detail__section-title">
            <Icon icon="mdi:magnify-scan" :size="14" />
            <span>AI 复审结论</span>
          </h4>
          <pre class="wf-detail__review-text">{{ detail.review_output_text }}</pre>
        </section>

        <section class="wf-detail__section">
          <h4 class="wf-detail__section-title">
            <Icon icon="mdi:timeline-clock-outline" :size="14" />
            <span>执行时间线</span>
          </h4>
          <WorkflowStepTimeline :step-list="detail.step_list" :current-chapter-ulid="detail.current_chapter_ulid" />
        </section>

        <div class="wf-detail__footer">
          <div class="wf-detail__footer-actions">
            <ElButton
              v-if="detail.run_status_code === 'running'"
              plain
              :loading="controlAction === 'pause'"
              @click="runControl('pause')"
            >
              暂停
            </ElButton>
            <ElButton
              v-if="detail.run_status_code === 'paused'"
              type="primary"
              plain
              :loading="controlAction === 'resume'"
              @click="runControl('resume')"
            >
              继续
            </ElButton>
            <ElButton
              v-if="detail.run_status_code === 'failed'"
              type="primary"
              :loading="controlAction === 'retry'"
              @click="runControl('retry')"
            >
              <Icon icon="mdi:restart" :size="14" />
              <span class="wf-detail__btn-text">重试本运行</span>
            </ElButton>
            <ElButton
              v-if="['running', 'waiting_review', 'paused', 'queued'].includes(detail.run_status_code)"
              plain
              type="danger"
              :loading="controlAction === 'cancel'"
              @click="runControl('cancel')"
            >
              取消
            </ElButton>
          </div>
          <span class="wf-detail__footer-hint">创建于 {{ formatTime(detail.created_at) }}</span>
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
    gap: 14px;

    &__head {
      display: flex;
      gap: 12px;
      align-items: flex-start;
      justify-content: space-between;
    }

    &__title-block {
      min-width: 0;
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

    &__id-row {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
    }

    &__id {
      display: inline-flex;
      gap: 6px;
      align-items: center;
      padding: 4px 8px;
      font-size: 11px;
      color: var(--el-text-color-secondary);
      cursor: pointer;
      background: var(--nw-surface-subtle);
      border: 1px solid var(--nw-border-soft);
      border-radius: var(--nw-radius-base);
      transition: border-color 0.16s ease, color 0.16s ease;

      code {
        font-family: var(--nw-font-mono);
        color: var(--nw-ink);
      }

      &:hover {
        color: var(--el-color-primary);
        border-color: var(--el-color-primary);
      }

      &.is-trace {
        cursor: default;
      }
    }

    &__id-action {
      display: inline-flex;
      padding: 0;
      color: var(--el-text-color-secondary);
      cursor: pointer;
      background: none;
      border: none;
      transition: color 0.16s ease;

      &:hover {
        color: var(--el-color-primary);
      }
    }

    &__id-label {
      font-size: 10.5px;
      color: var(--el-text-color-placeholder);
    }

    &__stats {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
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

    &__failed-step {
      font-size: 11.5px;
      color: var(--el-text-color-regular);
    }

    &__section {
      display: flex;
      flex-direction: column;
      gap: 8px;
    }

    &__section-title {
      display: flex;
      gap: 6px;
      align-items: center;
      margin: 0;
      font-size: 13px;
      font-weight: 700;
      color: var(--el-text-color-primary);
    }

    &__review-text {
      max-height: 200px;
      padding: 10px;
      margin: 0;
      overflow: auto;
      font-family: inherit;
      font-size: 12px;
      line-height: 1.7;
      white-space: pre-wrap;
      word-break: break-word;
      background: var(--nw-surface-subtle);
      border: 1px solid var(--nw-border);
      border-radius: var(--nw-radius-md);
    }

    &__footer {
      display: flex;
      flex-wrap: wrap;
      gap: 10px;
      align-items: center;
      justify-content: space-between;
      padding-top: 10px;
      border-top: 1px solid var(--nw-border-soft);
    }

    &__footer-actions {
      display: flex;
      gap: 6px;
    }

    &__footer-hint {
      font-size: 11.5px;
      color: var(--el-text-color-placeholder);
    }

    &__btn-text {
      margin-left: 4px;
    }
  }
</style>

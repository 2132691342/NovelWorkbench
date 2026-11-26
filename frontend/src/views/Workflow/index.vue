<!-- 自动化工作流中心：选模板开跑、监控进度、审核产出。核心心智是「AI 干活，人点头」。 -->
<script setup lang="ts">
  import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
  import { ElMessage } from 'element-plus'
  import { getProjectTree } from '@/api/project'
  import type { NWChapterBriefVo } from '@/api/contracts'
  import {
    fetchWorkflowRunStats,
    listWorkflowRuns,
    listWorkflowTemplates,
    pauseWorkflowRun,
    resumeWorkflowRun,
    retryWorkflowRun,
    type NWWorkflowRunStatsVo,
    type NWWorkflowRunVo,
    type NWWorkflowTemplateVo
  } from '@/api/workflow'
  import { useProjectStore } from '@/store/modules/project'
  import { usePageQuery } from '@/composables/usePageQuery'
  import WorkflowCreateDialog from './components/WorkflowCreateDialog.vue'
  import WorkflowRunCard from './components/WorkflowRunCard.vue'
  import WorkflowRunDrawer from './components/WorkflowRunDrawer.vue'

  const projectStore = useProjectStore()
  const projectUlid = computed(() => projectStore.currentProjectUlid)

  const templateList = ref<NWWorkflowTemplateVo[]>([])
  const chapterList = ref<NWChapterBriefVo[]>([])
  const stats = ref<NWWorkflowRunStatsVo>({
    running_count: 0,
    waiting_review_count: 0,
    finished_count: 0,
    failed_count: 0
  })
  const statsLoading = ref(false)
  const createVisible = ref(false)
  const drawerVisible = ref(false)
  const activeRunUlid = ref('')
  const actingUlid = ref('')

  const { items, total, pageNum, pageSize, loading, query, load, search, changePage } = usePageQuery(
    (req) => listWorkflowRuns({ ...req, project_ulid: projectUlid.value || undefined }),
    { run_status_code: '', keyword_text: '' }
  )

  const statusOptions = [
    { value: '', label: '全部状态' },
    { value: 'running', label: '执行中' },
    { value: 'waiting_review', label: '待审核' },
    { value: 'succeeded', label: '已完成' },
    { value: 'failed', label: '失败' },
    { value: 'paused', label: '已暂停' },
    { value: 'canceled', label: '已取消' }
  ]

  const statCards = computed(() => [
    { label: '执行中', value: stats.value.running_count, tone: 'primary', icon: 'mdi:progress-clock' },
    { label: '待审核', value: stats.value.waiting_review_count, tone: 'warning', icon: 'mdi:account-check-outline' },
    { label: '已完成', value: stats.value.finished_count, tone: 'success', icon: 'mdi:check-decagram-outline' },
    { label: '失败', value: stats.value.failed_count, tone: 'danger', icon: 'mdi:alert-circle-outline' }
  ])

  /** 有运行在跑时自动刷新，用户不用手点 */
  let pollTimerId: number | null = null
  const hasActiveRun = computed(() =>
    items.value.some((run) => run.run_status_code === 'running' || run.run_status_code === 'queued')
  )

  const stopPoll = () => {
    if (pollTimerId !== null) {
      window.clearInterval(pollTimerId)
      pollTimerId = null
    }
  }

  const startPoll = () => {
    stopPoll()
    pollTimerId = window.setInterval(() => {
      if (!hasActiveRun.value) {
        stopPoll()
        return
      }
      void refresh()
    }, 5000)
  }

  const loadStats = async () => {
    statsLoading.value = true
    try {
      stats.value = await fetchWorkflowRunStats(projectUlid.value || undefined)
    } catch {
      // 概览失败保留旧值
    } finally {
      statsLoading.value = false
    }
  }

  const loadTemplates = async () => {
    try {
      templateList.value = await listWorkflowTemplates()
    } catch {
      templateList.value = []
    }
  }

  const loadChapters = async () => {
    if (!projectUlid.value) {
      chapterList.value = []
      return
    }
    try {
      const tree = await getProjectTree(projectUlid.value)
      chapterList.value = (tree.volume_list ?? []).flatMap((volume) => volume.chapter_list)
    } catch {
      chapterList.value = []
    }
  }

  const refresh = () => Promise.all([load(), loadStats()])

  const openCreate = () => {
    if (!projectUlid.value) {
      ElMessage.warning('请先在顶栏选择项目')
      return
    }
    void loadChapters()
    createVisible.value = true
  }

  const openDetail = (run: NWWorkflowRunVo) => {
    activeRunUlid.value = run.run_ulid
    drawerVisible.value = true
  }

  const runControl = async (run: NWWorkflowRunVo, action: 'pause' | 'resume' | 'retry') => {
    actingUlid.value = run.run_ulid
    try {
      if (action === 'pause') await pauseWorkflowRun(run.run_ulid)
      if (action === 'resume') await resumeWorkflowRun(run.run_ulid)
      if (action === 'retry') await retryWorkflowRun(run.run_ulid)
      ElMessage.success(action === 'pause' ? '已请求暂停' : action === 'resume' ? '已继续' : '已重新入队')
      await refresh()
      startPoll()
    } finally {
      actingUlid.value = ''
    }
  }

  const handleCreated = async (runUlid: string) => {
    activeRunUlid.value = runUlid
    drawerVisible.value = true
    await refresh()
    startPoll()
  }

  watch(
    projectUlid,
    () => {
      void refresh()
      stopPoll()
    }
  )

  watch(hasActiveRun, (active) => {
    if (active) startPoll()
  })

  onMounted(async () => {
    if (!projectUlid.value) await projectStore.loadProjectOptions()
    await Promise.all([loadTemplates(), refresh()])
    startPoll()
  })

  onBeforeUnmount(stopPoll)
</script>

<template>
  <div class="nw-page wf-page">
    <ElEmpty v-if="!projectUlid" description="请先在顶栏选择项目，再启动自动化任务" :image-size="90" />

    <template v-else>
      <div class="wf-page__stats">
        <div v-for="card in statCards" :key="card.label" v-loading="statsLoading" class="wf-stat">
          <Icon :icon="card.icon" :size="18" :class="`wf-stat__icon is-${card.tone}`" />
          <div>
            <span class="wf-stat__label">{{ card.label }}</span>
            <strong :class="`wf-stat__value is-${card.tone}`">{{ card.value }}</strong>
          </div>
        </div>
      </div>

      <div class="nw-toolbar">
        <ElButton type="primary" @click="openCreate()">
          <Icon icon="mdi:play-circle-outline" :size="15" />
          <span class="wf-page__btn-text">新建自动化任务</span>
        </ElButton>
        <ElInput
          v-model="query.keyword_text"
          placeholder="搜索运行 ID / 模板名"
          clearable
          class="wf-page__search"
          @keyup.enter="search()"
          @clear="search()"
        />
        <ElSelect v-model="query.run_status_code" class="wf-page__status" @change="search()">
          <ElOption v-for="item in statusOptions" :key="item.value" :label="item.label" :value="item.value" />
        </ElSelect>
        <div class="wf-page__spacer" />
        <span class="nw-hint">共 {{ total }} 次运行</span>
      </div>

      <div v-loading="loading" class="wf-page__list">
        <WorkflowRunCard
          v-for="run in items"
          :key="run.run_ulid"
          :run="run"
          :acting="actingUlid === run.run_ulid"
          @detail="openDetail"
          @control="runControl"
        />

        <ElEmpty v-if="items.length === 0 && !loading" description="还没有运行记录，点上方按钮开始第一次自动化" />
      </div>

      <div class="nw-pager">
        <ElPagination
          :current-page="pageNum"
          :page-size="pageSize"
          :total="total"
          layout="prev, pager, next, total"
          background
          @current-change="changePage"
        />
      </div>
    </template>

    <WorkflowCreateDialog
      v-model:visible="createVisible"
      :project-ulid="projectUlid"
      :template-list="templateList"
      :chapter-list="chapterList"
      @created="handleCreated"
    />

    <WorkflowRunDrawer
      v-model:visible="drawerVisible"
      :run-ulid="activeRunUlid"
      @changed="refresh()"
    />
  </div>
</template>

<style lang="less" scoped>
  .wf-page {
    &__stats {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
      gap: 10px;
    }

    &__search {
      width: 220px;
    }

    &__status {
      width: 140px;
    }

    &__spacer {
      flex: 1;
    }

    &__list {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
      gap: 12px;
      min-height: 120px;
    }

    &__btn-text {
      margin-left: 4px;
    }
  }

  .wf-stat {
    display: flex;
    gap: 10px;
    align-items: center;
    padding: 12px 14px;
    background: var(--nw-surface);
    border: 1px solid var(--nw-border);
    border-radius: var(--nw-radius-lg);
    box-shadow: var(--nw-shadow-card);

    &__icon {
      &.is-primary {
        color: var(--el-color-primary);
      }

      &.is-warning {
        color: var(--nw-warning);
      }

      &.is-success {
        color: var(--nw-success);
      }

      &.is-danger {
        color: var(--nw-danger);
      }
    }

    &__label {
      display: block;
      font-size: 11.5px;
      color: var(--el-text-color-secondary);
    }

    &__value {
      font-size: 18px;
      font-weight: 700;
      color: var(--nw-ink);

      &.is-warning {
        color: var(--nw-warning);
      }

      &.is-success {
        color: var(--nw-success);
      }

      &.is-danger {
        color: var(--nw-danger);
      }
    }
  }

</style>

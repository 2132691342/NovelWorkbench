<!-- 仪表盘：统计卡、趋势图与项目卡片网格。 -->
<script setup lang="ts">
  import { onMounted, ref } from 'vue'
  import { useRouter } from 'vue-router'
  import { ElMessage, ElMessageBox } from 'element-plus'
  import { useI18n } from 'vue-i18n'
  import { deleteProject, listProjects } from '@/api/project'
  import { getStatsOverview } from '@/api/stats'
  import type { NWProjectVo, NWStatsOverviewVo } from '@/api/contracts'
  import { usePageQuery } from '@/composables/usePageQuery'
  import { useProjectStore } from '@/store/modules/project'
  import BootstrapDialog from './components/BootstrapDialog.vue'
  import ProjectFormDialog from './components/ProjectFormDialog.vue'
  import TrendChart from './components/TrendChart.vue'

  const { t } = useI18n()
  const router = useRouter()
  const projectStore = useProjectStore()

  const { items, total, pageNum, pageSize, loading, query, load, search, changePage } = usePageQuery(
    listProjects,
    { keyword_text: '', project_status: '' }
  )

  const stats = ref<NWStatsOverviewVo>({
    project_count: 0,
    total_word_count: 0,
    today_word_delta: 0,
    ai_word_total: 0,
    trend_list: []
  })

  const formDialog = ref<InstanceType<typeof ProjectFormDialog>>()
  const bootstrapDialog = ref<InstanceType<typeof BootstrapDialog>>()

  const statusOptions = [
    { value: '', label: '全部状态' },
    { value: 'draft', label: '草稿' },
    { value: 'serial', label: '连载中' },
    { value: 'completed', label: '已完结' }
  ]

  const statusLabel = (statusCode: string) =>
    statusOptions.find((item) => item.value === statusCode)?.label ?? statusCode

  const statusBadge = (statusCode: string) =>
    statusCode === 'serial' ? 'success' : statusCode === 'completed' ? 'neutral' : 'warning'

  const loadStats = async () => {
    stats.value = await getStatsOverview()
  }

  const refreshAll = () => Promise.all([load(), loadStats(), projectStore.loadProjectOptions()])

  const remove = async (row: NWProjectVo) => {
    try {
      await ElMessageBox.confirm(
        `将删除「${row.project_name}」及其全部卷章、设定与知识库文档，${t('common.confirmDelete')}`,
        t('common.reminder'),
        { type: 'warning' }
      )
    } catch {
      return
    }
    await deleteProject(row.project_ulid)
    ElMessage.success(t('common.deleteSuccess'))
    await refreshAll()
  }

  const enterProject = (path: string, row: NWProjectVo) => {
    projectStore.selectProject(row.project_ulid)
    void router.push(`/${path}/${row.project_ulid}`)
  }

  onMounted(() => {
    void load()
    void loadStats()
  })
</script>

<template>
  <div class="nw-page">
    <div class="nw-toolbar">
      <ElInput
        v-model="query.keyword_text"
        placeholder="搜索项目名"
        clearable
        class="dashboard__search"
        @keyup.enter="search()"
        @clear="search()"
      />
      <ElSelect
        v-model="query.project_status"
        class="dashboard__status"
        placeholder="全部状态"
        @change="search()"
      >
        <ElOption
          v-for="item in statusOptions"
          :key="item.value"
          :label="item.label"
          :value="item.value"
        />
      </ElSelect>
      <ElButton type="primary" plain @click="bootstrapDialog?.open()">
        <Icon icon="mdi:sparkles" :size="15" />
        <span class="ml-4px">AI 开书</span>
      </ElButton>
      <ElButton plain @click="formDialog?.open()">
        <Icon icon="mdi:plus" :size="15" />
        <span class="ml-4px">手动新建</span>
      </ElButton>
      <div class="dashboard__spacer" />
      <span class="nw-hint">{{ t('common.total', { total }) }}</span>
    </div>

    <div class="dashboard__stats">
      <ElCard shadow="never" class="dashboard__stat">
        <span class="dashboard__stat-label">项目数</span>
        <strong class="dashboard__stat-value">{{ stats.project_count }}</strong>
      </ElCard>
      <ElCard shadow="never" class="dashboard__stat">
        <span class="dashboard__stat-label">总字数</span>
        <strong class="dashboard__stat-value">{{ stats.total_word_count.toLocaleString() }}</strong>
      </ElCard>
      <ElCard shadow="never" class="dashboard__stat">
        <span class="dashboard__stat-label">今日新增</span>
        <strong class="dashboard__stat-value is-accent">
          {{ stats.today_word_delta.toLocaleString() }}
        </strong>
      </ElCard>
      <ElCard shadow="never" class="dashboard__stat">
        <span class="dashboard__stat-label">AI 产出</span>
        <strong class="dashboard__stat-value">{{ stats.ai_word_total.toLocaleString() }}</strong>
      </ElCard>
    </div>

    <TrendChart :trend-list="stats.trend_list" />

    <div v-loading="loading" class="dashboard__grid">
      <ElCard v-for="row in items" :key="row.project_ulid" shadow="hover" class="project-card">
        <div class="project-card__head">
          <h3 class="project-card__name" :title="row.project_name">{{ row.project_name }}</h3>
          <span class="nw-badge" :class="`nw-badge--${statusBadge(row.project_status)}`">
            {{ statusLabel(row.project_status) }}
          </span>
        </div>
        <p class="project-card__synopsis">
          {{ row.synopsis_text || '还没有简介，点「编辑」补一句。' }}
        </p>
        <div class="project-card__meta">
          <span>{{ row.volume_count }} 卷 / {{ row.chapter_count }} 章</span>
          <span>{{ row.total_word_count.toLocaleString() }} 字</span>
        </div>
        <div class="project-card__actions">
          <ElButton type="primary" plain size="small" @click="enterProject('workspace', row)">
            <Icon icon="mdi:pencil-outline" :size="14" />
            <span class="ml-4px">写作台</span>
          </ElButton>
          <ElButton size="small" plain @click="enterProject('outline', row)">大纲</ElButton>
          <ElButton size="small" plain @click="enterProject('material', row)">素材</ElButton>
          <div class="project-card__spacer" />
          <ElButton size="small" text @click="formDialog?.open(row)">编辑</ElButton>
          <ElButton size="small" text type="danger" @click="remove(row)">删除</ElButton>
        </div>
      </ElCard>

      <ElEmpty v-if="!loading && items.length === 0" description="还没有项目，先建一个吧" />
    </div>

    <div v-if="total > pageSize" class="dashboard__pager">
      <ElPagination
        :current-page="pageNum"
        :page-size="pageSize"
        :total="total"
        layout="prev, pager, next"
        background
        @current-change="changePage"
      />
    </div>

    <ProjectFormDialog ref="formDialog" @saved="refreshAll" />
    <BootstrapDialog ref="bootstrapDialog" @created="enterProject('workspace', $event)" />
  </div>
</template>

<style lang="less" scoped>
  .dashboard {
    &__search {
      width: 220px;
    }

    &__status {
      width: 140px;
    }

    &__spacer {
      flex: 1;
    }

    &__stats {
      display: grid;
      gap: 14px;
      grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
    }

    &__stat {
      :deep(.el-card__body) {
        display: flex;
        flex-direction: column;
        gap: 6px;
        padding: 16px 18px;
      }
    }

    &__stat-label {
      font-size: 12px;
      color: var(--el-text-color-secondary);
    }

    &__stat-value {
      font-size: 26px;
      font-weight: 700;
      letter-spacing: -0.02em;
      color: var(--nw-ink);
      font-variant-numeric: tabular-nums;

      &.is-accent {
        color: var(--el-color-primary);
      }
    }

    &__grid {
      display: grid;
      gap: 14px;
      min-height: 120px;
      grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
    }

    &__pager {
      display: flex;
      justify-content: flex-end;
    }
  }

  .project-card {
    border-radius: var(--nw-radius-lg);

    &__head {
      display: flex;
      gap: 10px;
      align-items: center;
      justify-content: space-between;
    }

    &__name {
      overflow: hidden;
      font-size: 15px;
      font-weight: 650;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    &__synopsis {
      display: -webkit-box;
      min-height: 38px;
      margin: 8px 0 10px;
      overflow: hidden;
      font-size: 12.5px;
      line-height: 1.6;
      color: var(--el-text-color-secondary);
      -webkit-box-orient: vertical;
      -webkit-line-clamp: 2;
    }

    &__meta {
      display: flex;
      gap: 14px;
      font-size: 12px;
      color: var(--el-text-color-placeholder);
    }

    &__actions {
      display: flex;
      gap: 4px;
      align-items: center;
      margin-top: 14px;
    }

    &__spacer {
      flex: 1;
    }
  }
</style>

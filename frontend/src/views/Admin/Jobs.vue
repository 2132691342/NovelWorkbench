<!-- 任务中心：查看异步任务状态并重试失败任务。 -->
<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import {
    ElButton,
    ElCard,
    ElInput,
    ElMessage,
    ElOption,
    ElPagination,
    ElSelect,
    ElTable,
    ElTableColumn
  } from 'element-plus'
  import {
    fetchAdminJobStats,
    listAdminJobs,
    retryAdminJob,
    type NWJobStatsVo,
    type NWJobVo
  } from '@/api/admin'
  import { usePageQuery } from '@/composables/usePageQuery'
  import { formatTime } from '@/utils/time'
  import { asRow } from '@/utils/table'

  const { items, total, pageNum, pageSize, loading, query, load, search, changePage } = usePageQuery(
    listAdminJobs,
    { job_type_code: '', job_status_code: '', keyword_text: '' }
  )

  const stats = ref<NWJobStatsVo>({
    queued_count: 0,
    running_count: 0,
    success_count: 0,
    failed_count: 0
  })
  const statsLoading = ref(false)
  const retryingUlid = ref('')

  const statusOptions = [
    { value: '', label: '全部状态' },
    { value: 'queued', label: '排队中' },
    { value: 'running', label: '执行中' },
    { value: 'success', label: '成功' },
    { value: 'failed', label: '失败' }
  ]

  const typeOptions = [
    { value: '', label: '全部类型' },
    { value: 'doc_index', label: '文档索引' },
    { value: 'doc_reindex', label: '重建索引' }
  ]

  const statusLabel = (statusCode: string) =>
    statusOptions.find((item) => item.value === statusCode)?.label ?? statusCode

  const statusBadge = (statusCode: string) => {
    if (statusCode === 'success') return 'success'
    if (statusCode === 'failed') return 'danger'
    if (statusCode === 'running') return 'warning'
    return 'neutral'
  }

  const typeLabel = (typeCode: string) =>
    typeOptions.find((item) => item.value === typeCode)?.label ?? typeCode

  /** 顶部概览卡片（随概览接口刷新） */
  const statCards = computed(() => [
    { label: '排队中', value: stats.value.queued_count, tone: 'neutral' },
    { label: '执行中', value: stats.value.running_count, tone: 'warning' },
    { label: '成功', value: stats.value.success_count, tone: 'success' },
    { label: '失败', value: stats.value.failed_count, tone: 'danger' }
  ])

  const loadStats = async () => {
    statsLoading.value = true
    try {
      stats.value = await fetchAdminJobStats()
    } catch {
      // 概览失败保留旧值，列表仍可用
    } finally {
      statsLoading.value = false
    }
  }

  const retry = async (row: NWJobVo) => {
    retryingUlid.value = row.job_ulid
    try {
      await retryAdminJob(row.job_ulid)
      ElMessage.success('已提交重试')
      await Promise.all([load(), loadStats()])
    } finally {
      retryingUlid.value = ''
    }
  }

  const refreshAll = () => Promise.all([load(), loadStats()])

  onMounted(() => {
    void refreshAll()
  })
</script>

<template>
  <div class="nw-page">
    <div class="admin-jobs__stats">
      <ElCard
        v-for="card in statCards"
        :key="card.label"
        v-loading="statsLoading"
        shadow="never"
        class="admin-jobs__stat"
      >
        <span class="admin-jobs__stat-label">{{ card.label }}</span>
        <strong class="admin-jobs__stat-value" :class="`is-${card.tone}`">{{ card.value }}</strong>
      </ElCard>
    </div>

    <div class="nw-toolbar">
      <ElInput
        v-model="query.keyword_text"
        placeholder="搜索任务或文档 ULID"
        clearable
        class="admin-jobs__search"
        @keyup.enter="search()"
        @clear="search()"
      />
      <ElSelect v-model="query.job_status_code" class="admin-jobs__select" @change="search()">
        <ElOption
          v-for="item in statusOptions"
          :key="item.value"
          :label="item.label"
          :value="item.value"
        />
      </ElSelect>
      <ElSelect v-model="query.job_type_code" class="admin-jobs__select" @change="search()">
        <ElOption
          v-for="item in typeOptions"
          :key="item.value"
          :label="item.label"
          :value="item.value"
        />
      </ElSelect>
      <ElButton type="primary" plain @click="search()">查询</ElButton>
      <ElButton @click="refreshAll()">
        <Icon icon="mdi:refresh" :size="15" />
        <span class="admin-jobs__btn-text">刷新</span>
      </ElButton>
      <div class="admin-jobs__spacer" />
      <span class="nw-hint">共 {{ total }} 个任务</span>
    </div>

    <ElTable v-loading="loading" :data="items" class="admin-jobs__table">
      <ElTableColumn label="任务" min-width="230">
        <template #default="{ row }">
          <div class="admin-jobs__job">
            <span class="admin-jobs__job-type">{{ typeLabel(row.job_type_code) }}</span>
            <span class="nw-hint">{{ row.doc_name || row.doc_ulid || row.job_ulid }}</span>
          </div>
        </template>
      </ElTableColumn>
      <ElTableColumn label="状态" width="100">
        <template #default="{ row }">
          <span class="nw-badge" :class="`nw-badge--${statusBadge(row.job_status_code)}`">
            {{ statusLabel(row.job_status_code) }}
          </span>
        </template>
      </ElTableColumn>
      <ElTableColumn prop="retry_count" label="重试次数" width="100" align="center" />
      <ElTableColumn label="失败原因" min-width="220" show-overflow-tooltip>
        <template #default="{ row }">
          <span :class="row.error_message ? 'admin-jobs__error' : 'nw-hint'">
            {{ row.error_message || '—' }}
          </span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="创建时间" width="180">
        <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
      </ElTableColumn>
      <ElTableColumn label="更新时间" width="180">
        <template #default="{ row }">{{ formatTime(row.updated_at) }}</template>
      </ElTableColumn>
      <ElTableColumn label="操作" width="100" fixed="right">
        <template #default="{ row }">
          <ElButton
            v-if="row.job_status_code === 'failed'"
            size="small"
            text
            type="primary"
            :loading="retryingUlid === row.job_ulid"
            @click="retry(asRow<NWJobVo>(row))"
          >
            重试
          </ElButton>
          <span v-else class="nw-hint">—</span>
        </template>
      </ElTableColumn>
      <template #empty>
        <span class="nw-hint">暂无任务</span>
      </template>
    </ElTable>

    <div v-if="total > pageSize" class="admin-jobs__pager">
      <ElPagination
        :current-page="pageNum"
        :page-size="pageSize"
        :total="total"
        layout="prev, pager, next"
        background
        @current-change="changePage"
      />
    </div>
  </div>
</template>

<style lang="less" scoped>
  .admin-jobs {
    &__stats {
      display: grid;
      gap: 14px;
      grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
    }

    &__stat {
      :deep(.el-card__body) {
        display: flex;
        flex-direction: column;
        gap: 6px;
        padding: 14px 18px;
      }
    }

    &__stat-label {
      font-size: 12px;
      color: var(--el-text-color-secondary);
    }

    &__stat-value {
      font-size: 24px;
      font-weight: 700;
      font-variant-numeric: tabular-nums;
      color: var(--nw-ink);

      &.is-success {
        color: var(--nw-success);
      }

      &.is-warning {
        color: var(--nw-warning);
      }

      &.is-danger {
        color: var(--nw-danger);
      }
    }

    &__search {
      width: 240px;
    }

    &__select {
      width: 130px;
    }

    &__btn-text {
      margin-left: 4px;
    }

    &__spacer {
      flex: 1;
    }

    &__table {
      border-radius: var(--nw-radius-lg);
    }

    &__job {
      display: flex;
      flex-direction: column;
      gap: 2px;
    }

    &__job-type {
      font-weight: 600;
      color: var(--nw-ink);
    }

    &__error {
      color: var(--nw-danger);
    }

    &__pager {
      display: flex;
      justify-content: flex-end;
    }
  }
</style>

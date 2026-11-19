<!--
  任务中心：知识库索引等异步任务的状态概览、分页筛选与失败重试。
  任务由 RabbitMQ worker 消费，重试会把任务与文档状态重置后重新入队。
-->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import {
  fetchAdminJobStats,
  listAdminJobs,
  retryAdminJob,
  type NWJobStatsVo,
  type NWJobVo,
} from '@/api/admin'
import NWButton from '@/components/NWButton.vue'
import NWIcon from '@/components/NWIcon.vue'
import NWPageHeader from '@/components/NWPageHeader.vue'
import NWStatTile from '@/components/NWStatTile.vue'
import NWCard from '@/components/NWCard.vue'
import NWEmpty from '@/components/NWEmpty.vue'
import NWLoading from '@/components/NWLoading.vue'

const { t } = useI18n()

const jobList = ref<NWJobVo[]>([])
const listLoading = ref(false)
const total = ref(0)
const pageNum = ref(1)
const pageSize = ref(10)
const statusFilter = ref('')
const typeFilter = ref('')
const keywordText = ref('')
const retryingKey = ref('')
const jobStats = ref<NWJobStatsVo>({
  queued_count: 0,
  running_count: 0,
  success_count: 0,
  failed_count: 0,
})

const statCardList = computed(() => [
  {
    stat_key: 'queued',
    stat_label: t('admin.jobs.queuedCount'),
    stat_value: jobStats.value.queued_count,
    stat_class: 'job-stat--queued',
  },
  {
    stat_key: 'running',
    stat_label: t('admin.jobs.runningCount'),
    stat_value: jobStats.value.running_count,
    stat_class: 'job-stat--running',
  },
  {
    stat_key: 'success',
    stat_label: t('admin.jobs.successCount'),
    stat_value: jobStats.value.success_count,
    stat_class: 'job-stat--success',
  },
  {
    stat_key: 'failed',
    stat_label: t('admin.jobs.failedCount'),
    stat_value: jobStats.value.failed_count,
    stat_class: 'job-stat--failed',
  },
])

/** 状态标签样式 */
function statusTagType(statusCode: string): 'success' | 'danger' | 'warning' | 'info' {
  if (statusCode === 'success') {
    return 'success'
  }
  if (statusCode === 'failed') {
    return 'danger'
  }
  if (statusCode === 'running') {
    return 'warning'
  }
  return 'info'
}

/** 状态文案 */
function statusLabel(statusCode: string): string {
  switch (statusCode) {
    case 'queued':
      return t('admin.jobs.statusQueued')
    case 'running':
      return t('admin.jobs.statusRunning')
    case 'success':
      return t('admin.jobs.statusSuccess')
    case 'failed':
      return t('admin.jobs.statusFailed')
    default:
      return statusCode
  }
}

/** 类型文案 */
function typeLabel(typeCode: string): string {
  return typeCode === 'doc_reindex' ? t('admin.jobs.typeReindex') : t('admin.jobs.typeIndex')
}

function formatTime(timeMs: number): string {
  return new Date(timeMs).toLocaleString()
}

async function loadJobStats(): Promise<void> {
  try {
    jobStats.value = await fetchAdminJobStats()
  } catch {
    // 失败提示由 http 层统一弹出
  }
}

async function loadJobList(): Promise<void> {
  listLoading.value = true
  try {
    const pageData = await listAdminJobs({
      page_num: pageNum.value,
      page_size: pageSize.value,
      job_status_code: statusFilter.value || undefined,
      job_type_code: typeFilter.value || undefined,
      keyword_text: keywordText.value.trim() || undefined,
    })
    jobList.value = pageData.items ?? []
    total.value = pageData.total ?? 0
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    listLoading.value = false
  }
}

function handleSearch(): void {
  pageNum.value = 1
  void loadJobList()
}

function handleResetFilters(): void {
  statusFilter.value = ''
  typeFilter.value = ''
  keywordText.value = ''
  handleSearch()
}

/** 重试失败任务 */
async function handleRetry(jobItem: NWJobVo): Promise<void> {
  try {
    await ElMessageBox.confirm(t('admin.jobs.retryConfirm'), t('common.confirm'), {
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel'),
      type: 'warning',
    })
  } catch {
    return
  }
  retryingKey.value = jobItem.job_ulid
  try {
    await retryAdminJob(jobItem.job_ulid)
    ElMessage.success(t('admin.jobs.retrySuccess'))
    await Promise.all([loadJobList(), loadJobStats()])
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    retryingKey.value = ''
  }
}

function handlePageChange(page: number): void {
  pageNum.value = page
  void loadJobList()
}

function handleSizeChange(size: number): void {
  pageSize.value = size
  pageNum.value = 1
  void loadJobList()
}

function handleRefresh(): void {
  void loadJobList()
  void loadJobStats()
}

onMounted(() => {
  void loadJobList()
  void loadJobStats()
})
</script>

<template>
  <div class="nw-page job-page">
    <NWPageHeader
      :title_text="t('admin.jobs.title')"
      :subtitle_text="t('admin.jobs.pageSubtitle')"
      icon_code="pulse"
    >
      <template #actions>
        <NWButton :loading="listLoading" @click="handleRefresh">
          <NWIcon icon_code="refresh" :size_px="15" />
          {{ t('admin.jobs.refresh') }}
        </NWButton>
      </template>
    </NWPageHeader>

    <div class="nw-stat-grid">
      <NWStatTile
        v-for="statCard in statCardList"
        :key="statCard.stat_key"
        :label_text="statCard.stat_label"
        :value_number="statCard.stat_value"
        :hint_text="statCard.stat_key === 'failed' ? t('admin.jobs.failedHint') : ''"
        :icon_code="statCard.stat_key === 'queued' ? 'clock' : statCard.stat_key === 'running' ? 'play' : statCard.stat_key === 'success' ? 'checkCircle' : 'alertCircle'"
        :tone_code="statCard.stat_key === 'queued' ? 'info' : statCard.stat_key === 'running' ? 'warning' : statCard.stat_key === 'success' ? 'success' : 'danger'"
      />
    </div>

    <NWCard :hoverable="false">

      <div class="job-page__toolbar">
        <el-input
          v-model="keywordText"
          class="job-page__filter"
          :placeholder="t('admin.jobs.keywordPlaceholder')"
          clearable
          @keyup.enter="handleSearch"
        />
        <el-select
          v-model="statusFilter"
          class="job-page__select"
          :placeholder="t('admin.jobs.statusAll')"
          clearable
          @change="handleSearch"
        >
          <el-option :label="t('admin.jobs.statusQueued')" value="queued" />
          <el-option :label="t('admin.jobs.statusRunning')" value="running" />
          <el-option :label="t('admin.jobs.statusSuccess')" value="success" />
          <el-option :label="t('admin.jobs.statusFailed')" value="failed" />
        </el-select>
        <el-select
          v-model="typeFilter"
          class="job-page__select"
          :placeholder="t('admin.jobs.typeAll')"
          clearable
          @change="handleSearch"
        >
          <el-option :label="t('admin.jobs.typeIndex')" value="doc_index" />
          <el-option :label="t('admin.jobs.typeReindex')" value="doc_reindex" />
        </el-select>
        <NWButton type="primary" @click="handleSearch">{{ t('admin.search') }}</NWButton>
        <NWButton @click="handleResetFilters">{{ t('admin.reset') }}</NWButton>
      </div>

      <NWLoading v-if="listLoading" />
      <template v-else>
        <el-table :data="jobList" stripe>
          <el-table-column :label="t('admin.jobs.createdAt')" width="170">
            <template #default="{ row }">{{ formatTime(row.created_at_ms) }}</template>
          </el-table-column>
          <el-table-column :label="t('admin.jobs.jobType')" width="120">
            <template #default="{ row }">{{ typeLabel(row.job_type_code) }}</template>
          </el-table-column>
          <el-table-column :label="t('admin.jobs.docName')" min-width="180">
            <template #default="{ row }">
              <div class="job-page__doc">
                <strong>{{ row.doc_name || t('admin.jobs.docDeleted') }}</strong>
                <span class="job-page__muted">{{ row.doc_ulid }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column :label="t('admin.jobs.status')" width="110">
            <template #default="{ row }">
              <el-tag size="small" effect="light" :type="statusTagType(row.job_status_code)">
                {{ statusLabel(row.job_status_code) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="retry_count" :label="t('admin.jobs.retryCount')" width="100" />
          <el-table-column :label="t('admin.jobs.errorMessage')" min-width="220">
            <template #default="{ row }">
              <el-tooltip v-if="row.error_message" :content="row.error_message" placement="top">
                <span class="job-page__error-text">{{ row.error_message }}</span>
              </el-tooltip>
              <span v-else class="job-page__muted">-</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('common.action')" width="120" fixed="right">
            <template #default="{ row }">
              <NWButton
                v-if="row.job_status_code === 'failed'"
                type="text"
                size="small"
                :loading="retryingKey === row.job_ulid"
                @click="handleRetry(row)"
              >
                {{ t('admin.jobs.retry') }}
              </NWButton>
              <span v-else class="job-page__muted">-</span>
            </template>
          </el-table-column>
          <template #empty>
            <NWEmpty :description_text="t('admin.jobs.emptyTip')" />
          </template>
        </el-table>
        <el-pagination
          v-if="total > 0"
          class="job-page__pagination"
          layout="total, prev, pager, next, sizes"
          :total="total"
          :current-page="pageNum"
          :page-size="pageSize"
          :page-sizes="[10, 20, 50]"
          @current-change="handlePageChange"
          @size-change="handleSizeChange"
        />
      </template>
    </NWCard>
  </div>
</template>

<style scoped>
.job-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.job-page__stats {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 12px;
}

.job-stat {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 16px 18px;
  border-radius: var(--radius-lg);
  background-color: var(--bg-card);
  border: 1px solid var(--border-light);
  box-shadow: var(--shadow-card);
}

.job-stat--queued {
  border-left: 3px solid var(--info);
}

.job-stat--running {
  border-left: 3px solid var(--warning);
}

.job-stat--success {
  border-left: 3px solid var(--success);
}

.job-stat--failed {
  border-left: 3px solid var(--error);
}

.job-stat__label {
  color: var(--text-secondary);
  font-size: 12.5px;
}

.job-stat__value {
  color: var(--text-main);
  font-size: 24px;
  font-weight: 700;
  letter-spacing: -0.02em;
}

.job-page__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  width: 100%;
}

.job-page__subtitle {
  margin: 4px 0 0;
  color: var(--text-secondary);
  font-size: 12.5px;
}

.job-page__toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 14px;
}

.job-page__filter {
  width: 220px;
}

.job-page__select {
  width: 150px;
}

.job-page__doc {
  display: flex;
  flex-direction: column;
  line-height: 1.4;
}

.job-page__muted {
  color: var(--text-secondary);
  font-size: 12px;
}

.job-page__pagination {
  margin-top: 14px;
  justify-content: flex-end;
}
.job-page__doc {
  display: flex;
  flex-direction: column;
  min-width: 0;
  line-height: 1.35;
}

.job-page__doc strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text-main);
  font-size: 12.5px;
  font-weight: 550;
}

.job-page__muted {
  color: var(--text-placeholder);
  font-size: 10px;
}

.job-page__error-text {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  color: var(--error-text);
  font-size: 11.5px;
  line-height: 1.5;
}
</style>

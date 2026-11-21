<!--
  登录日志页：账号/IP、结果与日期筛选 + 只读分页表格。
  记录成功、密码错误与禁用账号三种结果，用于安全审计。
-->
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { listAdminLoginLogs, type NWLoginLogVo } from '@/api/admin'
import NWButton from '@/components/NWButton.vue'
import NWPageHeader from '@/components/NWPageHeader.vue'
import NWCard from '@/components/NWCard.vue'
import NWEmpty from '@/components/NWEmpty.vue'
import NWLoading from '@/components/NWLoading.vue'
import { formatTime } from '@/utils/time'

const { t } = useI18n()

const loginLogList = ref<NWLoginLogVo[]>([])
const listLoading = ref(false)
const total = ref(0)
const pageNum = ref(1)
const pageSize = ref(10)
const keywordText = ref('')
const resultFilter = ref('')
const dateRange = ref<[string, string] | null>(null)

/** 结果标签样式 */
function resultTagType(resultCode: string): 'success' | 'danger' | 'warning' {
  if (resultCode === 'success') {
    return 'success'
  }
  if (resultCode === 'disabled') {
    return 'warning'
  }
  return 'danger'
}

/** 结果文案 */
function resultLabel(resultCode: string): string {
  if (resultCode === 'success') {
    return t('admin.loginLogs.resultSuccess')
  }
  if (resultCode === 'disabled') {
    return t('admin.loginLogs.resultDisabled')
  }
  return t('admin.loginLogs.resultFailed')
}

/** 加载登录日志分页 */
async function loadLoginLogList(): Promise<void> {
  listLoading.value = true
  try {
    const pageData = await listAdminLoginLogs({
      page_num: pageNum.value,
      page_size: pageSize.value,
      keyword_text: keywordText.value.trim() || undefined,
      login_result_code: resultFilter.value || undefined,
      date_from: dateRange.value?.[0],
      date_to: dateRange.value?.[1],
    })
    loginLogList.value = pageData.items ?? []
    total.value = pageData.total ?? 0
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    listLoading.value = false
  }
}

function handleSearch(): void {
  pageNum.value = 1
  void loadLoginLogList()
}

function handleResetFilters(): void {
  keywordText.value = ''
  resultFilter.value = ''
  dateRange.value = null
  handleSearch()
}

function handlePageChange(page: number): void {
  pageNum.value = page
  void loadLoginLogList()
}

function handleSizeChange(size: number): void {
  pageSize.value = size
  pageNum.value = 1
  void loadLoginLogList()
}

onMounted(() => {
  void loadLoginLogList()
})
</script>

<template>
  <div class="nw-page login-log-page">
    <NWPageHeader
      :title_text="t('admin.loginLogs.title')"
      :subtitle_text="t('admin.loginLogs.subtitle')"
      icon_code="lock"
    />
    <NWCard :hoverable="false">
      <div class="login-log-page__toolbar">
        <el-input
          v-model="keywordText"
          class="login-log-page__filter"
          :placeholder="t('admin.loginLogs.keywordPlaceholder')"
          clearable
          @keyup.enter="handleSearch"
        />
        <el-select
          v-model="resultFilter"
          class="login-log-page__filter"
          :placeholder="t('admin.loginLogs.resultAll')"
          clearable
          @change="handleSearch"
        >
          <el-option :label="t('admin.loginLogs.resultSuccess')" value="success" />
          <el-option :label="t('admin.loginLogs.resultFailed')" value="failed" />
          <el-option :label="t('admin.loginLogs.resultDisabled')" value="disabled" />
        </el-select>
        <el-date-picker
          v-model="dateRange"
          type="daterange"
          value-format="YYYY-MM-DD"
          :start-placeholder="t('admin.loginLogs.dateFrom')"
          :end-placeholder="t('admin.loginLogs.dateTo')"
          class="login-log-page__date"
          @change="handleSearch"
        />
        <NWButton type="primary" @click="handleSearch">{{ t('admin.search') }}</NWButton>
        <NWButton @click="handleResetFilters">{{ t('admin.reset') }}</NWButton>
      </div>

      <NWLoading v-if="listLoading" />
      <template v-else>
        <el-table :data="loginLogList" stripe>
          <el-table-column :label="t('admin.loginLogs.createdAt')" width="170">
            <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
          </el-table-column>
          <el-table-column prop="login_account_text" :label="t('admin.loginLogs.account')" width="180" />
          <el-table-column :label="t('admin.loginLogs.result')" width="110">
            <template #default="{ row }">
              <el-tag size="small" effect="light" :type="resultTagType(row.login_result_code)">
                {{ resultLabel(row.login_result_code) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="fail_reason_text" :label="t('admin.loginLogs.failReason')" min-width="150" />
          <el-table-column prop="ip_addr" :label="t('admin.loginLogs.ipAddr')" width="140" />
          <el-table-column :label="t('admin.loginLogs.userAgent')" min-width="220" show-overflow-tooltip>
            <template #default="{ row }">{{ row.user_agent_text || '-' }}</template>
          </el-table-column>
          <el-table-column prop="user_ulid" :label="t('admin.loginLogs.userUlid')" width="220">
            <template #default="{ row }">
              <span class="login-log-page__muted">{{ row.user_ulid || '-' }}</span>
            </template>
          </el-table-column>
          <template #empty>
            <NWEmpty :description_text="t('admin.loginLogs.emptyTip')" />
          </template>
        </el-table>
        <el-pagination
          v-if="total > 0"
          class="login-log-page__pagination"
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
.login-log-page__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  width: 100%;
}

.login-log-page__subtitle {
  margin: 4px 0 0;
  color: var(--text-secondary);
  font-size: 12.5px;
}

.login-log-page__toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 14px;
}

.login-log-page__filter {
  width: 200px;
}

.login-log-page__date {
  width: 260px;
}

.login-log-page__muted {
  color: var(--text-secondary);
  font-size: 12px;
}

.login-log-page__pagination {
  margin-top: 14px;
  justify-content: flex-end;
}
</style>

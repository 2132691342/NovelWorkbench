<!--
  审计日志页：筛选区（动作编码 / 用户 ULID）+ 只读分页表格。
-->
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { listAdminAudits, type NWAuditVo } from '@/api/admin'
import NWButton from '@/components/NWButton.vue'
import NWCard from '@/components/NWCard.vue'
import NWEmpty from '@/components/NWEmpty.vue'
import NWLoading from '@/components/NWLoading.vue'

const { t } = useI18n()

const auditList = ref<NWAuditVo[]>([])
const listLoading = ref(false)
const total = ref(0)
const pageNum = ref(1)
const pageSize = ref(10)
const actionFilter = ref('')
const userFilter = ref('')

/** 毫秒时间戳转本地时间 */
function formatTime(timeMs: number): string {
  return new Date(timeMs).toLocaleString()
}

/** 加载审计日志分页 */
async function loadAuditList(): Promise<void> {
  listLoading.value = true
  try {
    const pageData = await listAdminAudits({
      page_num: pageNum.value,
      page_size: pageSize.value,
      action_code: actionFilter.value.trim() || undefined,
      user_ulid: userFilter.value.trim() || undefined,
    })
    auditList.value = pageData.items ?? []
    total.value = pageData.total ?? 0
  } catch {
    // 失败提示由 http 层统一弹出（静默），保持当前列表
  } finally {
    listLoading.value = false
  }
}

/** 搜索：回到第一页重新加载 */
function handleSearch(): void {
  pageNum.value = 1
  void loadAuditList()
}

/** 重置筛选条件 */
function handleResetFilters(): void {
  actionFilter.value = ''
  userFilter.value = ''
  handleSearch()
}

function handlePageChange(page: number): void {
  pageNum.value = page
  void loadAuditList()
}

function handleSizeChange(size: number): void {
  pageSize.value = size
  pageNum.value = 1
  void loadAuditList()
}

onMounted(() => {
  void loadAuditList()
})
</script>

<template>
  <div class="admin-audit-page">
    <NWCard :hoverable="false">
      <template #header>
        <h3 class="nw-card__title">{{ t('admin.audit.title') }}</h3>
        <div class="admin-audit-page__toolbar">
          <el-input
            v-model="actionFilter"
            class="admin-audit-page__filter"
            :placeholder="t('admin.audit.actionPlaceholder')"
            clearable
            @keyup.enter="handleSearch"
          />
          <el-input
            v-model="userFilter"
            class="admin-audit-page__filter"
            :placeholder="t('admin.audit.userPlaceholder')"
            clearable
            @keyup.enter="handleSearch"
          />
          <NWButton type="primary" @click="handleSearch">{{ t('admin.search') }}</NWButton>
          <NWButton @click="handleResetFilters">{{ t('admin.reset') }}</NWButton>
        </div>
      </template>

      <NWLoading v-if="listLoading" />
      <template v-else>
        <el-table :data="auditList" stripe>
          <el-table-column :label="t('admin.audit.createdAt')" width="170">
            <template #default="{ row }">
              {{ formatTime(row.created_at_ms) }}
            </template>
          </el-table-column>
          <el-table-column :label="t('admin.audit.user')" width="180">
            <template #default="{ row }">
              <div class="admin-audit-page__user">
                <span>{{ row.user_name || '-' }}</span>
                <span class="admin-audit-page__muted">{{ row.user_ulid }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column prop="action_code" :label="t('admin.audit.actionCode')" width="160" />
          <el-table-column prop="target_type" :label="t('admin.audit.targetType')" width="120" />
          <el-table-column prop="target_ulid" :label="t('admin.audit.targetUlid')" width="200" />
          <el-table-column prop="detail_text" :label="t('admin.audit.detailText')" min-width="220" />
          <el-table-column prop="ip_addr" :label="t('admin.audit.ipAddr')" width="130" />
          <template #empty>
            <NWEmpty :description_text="t('admin.audit.emptyTip')" />
          </template>
        </el-table>
        <el-pagination
          v-if="total > 0"
          class="admin-audit-page__pagination"
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
.admin-audit-page__toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.admin-audit-page__filter {
  width: 200px;
}

.admin-audit-page__user {
  display: flex;
  flex-direction: column;
  line-height: 1.4;
}

.admin-audit-page__muted {
  color: var(--text-secondary);
  font-size: 12px;
}

.admin-audit-page__pagination {
  margin-top: 14px;
  justify-content: flex-end;
}
</style>

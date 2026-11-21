<!-- 审计日志页：按动作码与目标查询写操作留痕。 -->
<script setup lang="ts">
  import { onMounted, ref } from 'vue'
  import {
    ElButton,
    ElDatePicker,
    ElInput,
    ElPagination,
    ElTable,
    ElTableColumn
  } from 'element-plus'
  import { listAdminAudits, type NWAuditVo } from '@/api/admin'
  import { formatTime } from '@/utils/time'

  const rows = ref<NWAuditVo[]>([])
  const total = ref(0)
  const pageNum = ref(1)
  const pageSize = ref(20)
  const loading = ref(false)
  const keyword = ref('')
  const dateRange = ref<string[] | null>(null)
  const scanCapped = ref(false)

  /** 日期筛选的扫描上限：最多回看 1000 条审计记录（接口未提供日期条件） */
  const SCAN_LIMIT = 1000
  const SCAN_PAGE_SIZE = 100

  const dateOf = (timeText: string) => (timeText ? timeText.slice(0, 10) : '')

  /** 无日期条件走服务端分页；有日期条件回看最近记录后按日期本地过滤 */
  const load = async () => {
    loading.value = true
    scanCapped.value = false
    const actionCode = keyword.value.trim() || undefined
    try {
      if (!dateRange.value) {
        const page = await listAdminAudits({
          page_num: pageNum.value,
          page_size: pageSize.value,
          action_code: actionCode
        })
        rows.value = page.items ?? []
        total.value = page.total ?? 0
        return
      }

      const rangeStart = dateRange.value[0]
      const rangeEnd = dateRange.value[1]
      const matched: NWAuditVo[] = []
      let scanned = 0
      let nextPage = 1
      while (scanned < SCAN_LIMIT) {
        const page = await listAdminAudits({
          page_num: nextPage,
          page_size: SCAN_PAGE_SIZE,
          action_code: actionCode
        })
        const items = page.items ?? []
        scanned += items.length
        matched.push(
          ...items.filter((item) => {
            const day = dateOf(item.created_at)
            return day >= rangeStart && day <= rangeEnd
          })
        )
        const oldest = items[items.length - 1]
        if (items.length < SCAN_PAGE_SIZE) break
        if (oldest && dateOf(oldest.created_at) < rangeStart) break
        nextPage += 1
      }

      scanCapped.value = scanned >= SCAN_LIMIT
      total.value = matched.length
      const start = (pageNum.value - 1) * pageSize.value
      rows.value = matched.slice(start, start + pageSize.value)
    } catch {
      rows.value = []
      total.value = 0
    } finally {
      loading.value = false
    }
  }

  const search = async () => {
    pageNum.value = 1
    await load()
  }

  const changePage = async (nextPage: number) => {
    pageNum.value = nextPage
    await load()
  }

  onMounted(() => {
    void load()
  })
</script>

<template>
  <div class="nw-page">
    <div class="nw-toolbar">
      <ElInput
        v-model="keyword"
        placeholder="搜索动作码"
        clearable
        class="admin-audit__search"
        @keyup.enter="search()"
        @clear="search()"
      />
      <ElDatePicker
        v-model="dateRange"
        type="daterange"
        value-format="YYYY-MM-DD"
        start-placeholder="开始日期"
        end-placeholder="结束日期"
        class="admin-audit__date"
        @change="search()"
      />
      <ElButton type="primary" plain @click="search()">查询</ElButton>
      <div class="admin-audit__spacer" />
      <span class="nw-hint">共 {{ total }} 条记录</span>
    </div>

    <p v-if="dateRange" class="nw-hint">
      按日期筛选时回看最近 {{ SCAN_LIMIT }} 条审计记录<span v-if="scanCapped">（已达上限，更早记录未纳入）</span>。
    </p>

    <ElTable v-loading="loading" :data="rows" class="admin-audit__table">
      <ElTableColumn label="时间" width="180">
        <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
      </ElTableColumn>
      <ElTableColumn label="操作人" min-width="130">
        <template #default="{ row }">
          <span :class="row.user_name ? '' : 'nw-hint'">{{ row.user_name || '未登录' }}</span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="动作码" min-width="220" show-overflow-tooltip>
        <template #default="{ row }">
          <code class="admin-audit__code">{{ row.action_code }}</code>
        </template>
      </ElTableColumn>
      <ElTableColumn label="目标" min-width="160" show-overflow-tooltip>
        <template #default="{ row }">
          <span v-if="row.target_type">
            {{ row.target_type }}
            <span class="nw-hint">{{ row.target_ulid }}</span>
          </span>
          <span v-else class="nw-hint">—</span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="详情" min-width="240" show-overflow-tooltip>
        <template #default="{ row }">
          <span :class="row.detail_text ? '' : 'nw-hint'">{{ row.detail_text || '—' }}</span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="IP" width="130">
        <template #default="{ row }">
          <span :class="row.ip_addr ? '' : 'nw-hint'">{{ row.ip_addr || '—' }}</span>
        </template>
      </ElTableColumn>
      <template #empty>
        <span class="nw-hint">暂无审计记录</span>
      </template>
    </ElTable>

    <div v-if="total > pageSize" class="admin-audit__pager">
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
  .admin-audit {
    &__search {
      width: 240px;
    }

    &__date {
      width: 260px;
    }

    &__spacer {
      flex: 1;
    }

    &__table {
      border-radius: var(--nw-radius-lg);
    }

    &__code {
      padding: 1px 6px;
      font-family: var(--el-font-family-mono, monospace);
      font-size: 11.5px;
      color: var(--nw-text-secondary);
      background: var(--nw-surface-subtle);
      border-radius: var(--nw-radius-sm);
    }

    &__pager {
      display: flex;
      justify-content: flex-end;
    }
  }
</style>

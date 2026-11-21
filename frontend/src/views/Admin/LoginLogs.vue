<!-- 登录日志页：查询登录成功与失败记录。 -->
<script setup lang="ts">
  import { onMounted, ref } from 'vue'
  import {
    ElButton,
    ElDatePicker,
    ElInput,
    ElOption,
    ElPagination,
    ElSelect,
    ElTable,
    ElTableColumn
  } from 'element-plus'
  import { listAdminLoginLogs } from '@/api/admin'
  import { usePageQuery } from '@/composables/usePageQuery'
  import { formatTime } from '@/utils/time'

  const { items, total, pageNum, pageSize, loading, query, load, search, changePage } = usePageQuery(
    listAdminLoginLogs,
    { keyword_text: '', login_result_code: '', date_from: '', date_to: '' }
  )

  /** 日期区间（YYYY-MM-DD），变更时拆分到查询条件 */
  const dateRange = ref<string[] | null>(null)

  const resultOptions = [
    { value: '', label: '全部结果' },
    { value: 'success', label: '成功' },
    { value: 'failed', label: '失败' },
    { value: 'disabled', label: '账号禁用' }
  ]

  const resultLabel = (resultCode: string) =>
    resultOptions.find((item) => item.value === resultCode)?.label ?? resultCode

  const resultBadge = (resultCode: string) =>
    resultCode === 'success' ? 'success' : resultCode === 'disabled' ? 'warning' : 'danger'

  const handleDateChange = (range: [string, string] | null) => {
    query.value.date_from = range?.[0] ?? ''
    query.value.date_to = range?.[1] ?? ''
    void search()
  }

  onMounted(() => {
    void load()
  })
</script>

<template>
  <div class="nw-page">
    <div class="nw-toolbar">
      <ElInput
        v-model="query.keyword_text"
        placeholder="搜索账号或 IP"
        clearable
        class="admin-login__search"
        @keyup.enter="search()"
        @clear="search()"
      />
      <ElSelect v-model="query.login_result_code" class="admin-login__result" @change="search()">
        <ElOption
          v-for="item in resultOptions"
          :key="item.value"
          :label="item.label"
          :value="item.value"
        />
      </ElSelect>
      <ElDatePicker
        v-model="dateRange"
        type="daterange"
        value-format="YYYY-MM-DD"
        start-placeholder="开始日期"
        end-placeholder="结束日期"
        class="admin-login__date"
        @change="handleDateChange"
      />
      <ElButton type="primary" plain @click="search()">查询</ElButton>
      <div class="admin-login__spacer" />
      <span class="nw-hint">共 {{ total }} 条记录</span>
    </div>

    <ElTable v-loading="loading" :data="items" class="admin-login__table">
      <ElTableColumn label="登录时间" width="180">
        <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
      </ElTableColumn>
      <ElTableColumn label="登录账号" min-width="170" show-overflow-tooltip>
        <template #default="{ row }">
          <span class="admin-login__account">{{ row.login_account_text }}</span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="IP" width="140">
        <template #default="{ row }">
          <span :class="row.ip_addr ? '' : 'nw-hint'">{{ row.ip_addr || '—' }}</span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="结果" width="110">
        <template #default="{ row }">
          <span class="nw-badge" :class="`nw-badge--${resultBadge(row.login_result_code)}`">
            {{ resultLabel(row.login_result_code) }}
          </span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="失败原因" min-width="180" show-overflow-tooltip>
        <template #default="{ row }">
          <span :class="row.fail_reason_text ? '' : 'nw-hint'">
            {{ row.fail_reason_text || '—' }}
          </span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="客户端" min-width="220" show-overflow-tooltip>
        <template #default="{ row }">
          <span class="nw-hint">{{ row.user_agent_text || '—' }}</span>
        </template>
      </ElTableColumn>
      <template #empty>
        <span class="nw-hint">暂无登录日志</span>
      </template>
    </ElTable>

    <div v-if="total > pageSize" class="admin-login__pager">
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
  .admin-login {
    &__search {
      width: 220px;
    }

    &__result {
      width: 130px;
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

    &__account {
      font-weight: 600;
      color: var(--nw-ink);
    }

    &__pager {
      display: flex;
      justify-content: flex-end;
    }
  }
</style>

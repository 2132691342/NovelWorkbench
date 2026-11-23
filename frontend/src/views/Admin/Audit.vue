<!-- 审计日志页：按级别、动作、链路 ID、目标与日期筛选写操作留痕，抽屉查看请求体与响应。 -->
<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import { ElMessage } from 'element-plus'
  import {
    getAdminAuditDetail,
    listAdminAudits,
    type NWAuditDetailVo,
    type NWAuditListReqVo,
    type NWAuditVo
  } from '@/api/admin'
  import { usePageQuery } from '@/composables/usePageQuery'
  import { formatTime } from '@/utils/time'

  const { items, total, pageNum, pageSize, loading, query, load, search, changePage } = usePageQuery<
    NWAuditVo,
    Omit<NWAuditListReqVo, 'page_num' | 'page_size'>
  >(listAdminAudits, {
    log_level_code: '',
    action_code: '',
    keyword_text: '',
    trace_id: '',
    date_from: '',
    date_to: ''
  })

  const levelOptions = [
    { value: '', label: '全部级别' },
    { value: 'info', label: '正常' },
    { value: 'warn', label: '警示' },
    { value: 'error', label: '错误' }
  ]

  const dateRange = ref<[string, string] | null>(null)
  const detailVisible = ref(false)
  const detailLoading = ref(false)
  const detail = ref<NWAuditDetailVo | null>(null)

  const levelTagType = (levelCode: string) =>
    levelCode === 'error'
      ? 'danger'
      : levelCode === 'warn'
        ? 'warning'
        : levelCode === 'info'
          ? 'success'
          : 'info'

  const levelLabel = (levelCode: string) => {
    if (!levelCode) return '—'
    return levelOptions.find((item) => item.value === levelCode)?.label ?? levelCode
  }

  const prettyBody = computed(() => {
    if (!detail.value?.request_body) return ''
    try {
      return JSON.stringify(JSON.parse(detail.value.request_body), null, 2)
    } catch {
      return detail.value.request_body
    }
  })

  const searchWithDate = () => {
    query.value.date_from = dateRange.value?.[0] ?? ''
    query.value.date_to = dateRange.value?.[1] ?? ''
    void search()
  }

  const openDetail = async (row: NWAuditVo) => {
    detailVisible.value = true
    detailLoading.value = true
    detail.value = null
    try {
      detail.value = await getAdminAuditDetail(row.audit_ulid)
    } catch {
      detailVisible.value = false
    } finally {
      detailLoading.value = false
    }
  }

  const copyTrace = async (traceId: string) => {
    if (!traceId) return
    try {
      await navigator.clipboard.writeText(traceId)
      ElMessage.success('链路 ID 已复制')
    } catch {
      ElMessage.warning('浏览器不允许写剪贴板，请手动选择复制')
    }
  }

  onMounted(() => {
    void load()
  })
</script>

<template>
  <div class="nw-page">
    <div class="nw-toolbar nw-toolbar--wrap">
      <ElInput
        v-model="query.keyword_text"
        placeholder="搜索路径 / 用户 / 响应说明"
        clearable
        class="audit-page__search"
        @keyup.enter="searchWithDate()"
        @clear="searchWithDate()"
      />
      <ElInput
        v-model="query.action_code"
        placeholder="动作码，如 admin/user"
        clearable
        class="audit-page__action"
        @keyup.enter="searchWithDate()"
        @clear="searchWithDate()"
      />
      <ElSelect
        v-model="query.log_level_code"
        class="audit-page__level"
        placeholder="全部级别"
        @change="searchWithDate()"
      >
        <ElOption v-for="item in levelOptions" :key="item.value" :label="item.label" :value="item.value" />
      </ElSelect>
      <ElDatePicker
        v-model="dateRange"
        type="daterange"
        value-format="YYYY-MM-DD"
        start-placeholder="开始日期"
        end-placeholder="结束日期"
        class="audit-page__date"
        @change="searchWithDate()"
      />
      <ElButton type="primary" @click="searchWithDate()">
        <Icon icon="mdi:magnify" :size="15" />
        <span class="audit-page__btn-text">查询</span>
      </ElButton>
      <div class="audit-page__spacer" />
      <span class="nw-hint">共 {{ total }} 条</span>
    </div>

    <ElTable v-loading="loading" :data="items" class="audit-page__table" row-key="audit_ulid">
      <ElTableColumn label="时间" width="170">
        <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
      </ElTableColumn>
      <ElTableColumn label="级别" width="86" align="center">
        <template #default="{ row }">
          <ElTag :type="levelTagType(row.log_level_code)" effect="light" size="small">
            {{ levelLabel(row.log_level_code) }}
          </ElTag>
        </template>
      </ElTableColumn>
      <ElTableColumn label="操作人" width="130">
        <template #default="{ row }">{{ row.user_name || '—' }}</template>
      </ElTableColumn>
      <ElTableColumn label="动作" min-width="190">
        <template #default="{ row }">
          <span class="audit-page__action-text">{{ row.action_code }}</span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="目标" min-width="170">
        <template #default="{ row }">
          <span v-if="row.target_type" class="audit-page__target">
            {{ row.target_type }}
            <span class="nw-hint">{{ row.target_ulid }}</span>
          </span>
          <span v-else class="nw-hint">—</span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="结果" min-width="200">
        <template #default="{ row }">
          <span :class="row.resp_code === 'SUCCESS' ? 'audit-page__ok' : 'audit-page__fail'">
            {{ row.resp_code || '—' }}
          </span>
          <span class="nw-hint audit-page__resp">{{ row.resp_message }}</span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="状态" width="80" align="center">
        <template #default="{ row }">{{ row.http_status }}</template>
      </ElTableColumn>
      <ElTableColumn label="耗时" width="90" align="right">
        <template #default="{ row }">
          <span :class="row.latency_ms > 1000 ? 'audit-page__slow' : ''">{{ row.latency_ms }} ms</span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="链路 ID" min-width="150">
        <template #default="{ row }">
          <button
            v-if="row.trace_id"
            type="button"
            class="audit-page__trace"
            title="点击复制链路 ID"
            @click="copyTrace(row.trace_id)"
          >
            {{ row.trace_id.slice(0, 12) }}…
          </button>
          <span v-else class="nw-hint">—</span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="操作" width="86" fixed="right">
        <template #default="{ row }">
          <ElButton size="small" text type="primary" @click="openDetail(row)">详情</ElButton>
        </template>
      </ElTableColumn>
      <template #empty>
        <ElEmpty description="没有符合条件的审计记录" />
      </template>
    </ElTable>

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

    <ElDrawer v-model="detailVisible" title="审计详情" size="560px" destroy-on-close>
      <div v-loading="detailLoading" class="audit-detail">
        <template v-if="detail">
          <ElDescriptions :column="1" border size="small">
            <ElDescriptionsItem label="审计 ID">{{ detail.audit_ulid }}</ElDescriptionsItem>
            <ElDescriptionsItem label="链路 ID">{{ detail.trace_id || '—' }}</ElDescriptionsItem>
            <ElDescriptionsItem label="时间">{{ formatTime(detail.created_at) }}</ElDescriptionsItem>
            <ElDescriptionsItem label="操作人">
              {{ detail.user_name || '—' }}
              <span class="nw-hint">{{ detail.user_ulid }}</span>
            </ElDescriptionsItem>
            <ElDescriptionsItem label="请求">
              {{ detail.request_method }} {{ detail.request_path }}
            </ElDescriptionsItem>
            <ElDescriptionsItem label="目标">
              {{ detail.target_type || '—' }}
              <span class="nw-hint">{{ detail.target_ulid }}</span>
            </ElDescriptionsItem>
            <ElDescriptionsItem label="响应">
              {{ detail.resp_code }}（HTTP {{ detail.http_status }}）
              <div class="nw-hint">{{ detail.resp_message }}</div>
            </ElDescriptionsItem>
            <ElDescriptionsItem label="耗时">{{ detail.latency_ms }} ms</ElDescriptionsItem>
            <ElDescriptionsItem label="来源">
              {{ detail.ip_addr }}
              <div class="nw-hint audit-detail__ua">{{ detail.user_agent_text }}</div>
            </ElDescriptionsItem>
          </ElDescriptions>
          <h4 class="audit-detail__title">请求体</h4>
          <pre class="audit-detail__body">{{ prettyBody || '（未采样：上传请求或超过 32KB）' }}</pre>
        </template>
        <ElEmpty v-else-if="!detailLoading" description="未读取到审计详情" />
      </div>
    </ElDrawer>
  </div>
</template>

<style lang="less" scoped>
  .audit-page {
    &__search {
      width: 240px;
    }

    &__action {
      width: 190px;
    }

    &__level {
      width: 130px;
    }

    &__date {
      width: 250px;
    }

    &__spacer {
      flex: 1;
    }

    &__table {
      margin-top: 12px;
    }

    &__action-text {
      font-family: ui-monospace, 'SFMono-Regular', Consolas, monospace;
      font-size: 12px;
    }

    &__target {
      display: inline-flex;
      gap: 6px;
      align-items: center;
    }

    &__resp {
      display: block;
      margin-top: 2px;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    &__ok {
      font-weight: 600;
      color: var(--nw-success);
    }

    &__fail {
      font-weight: 600;
      color: var(--nw-danger);
    }

    &__slow {
      color: var(--nw-warning);
    }

    &__trace {
      padding: 0;
      font-family: ui-monospace, 'SFMono-Regular', Consolas, monospace;
      font-size: 12px;
      color: var(--el-color-primary);
      cursor: pointer;
      background: transparent;
      border: 0;

      &:hover {
        text-decoration: underline;
      }
    }
  }

  .audit-detail {
    display: flex;
    flex-direction: column;
    gap: 14px;

    &__title {
      margin: 4px 0 0;
      font-size: 13px;
      font-weight: 700;
      color: var(--el-text-color-primary);
    }

    &__ua {
      margin-top: 4px;
      word-break: break-all;
    }

    &__body {
      max-height: 420px;
      padding: 12px;
      overflow: auto;
      font-family: ui-monospace, 'SFMono-Regular', Consolas, monospace;
      font-size: 12px;
      line-height: 1.6;
      white-space: pre-wrap;
      word-break: break-all;
      background: var(--nw-surface-subtle);
      border: 1px solid var(--nw-border);
      border-radius: var(--nw-radius-md);
    }
  }

  .nw-toolbar--wrap {
    flex-wrap: wrap;
  }
</style>

<!-- 审计日志页：默认只看异常与失败，按链路 ID 追一次请求的完整时间线。 -->
<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import { ElMessage } from 'element-plus'
  import {
    fetchAuditActionOptions,
    fetchAuditRetention,
    getAdminAuditDetail,
    listAdminAudits,
    purgeAdminAudit,
    type NWAuditActionOptionVo,
    type NWAuditDetailVo,
    type NWAuditListReqVo,
    type NWAuditRetentionVo,
    type NWAuditVo
  } from '@/api/admin'
  import { usePageQuery } from '@/composables/usePageQuery'
  import { formatTime } from '@/utils/time'
  import AuditTraceDrawer from './components/AuditTraceDrawer.vue'

  const { items, total, pageNum, pageSize, loading, query, load, search, changePage } = usePageQuery<
    NWAuditVo,
    Omit<NWAuditListReqVo, 'page_num' | 'page_size'>
  >(listAdminAudits, {
    log_level_code: '',
    action_code: '',
    keyword_text: '',
    trace_id: '',
    user_ulid: '',
    target_type: '',
    date_from: '',
    date_to: ''
  })

  const levelOptions = [
    { value: '', label: '全部级别' },
    { value: 'error', label: '错误' },
    { value: 'warn', label: '警示' },
    { value: 'info', label: '正常' }
  ]

  const dateRange = ref<[string, string] | null>(null)
  const actionOptions = ref<NWAuditActionOptionVo[]>([])
  const retention = ref<NWAuditRetentionVo | null>(null)
  const detailVisible = ref(false)
  const detailLoading = ref(false)
  const detail = ref<NWAuditDetailVo | null>(null)
  const traceVisible = ref(false)
  const traceId = ref('')
  const purging = ref(false)

  const levelTagType = (levelCode: string) =>
    levelCode === 'error' ? 'danger' : levelCode === 'warn' ? 'warning' : 'success'

  const levelLabel = (levelCode: string) =>
    levelOptions.find((item) => item.value === levelCode)?.label ?? (levelCode || '—')

  const prettyBody = computed(() => prettyJson(detail.value?.request_body))
  const prettyRespBody = computed(() => prettyJson(detail.value?.resp_body))

  /** 把落库文本格式化为可读 JSON；非 JSON（上传占位、纯文本）原样返回 */
  function prettyJson(raw?: string) {
    if (!raw) return ''
    try {
      return JSON.stringify(JSON.parse(raw), null, 2)
    } catch {
      return raw
    }
  }

  const searchWithDate = () => {
    query.value.date_from = dateRange.value?.[0] ?? ''
    query.value.date_to = dateRange.value?.[1] ?? ''
    void search()
  }

  const loadOptions = async () => {
    try {
      actionOptions.value = await fetchAuditActionOptions()
    } catch {
      actionOptions.value = []
    }
  }

  const loadRetention = async () => {
    try {
      retention.value = await fetchAuditRetention()
    } catch {
      retention.value = null
    }
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

  const openTrace = (nextTraceId: string) => {
    if (!nextTraceId) return
    traceId.value = nextTraceId
    traceVisible.value = true
  }

  /** 时间线里点某条记录，直接跳到该条的详情抽屉 */
  const openTraceDetail = async (row: NWAuditVo) => {
    traceVisible.value = false
    await openDetail(row)
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

  const purgeNow = async () => {
    purging.value = true
    try {
      const result = await purgeAdminAudit()
      ElMessage.success(result.purged_count > 0 ? `已清理 ${result.purged_count} 条过期记录` : '没有过期记录')
      await Promise.all([loadRetention(), load()])
    } finally {
      purging.value = false
    }
  }

  onMounted(() => {
    void Promise.all([load(), loadOptions(), loadRetention()])
  })
</script>

<template>
  <div class="nw-page audit-page">
    <div v-if="retention" class="audit-page__retention">
      <Icon icon="mdi:archive-clock-outline" :size="16" />
      <span>
        留存 {{ retention.retention_days }} 天 · 当前 {{ retention.total_count.toLocaleString() }} 条 ·
        {{ retention.next_purge_at_text }}
      </span>
      <span v-if="retention.last_purged_at" class="nw-hint">
        上次清理 {{ formatTime(retention.last_purged_at) }}
      </span>
      <ElButton size="small" text :loading="purging" @click="purgeNow()">立即清理</ElButton>
    </div>

    <div class="nw-toolbar nw-toolbar--wrap">
      <ElInput
        v-model="query.keyword_text"
        placeholder="搜索动作 / 用户 / 路径 / 说明"
        clearable
        class="audit-page__search"
        @keyup.enter="searchWithDate()"
        @clear="searchWithDate()"
      />
      <ElSelect
        v-model="query.action_code"
        class="audit-page__action"
        placeholder="全部动作"
        clearable
        filterable
        @change="searchWithDate()"
      >
        <ElOption
          v-for="option in actionOptions"
          :key="option.action_code"
          :label="`${option.action_code}（${option.total}）`"
          :value="option.action_code"
        />
      </ElSelect>
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
      <ElTableColumn label="时间" width="160">
        <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
      </ElTableColumn>
      <ElTableColumn label="级别" width="80" align="center">
        <template #default="{ row }">
          <ElTag :type="levelTagType(row.log_level_code)" effect="light" size="small">
            {{ levelLabel(row.log_level_code) }}
          </ElTag>
        </template>
      </ElTableColumn>
      <ElTableColumn label="操作人" width="120">
        <template #default="{ row }">{{ row.user_name || '—' }}</template>
      </ElTableColumn>
      <ElTableColumn label="动作" min-width="200">
        <template #default="{ row }">
          <span class="audit-page__action-text">{{ row.action_code }}</span>
          <span class="nw-hint audit-page__path">{{ row.request_path }}</span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="结果" min-width="180">
        <template #default="{ row }">
          <span :class="row.resp_code === 'SUCCESS' ? 'audit-page__ok' : 'audit-page__fail'">
            {{ row.resp_code || '—' }}
          </span>
          <span class="nw-hint audit-page__resp">{{ row.resp_message }}</span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="耗时" width="88" align="right">
        <template #default="{ row }">
          <span :class="row.latency_ms > 1000 ? 'audit-page__slow' : ''">{{ row.latency_ms }} ms</span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="链路" width="110" align="center">
        <template #default="{ row }">
          <ElButton v-if="row.trace_id" size="small" text type="primary" @click="openTrace(row.trace_id)">
            时间线
          </ElButton>
          <span v-else class="nw-hint">—</span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="操作" width="120" fixed="right">
        <template #default="{ row }">
          <ElButton size="small" text type="primary" @click="openDetail(row)">详情</ElButton>
          <ElButton v-if="row.trace_id" size="small" text @click="copyTrace(row.trace_id)">复制</ElButton>
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

    <ElDrawer v-model="detailVisible" title="审计详情" size="620px" destroy-on-close>
      <div v-loading="detailLoading" class="audit-detail">
        <template v-if="detail">
          <ElDescriptions :column="1" border size="small">
            <ElDescriptionsItem label="审计 ID">{{ detail.audit_ulid }}</ElDescriptionsItem>
            <ElDescriptionsItem label="链路 ID">
              <span class="audit-detail__mono">{{ detail.trace_id || '—' }}</span>
              <ElButton v-if="detail.trace_id" size="small" text type="primary" @click="openTrace(detail.trace_id)">
                查看时间线
              </ElButton>
            </ElDescriptionsItem>
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
          <h4 class="audit-detail__title">响应体</h4>
          <pre class="audit-detail__body">{{ prettyRespBody || '（未采样：流式响应、上传或超过 32KB）' }}</pre>
        </template>
        <ElEmpty v-else-if="!detailLoading" description="未读取到审计详情" />
      </div>
    </ElDrawer>

    <AuditTraceDrawer
      v-model:visible="traceVisible"
      :trace-id="traceId"
      @open-detail="openTraceDetail"
    />
  </div>
</template>

<style lang="less" scoped>
  .audit-page {
    &__retention {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      align-items: center;
      padding: 8px 12px;
      font-size: 12px;
      color: var(--el-text-color-secondary);
      background: var(--nw-surface-subtle);
      border: 1px solid var(--nw-border-soft);
      border-radius: var(--nw-radius-md);
    }

    &__search {
      width: 250px;
    }

    &__action {
      width: 220px;
    }

    &__level {
      width: 120px;
    }

    &__date {
      width: 240px;
    }

    &__spacer {
      flex: 1;
    }

    &__table {
      margin-top: 4px;
    }

    &__action-text {
      font-family: var(--nw-font-mono);
      font-size: 12px;
    }

    &__path {
      display: block;
      margin-top: 2px;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
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

    &__btn-text {
      margin-left: 4px;
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

    &__mono {
      font-family: var(--nw-font-mono);
      font-size: 12px;
    }

    &__body {
      max-height: 420px;
      padding: 12px;
      overflow: auto;
      font-family: var(--nw-font-mono);
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

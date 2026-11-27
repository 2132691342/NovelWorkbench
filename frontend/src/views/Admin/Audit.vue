<!-- 审计日志页：默认按「链路」展示——一行一次入口请求，点开看它下面的完整调用瀑布。
     链路视图回答「我点了按钮，系统做了什么、慢在哪一步、烧了多少 token」；
     记录视图保留逐 span 平铺，供精确排查单条记录。 -->
<script setup lang="ts">
  import { computed, onMounted, ref, watch } from 'vue'
  import { useRoute, useRouter } from 'vue-router'
  import { ElButton, ElSwitch } from 'element-plus'
  import {
    fetchAdminAuditStats,
    fetchAuditActionOptions,
    fetchAuditRetention,
    listAdminAuditTraces,
    listAdminAudits,
    type NWAuditActionOptionVo,
    type NWAuditListReqVo,
    type NWAuditRetentionVo,
    type NWAuditStatsVo,
    type NWAuditTraceListReqVo,
    type NWAuditTraceSummaryVo,
    type NWAuditVo
  } from '@/api/admin'
  import { asRow } from '@/utils/table'
  import AuditDetailDrawer from './components/AuditDetailDrawer.vue'
  import AuditFilterBar from './components/AuditFilterBar.vue'
  import AuditRecordTable from './components/AuditRecordTable.vue'
  import AuditStatsBar from './components/AuditStatsBar.vue'
  import AuditSummaryBar from './components/AuditSummaryBar.vue'
  import AuditTraceDrawer from './components/AuditTraceDrawer.vue'
  import AuditTraceTable from './components/AuditTraceTable.vue'

  /** 视图模式：链路优先（默认）与记录平铺 */
  type AuditViewMode = 'trace' | 'record'

  /** 筛选条件（不含分页字段） */
  type AuditQuery = Omit<NWAuditTraceListReqVo, 'page_num' | 'page_size'>

  const initialQuery: AuditQuery = {
    log_level_code: '',
    span_type_code: '',
    action_code: '',
    keyword_text: '',
    user_ulid: '',
    time_from: '',
    time_to: '',
    date_from: '',
    date_to: ''
  }

  const route = useRoute()
  const router = useRouter()

  const viewMode = ref<AuditViewMode>('trace')
  const query = ref<AuditQuery>({ ...initialQuery })

  /** 链路视图与记录视图各自持有一套分页状态，切换时互不干扰 */
  const tracePage = ref({ num: 1, size: 20, total: 0, loading: false, items: [] as NWAuditTraceSummaryVo[] })

  /** 记录视图状态（逐 span 平铺，条件与链路视图共用同一份筛选） */
  const recordList = ref<NWAuditVo[]>([])
  const recordTotal = ref(0)
  const recordPageNum = ref(1)
  const recordPageSize = ref(20)
  const recordLoading = ref(false)

  const stats = ref<NWAuditStatsVo | null>(null)
  const statsLoading = ref(false)
  const actionOptions = ref<NWAuditActionOptionVo[]>([])
  const optionsLoading = ref(false)
  const retention = ref<NWAuditRetentionVo | null>(null)
  const retentionLoading = ref(false)
  const levelCounts = ref<Record<string, number>>({})
  const failed = ref(false)
  const slowOnly = ref(false)

  const quickRange = ref<string>('week')
  const dateRange = ref<[string, string] | null>(null)

  const detailVisible = ref(false)
  const detailRow = ref<NWAuditVo | null>(null)
  const traceVisible = ref(false)
  const traceId = ref('')

  const quickRanges = [
    { value: 'hour', label: '近 1 小时' },
    { value: 'today', label: '今天' },
    { value: 'week', label: '近 7 天' },
    { value: 'custom', label: '自定义' }
  ] as const

  const spanOptions = [
    { value: '', label: '全部类型' },
    { value: 'ai_call', label: '含 AI 调用' },
    { value: 'workflow_step', label: '含工作流步骤' },
    { value: 'system', label: '含系统动作' }
  ]

  const levelChips = [
    { value: '', label: '全部' },
    { value: 'error', label: '错误' },
    { value: 'warn', label: '警示' },
    { value: 'info', label: '正常' }
  ]

  /** 请求条件：链路视图复用同一套筛选，慢链路开关是链路视图独有的后置过滤 */
  const requestPayload = (): NWAuditTraceListReqVo => ({
    ...query.value,
    page_num: 1,
    page_size: 20
  })

  /** 按快捷项算出精确时间区间（RFC3339），空串表示不限 */
  const rangeOf = (value: string): { from: string; to: string } => {
    const now = new Date()
    if (value === 'hour') {
      return { from: new Date(now.getTime() - 3600_000).toISOString(), to: now.toISOString() }
    }
    if (value === 'today') {
      const start = new Date(now.getFullYear(), now.getMonth(), now.getDate())
      return { from: start.toISOString(), to: now.toISOString() }
    }
    if (value === 'week') {
      return { from: new Date(now.getTime() - 7 * 86400_000).toISOString(), to: now.toISOString() }
    }
    return { from: '', to: '' }
  }

  /** 把快捷/自定义区间写进查询条件（精确时刻优先于日期） */
  const applyRange = () => {
    if (quickRange.value === 'custom') {
      query.value.date_from = dateRange.value?.[0] ?? ''
      query.value.date_to = dateRange.value?.[1] ?? ''
      query.value.time_from = ''
      query.value.time_to = ''
      return
    }
    const range = rangeOf(quickRange.value)
    query.value.time_from = range.from
    query.value.time_to = range.to
    query.value.date_from = ''
    query.value.date_to = ''
  }

  const loadTraces = async () => {
    tracePage.value.loading = true
    failed.value = false
    try {
      const page = await listAdminAuditTraces({
        ...requestPayload(),
        page_num: tracePage.value.num,
        page_size: tracePage.value.size
      })
      tracePage.value.items = page.items ?? []
      tracePage.value.total = page.total
    } catch {
      tracePage.value.items = []
      tracePage.value.total = 0
      failed.value = true
    } finally {
      tracePage.value.loading = false
    }
  }

  /** 记录视图：按 span 平铺查询（条件与链路视图同源） */
  const loadRecords = async () => {
    recordLoading.value = true
    failed.value = false
    try {
      const page = await listAdminAudits({
        ...(query.value as unknown as NWAuditListReqVo),
        page_num: recordPageNum.value,
        page_size: recordPageSize.value
      })
      recordList.value = page.items ?? []
      recordTotal.value = page.total
    } catch {
      recordList.value = []
      recordTotal.value = 0
      failed.value = true
    } finally {
      recordLoading.value = false
    }
  }

  const loadStats = async () => {
    statsLoading.value = true
    try {
      stats.value = await fetchAdminAuditStats(requestPayload())
    } catch {
      stats.value = null
    } finally {
      statsLoading.value = false
    }
  }

  /** 各级别条数：用 page_size=1 的轻量查询并行拉取链路计数 */
  const loadLevelCounts = async () => {
    const results = await Promise.all(
      ['', 'error', 'warn', 'info'].map(async (levelCode) => {
        try {
          const page = await listAdminAuditTraces({
            ...requestPayload(),
            log_level_code: levelCode,
            page_num: 1,
            page_size: 1
          })
          return [levelCode, page.total] as const
        } catch {
          return [levelCode, 0] as const
        }
      })
    )
    levelCounts.value = Object.fromEntries(results)
  }

  const loadOptions = async () => {
    optionsLoading.value = true
    try {
      actionOptions.value = await fetchAuditActionOptions()
    } catch {
      actionOptions.value = []
    } finally {
      optionsLoading.value = false
    }
  }

  const loadRetention = async () => {
    retentionLoading.value = true
    try {
      retention.value = await fetchAuditRetention()
    } catch {
      retention.value = null
    } finally {
      retentionLoading.value = false
    }
  }

  /** 筛选变更即重查：当前视图刷新列表，两张视图的统计与级别计数同步更新 */
  const refresh = async () => {
    applyRange()
    tracePage.value.num = 1
    recordPageNum.value = 1
    const tasks: Promise<unknown>[] = [loadTraces(), loadStats(), loadLevelCounts()]
    if (viewMode.value === 'record') tasks.push(loadRecords())
    await Promise.all(tasks)
  }

  const switchMode = async (mode: AuditViewMode) => {
    if (viewMode.value === mode) return
    viewMode.value = mode
    if (mode === 'record') {
      recordPageNum.value = 1
      await loadRecords()
      return
    }
    await loadTraces()
  }

  const changeRecordPage = async (nextPage: number) => {
    recordPageNum.value = nextPage
    await loadRecords()
  }

  const resetFilters = async () => {
    query.value = { ...initialQuery }
    quickRange.value = 'week'
    dateRange.value = null
    slowOnly.value = false
    await refresh()
  }

  const changeTracePage = async (nextPage: number) => {
    tracePage.value.num = nextPage
    await loadTraces()
  }

  const openDetail = (row: NWAuditVo) => {
    detailRow.value = row
    detailVisible.value = true
  }

  const openTrace = (nextTraceId: string) => {
    if (!nextTraceId) return
    traceId.value = nextTraceId
    traceVisible.value = true
  }

  /** 详情里点「查看时间线」：先关详情再开链路，避免抽屉叠在一起 */
  const openTraceFromDetail = (nextTraceId: string) => {
    detailVisible.value = false
    openTrace(nextTraceId)
  }

  const onPurged = async (next: NWAuditRetentionVo) => {
    retention.value = next
    await Promise.all([loadTraces(), loadLevelCounts(), loadStats()])
  }

  /** 工作流运行抽屉用 ?trace_id= 跳到这里：直接打开该链路瀑布并清掉 query */
  const consumeTraceQuery = () => {
    const traceQuery = route.query.trace_id
    if (typeof traceQuery !== 'string' || !traceQuery) return
    openTrace(traceQuery)
    void router.replace({ query: { ...route.query, trace_id: undefined } })
  }

  watch(() => route.query.trace_id, () => consumeTraceQuery())

  /** 慢链路开关：前端二次过滤已取到的当页（后端 HAVING 只用于统计口径） */
  const visibleTraceList = computed(() =>
    slowOnly.value
      ? tracePage.value.items.filter((row) => row.total_latency_ms >= 5000)
      : tracePage.value.items
  )

  onMounted(() => {
    void Promise.all([loadOptions(), loadRetention(), loadTraces(), loadStats(), loadLevelCounts()])
    consumeTraceQuery()
  })
</script>

<template>
  <div class="nw-page audit-page">
    <AuditSummaryBar
      :retention="retention"
      :loading="retentionLoading"
      @purged="onPurged"
      @retry="loadRetention()"
    />

    <AuditStatsBar :stats="stats" :loading="statsLoading" />

    <AuditFilterBar
      v-model:quick-range="quickRange"
      v-model:date-range="dateRange"
      :query="query"
      :quick-ranges="quickRanges"
      :level-chips="levelChips"
      :span-options="spanOptions"
      :action-options="actionOptions"
      :level-counts="levelCounts"
      :options-loading="optionsLoading"
      :total="viewMode === 'trace' ? tracePage.total : recordTotal"
      @refresh="refresh()"
      @reset="resetFilters()"
    />

    <div class="audit-page__mode">
      <div class="audit-page__mode-group">
        <button
          type="button"
          class="audit-page__mode-btn"
          :class="{ 'is-active': viewMode === 'trace' }"
          @click="switchMode('trace')"
        >
          <Icon icon="mdi:transit-connection-variant" :size="14" />
          <span>链路</span>
        </button>
        <button
          type="button"
          class="audit-page__mode-btn"
          :class="{ 'is-active': viewMode === 'record' }"
          @click="switchMode('record')"
        >
          <Icon icon="mdi:format-list-bulleted" :size="14" />
          <span>记录</span>
        </button>
      </div>
      <ElSwitch v-if="viewMode === 'trace'" v-model="slowOnly" active-text="只看慢链路" />
      <span class="nw-hint">
        {{ viewMode === 'trace' ? '一行 = 一次请求，点开看完整调用瀑布' : '一行 = 一条 span，适合精确排查' }}
      </span>
    </div>

    <div v-if="failed && !tracePage.loading" class="audit-page__failed">
      <span class="nw-hint">审计查询失败，请检查 Manticore 是否可用</span>
      <ElButton size="small" type="primary" plain @click="refresh()">重试</ElButton>
    </div>

    <!-- 链路视图：一次请求一行 -->
    <AuditTraceTable
      v-if="viewMode === 'trace'"
      :items="visibleTraceList"
      :total="tracePage.total"
      :page-num="tracePage.num"
      :page-size="tracePage.size"
      :loading="tracePage.loading"
      @open-trace="openTrace"
      @change-page="changeTracePage"
    />

    <!-- 记录视图：逐 span 平铺 -->
    <AuditRecordTable
      v-else
      :items="recordList"
      :total="recordTotal"
      :page-num="recordPageNum"
      :page-size="recordPageSize"
      :loading="recordLoading"
      @open-detail="(row) => openDetail(asRow<NWAuditVo>(row))"
      @open-trace="openTrace"
      @change-page="changeRecordPage"
    />

    <AuditDetailDrawer
      v-model:visible="detailVisible"
      :row="detailRow"
      @open-trace="openTraceFromDetail"
    />
    <AuditTraceDrawer v-model:visible="traceVisible" :trace-id="traceId" @open-detail="openDetail" />
  </div>
</template>

<style lang="less" scoped>
  @import './components/auditPage.less';

  .audit-page {
    &__mode {
      display: flex;
      flex-wrap: wrap;
      gap: 10px;
      align-items: center;
    }

    &__mode-group {
      display: inline-flex;
      padding: 2px;
      background: var(--nw-surface-subtle);
      border: 1px solid var(--nw-border);
      border-radius: var(--nw-radius-md);
    }

    &__mode-btn {
      display: inline-flex;
      gap: 5px;
      align-items: center;
      padding: 5px 12px;
      font-size: 12px;
      color: var(--el-text-color-regular);
      cursor: pointer;
      background: transparent;
      border: 0;
      border-radius: calc(var(--nw-radius-md) - 2px);
      transition:
        color 0.16s ease,
        background 0.16s ease,
        transform 0.16s ease;

      &:hover {
        color: var(--el-color-primary);
      }

      &:active {
        transform: scale(0.97);
      }

      &.is-active {
        color: var(--el-color-primary);
        background: var(--nw-surface);
        box-shadow: var(--nw-shadow-card);
      }
    }

    &__failed {
      display: flex;
      gap: 10px;
      align-items: center;
      padding: 10px 12px;
      background: color-mix(in srgb, var(--nw-danger) 8%, transparent);
      border: 1px solid color-mix(in srgb, var(--nw-danger) 35%, transparent);
      border-radius: var(--nw-radius-md);
    }
  }
</style>

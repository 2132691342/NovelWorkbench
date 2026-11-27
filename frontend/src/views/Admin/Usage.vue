<!-- 用量分析页：按天展示调用量、token 与延迟趋势。 -->
<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import { ElButton, ElCard, ElDatePicker, ElEmpty, ElTable, ElTableColumn } from 'element-plus'
  import type { EChartsOption, TooltipComponentFormatterCallbackParams } from 'echarts'
  import { getModelUsageOverview } from '@/api/model'
  import type { NWModelUsageGroupVo, NWModelUsageOverviewVo } from '@/api/model'
  import Echart from '@/components/Echart/index.vue'
  import { useAppStore } from '@/store/modules/app'

  /** 概览指标卡（hint 为补充说明，tone 控制数值配色） */
  interface SummaryCard {
    label: string
    value: string
    tone: '' | 'success' | 'danger' | 'warning' | 'muted'
    hint: string
  }

  const appStore = useAppStore()

  const overview = ref<NWModelUsageOverviewVo>({
    call_count_total: 0,
    fail_count_total: 0,
    unknown_usage_count: 0,
    total_tokens_total: 0,
    avg_latency_ms: 0,
    success_rate_value: 0,
    daily_list: [],
    provider_list: [],
    model_list: []
  })
  const loading = ref(false)
  const dateRange = ref<string[] | null>(null)

  /** 日期快捷项（value 为 null 表示全部区间/自定义） */
  const quickRanges = [
    { value: '7', label: '近 7 天' },
    { value: '30', label: '近 30 天' },
    { value: '90', label: '近 90 天' }
  ] as const

  const activeQuick = ref<string>('')

  /** 按「近 N 天」算出 YYYY-MM-DD 区间并写入日期选择器 */
  const applyQuickRange = (days: string) => {
    activeQuick.value = days
    const end = new Date()
    const start = new Date(end.getTime() - Number(days) * 86400_000)
    const toDateText = (date: Date) =>
      `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
    dateRange.value = [toDateText(start), toDateText(end)]
    void load()
  }

  /** 图表轴/网格线配色跟随明暗主题 */
  const axisColor = computed(() => (appStore.isDark ? '#94a3b8' : '#94a3b8'))
  const splitColor = computed(() => (appStore.isDark ? '#273244' : '#eef2f7'))
  const lineColor = computed(() => (appStore.isDark ? '#344055' : '#e2e8f0'))

  /** 未返回用量：网关未给 usage 的调用数，0 时仍展示但用次要色 */
  const unknownUsageCount = computed(() => overview.value.unknown_usage_count ?? 0)

  const summaryCards = computed<SummaryCard[]>(() => [
    {
      label: '总调用次数',
      value: overview.value.call_count_total.toLocaleString(),
      tone: '',
      hint: ''
    },
    {
      label: '失败次数',
      value: overview.value.fail_count_total.toLocaleString(),
      tone: 'danger',
      hint: ''
    },
    {
      label: '总 Token',
      value: overview.value.total_tokens_total.toLocaleString(),
      tone: '',
      hint: ''
    },
    {
      label: '平均延迟',
      value: `${overview.value.avg_latency_ms} ms`,
      tone: '',
      hint: ''
    },
    {
      label: '成功率',
      value: `${(overview.value.success_rate_value * 100).toFixed(1)}%`,
      tone: 'success',
      hint: ''
    },
    {
      label: '未返回用量',
      value: unknownUsageCount.value.toLocaleString(),
      tone: unknownUsageCount.value > 0 ? 'warning' : 'muted',
      hint: '网关未返回 token，非失败'
    }
  ])

  /** 提示框：在默认两行指标后补「未返回用量」，该日无未知用量时不显示 */
  const chartTooltip = (params: TooltipComponentFormatterCallbackParams) => {
    const rows = Array.isArray(params) ? params : [params]
    if (rows.length === 0) {
      return ''
    }
    const index = rows[0].dataIndex
    const daily = overview.value.daily_list[index]
    if (!daily) {
      return ''
    }
    const unknownCount = daily.unknown_usage_count ?? 0
    const lines = [
      `<div style="margin-bottom:4px">${daily.stat_date}</div>`,
      `${rows[0].marker ?? ''}调用次数：${daily.call_count}`,
      `${rows[1]?.marker ?? ''}Token 数：${daily.total_tokens.toLocaleString()}`
    ]
    if (unknownCount > 0) {
      lines.push(`未返回用量：${unknownCount}`)
    }
    return lines.join('<br/>')
  }

  /** 每日调用量柱状 + Token 折线（双轴） */
  const chartOption = computed<EChartsOption>(() => {
    const dailyList = overview.value.daily_list
    return {
      grid: { left: 52, right: 56, top: 40, bottom: 34 },
      tooltip: { trigger: 'axis', formatter: chartTooltip },
      legend: {
        data: ['调用次数', 'Token 数'],
        right: 12,
        top: 4,
        textStyle: { color: axisColor.value, fontSize: 11 }
      },
      xAxis: {
        type: 'category',
        data: dailyList.map((item) => item.stat_date.slice(5)),
        axisLine: { lineStyle: { color: lineColor.value } },
        axisLabel: { color: axisColor.value, fontSize: 11 }
      },
      yAxis: [
        {
          type: 'value',
          name: '次数',
          nameTextStyle: { color: axisColor.value, fontSize: 11 },
          splitLine: { lineStyle: { color: splitColor.value } },
          axisLabel: { color: axisColor.value, fontSize: 11 }
        },
        {
          type: 'value',
          name: 'Token',
          nameTextStyle: { color: axisColor.value, fontSize: 11 },
          splitLine: { show: false },
          axisLabel: { color: axisColor.value, fontSize: 11 }
        }
      ],
      series: [
        {
          name: '调用次数',
          type: 'bar',
          barMaxWidth: 22,
          itemStyle: { color: '#6366f1', borderRadius: [4, 4, 0, 0] },
          data: dailyList.map((item) => item.call_count)
        },
        {
          name: 'Token 数',
          type: 'line',
          yAxisIndex: 1,
          smooth: true,
          showSymbol: false,
          lineStyle: { width: 2, color: '#10b981' },
          areaStyle: { color: 'rgba(16, 185, 129, 0.12)' },
          data: dailyList.map((item) => item.total_tokens)
        }
      ]
    }
  })

  const load = async () => {
    loading.value = true
    try {
      overview.value = await getModelUsageOverview(
        dateRange.value?.[0] || undefined,
        dateRange.value?.[1] || undefined
      )
    } catch {
      // 失败提示由请求层统一弹出，保留上次数据
    } finally {
      loading.value = false
    }
  }

  const groupRows = computed<NWModelUsageGroupVo[]>(() => overview.value.model_list ?? [])

  onMounted(() => {
    void load()
  })
</script>

<template>
  <div class="nw-page">
    <div class="nw-toolbar">
      <div class="admin-usage__quick">
        <button
          v-for="item in quickRanges"
          :key="item.value"
          type="button"
          class="admin-usage__chip"
          :class="{ 'is-active': activeQuick === item.value }"
          @click="applyQuickRange(item.value)"
        >
          {{ item.label }}
        </button>
      </div>
      <ElDatePicker
        v-model="dateRange"
        type="daterange"
        value-format="YYYY-MM-DD"
        start-placeholder="开始日期"
        end-placeholder="结束日期"
        class="admin-usage__date"
        @change="activeQuick = ''; load()"
      />
      <ElButton type="primary" plain @click="load()">查询</ElButton>
      <div class="admin-usage__spacer" />
      <span class="nw-hint">按日聚合模型调用与 Token 消耗</span>
    </div>

    <div class="admin-usage__summary">
      <ElCard
        v-for="card in summaryCards"
        :key="card.label"
        v-loading="loading"
        shadow="never"
        class="admin-usage__stat"
      >
        <span class="admin-usage__stat-label">{{ card.label }}</span>
        <strong class="admin-usage__stat-value" :class="card.tone ? `is-${card.tone}` : ''">
          {{ card.value }}
        </strong>
        <span v-if="card.hint" class="admin-usage__stat-hint">{{ card.hint }}</span>
      </ElCard>
    </div>

    <ElCard v-loading="loading" shadow="never" class="admin-usage__chart-card">
      <template #header>
        <div class="admin-usage__chart-head">
          <span class="admin-usage__chart-title">每日调用趋势</span>
          <span class="nw-hint">柱：调用次数 / 线：Token 数</span>
        </div>
      </template>
      <Echart v-if="overview.daily_list.length > 0" :option="chartOption" :height="300" />
      <ElEmpty v-else description="所选区间暂无调用数据" :image-size="70" />
    </ElCard>

    <ElCard shadow="never" class="admin-usage__table-card">
      <template #header>
        <span class="admin-usage__chart-title">按模型分组</span>
      </template>
      <ElTable v-loading="loading" :data="groupRows" class="admin-usage__table">
        <ElTableColumn prop="group_name" label="模型" min-width="200" show-overflow-tooltip>
          <template #default="{ row }">
            <span class="admin-usage__model">{{ row.group_name }}</span>
          </template>
        </ElTableColumn>
        <ElTableColumn prop="call_count" label="调用次数" width="120" align="right" />
        <ElTableColumn prop="fail_count" label="失败次数" width="110" align="right" />
        <ElTableColumn label="未返回用量" width="110" align="right">
          <template #default="{ row }">
            {{ (row.unknown_usage_count ?? 0) > 0 ? row.unknown_usage_count : '-' }}
          </template>
        </ElTableColumn>
        <ElTableColumn label="Token 数" width="140" align="right">
          <template #default="{ row }">{{ row.total_tokens.toLocaleString() }}</template>
        </ElTableColumn>
        <ElTableColumn label="平均延迟" width="120" align="right">
          <template #default="{ row }">{{ row.avg_latency_ms }} ms</template>
        </ElTableColumn>
        <ElTableColumn label="成功率" width="120" align="right">
          <template #default="{ row }">
            <span
              class="nw-badge"
              :class="row.success_rate_value >= 0.95 ? 'nw-badge--success' : 'nw-badge--warning'"
            >
              {{ (row.success_rate_value * 100).toFixed(1) }}%
            </span>
          </template>
        </ElTableColumn>
        <template #empty>
          <span class="nw-hint">暂无模型用量数据</span>
        </template>
      </ElTable>
    </ElCard>
  </div>
</template>

<style lang="less" scoped>
  .admin-usage {
    &__quick {
      display: flex;
      flex-wrap: wrap;
      gap: 6px;
      align-items: center;
    }

    &__chip {
      height: 30px;
      padding: 0 12px;
      font-size: 12px;
      color: var(--el-text-color-regular);
      cursor: pointer;
      background: var(--nw-surface);
      border: 1px solid var(--nw-border);
      border-radius: 999px;
      transition:
        color 0.15s ease,
        border-color 0.15s ease,
        background 0.15s ease;

      &:hover {
        color: var(--el-color-primary);
        border-color: var(--el-color-primary);
      }

      &.is-active {
        color: var(--el-color-primary);
        background: var(--nw-primary-soft);
        border-color: var(--el-color-primary);
      }
    }

    &__date {
      width: 260px;
    }

    &__spacer {
      flex: 1;
    }

    &__summary {
      display: grid;
      gap: 14px;
      grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
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
      font-size: 22px;
      font-weight: 700;
      font-variant-numeric: tabular-nums;
      color: var(--nw-ink);

      &.is-success {
        color: var(--nw-success);
      }

      &.is-danger {
        color: var(--nw-danger);
      }

      &.is-warning {
        color: var(--el-color-warning);
      }

      &.is-muted {
        color: var(--el-text-color-secondary);
      }
    }

    &__stat-hint {
      font-size: 11px;
      line-height: 1.3;
      color: var(--el-text-color-secondary);
    }

    &__chart-card,
    &__table-card {
      border-radius: var(--nw-radius-lg);
    }

    &__chart-head {
      display: flex;
      gap: 12px;
      align-items: baseline;
    }

    &__chart-title {
      font-size: 14px;
      font-weight: 650;
    }

    &__table {
      border-radius: var(--nw-radius-md);
    }

    &__model {
      font-weight: 600;
      color: var(--nw-ink);
    }
  }
</style>

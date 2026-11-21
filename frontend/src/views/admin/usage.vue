<!-- 用量分析页：按天展示调用量、token 与延迟趋势。 -->
<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import { ElButton, ElCard, ElDatePicker, ElEmpty, ElTable, ElTableColumn } from 'element-plus'
  import type { EChartsOption } from 'echarts'
  import { getModelUsageOverview } from '@/api/model'
  import type { NWModelUsageGroupVo, NWModelUsageOverviewVo } from '@/api/model'
  import Echart from '@/components/Echart/index.vue'
  import { useAppStore } from '@/store/modules/app'

  const appStore = useAppStore()

  const overview = ref<NWModelUsageOverviewVo>({
    call_count_total: 0,
    fail_count_total: 0,
    total_tokens_total: 0,
    avg_latency_ms: 0,
    success_rate_value: 0,
    daily_list: [],
    provider_list: [],
    model_list: []
  })
  const loading = ref(false)
  const dateRange = ref<string[] | null>(null)

  /** 图表轴/网格线配色跟随明暗主题 */
  const axisColor = computed(() => (appStore.isDark ? '#94a3b8' : '#94a3b8'))
  const splitColor = computed(() => (appStore.isDark ? '#273244' : '#eef2f7'))
  const lineColor = computed(() => (appStore.isDark ? '#344055' : '#e2e8f0'))

  const summaryCards = computed(() => [
    { label: '总调用次数', value: overview.value.call_count_total.toLocaleString() },
    { label: '失败次数', value: overview.value.fail_count_total.toLocaleString(), tone: 'danger' },
    { label: '总 Token', value: overview.value.total_tokens_total.toLocaleString() },
    { label: '平均延迟', value: `${overview.value.avg_latency_ms} ms` },
    {
      label: '成功率',
      value: `${(overview.value.success_rate_value * 100).toFixed(1)}%`,
      tone: 'success'
    }
  ])

  /** 每日调用量柱状 + Token 折线（双轴） */
  const chartOption = computed<EChartsOption>(() => {
    const dailyList = overview.value.daily_list
    return {
      grid: { left: 52, right: 56, top: 40, bottom: 34 },
      tooltip: { trigger: 'axis' },
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
      <ElDatePicker
        v-model="dateRange"
        type="daterange"
        value-format="YYYY-MM-DD"
        start-placeholder="开始日期"
        end-placeholder="结束日期"
        class="admin-usage__date"
        @change="load()"
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

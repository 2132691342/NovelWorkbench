<!--
  用量分析页：区间选择 + 4 项关键指标 + 按日趋势（ECharts 双轴：调用量柱状 / token 折线）+
  按供应商与按模型分布表。
  数据来源：后端 model_usage_stats 按日聚合（供应商 × 模型 × 调用类型）。
-->
<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import * as echarts from 'echarts'
import { useI18n } from 'vue-i18n'
import { getModelUsageOverview, type NWModelUsageOverviewVo } from '@/api/model'
import NWCard from '@/components/NWCard.vue'
import NWEmpty from '@/components/NWEmpty.vue'
import NWLoading from '@/components/NWLoading.vue'

const { t } = useI18n()

/** 图表配色（对齐设计系统） */
const NW_CHART_CALL_COLOR = '#6366f1'
const NW_CHART_TOKEN_COLOR = '#818cf8'

/** 区间预设天数 */
const NW_RANGE_OPTION_LIST = [7, 14, 30] as const

const rangeDays = ref<number>(14)
const overviewLoading = ref(false)
const overviewVo = ref<NWModelUsageOverviewVo>({
  call_count_total: 0,
  fail_count_total: 0,
  total_tokens_total: 0,
  avg_latency_ms: 0,
  success_rate_value: 0,
  daily_list: [],
  provider_list: [],
  model_list: [],
})

const trendChartElement = ref<HTMLDivElement | null>(null)
let trendChartInstance: echarts.ECharts | null = null

/** 区间起始日期（YYYY-MM-DD） */
function rangeFromDate(days: number): string {
  const fromDate = new Date()
  fromDate.setDate(fromDate.getDate() - (days - 1))
  return fromDate.toISOString().slice(0, 10)
}

const statCardList = computed(() => [
  {
    stat_key: 'call_count_total',
    stat_label: t('usage.callCount'),
    stat_value: overviewVo.value.call_count_total.toLocaleString(),
    stat_class: 'stat-card--indigo',
  },
  {
    stat_key: 'total_tokens_total',
    stat_label: t('usage.totalTokens'),
    stat_value: overviewVo.value.total_tokens_total.toLocaleString(),
    stat_class: 'stat-card--violet',
  },
  {
    stat_key: 'avg_latency_ms',
    stat_label: t('usage.avgLatency'),
    stat_value: `${overviewVo.value.avg_latency_ms} ms`,
    stat_class: 'stat-card--blue',
  },
  {
    stat_key: 'success_rate_value',
    stat_label: t('usage.successRate'),
    stat_value: `${(overviewVo.value.success_rate_value * 100).toFixed(1)}%`,
    stat_class: 'stat-card--slate',
  },
])

/** 渲染按日趋势图 */
function renderTrendChart(): void {
  if (!trendChartElement.value) {
    return
  }
  if (!trendChartInstance) {
    trendChartInstance = echarts.init(trendChartElement.value)
  }
  const dailyList = overviewVo.value.daily_list
  trendChartInstance.setOption({
    color: [NW_CHART_CALL_COLOR, NW_CHART_TOKEN_COLOR],
    tooltip: { trigger: 'axis' },
    legend: {
      data: [t('usage.callCount'), t('usage.totalTokens')],
      top: 0,
    },
    grid: { left: 56, right: 64, top: 44, bottom: 36 },
    xAxis: {
      type: 'category',
      data: dailyList.map((dailyItem) => dailyItem.stat_date),
      boundaryGap: true,
    },
    yAxis: [
      { type: 'value', name: t('usage.callCount'), splitLine: { show: true } },
      { type: 'value', name: t('usage.totalTokens'), splitLine: { show: false } },
    ],
    series: [
      {
        name: t('usage.callCount'),
        type: 'bar',
        data: dailyList.map((dailyItem) => dailyItem.call_count),
        itemStyle: { color: NW_CHART_CALL_COLOR, borderRadius: [4, 4, 0, 0] },
        barMaxWidth: 20,
      },
      {
        name: t('usage.totalTokens'),
        type: 'line',
        yAxisIndex: 1,
        smooth: true,
        data: dailyList.map((dailyItem) => dailyItem.total_tokens),
        itemStyle: { color: NW_CHART_TOKEN_COLOR },
        lineStyle: { width: 2 },
        symbolSize: 6,
      },
    ],
  })
}

function handleWindowResize(): void {
  trendChartInstance?.resize()
}

/** 加载用量总览 */
async function loadOverview(): Promise<void> {
  overviewLoading.value = true
  try {
    const toDate = new Date().toISOString().slice(0, 10)
    const data = await getModelUsageOverview(rangeFromDate(rangeDays.value), toDate)
    overviewVo.value = {
      ...data,
      daily_list: Array.isArray(data.daily_list) ? data.daily_list : [],
      provider_list: Array.isArray(data.provider_list) ? data.provider_list : [],
      model_list: Array.isArray(data.model_list) ? data.model_list : [],
    }
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    overviewLoading.value = false
  }
  // 图表容器在 loading 结束后才挂载，必须等 DOM 更新后再初始化
  await nextTick()
  renderTrendChart()
}

/** 切换区间 */
function handleRangeChange(days: number): void {
  if (rangeDays.value === days) {
    return
  }
  rangeDays.value = days
  void loadOverview()
}

/** 成功率百分比文案 */
function successRateText(rateValue: number): string {
  return `${(rateValue * 100).toFixed(1)}%`
}

onMounted(() => {
  void loadOverview()
  window.addEventListener('resize', handleWindowResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleWindowResize)
  trendChartInstance?.dispose()
  trendChartInstance = null
})
</script>

<template>
  <div class="nw-page usage-page">
    <header class="usage-page__header">
      <div>
        <h2 class="usage-page__title">{{ t('usage.title') }}</h2>
        <p class="usage-page__subtitle">{{ t('usage.subtitle') }}</p>
      </div>
      <el-radio-group :model-value="rangeDays" @change="handleRangeChange(Number($event))">
        <el-radio-button v-for="days in NW_RANGE_OPTION_LIST" :key="days" :value="days">
          {{ t('usage.rangeDays', { days }) }}
        </el-radio-button>
      </el-radio-group>
    </header>

    <NWLoading v-if="overviewLoading" />
    <template v-else>
      <div class="usage-page__stats">
        <NWCard v-for="statItem in statCardList" :key="statItem.stat_key">
          <div class="stat-card" :class="statItem.stat_class">
            <span class="stat-card__label">{{ statItem.stat_label }}</span>
            <span class="stat-card__value">{{ statItem.stat_value }}</span>
          </div>
        </NWCard>
      </div>

      <NWCard :title="t('usage.trendTitle')" class="usage-page__trend-card">
        <div ref="trendChartElement" class="usage-page__chart" />
      </NWCard>

      <div class="usage-page__tables">
        <NWCard :title="t('usage.byProvider')">
          <NWEmpty v-if="overviewVo.provider_list.length === 0" :description_text="t('usage.emptyTip')" />
          <el-table v-else :data="overviewVo.provider_list" size="small">
            <el-table-column prop="group_name" :label="t('usage.groupName')" min-width="160" />
            <el-table-column prop="call_count" :label="t('usage.callCount')" width="110" />
            <el-table-column prop="fail_count" :label="t('usage.failCount')" width="100" />
            <el-table-column prop="total_tokens" :label="t('usage.totalTokens')" width="130" />
            <el-table-column :label="t('usage.avgLatency')" width="120">
              <template #default="{ row }">{{ row.avg_latency_ms }} ms</template>
            </el-table-column>
            <el-table-column :label="t('usage.successRate')" width="110">
              <template #default="{ row }">{{ successRateText(row.success_rate_value) }}</template>
            </el-table-column>
          </el-table>
        </NWCard>

        <NWCard :title="t('usage.byModel')">
          <NWEmpty v-if="overviewVo.model_list.length === 0" :description_text="t('usage.emptyTip')" />
          <el-table v-else :data="overviewVo.model_list" size="small">
            <el-table-column prop="group_name" :label="t('usage.groupName')" min-width="160" />
            <el-table-column prop="call_count" :label="t('usage.callCount')" width="110" />
            <el-table-column prop="fail_count" :label="t('usage.failCount')" width="100" />
            <el-table-column prop="total_tokens" :label="t('usage.totalTokens')" width="130" />
            <el-table-column :label="t('usage.successRate')" width="110">
              <template #default="{ row }">{{ successRateText(row.success_rate_value) }}</template>
            </el-table-column>
          </el-table>
        </NWCard>
      </div>
    </template>
  </div>
</template>

<style scoped>
.usage-page__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}

.usage-page__title {
  margin: 0 0 4px;
  font-size: 20px;
  color: var(--text-main);
}

.usage-page__subtitle {
  margin: 0;
  color: var(--text-secondary);
  font-size: 13px;
}

.usage-page__stats {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 16px;
  margin-bottom: 16px;
}

.usage-page__trend-card {
  margin-bottom: 16px;
}

.usage-page__chart {
  width: 100%;
  height: 320px;
}

.usage-page__tables {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(420px, 1fr));
  gap: 16px;
}

.stat-card {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding-left: 12px;
  border-left: 3px solid var(--primary);
}

.stat-card--slate {
  border-left-color: var(--saas_dashboard-color-5);
}

.stat-card--violet {
  border-left-color: var(--saas_dashboard-color-2);
}

.stat-card--blue {
  border-left-color: var(--info);
}

.stat-card__label {
  color: var(--text-secondary);
  font-size: 13px;
}

.stat-card__value {
  color: var(--text-main);
  font-size: 24px;
  font-weight: 600;
}
</style>

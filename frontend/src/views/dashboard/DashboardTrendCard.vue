<!-- 仪表盘趋势卡：近 30 天人工与 AI 字数双系列图。 -->
<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import * as echarts from 'echarts'
import { useI18n } from 'vue-i18n'
import type { NWStatsOverviewVo } from '@/api/contracts'
import NWIcon from '@/components/NWIcon.vue'

const props = defineProps<{
  overview: NWStatsOverviewVo
  loading: boolean
}>()

const { t } = useI18n()
const chartElement = ref<HTMLDivElement | null>(null)
let chartInstance: echarts.ECharts | null = null

const hasData = computed(() =>
  props.overview.trend_list.some((item) => item.word_delta > 0 || item.ai_word_delta > 0),
)

function renderChart(): void {
  if (!chartElement.value) return
  if (!chartInstance) chartInstance = echarts.init(chartElement.value)
  const trendList = props.overview.trend_list
  chartInstance.setOption(
    {
      color: [t('dashboard.trendWordDelta'), t('dashboard.trendAiWordDelta')],
      tooltip: {
        trigger: 'axis',
        backgroundColor: '#ffffff',
        borderColor: '#eef2f7',
        textStyle: { color: '#1e293b', fontSize: 12 },
        extraCssText: 'box-shadow: 0 12px 32px rgba(15,23,42,.12); border-radius: 12px;',
      },
      legend: {
        data: [t('dashboard.trendWordDelta'), t('dashboard.trendAiWordDelta')],
        top: 0,
        right: 8,
        icon: 'roundRect',
        itemWidth: 10,
        itemHeight: 10,
        textStyle: { color: '#64748b', fontSize: 11 },
      },
      grid: { left: 44, right: 18, top: 38, bottom: 28 },
      xAxis: {
        type: 'category',
        data: trendList.map((item) => item.stat_date.slice(5)),
        axisLine: { lineStyle: { color: '#e2e8f0' } },
        axisTick: { show: false },
        axisLabel: { color: '#94a3b8', fontSize: 10.5, interval: 4 },
      },
      yAxis: {
        type: 'value',
        splitLine: { lineStyle: { color: '#eef2f7', type: 'dashed' } },
        axisLabel: { color: '#94a3b8', fontSize: 10.5 },
      },
      series: [
        {
          name: t('dashboard.trendWordDelta'),
          type: 'bar',
          data: trendList.map((item) => item.word_delta),
          itemStyle: { color: '#6366f1', borderRadius: [5, 5, 0, 0] },
          barMaxWidth: 12,
        },
        {
          name: t('dashboard.trendAiWordDelta'),
          type: 'line',
          smooth: true,
          data: trendList.map((item) => item.ai_word_delta),
          itemStyle: { color: '#a78bfa' },
          lineStyle: { width: 2.5 },
          symbol: 'circle',
          symbolSize: 5,
          areaStyle: {
            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
              { offset: 0, color: 'rgba(167,139,250,0.22)' },
              { offset: 1, color: 'rgba(167,139,250,0)' },
            ]),
          },
        },
      ],
    },
    true,
  )
}

function handleResize(): void {
  chartInstance?.resize()
}

watch(() => props.overview, renderChart, { deep: true })

onMounted(() => {
  renderChart()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  chartInstance?.dispose()
  chartInstance = null
})
</script>

<template>
  <section class="nw-panel dashboard-trend">
    <header class="nw-panel__header">
      <div>
        <h3 class="nw-panel__title">{{ t('dashboard.trendTitle') }}</h3>
        <p class="nw-panel__subtitle">{{ t('dashboard.trendSubtitle') }}</p>
      </div>
      <span class="nw-badge nw-badge--neutral">{{ t('dashboard.last30Days') }}</span>
    </header>
    <div class="nw-panel__body dashboard-trend__body">
      <div ref="chartElement" class="dashboard-trend__chart" />
      <div v-if="!hasData && !loading" class="dashboard-trend__empty">
        <NWIcon icon_code="chart" :size_px="26" />
        <p>{{ t('dashboard.trendEmpty') }}</p>
      </div>
    </div>
  </section>
</template>

<style scoped>
.dashboard-trend {
  min-width: 0;
}

.dashboard-trend__body {
  position: relative;
  padding: 8px 12px 10px;
}

.dashboard-trend__chart {
  width: 100%;
  height: 286px;
}

.dashboard-trend__empty {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6px;
  color: var(--text-placeholder);
  font-size: 12.5px;
  pointer-events: none;
}

.dashboard-trend__empty p {
  margin: 0;
}
</style>

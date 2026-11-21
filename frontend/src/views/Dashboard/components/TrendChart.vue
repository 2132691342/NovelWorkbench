<!-- 字数趋势图：双系列展示日增量与 AI 产出。 -->
<script setup lang="ts">
  import { computed } from 'vue'
  import type { EChartsOption } from 'echarts'
  import type { NWTrendItemVo } from '@/api/contracts'
  import { useAppStore } from '@/store/modules/app'
  import Echart from '@/components/Echart/index.vue'

  const props = defineProps<{ trendList: NWTrendItemVo[] }>()

  const appStore = useAppStore()

  const option = computed<EChartsOption>(() => ({
    grid: { left: 44, right: 18, top: 34, bottom: 30 },
    tooltip: { trigger: 'axis' },
    legend: {
      data: ['新增字数', 'AI 产出'],
      right: 12,
      top: 4,
      textStyle: { color: appStore.isDark ? '#94a3b8' : '#64748b', fontSize: 11 }
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: props.trendList.map((item) => item.stat_date.slice(5)),
      axisLine: { lineStyle: { color: appStore.isDark ? '#344055' : '#e2e8f0' } },
      axisLabel: { color: appStore.isDark ? '#94a3b8' : '#94a3b8', fontSize: 11 }
    },
    yAxis: {
      type: 'value',
      splitLine: { lineStyle: { color: appStore.isDark ? '#273244' : '#eef2f7' } },
      axisLabel: { color: appStore.isDark ? '#94a3b8' : '#94a3b8', fontSize: 11 }
    },
    series: [
      {
        name: '新增字数',
        type: 'line',
        smooth: true,
        showSymbol: false,
        lineStyle: { width: 2, color: '#6366f1' },
        areaStyle: { color: 'rgba(99, 102, 241, 0.14)' },
        data: props.trendList.map((item) => item.word_delta)
      },
      {
        name: 'AI 产出',
        type: 'line',
        smooth: true,
        showSymbol: false,
        lineStyle: { width: 2, color: '#10b981' },
        data: props.trendList.map((item) => item.ai_word_delta)
      }
    ]
  }))
</script>

<template>
  <ElCard shadow="never" class="trend-card">
    <template #header>
      <div class="trend-card__head">
        <span class="trend-card__title">近 30 天写作趋势</span>
        <span class="nw-hint">按日统计正文字数增量</span>
      </div>
    </template>
    <Echart v-if="trendList.length > 0" :option="option" :height="220" />
    <ElEmpty v-else description="暂无写作数据" :image-size="70" />
  </ElCard>
</template>

<style lang="less" scoped>
  .trend-card {
    border-radius: var(--nw-radius-lg);

    &__head {
      display: flex;
      gap: 12px;
      align-items: baseline;
    }

    &__title {
      font-size: 14px;
      font-weight: 650;
    }
  }
</style>

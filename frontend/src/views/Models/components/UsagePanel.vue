<!-- 用量面板：按供应商与模型展示调用量与 token。 -->
<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import type { EChartsOption } from 'echarts'
  import { ElCard, ElEmpty, ElTag } from 'element-plus'
  import { getModelUsageOverview, type NWModelUsageOverviewVo } from '@/api/model'
  import { useAppStore } from '@/store/modules/app'
  import Echart from '@/components/Echart/index.vue'

  const appStore = useAppStore()

  const loading = ref(false)
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

  const axisColor = '#94a3b8'
  const splitColor = computed(() => (appStore.isDark ? '#273244' : '#eef2f7'))

  const option = computed<EChartsOption>(() => ({
    grid: { left: 46, right: 46, top: 34, bottom: 30 },
    tooltip: { trigger: 'axis' },
    legend: {
      data: ['调用次数', 'Token 用量'],
      right: 12,
      top: 4,
      textStyle: { color: axisColor, fontSize: 11 }
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: overview.value.daily_list.map((item) => item.stat_date.slice(5)),
      axisLine: { lineStyle: { color: splitColor.value } },
      axisLabel: { color: axisColor, fontSize: 11 }
    },
    yAxis: [
      {
        type: 'value',
        name: '次数',
        nameTextStyle: { color: axisColor, fontSize: 11 },
        splitLine: { lineStyle: { color: splitColor.value } },
        axisLabel: { color: axisColor, fontSize: 11 }
      },
      {
        type: 'value',
        name: 'token',
        nameTextStyle: { color: axisColor, fontSize: 11 },
        splitLine: { show: false },
        axisLabel: { color: axisColor, fontSize: 11 }
      }
    ],
    series: [
      {
        name: '调用次数',
        type: 'line',
        smooth: true,
        showSymbol: false,
        lineStyle: { width: 2, color: '#6366f1' },
        areaStyle: { color: 'rgba(99, 102, 241, 0.14)' },
        data: overview.value.daily_list.map((item) => item.call_count)
      },
      {
        name: 'Token 用量',
        type: 'line',
        yAxisIndex: 1,
        smooth: true,
        showSymbol: false,
        lineStyle: { width: 2, color: '#10b981' },
        data: overview.value.daily_list.map((item) => item.total_tokens)
      }
    ]
  }))

  const load = async () => {
    loading.value = true
    try {
      overview.value = await getModelUsageOverview()
    } catch {
      // 失败提示由请求层统一弹出，保留上次数据
    } finally {
      loading.value = false
    }
  }

  onMounted(() => {
    void load()
  })
</script>

<template>
  <ElCard shadow="never" class="usage-panel" v-loading="loading">
    <template #header>
      <div class="usage-panel__head">
        <span class="usage-panel__title">用量概览</span>
        <span class="nw-hint">近 14 天模型调用与 token 消耗</span>
      </div>
    </template>

    <div class="usage-panel__stats">
      <div class="usage-panel__stat">
        <span class="usage-panel__label">调用次数</span>
        <strong class="usage-panel__value">{{ overview.call_count_total.toLocaleString() }}</strong>
      </div>
      <div class="usage-panel__stat">
        <span class="usage-panel__label">Token 总量</span>
        <strong class="usage-panel__value">
          {{ overview.total_tokens_total.toLocaleString() }}
        </strong>
      </div>
      <div class="usage-panel__stat">
        <span class="usage-panel__label">平均耗时</span>
        <strong class="usage-panel__value">{{ Math.round(overview.avg_latency_ms) }} ms</strong>
      </div>
      <div class="usage-panel__stat">
        <span class="usage-panel__label">成功率</span>
        <strong class="usage-panel__value">
          {{ (overview.success_rate_value * 100).toFixed(1) }}%
        </strong>
      </div>
      <div class="usage-panel__stat">
        <span class="usage-panel__label">失败次数</span>
        <strong class="usage-panel__value is-danger">
          {{ overview.fail_count_total.toLocaleString() }}
        </strong>
      </div>
    </div>

    <Echart v-if="overview.daily_list.length > 0" :option="option" :height="240" />
    <ElEmpty v-else description="暂无调用记录" :image-size="70" />

    <div v-if="overview.provider_list.length > 0" class="usage-panel__groups">
      <div class="usage-panel__group">
        <span class="usage-panel__label">按供应商</span>
        <div class="usage-panel__tags">
          <ElTag
            v-for="item in overview.provider_list"
            :key="item.group_name"
            size="small"
            effect="plain"
          >
            {{ item.group_name }} · {{ item.call_count }} 次
          </ElTag>
        </div>
      </div>
      <div class="usage-panel__group">
        <span class="usage-panel__label">按模型</span>
        <div class="usage-panel__tags">
          <ElTag
            v-for="item in overview.model_list"
            :key="item.group_name"
            size="small"
            effect="plain"
            type="success"
          >
            {{ item.group_name }} · {{ item.call_count }} 次
          </ElTag>
        </div>
      </div>
    </div>
  </ElCard>
</template>

<style lang="less" scoped>
  .usage-panel {
    border-radius: var(--nw-radius-lg);

    &__head {
      display: flex;
      gap: 12px;
      align-items: baseline;
    }

    &__title {
      font-size: 14px;
      font-weight: 650;
      color: var(--nw-ink);
    }

    &__stats {
      display: grid;
      gap: 12px;
      margin-bottom: 8px;
      grid-template-columns: repeat(auto-fit, minmax(130px, 1fr));
    }

    &__stat {
      display: flex;
      flex-direction: column;
      gap: 4px;
      padding: 10px 12px;
      background: var(--nw-surface-subtle);
      border-radius: var(--nw-radius-md);
    }

    &__label {
      font-size: 12px;
      color: var(--nw-text-secondary);
    }

    &__value {
      font-size: 18px;
      font-weight: 650;
      color: var(--nw-ink);

      &.is-danger {
        color: var(--nw-danger);
      }
    }

    &__groups {
      display: flex;
      flex-direction: column;
      gap: 10px;
      padding-top: 12px;
      margin-top: 12px;
      border-top: 1px solid var(--nw-border-soft);
    }

    &__group {
      display: flex;
      gap: 10px;
      align-items: flex-start;
    }

    &__tags {
      display: flex;
      flex-wrap: wrap;
      gap: 6px;
    }
  }
</style>

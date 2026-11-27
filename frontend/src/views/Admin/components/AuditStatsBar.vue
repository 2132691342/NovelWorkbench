<!-- 审计概览条：区间内的链路数、错误链路、模型调用、平均耗时。
     与「留存摘要」分工不同：留存管数据活多久，这里管这段时间系统跑得怎么样。 -->
<script setup lang="ts">
  import { computed } from 'vue'
  import { ElSkeleton } from 'element-plus'
  import type { NWAuditStatsVo } from '@/api/admin'

  const props = defineProps<{
    /** 区间统计（null 表示尚未取到） */
    stats: NWAuditStatsVo | null
    loading: boolean
  }>()

  /** 卡片定义：值与配色在计算属性里统一决定，避免模板里堆三元表达式 */
  const cardList = computed(() => {
    const stats = props.stats
    if (!stats) return []
    return [
      { key: 'trace', label: '链路', value: stats.trace_count.toLocaleString(), unit: '次', tone: 'primary' },
      {
        key: 'error',
        label: '异常链路',
        value: (stats.error_trace_count + stats.warn_trace_count).toLocaleString(),
        unit: `错误 ${stats.error_trace_count} / 警示 ${stats.warn_trace_count}`,
        tone: stats.error_trace_count > 0 ? 'danger' : stats.warn_trace_count > 0 ? 'warning' : 'success'
      },
      {
        key: 'ai',
        label: '模型调用',
        value: stats.ai_call_count.toLocaleString(),
        unit: `${stats.token_count.toLocaleString()} token`,
        tone: 'primary'
      },
      {
        key: 'latency',
        label: '平均耗时',
        value: stats.avg_latency_ms.toLocaleString(),
        unit: 'ms',
        tone: stats.avg_latency_ms >= 5000 ? 'warning' : 'success'
      },
      {
        key: 'slow',
        label: '慢链路',
        value: stats.slow_trace_count.toLocaleString(),
        unit: '超过 5 秒',
        tone: stats.slow_trace_count > 0 ? 'warning' : 'success'
      },
      { key: 'span', label: '记录数', value: stats.span_count.toLocaleString(), unit: '条 span', tone: 'primary' }
    ]
  })
</script>

<template>
  <section class="audit-stats">
    <ElSkeleton v-if="loading && !stats" :rows="1" animated class="audit-stats__skeleton" />
    <template v-else-if="stats">
      <div v-for="card in cardList" :key="card.key" class="audit-stat">
        <span class="audit-stat__label">{{ card.label }}</span>
        <strong :class="['audit-stat__value', `is-${card.tone}`]">{{ card.value }}</strong>
        <span class="audit-stat__unit">{{ card.unit }}</span>
      </div>
    </template>
    <div v-else class="nw-hint">概览统计读取失败（可能是 Manticore 不可用）</div>
  </section>
</template>

<style lang="less" scoped>
  .audit-stats {
    display: grid;
    gap: 10px;
    grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));

    &__skeleton {
      grid-column: 1 / -1;
    }
  }

  .audit-stat {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 10px 12px;
    background: var(--nw-surface);
    border: 1px solid var(--nw-border);
    border-radius: var(--nw-radius-lg);
    box-shadow: var(--nw-shadow-card);
    transition:
      border-color 0.16s ease,
      transform 0.16s ease;

    &:hover {
      border-color: var(--el-color-primary);
      transform: translateY(-1px);
    }

    &__label {
      font-size: 11.5px;
      color: var(--el-text-color-secondary);
    }

    &__value {
      font-size: 19px;
      font-weight: 700;
      font-variant-numeric: tabular-nums;
      color: var(--nw-ink);

      &.is-danger {
        color: var(--nw-danger);
      }

      &.is-warning {
        color: var(--nw-warning);
      }

      &.is-success {
        color: var(--nw-success);
      }

      &.is-primary {
        color: var(--el-color-primary);
      }
    }

    &__unit {
      font-size: 11px;
      color: var(--el-text-color-placeholder);
    }
  }
</style>

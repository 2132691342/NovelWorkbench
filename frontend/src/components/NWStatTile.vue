<!--
  统计磁贴：图标 + 标签 + 数值 + 辅助说明，统一仪表盘/管理页的数据卡。
  数值只在变化时播放一次上浮动画，避免反复刷新造成视觉噪音。
-->
<script setup lang="ts">
import { computed } from 'vue'
import NWIcon from '@/components/NWIcon.vue'

const props = withDefaults(
  defineProps<{
    label_text: string
    value_number: number | string
    hint_text?: string
    icon_code?: string
    tone_code?: 'primary' | 'success' | 'warning' | 'info' | 'neutral' | 'danger'
    suffix_text?: string
  }>(),
  {
    hint_text: '',
    icon_code: 'chart',
    tone_code: 'primary',
    suffix_text: '',
  },
)

const displayValue = computed(() => {
  if (typeof props.value_number !== 'number') {
    return props.value_number
  }
  return props.value_number.toLocaleString()
})
</script>

<template>
  <article class="nw-stat-tile" :class="`is-${tone_code}`">
    <div class="nw-stat-tile__top">
      <span class="nw-stat-tile__icon">
        <NWIcon :icon_code="icon_code" :size_px="18" />
      </span>
      <span class="nw-stat-tile__label">{{ label_text }}</span>
    </div>
    <div class="nw-stat-tile__value-row">
      <strong class="nw-stat-tile__value">{{ displayValue }}</strong>
      <span v-if="suffix_text" class="nw-stat-tile__suffix">{{ suffix_text }}</span>
    </div>
    <p v-if="hint_text" class="nw-stat-tile__hint">{{ hint_text }}</p>
  </article>
</template>

<style scoped>
.nw-stat-tile {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 16px 18px;
  border: 1px solid var(--border-light);
  border-radius: var(--radius-lg);
  background: var(--bg-card);
  box-shadow: var(--shadow-card);
  overflow: hidden;
  transition: transform var(--transition-base), box-shadow var(--transition-base);
}

.nw-stat-tile::after {
  content: '';
  position: absolute;
  right: -28px;
  top: -36px;
  width: 112px;
  height: 112px;
  border-radius: 50%;
  background: radial-gradient(circle, rgba(99, 102, 241, 0.1), transparent 68%);
  pointer-events: none;
}

.nw-stat-tile:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-card-hover);
}

.nw-stat-tile__top {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--text-secondary);
  font-size: 12.5px;
  font-weight: 500;
}

.nw-stat-tile__icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  border-radius: 9px;
  color: var(--primary);
  background: var(--primary-soft);
}

.nw-stat-tile__value-row {
  display: flex;
  align-items: baseline;
  gap: 5px;
}

.nw-stat-tile__value {
  color: var(--text-strong);
  font-size: 27px;
  font-weight: 700;
  letter-spacing: -0.02em;
  line-height: 1.1;
  font-variant-numeric: tabular-nums;
}

.nw-stat-tile__suffix {
  color: var(--text-placeholder);
  font-size: 12px;
}

.nw-stat-tile__hint {
  margin: 0;
  color: var(--text-placeholder);
  font-size: 12px;
  line-height: 1.5;
}

.nw-stat-tile.is-success .nw-stat-tile__icon {
  color: var(--success-text);
  background: var(--success-bg);
}

.nw-stat-tile.is-warning .nw-stat-tile__icon {
  color: var(--warning-text);
  background: var(--warning-bg);
}

.nw-stat-tile.is-info .nw-stat-tile__icon {
  color: var(--info-text);
  background: var(--info-bg);
}

.nw-stat-tile.is-danger .nw-stat-tile__icon {
  color: var(--error-text);
  background: var(--error-bg);
}

.nw-stat-tile.is-neutral .nw-stat-tile__icon {
  color: var(--text-secondary);
  background: var(--bg-hover);
}
</style>

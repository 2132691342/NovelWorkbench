<!--
  通用空态：列表无数据占位。支持标题、描述、图标与操作插槽。
  兼容旧用法：仅传 description_text 时渲染单行说明。
-->
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import NWIcon from '@/components/NWIcon.vue'

const props = withDefaults(
  defineProps<{
    title_text?: string
    description_text?: string
    icon_code?: string
    compact_flag?: boolean
  }>(),
  {
    title_text: '',
    description_text: '',
    icon_code: 'book',
    compact_flag: false,
  },
)

const { t } = useI18n()

const resolvedTitle = computed(() => props.title_text || t('common.empty'))
</script>

<template>
  <div class="nw-empty-state" :class="{ 'is-compact': compact_flag }">
    <span class="nw-empty-state__icon">
      <NWIcon :icon_code="icon_code" :size_px="compact_flag ? 22 : 30" />
    </span>
    <p class="nw-empty-state__title">{{ resolvedTitle }}</p>
    <p v-if="description_text" class="nw-empty-state__desc">{{ description_text }}</p>
    <div v-if="$slots.actions" class="nw-empty-state__actions">
      <slot name="actions" />
    </div>
  </div>
</template>

<style scoped>
.nw-empty-state.is-compact {
  padding: 28px 16px;
  gap: 6px;
}

.nw-empty-state.is-compact .nw-empty-state__icon {
  width: 48px;
  height: 48px;
  border-radius: var(--radius-lg);
}

.nw-empty-state.is-compact .nw-empty-state__title {
  font-size: 14px;
}
</style>

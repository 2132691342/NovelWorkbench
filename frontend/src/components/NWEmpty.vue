<script setup lang="ts">

// 通用空态组件：列表无数据时的占位展示，含图标与说明文案。
import { useI18n } from 'vue-i18n'

withDefaults(
  defineProps<{
    description_text?: string
  }>(),
  {
    description_text: '',
  },
)

const { t } = useI18n()
</script>

<template>
  <div class="nw-empty">
    <svg class="nw-empty__icon" viewBox="0 0 48 48" fill="none" aria-hidden="true">
      <rect x="8" y="14" width="32" height="24" rx="4" stroke="currentColor" stroke-width="2.5" />
      <path d="M8 22h32" stroke="currentColor" stroke-width="2.5" />
      <path d="M20 30h8" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" />
    </svg>
    <span class="nw-empty__description">{{ description_text || t('common.empty') }}</span>
  </div>
</template>

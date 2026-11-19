<!--
  页面标题区：统一标题、说明、图标、徽标与右侧操作区。
  所有业务页/管理页使用同一节奏，避免每页各自拼标题栏。
-->
<script setup lang="ts">
import { useRouter } from 'vue-router'
import NWIcon from '@/components/NWIcon.vue'

const props = withDefaults(
  defineProps<{
    title_text: string
    subtitle_text?: string
    icon_code?: string
    back_path?: string
    eyebrow_text?: string
  }>(),
  {
    subtitle_text: '',
    icon_code: '',
    back_path: '',
    eyebrow_text: '',
  },
)

const router = useRouter()

function handleBack(): void {
  if (props.back_path) {
    void router.push(props.back_path)
    return
  }
  router.back()
}
</script>

<template>
  <header class="nw-page-header">
    <div class="nw-page-header__main">
      <span v-if="icon_code" class="nw-page-header__icon">
        <NWIcon :icon_code="icon_code" :size_px="22" />
      </span>
      <div class="nw-page-header__text">
        <button v-if="back_path" type="button" class="nw-page-header__back" @click="handleBack">
          ← 返回
        </button>
        <span v-if="eyebrow_text" class="nw-page-header__eyebrow">{{ eyebrow_text }}</span>
        <div class="nw-page-header__title-row">
          <h1 class="nw-page-header__title">{{ title_text }}</h1>
          <slot name="badge" />
        </div>
        <p v-if="subtitle_text" class="nw-page-header__subtitle">{{ subtitle_text }}</p>
      </div>
    </div>
    <div v-if="$slots.actions" class="nw-page-header__actions">
      <slot name="actions" />
    </div>
  </header>
</template>

<style scoped>
.nw-page-header__eyebrow {
  display: block;
  margin-bottom: 4px;
  color: var(--text-placeholder);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}
</style>

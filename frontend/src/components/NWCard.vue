<script setup lang="ts">

// 通用卡片组件：提供统一的容器边框、圆角、阴影与标题插槽。
import NWLoading from '@/components/NWLoading.vue'

withDefaults(
  defineProps<{
    title?: string
    hoverable?: boolean
    loading?: boolean
  }>(),
  {
    title: '',
    hoverable: true,
    loading: false,
  },
)
</script>

<template>
  <section class="nw-card" :class="{ 'nw-card-hover': hoverable }">
    <header v-if="title || $slots.header" class="nw-card__header">
      <slot name="header">
        <h3 class="nw-card__title">{{ title }}</h3>
      </slot>
    </header>
    <div class="nw-card__body">
      <NWLoading v-if="loading" />
      <slot v-else />
    </div>
  </section>
</template>

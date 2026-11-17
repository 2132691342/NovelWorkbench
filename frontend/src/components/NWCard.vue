<script setup lang="ts">
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

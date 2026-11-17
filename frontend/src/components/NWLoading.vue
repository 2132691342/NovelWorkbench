<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
</script>

<template>
  <div class="nw-loading" role="status">
    <span class="nw-loading__spinner" />
    <span class="nw-loading__text">{{ t('common.loading') }}</span>
  </div>
</template>

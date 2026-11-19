<script setup lang="ts">

// 通用加载态组件：数据加载中的占位展示，支持自定义提示文案。
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
</script>

<template>
  <div class="nw-loading" role="status">
    <span class="nw-loading__spinner" />
    <span class="nw-loading__text">{{ t('common.loading') }}</span>
  </div>
</template>

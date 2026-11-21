<!-- 站点标识：展示品牌图标与名称，可折叠为纯图标。 -->
<script setup lang="ts">
  import { computed } from 'vue'
  import { useAppStore } from '@/store/modules/app'
  import { appConfig } from '@/config/app'

  const appStore = useAppStore()
  const title = appConfig.title
  const showTitle = computed(() => !appStore.collapse || appStore.mobile)
</script>

<template>
  <router-link class="nw-logo" to="/">
    <span class="nw-logo__mark">N</span>
    <span v-if="showTitle" class="nw-logo__title">{{ title }}</span>
  </router-link>
</template>

<style lang="less" scoped>
  .nw-logo {
    display: flex;
    flex: none;
    gap: 10px;
    align-items: center;
    height: var(--logo-height);
    padding: 0 16px;
    overflow: hidden;
    text-decoration: none;

    &__mark {
      display: grid;
      flex: none;
      width: 34px;
      height: 34px;
      font-size: 17px;
      font-weight: 700;
      color: #fff;
      background: linear-gradient(135deg, var(--saas_dashboard-color-1), var(--saas_dashboard-color-2));
      border-radius: 9px;
      place-items: center;
    }

    &__title {
      font-size: 15px;
      font-weight: 650;
      letter-spacing: -0.01em;
      color: var(--logo-title-text-color);
      white-space: nowrap;
    }
  }
</style>

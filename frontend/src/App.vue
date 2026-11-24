<!-- 应用根组件：注入全局配置、监听视口宽度并承载路由视图。 -->
<script setup lang="ts">
  import { watch } from 'vue'
  import { useMediaQuery } from '@vueuse/core'
  import ConfigGlobal from '@/components/ConfigGlobal/index.vue'
  import { useTheme } from '@/hooks/useTheme'
  import { useTitle } from '@/hooks/useTitle'
  import { useAppStore } from '@/store/modules/app'

  useTheme()
  useTitle()

  const appStore = useAppStore()

  /** 与 Layout 的移动断点保持一致：窄屏导航变成抽屉，默认收起 */
  const isMobileViewport = useMediaQuery('(max-width: 767px)')

  watch(
    isMobileViewport,
    (mobile) => {
      appStore.mobile = mobile
      if (mobile) appStore.collapse = true
    },
    { immediate: true }
  )
</script>

<template>
  <ConfigGlobal>
    <RouterView />
  </ConfigGlobal>
</template>

<style lang="less">
  .size {
    width: 100%;
    height: 100%;
  }

  html,
  body {
    padding: 0 !important;
    margin: 0;
    overflow: hidden;
    .size;

    #app {
      .size;
    }
  }

  #nprogress .bar {
    background: var(--el-color-primary) !important;
  }
</style>

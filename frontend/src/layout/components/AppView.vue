<!-- 内容区容器：承载路由视图与过渡动画。 -->
<script setup lang="ts">
  import { computed } from 'vue'
  import { useTagsViewStore } from '@/store/modules/tagsView'

  const tagsViewStore = useTagsViewStore()

  const cachedViews = computed(() => tagsViewStore.cachedViews)
</script>

<template>
  <section class="nw-app-view">
    <router-view v-slot="{ Component, route }">
      <transition name="nw-fade" mode="out-in">
        <keep-alive :include="cachedViews">
          <component :is="Component" :key="route.fullPath" />
        </keep-alive>
      </transition>
    </router-view>
  </section>
</template>

<style lang="less" scoped>
  .nw-app-view {
    box-sizing: border-box;
    width: 100%;
    min-height: calc(100vh - var(--top-tool-height) - var(--tags-view-height));
    padding: var(--app-content-padding);
    background: var(--app-content-bg-color);
  }
</style>

<!--
  刷新中转页：TagsView 右键「刷新页面」跳到这里，再立即回跳原页面，
  用于强制重建组件实例（keep-alive 缓存下的页面刷新手段）。
-->
<script setup lang="ts">
import { onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()

onMounted(() => {
  const targetPath = String(route.query.target_path ?? '/')
  void router.replace(targetPath)
})
</script>

<template>
  <div class="redirect-view" />
</template>

<style scoped>
.redirect-view {
  height: 1px;
}
</style>

<!--
  内容容器：统一页面内边距与卡片间距，页面只需关心自己的业务内容。
-->
<script setup lang="ts">
withDefaults(
  defineProps<{
    /** 是否使用紧凑内边距（写作台等需要满屏的页面传 true） */
    compact_flag?: boolean
  }>(),
  { compact_flag: false },
)
</script>

<template>
  <div class="nw-content-wrap" :class="{ 'is-compact': compact_flag }">
    <slot />
  </div>
</template>

<style scoped src="./ContentWrap.css"></style>

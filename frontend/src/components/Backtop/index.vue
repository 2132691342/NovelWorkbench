<!-- 回到顶部按钮：绑定内容区滚动容器，滚动超过阈值后出现，点击平滑回顶。 -->
<script setup lang="ts">
  import { ElBacktop } from 'element-plus'

  /** 内容区滚动容器（与 Layout.vue 的 ElScrollbar 类名同源） */
  const scrollTarget = '.v-layout-content-scrollbar > .el-scrollbar__wrap'
</script>

<template>
  <ElBacktop :target="scrollTarget" :visibility-height="240" :right="28" :bottom="32" />
</template>

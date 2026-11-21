<!-- 回到顶部按钮：滚动超过阈值后出现，点击平滑回顶。 -->
<script setup lang="ts">
  import { ElBacktop } from 'element-plus'
</script>

<template>
  <ElBacktop target=".nw-layout__scroll .el-scrollbar__wrap" :visibility-height="240" />
</template>

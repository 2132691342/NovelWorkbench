<!-- 主题切换按钮：在明暗两种主题间切换。 -->
<script setup lang="ts">
  import { ElSwitch } from 'element-plus'

  const isDark = defineModel<boolean>({ required: true })
</script>

<template>
  <ElSwitch
    v-model="isDark"
    inline-prompt
    active-text="☾"
    inactive-text="☀"
    aria-label="Toggle dark mode"
  />
</template>

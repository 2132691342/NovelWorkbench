<!--
  创作笔记 Markdown 编辑器：mavon-editor Vue3 版本封装。
  与富文本编辑器共享字符串值契约，上层无感知切换。
-->
<script setup lang="ts">
import { computed } from 'vue'
import type { Component } from 'vue'
import mavonEditor from 'mavon-editor'
import 'mavon-editor/dist/css/index.css'

const props = defineProps<{
  modelValue: string
  placeholder?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [markdownText: string]
  change: [markdownText: string]
}>()

/** mavon-editor 的默认导出携带多个 Vue 组件，这里取主编辑器组件 */
const MavonEditor = computed<Component>(() => mavonEditor.mavonEditor)

function handleChange(markdownText: string): void {
  emit('update:modelValue', markdownText)
  emit('change', markdownText)
}
</script>

<template>
  <component
    :is="MavonEditor"
    class="markdown-note-editor"
    :model-value="props.modelValue"
    :placeholder="props.placeholder"
    :subfield="false"
    :default-open="'edit'"
    :html="false"
    @change="handleChange"
  />
</template>

<style scoped>
.markdown-note-editor {
  height: 100%;
  min-height: 0;
}
</style>

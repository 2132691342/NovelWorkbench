<!--
  创作笔记富文本编辑器：WangEditor（@wangeditor/editor-for-vue Vue3 适配器）。
  v-model 双向绑定 HTML；组件卸载时 destroy 编辑器实例，防内存泄漏。
-->
<script setup lang="ts">
import '@wangeditor/editor/dist/css/style.css'
import { onBeforeUnmount, shallowRef, watch } from 'vue'
import { Editor, Toolbar } from '@wangeditor/editor-for-vue'
import type { IDomEditor, IToolbarConfig } from '@wangeditor/editor'

const props = defineProps<{
  modelValue: string
  placeholder?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [html: string]
  change: [html: string]
}>()

/** 编辑器实例（WangEditor 文档要求 shallowRef 避免深度响应式代理） */
const editorInstance = shallowRef<IDomEditor | null>(null)

const toolbarConfig: Partial<IToolbarConfig> = {}

/** 双向绑定：外部更新 HTML 时同步到编辑器（去重回环） */
watch(
  () => props.modelValue,
  (html) => {
    const editor = editorInstance.value
    if (editor && html !== editor.getHtml()) {
      editor.setHtml(html)
    }
  },
)

function handleCreated(editor: IDomEditor): void {
  editorInstance.value = editor
}

function handleChange(editor: IDomEditor): void {
  const html = editor.getHtml()
  emit('update:modelValue', html)
  emit('change', html)
}

onBeforeUnmount(() => {
  // 卸载时销毁编辑器实例，释放 DOM 与事件监听
  editorInstance.value?.destroy()
  editorInstance.value = null
})
</script>

<template>
  <div class="note-editor">
    <Toolbar class="note-editor__toolbar" :editor="editorInstance" :default-config="toolbarConfig" mode="default" />
    <Editor
      class="note-editor__body"
      :model-value="props.modelValue"
      :default-config="{ placeholder: props.placeholder ?? '' }"
      mode="default"
      @on-created="handleCreated"
      @on-change="handleChange"
    />
  </div>
</template>

<style scoped>
.note-editor {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  border: 1px solid var(--border-light);
  border-radius: var(--radius-base);
  overflow: hidden;
  background-color: var(--bg-card);
}

.note-editor__toolbar {
  border-bottom: 1px solid var(--border-light);
}

.note-editor__body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}
</style>

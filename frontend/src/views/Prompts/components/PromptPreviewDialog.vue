<!-- 提示词预览弹窗：只读展示提示词全文与占位符说明。 -->
<script setup lang="ts">
  import { computed } from 'vue'
  import { ElButton, ElDialog, ElTag } from 'element-plus'
  import type { NWPromptVo } from '@/api/contracts'
  import { formatTime } from '@/utils/time'

  const props = defineProps<{ prompt: NWPromptVo | null }>()
  const visible = defineModel<boolean>('visible', { required: true })

  const title = computed(() =>
    props.prompt ? `提示词预览 · ${props.prompt.prompt_name}` : '提示词预览'
  )

  const charCount = computed(() => props.prompt?.prompt_text.length ?? 0)
  const kindLabel = computed(() => (props.prompt?.prompt_kind_code === 'system' ? '身份' : '指令'))
</script>

<template>
  <ElDialog v-model="visible" :title="title" width="760px" append-to-body destroy-on-close>
    <div v-if="prompt" class="preview">
      <div class="preview__meta">
        <ElTag size="small" effect="plain">
          {{ prompt.prompt_kind_code === 'command' ? '/' : '' }}{{ prompt.prompt_code }}
        </ElTag>
        <ElTag size="small" type="success" effect="plain">{{ kindLabel }}</ElTag>
        <ElTag v-if="prompt.domain_name" size="small" type="info" effect="plain">
          {{ prompt.domain_name }}
        </ElTag>
        <span class="nw-hint">{{ charCount }} 字符 · 更新于 {{ formatTime(prompt.updated_at) }}</span>
      </div>
      <pre class="preview__body">{{ prompt.prompt_text }}</pre>
      <p class="nw-hint" v-pre>
        注入方式：以系统级消息进入消息列表；用户输入始终是用户级消息。可用占位符
        <code>{{context}}</code>、<code>{{chapter_content}}</code>、<code>{{user_input}}</code>。
      </p>
    </div>
    <template #footer>
      <ElButton @click="visible = false">关闭</ElButton>
    </template>
  </ElDialog>
</template>

<style lang="less" scoped>
  .preview {
    display: flex;
    flex-direction: column;
    gap: 10px;

    &__meta {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      align-items: center;
    }

    &__body {
      max-height: 58vh;
      padding: 14px;
      overflow: auto;
      font-family: var(--el-font-family-mono, monospace);
      font-size: 12px;
      line-height: 1.8;
      color: var(--nw-ink);
      white-space: pre-wrap;
      word-break: break-word;
      background: var(--nw-surface-subtle);
      border: 1px solid var(--nw-border-soft);
      border-radius: var(--nw-radius-md);
    }
  }
</style>

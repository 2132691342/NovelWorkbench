<!-- 消息条目：渲染思考过程、正文与召回依据。 -->
<script setup lang="ts">
  import { computed } from 'vue'
  import { ElMessage } from 'element-plus'
  import type { ChatDisplayMessage } from './types'

  const props = defineProps<{ message: ChatDisplayMessage }>()

  const emit = defineEmits<{
    retry: []
    'insert-to-chapter': [contentText: string]
    'replace-chapter': [contentText: string]
  }>()

  const isUser = computed(() => props.message.role_code === 'user')
  const isAssistant = computed(() => props.message.role_code === 'assistant')
  const reasoningVisible = computed(() => props.message.reasoning_text.length > 0)
  const waitedText = computed(() =>
    props.message.waited_ms > 1000 ? `${(props.message.waited_ms / 1000).toFixed(1)}s` : ''
  )

  const copy = async () => {
    await navigator.clipboard.writeText(props.message.content_text)
    ElMessage.success('已复制')
  }
</script>

<template>
  <article :class="['chat-message', { 'is-user': isUser }]">
    <div v-if="!isUser" class="chat-message__avatar">
      <Icon icon="mdi:sparkles" :size="14" />
    </div>

    <div class="chat-message__body">
      <details v-if="reasoningVisible" class="chat-message__reasoning">
        <summary>思考过程</summary>
        <pre>{{ message.reasoning_text }}</pre>
      </details>

      <div v-if="message.content_text" class="chat-message__content">{{ message.content_text }}</div>

      <p v-else-if="message.streaming_flag" class="chat-message__waiting">
        正在生成{{ waitedText ? `，已等待 ${waitedText}` : '…' }}
      </p>

      <p v-if="message.error_message" class="chat-message__error">
        <Icon icon="mdi:alert-circle-outline" :size="13" />
        {{ message.error_message }}
        <button type="button" class="chat-message__retry" @click="emit('retry')">重试</button>
      </p>

      <div v-if="message.reference_list.length > 0" class="chat-message__refs">
        <span v-for="ref in message.reference_list" :key="ref.chunk_ulid" class="chat-message__ref">
          {{ ref.doc_name }}
        </span>
      </div>

      <div v-if="isAssistant && message.content_text && !message.streaming_flag" class="chat-message__actions">
        <button type="button" @click="copy">复制</button>
        <button type="button" @click="emit('insert-to-chapter', message.content_text)">插入正文</button>
        <button type="button" @click="emit('replace-chapter', message.content_text)">替换整章</button>
      </div>
    </div>
  </article>
</template>

<style lang="less" scoped>
  .chat-message {
    display: flex;
    gap: 8px;
    margin-bottom: 12px;

    &__avatar {
      display: grid;
      flex: none;
      width: 24px;
      height: 24px;
      color: var(--el-color-primary);
      background: var(--nw-primary-soft);
      border-radius: 7px;
      place-items: center;
    }

    &__body {
      flex: 1;
      min-width: 0;
    }

    &__content {
      font-size: 13px;
      line-height: 1.75;
      color: var(--nw-ink);
      white-space: pre-wrap;
      word-break: break-word;
    }

    &__reasoning {
      margin-bottom: 8px;
      font-size: 12px;
      color: var(--el-text-color-secondary);

      summary {
        cursor: pointer;
      }

      pre {
        padding: 8px;
        margin-top: 6px;
        overflow-x: auto;
        font-family: inherit;
        font-size: 11.5px;
        line-height: 1.7;
        white-space: pre-wrap;
        background: var(--nw-surface-subtle);
        border-radius: var(--nw-radius-base);
      }
    }

    &__waiting {
      font-size: 12.5px;
      color: var(--el-text-color-secondary);
    }

    &__error {
      display: flex;
      gap: 6px;
      align-items: center;
      margin-top: 6px;
      font-size: 12px;
      color: var(--nw-danger);
    }

    &__retry {
      padding: 0 6px;
      font-size: 12px;
      color: var(--el-color-primary);
      cursor: pointer;
      background: transparent;
      border: 0;
    }

    &__refs {
      display: flex;
      flex-wrap: wrap;
      gap: 4px;
      margin-top: 8px;
    }

    &__ref {
      padding: 1px 7px;
      font-size: 11px;
      color: var(--el-text-color-secondary);
      background: var(--nw-surface-subtle);
      border-radius: 999px;
    }

    &__actions {
      display: flex;
      gap: 10px;
      margin-top: 8px;

      button {
        padding: 0;
        font-size: 11.5px;
        color: var(--el-text-color-secondary);
        cursor: pointer;
        background: transparent;
        border: 0;

        &:hover {
          color: var(--el-color-primary);
        }
      }
    }

    &.is-user {
      flex-direction: row-reverse;

      .chat-message__body {
        display: flex;
        justify-content: flex-end;
      }

      .chat-message__content {
        max-width: 88%;
        padding: 8px 11px;
        background: var(--nw-primary-soft);
        border-radius: var(--nw-radius-lg) var(--nw-radius-sm) var(--nw-radius-lg)
          var(--nw-radius-lg);
      }
    }
  }
</style>

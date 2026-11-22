<!-- 消息条目：思考过程面板、等待态、流式光标、AI 依据折叠与正文写入操作。 -->
<script setup lang="ts">
  import { computed, ref } from 'vue'
  import { ElMessage } from 'element-plus'
  import { Icon } from '@novelworkbench/components'
  import type { ChatDisplayMessage } from './types'
  import ThinkingPanel from './ThinkingPanel.vue'

  const props = defineProps<{ message: ChatDisplayMessage }>()

  const emit = defineEmits<{
    retry: []
    'insert-to-chapter': [contentText: string]
    'replace-chapter': [contentText: string]
  }>()

  const referenceExpandedFlag = ref(false)

  const isUser = computed(() => props.message.role_code === 'user')
  const isAssistant = computed(() => props.message.role_code === 'assistant')

  /** 生成结束且有正文：展示依据面板与写入操作 */
  const finishedFlag = computed(
    () =>
      isAssistant.value &&
      !props.message.streaming_flag &&
      !props.message.error_message &&
      props.message.content_text.length > 0,
  )

  /** 等待态：流式中且正文与思考都没有内容，此时界面最像卡死 */
  const waitingFlag = computed(
    () =>
      isAssistant.value &&
      props.message.streaming_flag &&
      !props.message.content_text &&
      !props.message.reasoning_text,
  )

  const waitedText = computed(() => (props.message.waited_ms / 1000).toFixed(1))

  const referencePreview = (chunkText: string) =>
    chunkText.length > 120 ? `${chunkText.slice(0, 120)}…` : chunkText

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
      <header class="chat-message__head">
        <span class="chat-message__role">{{ isUser ? '我' : 'AI' }}</span>
        <span v-if="message.skill_code" class="chat-message__skill">/{{ message.skill_code }}</span>
        <span v-if="message.created_at" class="chat-message__time">
          {{ new Date(message.created_at).toLocaleTimeString() }}
        </span>
      </header>

      <ThinkingPanel
        v-if="isAssistant && message.reasoning_text"
        :reasoning-text="message.reasoning_text"
        :streaming_flag="message.streaming_flag"
        :has_content_flag="Boolean(message.content_text)"
      />

      <p v-if="waitingFlag" class="chat-message__waiting">
        <span class="chat-message__dots"><i /><i /><i /></span>
        <span>正在生成，已等待 {{ waitedText }}s</span>
      </p>

      <div v-if="message.content_text" class="chat-message__content">
        {{ message.content_text }}<span
          v-if="message.streaming_flag"
          class="chat-message__cursor"
        />
      </div>

      <p v-if="message.error_message" class="chat-message__error">
        <Icon icon="mdi:alert-circle-outline" :size="13" />
        <span class="chat-message__error-text">{{ message.error_message }}</span>
        <button type="button" class="chat-message__retry" @click="emit('retry')">重试</button>
      </p>

      <div v-if="finishedFlag" class="chat-message__actions">
        <button type="button" class="chat-message__link" @click="referenceExpandedFlag = !referenceExpandedFlag">
          <Icon
            icon="mdi:chevron-right"
            :size="12"
            :class="['chat-message__arrow', { 'is-expanded': referenceExpandedFlag }]"
          />
          AI 依据
          <template v-if="message.reference_list.length > 0">
            ({{ message.reference_list.length }})
          </template>
        </button>
        <span class="chat-message__spacer" />
        <button type="button" class="chat-message__link" @click="copy">复制</button>
        <button
          type="button"
          class="chat-message__link"
          @click="emit('insert-to-chapter', message.content_text)"
        >
          插入到章节
        </button>
        <button
          type="button"
          class="chat-message__link"
          @click="emit('replace-chapter', message.content_text)"
        >
          替换整章
        </button>
      </div>

      <div v-if="finishedFlag && referenceExpandedFlag" class="chat-message__refs">
        <p v-if="message.reference_list.length === 0" class="chat-message__refs-empty">
          本次回复未召回知识库片段；召回基于关键词与词法混合检索。
        </p>
        <div v-for="chunk in message.reference_list" :key="chunk.chunk_ulid" class="chat-ref">
          <div class="chat-ref__head">
            <span class="chat-ref__doc" :title="chunk.doc_name">{{ chunk.doc_name }}</span>
            <span class="chat-ref__score">
              {{ chunk.score_value.toFixed(3) }}<template v-if="chunk.source_mode"> / {{ chunk.source_mode }}</template>
            </span>
          </div>
          <p class="chat-ref__text">{{ referencePreview(chunk.chunk_text) }}</p>
        </div>
      </div>
    </div>
  </article>
</template>

<style lang="less" scoped>
  .chat-message {
    display: flex;
    gap: 8px;
    margin-bottom: 12px;
    animation: chat-message-in 0.22s ease;

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

    &__head {
      display: flex;
      gap: 6px;
      align-items: center;
      margin-bottom: 4px;
      font-size: 10.5px;
      color: var(--el-text-color-placeholder);
    }

    &__role {
      font-weight: 700;
      color: var(--el-text-color-secondary);
    }

    &__skill {
      padding: 0 6px;
      font-size: 10px;
      color: var(--el-color-primary);
      background: var(--nw-primary-soft);
      border-radius: 999px;
    }

    &__time {
      margin-left: auto;
      font-variant-numeric: tabular-nums;
    }

    &__content {
      font-size: 13px;
      line-height: 1.75;
      color: var(--nw-ink);
      white-space: pre-wrap;
      word-break: break-word;
    }

    &__cursor {
      display: inline-block;
      width: 2px;
      height: 13px;
      margin-left: 2px;
      vertical-align: text-bottom;
      background: var(--el-color-primary);
      animation: chat-cursor-blink 0.9s step-start infinite;
    }

    &__waiting {
      display: flex;
      gap: 7px;
      align-items: center;
      padding: 3px 0 1px;
      font-size: 11.5px;
      color: var(--el-text-color-placeholder);
      font-variant-numeric: tabular-nums;
    }

    &__dots {
      display: inline-flex;
      gap: 2px;
      align-items: flex-end;
      height: 10px;

      i {
        width: 4px;
        height: 4px;
        background: var(--el-color-primary);
        border-radius: 50%;
        animation: chat-dots-bounce 1s ease-in-out infinite;

        &:nth-child(2) {
          animation-delay: 0.16s;
        }

        &:nth-child(3) {
          animation-delay: 0.32s;
        }
      }
    }

    &__error {
      display: flex;
      gap: 6px;
      align-items: center;
      padding: 6px 9px;
      margin-top: 6px;
      font-size: 11.5px;
      color: var(--nw-danger);
      background: #fef2f2;
      border-radius: var(--nw-radius-base);
    }

    &__error-text {
      flex: 1;
      line-height: 1.5;
    }

    &__retry {
      padding: 0 6px;
      font-size: 12px;
      color: var(--el-color-primary);
      cursor: pointer;
      background: transparent;
      border: 0;
    }

    &__actions {
      display: flex;
      gap: 8px;
      align-items: center;
      padding-top: 7px;
      margin-top: 8px;
      border-top: 1px dashed var(--nw-border-soft);
    }

    &__spacer {
      flex: 1;
    }

    &__link {
      display: inline-flex;
      gap: 2px;
      align-items: center;
      height: 22px;
      padding: 0 4px;
      font-size: 11px;
      color: var(--el-text-color-secondary);
      cursor: pointer;
      background: transparent;
      border: 0;
      border-radius: var(--nw-radius-sm);
      transition:
        color 0.18s ease,
        background-color 0.18s ease;

      &:hover {
        color: var(--el-color-primary);
        background: var(--nw-primary-soft);
      }
    }

    &__arrow {
      transition: transform 0.2s ease;

      &.is-expanded {
        transform: rotate(90deg);
      }
    }

    &__refs {
      padding-top: 8px;
      margin-top: 8px;
      border-top: 1px dashed var(--nw-border-soft);
    }

    &__refs-empty {
      font-size: 11px;
      color: var(--el-text-color-placeholder);
    }

    &.is-user {
      flex-direction: row-reverse;

      .chat-message__body {
        display: flex;
        flex-direction: column;
        align-items: flex-end;
      }

      .chat-message__head {
        flex-direction: row-reverse;
        width: auto;
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

  .chat-ref {
    margin-bottom: 8px;

    &__head {
      display: flex;
      gap: 8px;
      align-items: center;
      justify-content: space-between;
    }

    &__doc {
      overflow: hidden;
      font-size: 11.5px;
      font-weight: 600;
      color: var(--el-color-primary);
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    &__score {
      font-size: 10px;
      color: var(--el-text-color-placeholder);
      white-space: nowrap;
    }

    &__text {
      margin: 2px 0 0;
      font-size: 11.5px;
      line-height: 1.6;
      color: var(--el-text-color-secondary);
      white-space: pre-wrap;
    }
  }

  @keyframes chat-message-in {
    from {
      opacity: 0;
      transform: translateY(4px);
    }

    to {
      opacity: 1;
      transform: none;
    }
  }

  @keyframes chat-cursor-blink {
    50% {
      opacity: 0;
    }
  }

  @keyframes chat-dots-bounce {
    0%,
    60%,
    100% {
      opacity: 0.45;
      transform: translateY(0);
    }

    30% {
      opacity: 1;
      transform: translateY(-4px);
    }
  }
</style>

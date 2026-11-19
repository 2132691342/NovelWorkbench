<!--
  AI 面板单条消息：用户/助手气泡、时间、技能徽标、流式打字光标、
  失败错误气泡 + 重试、done 后的"AI 依据"折叠面板与"插入到章节"。
-->
<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import NWButton from '@/components/NWButton.vue'
import type { NWChatDisplayMessage } from './chatTypes'

const props = defineProps<{
  message: NWChatDisplayMessage
}>()

const emit = defineEmits<{
  retry: []
  'insert-to-chapter': [contentText: string]
}>()

const { t } = useI18n()

const referenceExpanded = ref(false)

/** 是否展示"AI 依据"与"插入到章节"（仅生成结束且有正文的助手消息） */
function showAssistantActions(): boolean {
  return (
    props.message.role_code === 'assistant' &&
    !props.message.streaming_flag &&
    props.message.error_message.length === 0 &&
    props.message.content_text.length > 0
  )
}

/** 切片文本预览（最多 120 字） */
function chunkPreview(chunkText: string): string {
  return chunkText.length > 120 ? `${chunkText.slice(0, 120)}...` : chunkText
}
</script>

<template>
  <div class="chat-message" :class="`chat-message--${message.role_code}`">
    <div class="chat-message__bubble">
      <div class="chat-message__head">
        <span class="chat-message__role">
          {{ message.role_code === 'user' ? t('workspace.ai.roleUser') : t('workspace.ai.roleAssistant') }}
        </span>
        <span v-if="message.skill_code" class="chat-message__skill">
          /{{ message.skill_code }}
        </span>
        <span class="chat-message__time">
          {{ new Date(message.created_at_ms).toLocaleTimeString() }}
        </span>
      </div>

      <div class="chat-message__content">
        {{ message.content_text }}<span v-if="message.streaming_flag" class="chat-message__cursor" />
      </div>

      <div v-if="message.error_message" class="chat-message__error">
        <span class="chat-message__error-text">{{ message.error_message }}</span>
        <NWButton size="small" type="danger" @click="emit('retry')">
          {{ t('workspace.ai.retry') }}
        </NWButton>
      </div>

      <div v-if="showAssistantActions()" class="chat-message__actions">
        <NWButton
          type="text"
          size="small"
          class="chat-message__toggle"
          @click="referenceExpanded = !referenceExpanded"
        >
          <span :class="{ 'is-expanded': referenceExpanded }">></span>
          {{ t('workspace.ai.references') }}
          <template v-if="message.reference_list.length > 0">
            ({{ message.reference_list.length }})
          </template>
        </NWButton>
        <NWButton size="small" @click="emit('insert-to-chapter', message.content_text)">
          {{ t('workspace.ai.insertToChapter') }}
        </NWButton>
      </div>

      <div v-if="showAssistantActions() && referenceExpanded" class="chat-message__references">
        <p
          v-if="message.reference_list.length === 0"
          class="chat-message__references-empty"
        >
          {{ t('workspace.ai.referencesEmpty') }}
        </p>
        <div
          v-for="chunk in message.reference_list"
          :key="chunk.chunk_ulid"
          class="chat-reference"
        >
          <div class="chat-reference__head">
            <span class="chat-reference__doc" :title="chunk.doc_name">{{ chunk.doc_name }}</span>
            <span class="chat-reference__score">
              {{ t('workspace.ai.score') }} {{ chunk.score_value.toFixed(3) }}
              <template v-if="chunk.source_mode"> / {{ chunk.source_mode }}</template>
            </span>
          </div>
          <p class="chat-reference__text">{{ chunkPreview(chunk.chunk_text) }}</p>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.chat-message {
  display: flex;
  margin-bottom: 14px;
}

.chat-message--user {
  justify-content: flex-end;
}

.chat-message--assistant {
  justify-content: flex-start;
}

.chat-message__bubble {
  max-width: 88%;
  padding: 10px 12px;
  border-radius: var(--radius-lg);
  border: 1px solid var(--border-light);
  background-color: var(--bg-card);
}

.chat-message--user .chat-message__bubble {
  background-color: var(--primary-light);
  border-color: var(--primary-light);
}

.chat-message__head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
  font-size: 12px;
  color: var(--text-secondary);
}

.chat-message__role {
  font-weight: 600;
  color: var(--text-main);
}

.chat-message__skill {
  padding: 0 6px;
  border-radius: var(--radius-pill);
  background-color: var(--bg-hover);
  color: var(--primary-active);
}

.chat-message__content {
  font-size: 14px;
  line-height: 1.7;
  color: var(--text-main);
  white-space: pre-wrap;
  word-break: break-word;
}

.chat-message__cursor {
  display: inline-block;
  width: 2px;
  height: 15px;
  margin-left: 2px;
  vertical-align: text-bottom;
  background-color: var(--primary);
  animation: chat-cursor-blink 0.9s step-start infinite;
}

@keyframes chat-cursor-blink {
  50% {
    opacity: 0;
  }
}

.chat-message__error {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 8px;
  padding: 6px 10px;
  border-radius: var(--radius-base);
  background-color: var(--bg-hover);
}

.chat-message__error-text {
  font-size: 13px;
  color: var(--error);
}

.chat-message__actions {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 8px;
}

.chat-message__toggle.nw-button {
  height: 24px;
  padding: 0 4px;
  font-size: 13px;
  color: var(--text-secondary);
}

.chat-message__toggle.nw-button:hover {
  color: var(--primary);
}

.chat-message__toggle span {
  display: inline-block;
  transition: transform var(--transition-base);
}

.chat-message__toggle span.is-expanded {
  transform: rotate(90deg);
}

.chat-message__references {
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px dashed var(--border);
}

.chat-message__references-empty {
  margin: 0;
  font-size: 12px;
  color: var(--text-placeholder);
}

.chat-reference {
  margin-bottom: 8px;
}

.chat-reference__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.chat-reference__doc {
  font-size: 12px;
  font-weight: 600;
  color: var(--primary-active);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.chat-reference__score {
  font-size: 12px;
  color: var(--text-secondary);
  white-space: nowrap;
}

.chat-reference__text {
  margin: 2px 0 0;
  font-size: 12px;
  line-height: 1.6;
  color: var(--text-secondary);
  white-space: pre-wrap;
}
</style>

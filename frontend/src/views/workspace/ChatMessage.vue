<!--
  AI 面板单条消息：用户/助手气泡、时间、技能徽标、流式打字光标、
  失败错误气泡 + 重试、done 后的"AI 依据"折叠面板与"插入到章节"。
-->
<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import NWButton from '@/components/NWButton.vue'
import NWIcon from '@/components/NWIcon.vue'
import type { NWChatDisplayMessage } from './chatTypes'
import ThinkingPanel from './ThinkingPanel.vue'

const props = defineProps<{
  message: NWChatDisplayMessage
}>()

const emit = defineEmits<{
  retry: []
  'insert-to-chapter': [contentText: string]
  'replace-chapter': [contentText: string]
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

/** 是否展示等待提示：仍在流式，且正文与思考都还没有内容（此时界面最容易像卡死） */
function showWaitingHint(): boolean {
  return (
    props.message.role_code === 'assistant' &&
    props.message.streaming_flag &&
    props.message.content_text.length === 0 &&
    props.message.reasoning_text.length === 0
  )
}

/** 已等待秒数（保留一位小数，前 10 秒用 0.x 让用户看到数字在动） */
function waitedSeconds(): string {
  return (props.message.waited_ms / 1000).toFixed(1)
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

      <ThinkingPanel
        v-if="message.role_code === 'assistant' && message.reasoning_text.length > 0"
        :reasoning-text="message.reasoning_text"
        :streaming-flag="message.streaming_flag"
        :has-content-flag="message.content_text.length > 0"
      />

      <div v-if="showWaitingHint()" class="chat-message__waiting">
        <span class="chat-message__waiting-dots"><i /><i /><i /></span>
        <span class="chat-message__waiting-text">
          {{ t('workspace.ai.waitingHint', { seconds: waitedSeconds() }) }}
        </span>
      </div>

      <div class="chat-message__content">
        {{ message.content_text }}<span v-if="message.streaming_flag && message.content_text.length > 0" class="chat-message__cursor" />
      </div>

      <div v-if="message.error_message" class="chat-message__error">
        <span class="chat-message__error-text">{{ message.error_message }}</span>
        <NWButton size="small" @click="emit('retry')">
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
          <NWIcon
            icon_code="chevronRight"
            :size_px="12"
            class="chat-message__toggle-arrow"
            :class="{ 'is-expanded': referenceExpanded }"
          />
          {{ t('workspace.ai.references') }}
          <template v-if="message.reference_list.length > 0">
            ({{ message.reference_list.length }})
          </template>
        </NWButton>
        <NWButton size="small" @click="emit('insert-to-chapter', message.content_text)">
          {{ t('workspace.ai.insertToChapter') }}
        </NWButton>
        <NWButton size="small" @click="emit('replace-chapter', message.content_text)">
          {{ t('workspace.ai.replaceChapter') }}
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
  margin-bottom: 12px;
  animation: chat-message-in 0.22s ease;
}

@keyframes chat-message-in {
  from { opacity: 0; transform: translateY(4px); }
  to { opacity: 1; transform: none; }
}

.chat-message--user {
  justify-content: flex-end;
}

.chat-message--assistant {
  justify-content: flex-start;
}

.chat-message__bubble {
  max-width: 92%;
  padding: 9px 11px;
  border-radius: 12px;
  border: 1px solid var(--border-light);
  background-color: var(--bg-subtle);
}

.chat-message--user .chat-message__bubble {
  border-color: var(--border-accent);
  background: var(--accent-gradient);
}

.chat-message__head {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 4px;
  color: var(--text-placeholder);
  font-size: 10.5px;
}

.chat-message__role {
  color: var(--text-secondary);
  font-weight: 700;
}

.chat-message--assistant .chat-message__role {
  color: var(--primary-active);
}

.chat-message__skill {
  padding: 0 6px;
  border-radius: var(--radius-pill);
  background-color: var(--bg-card);
  color: var(--primary-active);
  font-size: 10px;
}

.chat-message__time {
  margin-left: auto;
}

.chat-message__content {
  color: var(--text-main);
  font-size: 13px;
  line-height: 1.72;
  white-space: pre-wrap;
  word-break: break-word;
}

.chat-message__waiting {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 3px 0 1px;
  color: var(--text-placeholder);
  font-size: 11.5px;
}

.chat-message__waiting-dots {
  display: inline-flex;
  align-items: flex-end;
  gap: 2px;
  height: 10px;
}

.chat-message__waiting-dots i {
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background-color: var(--primary);
  animation: chat-waiting-bounce 1s ease-in-out infinite;
}

.chat-message__waiting-dots i:nth-child(2) {
  animation-delay: 0.16s;
}

.chat-message__waiting-dots i:nth-child(3) {
  animation-delay: 0.32s;
}

@keyframes chat-waiting-bounce {
  0%, 60%, 100% { transform: translateY(0); opacity: 0.45; }
  30% { transform: translateY(-4px); opacity: 1; }
}

.chat-message__waiting-text {
  font-variant-numeric: tabular-nums;
}

.chat-message__cursor {
  display: inline-block;
  width: 2px;
  height: 13px;
  margin-left: 2px;
  vertical-align: text-bottom;
  background-color: var(--primary);
  animation: chat-cursor-blink 0.9s step-start infinite;
}

@keyframes chat-cursor-blink {
  50% { opacity: 0; }
}

.chat-message__error {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 8px;
  padding: 6px 9px;
  border-radius: var(--radius-base);
  background-color: var(--error-bg);
  color: var(--error-text);
}

.chat-message__error-text {
  flex: 1;
  font-size: 11.5px;
  line-height: 1.5;
}

.chat-message__actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-top: 8px;
  padding-top: 7px;
  border-top: 1px dashed var(--border);
}

.chat-message__toggle.nw-button {
  height: 22px;
  padding: 0 4px;
  color: var(--text-placeholder);
  font-size: 11px;
}

.chat-message__toggle.nw-button:hover {
  color: var(--primary);
}

.chat-message__toggle-arrow {
  transition: transform var(--transition-base);
}

.chat-message__toggle-arrow.is-expanded {
  transform: rotate(90deg);
}

.chat-message__insert-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  height: 24px;
  padding: 0 8px;
  border: 1px solid var(--border);
  border-radius: var(--radius-base);
  background: var(--bg-card);
  color: var(--text-secondary);
  font-size: 11px;
  cursor: pointer;
  transition: all var(--transition-base);
}

.chat-message__insert-btn:hover {
  border-color: var(--primary-hover);
  background: var(--primary-soft);
  color: var(--primary);
}

.chat-message__insert-btn:active {
  transform: scale(0.96);
}

.chat-message__references {
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px dashed var(--border);
}

.chat-message__references-empty {
  margin: 0;
  color: var(--text-placeholder);
  font-size: 11px;
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
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--primary-active);
  font-size: 11.5px;
  font-weight: 600;
}

.chat-reference__score {
  color: var(--text-placeholder);
  font-size: 10px;
  white-space: nowrap;
}

.chat-reference__text {
  margin: 2px 0 0;
  color: var(--text-secondary);
  font-size: 11.5px;
  line-height: 1.6;
  white-space: pre-wrap;
}
</style>

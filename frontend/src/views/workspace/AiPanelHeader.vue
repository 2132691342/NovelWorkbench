<!--
  AI 面板头部与控制区：会话切换/新建/删除 + 模型选择 + 当前章节上下文开关。
  纯展示与事件上抛，状态仍由 AiPanel 持有（会话与模型选择是发送请求的一部分）。
-->
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { NWChatSessionVo } from '@/api/contracts'
import type { NWModelOptionVo } from '@/api/model'
import NWIcon from '@/components/NWIcon.vue'

const props = defineProps<{
  sessionList: NWChatSessionVo[]
  currentSessionUlid: string
  sessionDeletableFlag: boolean
  modelOptionList: NWModelOptionVo[]
  selectedModelUlid: string
  currentChapterUlid?: string
  currentChapterName?: string
  attachChapterFlag: boolean
  messageCount: number
}>()

const emit = defineEmits<{
  'update:currentSessionUlid': [sessionUlid: string]
  'update:selectedModelUlid': [modelUlid: string]
  'update:attachChapterFlag': [attachFlag: boolean]
  selectSession: [sessionUlid: string]
  createSession: []
  deleteSession: []
}>()

const { t } = useI18n()

/** 当前选中模型（用于展示协议徽标与提示） */
const selectedModelVo = computed(
  () => props.modelOptionList.find((item) => item.model_ulid === props.selectedModelUlid) ?? null,
)

/** 切换章节上下文开关 */
function toggleAttachChapter(): void {
  emit('update:attachChapterFlag', !props.attachChapterFlag)
}
</script>

<template>
  <header class="ai-panel__header">
    <div class="ai-panel__title-row">
      <span class="ai-panel__title-icon"><NWIcon icon_code="sparkles" :size_px="15" /></span>
      <span class="ai-panel__title">{{ t('workspace.ai.title') }}</span>
      <span v-if="messageCount > 0" class="ai-panel__count">
        {{ t('workspace.ai.messageCount', { count: messageCount }) }}
      </span>
    </div>
    <div class="ai-panel__header-actions">
      <el-select
        :model-value="currentSessionUlid"
        class="ai-panel__session-select"
        size="small"
        :placeholder="t('workspace.ai.sessionPlaceholder')"
        @update:model-value="emit('update:currentSessionUlid', $event)"
        @change="emit('selectSession', $event)"
      >
        <el-option
          v-for="sessionItem in sessionList"
          :key="sessionItem.session_ulid"
          :value="sessionItem.session_ulid"
          :label="sessionItem.session_name || sessionItem.session_ulid"
        />
      </el-select>
      <el-tooltip :content="t('workspace.ai.sessionCreate')" placement="bottom">
        <button type="button" class="ai-panel__icon-btn" @click="emit('createSession')">
          <NWIcon icon_code="plus" :size_px="15" />
        </button>
      </el-tooltip>
      <el-tooltip :content="t('workspace.ai.sessionDelete')" placement="bottom">
        <button
          type="button"
          class="ai-panel__icon-btn is-danger"
          :disabled="!sessionDeletableFlag"
          @click="emit('deleteSession')"
        >
          <NWIcon icon_code="trash" :size_px="14" />
        </button>
      </el-tooltip>
    </div>
  </header>

  <div class="ai-panel__controls">
    <div class="ai-panel__control-row">
      <span class="ai-panel__control-label">{{ t('workspace.ai.modelLabel') }}</span>
      <el-select
        v-if="modelOptionList.length > 0"
        :model-value="selectedModelUlid"
        size="small"
        class="ai-panel__model-select"
        :placeholder="t('workspace.ai.modelPlaceholder')"
        @update:model-value="emit('update:selectedModelUlid', $event)"
      >
        <el-option
          v-for="modelOption in modelOptionList"
          :key="modelOption.model_ulid"
          :value="modelOption.model_ulid"
          :label="`${modelOption.model_label || modelOption.model_name} · ${modelOption.provider_name}`"
        >
          <span class="ai-panel__model-option">
            <span class="ai-panel__model-option-name">{{ modelOption.model_label || modelOption.model_name }}</span>
            <span class="ai-panel__model-option-meta">{{ modelOption.provider_name }}</span>
          </span>
        </el-option>
      </el-select>
      <span v-else class="ai-panel__model-empty">{{ t('workspace.ai.modelUnavailable') }}</span>
      <el-tooltip v-if="selectedModelVo" placement="bottom">
        <template #content>
          <div>{{ t('workspace.ai.modelTooltip') }}</div>
          <div>{{ t('workspace.ai.modelTooltipMeta', {
            provider: selectedModelVo.provider_name,
            tokens: selectedModelVo.max_tokens,
          }) }}</div>
        </template>
        <span class="ai-panel__model-badge">{{ selectedModelVo.protocol_code }}</span>
      </el-tooltip>
    </div>

    <div class="ai-panel__control-row">
      <span class="ai-panel__control-label">{{ t('workspace.ai.contextLabel') }}</span>
      <button
        type="button"
        class="ai-panel__context-chip"
        :class="{ 'is-active': attachChapterFlag }"
        :disabled="!currentChapterUlid"
        @click="toggleAttachChapter"
      >
        <span class="ai-panel__context-dot" />
        {{ currentChapterName || t('workspace.ai.noChapter') }}
      </button>
      <span class="ai-panel__context-hint">
        {{ attachChapterFlag ? t('workspace.ai.contextOn') : t('workspace.ai.contextOff') }}
      </span>
    </div>
  </div>
</template>

<style scoped src="./AiPanelHeader.css"></style>

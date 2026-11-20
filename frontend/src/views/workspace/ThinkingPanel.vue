<!--
  思考过程面板：推理模型的 reasoning 流在生成期间实时展开，正文开始后自动收起为一行摘要。
  等待期没有正文也没有思考时展示「已连接，正在等待模型首个字节」，避免界面像卡死。
-->
<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import NWIcon from '@/components/NWIcon.vue'

const props = defineProps<{
  reasoningText: string
  streamingFlag: boolean
  hasContentFlag: boolean
}>()

const { t } = useI18n()

/** 用户是否手动收起/展开（null 表示跟随流式状态自动控制） */
const manualExpanded = ref<boolean | null>(null)
const bodyHost = ref<HTMLDivElement | null>(null)

/** 思考中的判定：仍在流式，且还没有正文 */
const thinkingFlag = computed(() => props.streamingFlag && !props.hasContentFlag)

/** 展开态：生成正文前自动展开；正文出现后自动收起；用户手动操作优先 */
const expandedFlag = computed(() => {
  if (manualExpanded.value !== null) {
    return manualExpanded.value
  }
  return thinkingFlag.value
})

/** 头部摘要：思考中显示已思考字数，结束后显示思考总字数 */
const summaryText = computed(() => {
  const charCount = props.reasoningText.length
  if (thinkingFlag.value) {
    return t('workspace.ai.thinkingActive', { count: charCount })
  }
  return t('workspace.ai.thinkingDone', { count: charCount })
})

/** 思考文本变化时保持滚动到底部（跟随最新一行） */
watch(
  () => props.reasoningText,
  () => {
    if (!expandedFlag.value) {
      return
    }
    void nextTick(() => {
      if (bodyHost.value) {
        bodyHost.value.scrollTop = bodyHost.value.scrollHeight
      }
    })
  },
)

/** 正文开始出现时清空手动状态，回到自动收起 */
watch(
  () => props.hasContentFlag,
  (hasContent) => {
    if (hasContent) {
      manualExpanded.value = null
    }
  },
)

/** 切换展开状态 */
function toggleExpanded(): void {
  manualExpanded.value = !expandedFlag.value
}
</script>

<template>
  <div v-if="reasoningText.length > 0" class="chat-thinking" :class="{ 'is-active': thinkingFlag }">
    <button type="button" class="chat-thinking__head" @click="toggleExpanded">
      <span class="chat-thinking__pulse" :class="{ 'is-running': thinkingFlag }" />
      <span class="chat-thinking__label">{{ t('workspace.ai.thinkingLabel') }}</span>
      <span class="chat-thinking__summary">{{ summaryText }}</span>
      <NWIcon
        icon_code="chevronRight"
        :size_px="12"
        class="chat-thinking__arrow"
        :class="{ 'is-expanded': expandedFlag }"
      />
    </button>
    <div v-if="expandedFlag" ref="bodyHost" class="chat-thinking__body">
      {{ reasoningText }}<span v-if="thinkingFlag" class="chat-thinking__cursor" />
    </div>
  </div>
</template>

<style scoped src="./ThinkingPanel.css"></style>

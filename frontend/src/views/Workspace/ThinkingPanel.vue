<!-- 思考过程面板：推理模型的推理流在生成期间展开，正文出现后自动收起为一行摘要。 -->
<script setup lang="ts">
  import { computed, nextTick, ref, watch } from 'vue'
  import { Icon } from '@novelworkbench/components'

  const props = defineProps<{
    reasoningText: string
    streaming_flag: boolean
    has_content_flag: boolean
  }>()

  /** 用户手动展开/收起（null 表示跟随流式状态自动控制） */
  const manualExpandedFlag = ref<boolean | null>(null)
  const bodyHost = ref<HTMLDivElement | null>(null)

  /** 思考中：仍在流式且尚无正文 */
  const thinkingFlag = computed(() => props.streaming_flag && !props.has_content_flag)

  /** 展开态：正文出现前自动展开，正文出现后自动收起，手动操作优先 */
  const expandedFlag = computed(() => manualExpandedFlag.value ?? thinkingFlag.value)

  const summaryText = computed(() =>
    thinkingFlag.value ? `思考中 · ${props.reasoningText.length} 字` : `已思考 ${props.reasoningText.length} 字`,
  )

  watch(
    () => props.reasoningText,
    () => {
      if (!expandedFlag.value) return
      void nextTick(() => {
        if (bodyHost.value) bodyHost.value.scrollTop = bodyHost.value.scrollHeight
      })
    },
  )

  watch(
    () => props.has_content_flag,
    (hasContent) => {
      if (hasContent) manualExpandedFlag.value = null
    },
  )

  const toggleExpanded = () => {
    manualExpandedFlag.value = !expandedFlag.value
  }
</script>

<template>
  <section v-if="reasoningText" :class="['thinking', { 'is-active': thinkingFlag }]">
    <button type="button" class="thinking__head" @click="toggleExpanded">
      <span :class="['thinking__pulse', { 'is-running': thinkingFlag }]" />
      <span class="thinking__label">思考过程</span>
      <span class="thinking__summary">{{ summaryText }}</span>
      <Icon
        icon="mdi:chevron-right"
        :size="12"
        :class="['thinking__arrow', { 'is-expanded': expandedFlag }]"
      />
    </button>
    <div v-if="expandedFlag" ref="bodyHost" class="thinking__body">
      {{ reasoningText }}<span v-if="thinkingFlag" class="thinking__cursor" />
    </div>
  </section>
</template>

<style lang="less" scoped>
  .thinking {
    margin-bottom: 8px;
    overflow: hidden;
    background: var(--nw-surface-subtle);
    border: 1px solid var(--nw-border-soft);
    border-radius: var(--nw-radius-md);
    transition:
      border-color 0.2s ease,
      box-shadow 0.2s ease;

    &.is-active {
      border-color: var(--nw-primary-light);
      box-shadow: 0 0 0 2px var(--nw-primary-soft);
    }

    &__head {
      display: flex;
      gap: 6px;
      align-items: center;
      width: 100%;
      padding: 6px 9px;
      font-size: 11.5px;
      color: var(--el-text-color-secondary);
      text-align: left;
      cursor: pointer;
      background: transparent;
      border: 0;
      transition:
        background-color 0.2s ease,
        color 0.2s ease;

      &:hover {
        color: var(--el-color-primary);
        background: var(--nw-primary-soft);
      }
    }

    &__pulse {
      flex: none;
      width: 7px;
      height: 7px;
      background: var(--el-text-color-placeholder);
      border-radius: 50%;

      &.is-running {
        background: var(--el-color-primary);
        animation: thinking-pulse 1.1s ease-in-out infinite;
      }
    }

    &__label {
      font-weight: 650;
      color: var(--nw-ink);
    }

    &.is-active &__label {
      color: var(--el-color-primary);
    }

    &__summary {
      font-size: 10.5px;
      color: var(--el-text-color-placeholder);
    }

    &__arrow {
      margin-left: auto;
      transition: transform 0.2s ease;

      &.is-expanded {
        transform: rotate(90deg);
      }
    }

    &__body {
      max-height: 180px;
      padding: 7px 10px 8px;
      overflow-y: auto;
      font-size: 11.5px;
      line-height: 1.7;
      color: var(--el-text-color-secondary);
      white-space: pre-wrap;
      word-break: break-word;
      border-top: 1px dashed var(--nw-border-soft);
    }

    &__cursor {
      display: inline-block;
      width: 2px;
      height: 11px;
      margin-left: 2px;
      vertical-align: text-bottom;
      background: var(--el-color-primary);
      animation: thinking-blink 0.9s step-start infinite;
    }
  }

  @keyframes thinking-pulse {
    0%,
    100% {
      opacity: 1;
      box-shadow: 0 0 0 0 var(--nw-primary-soft);
    }

    50% {
      opacity: 0.55;
      box-shadow: 0 0 0 4px transparent;
    }
  }

  @keyframes thinking-blink {
    50% {
      opacity: 0;
    }
  }
</style>

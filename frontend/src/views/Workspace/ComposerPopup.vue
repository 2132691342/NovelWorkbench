<!-- 输入区弹层：斜杠/井号命令候选与 @ 引用选择器（同一定位层，互斥展示）。 -->
<script setup lang="ts">
  import type { ChatCommandOption, ChatRefItem, ChatRefTabCode } from './types'

  /** 命令候选（技能与工具归一后的形状，类型定义见 types.ts） */
  type CommandOptionItem = ChatCommandOption

  defineProps<{
    commandKind: 'slash' | 'hash' | ''
    commandOptionList: CommandOptionItem[]
    commandActiveIndex: number
    refPickerVisible: boolean
    refTabs: Array<{ code: ChatRefTabCode; label: string }>
    refPickerTab: ChatRefTabCode
    refCandidateList: ChatRefItem[]
    refSelectedUlidList: string[]
  }>()

  const emit = defineEmits<{
    'select-command': [option: CommandOptionItem]
    'hover-command': [index: number]
    'switch-tab': [tabCode: ChatRefTabCode]
    'select-ref': [item: ChatRefItem]
  }>()
</script>

<template>
  <div v-if="commandKind" class="composer__popup">
    <p class="composer__popup-title">
      {{ commandKind === 'slash' ? '技能' : '工具（服务端确定性执行，不消耗模型）' }}
    </p>
    <button
      v-for="(option, index) in commandOptionList"
      :key="option.code"
      type="button"
      :class="[
        'composer__item',
        { 'is-active': index === commandActiveIndex, 'is-picked': option.active }
      ]"
      @mouseenter="emit('hover-command', index)"
      @click="emit('select-command', option)"
    >
      <strong>{{ commandKind === 'slash' ? '/' : '#' }}{{ option.code }}</strong>
      <span>{{ option.title }}</span>
      <small>{{ option.desc }}</small>
    </button>
    <p v-if="commandOptionList.length === 0" class="composer__empty">
      {{ commandKind === 'slash' ? '没有匹配的技能' : '没有匹配的工具' }}
    </p>
  </div>

  <div v-if="refPickerVisible" class="composer__popup">
    <div class="composer__tabs">
      <button
        v-for="tab in refTabs"
        :key="tab.code"
        type="button"
        :class="['composer__tab', { 'is-active': refPickerTab === tab.code }]"
        @click="emit('switch-tab', tab.code)"
      >
        {{ tab.label }}
      </button>
    </div>
    <button
      v-for="item in refCandidateList"
      :key="item.ref_ulid"
      type="button"
      :class="['composer__item', { 'is-active': refSelectedUlidList.includes(item.ref_ulid) }]"
      @click="emit('select-ref', item)"
    >
      <strong>{{ item.ref_name }}</strong>
      <small>{{ item.ref_desc }}</small>
    </button>
    <p v-if="refCandidateList.length === 0" class="composer__empty">暂无可引用内容</p>
  </div>
</template>

<style lang="less" scoped>
  .composer__popup {
    position: absolute;
    right: 0;
    bottom: calc(100% + 6px);
    left: 0;
    z-index: 20;
    max-height: 260px;
    padding: 6px;
    overflow-y: auto;
    background: var(--el-bg-color-overlay);
    border: 1px solid var(--nw-border);
    border-radius: var(--nw-radius-lg);
    box-shadow: 0 14px 36px rgb(15 23 42 / 14%);
  }

  .composer__popup-title {
    padding: 4px 8px 6px;
    font-size: 11px;
    color: var(--el-text-color-placeholder);
    border-bottom: 1px solid var(--nw-border-soft);
  }

  .composer__item {
    display: flex;
    flex-direction: column;
    gap: 2px;
    width: 100%;
    padding: 7px 9px;
    text-align: left;
    cursor: pointer;
    background: transparent;
    border: 0;
    border-radius: var(--nw-radius-base);

    strong {
      font-size: 12.5px;
      color: var(--nw-ink);
    }

    span {
      font-size: 12px;
      color: var(--el-text-color-regular);
    }

    small {
      overflow: hidden;
      font-size: 11px;
      color: var(--el-text-color-placeholder);
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    &:hover,
    &.is-active {
      background: var(--nw-primary-soft);
    }

    &.is-picked strong {
      color: var(--el-color-primary);
    }
  }

  .composer__empty {
    padding: 10px;
    font-size: 12px;
    color: var(--el-text-color-placeholder);
    text-align: center;
  }

  .composer__tabs {
    display: flex;
    gap: 4px;
    padding: 2px 2px 6px;
    border-bottom: 1px solid var(--nw-border-soft);
  }

  .composer__tab {
    padding: 3px 9px;
    font-size: 12px;
    color: var(--el-text-color-secondary);
    cursor: pointer;
    background: transparent;
    border: 0;
    border-radius: 999px;

    &.is-active {
      color: var(--el-color-primary);
      background: var(--nw-primary-soft);
    }
  }
</style>

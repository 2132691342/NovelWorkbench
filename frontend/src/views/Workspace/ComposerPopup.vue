<!-- 输入区弹层：斜杠（本地动作 + 指令提示词）、井号工具与 @ 引用选择器，同一定位层互斥展示。 -->
<script setup lang="ts">
  import type { ChatCommandOption, ChatRefItem, ChatRefTabCode } from './types'

  defineProps<{
    commandKind: 'slash' | 'hash' | ''
    commandOptionList: ChatCommandOption[]
    commandActiveIndex: number
    refPickerVisible: boolean
    refTabs: Array<{ code: ChatRefTabCode; label: string }>
    refPickerTab: ChatRefTabCode
    refCandidateList: ChatRefItem[]
    refSelectedUlidList: string[]
    refActiveIndex: number
  }>()

  const emit = defineEmits<{
    'select-command': [option: ChatCommandOption]
    'hover-command': [index: number]
    'switch-tab': [tabCode: ChatRefTabCode]
    'select-ref': [item: ChatRefItem]
    'hover-ref': [index: number]
  }>()

  /** 命令来源标签：让用户一眼看出哪条会耗模型、哪条只是本地开关 */
  const sourceLabel = (option: ChatCommandOption): string => {
    if (option.source_code === 'action') return '本地'
    if (option.source_code === 'tool') return '工具'
    return '注入'
  }

  /** 来源标题：动作、提示词与工具混排时只在切换处插一条分隔 */
  const SOURCE_TITLE_MAP: Record<ChatCommandOption['source_code'], string> = {
    action: '本地动作（立即执行，不消耗模型）',
    prompt: '指令提示词（以系统级消息注入本轮）',
    tool: '工具（服务端确定性执行，不消耗模型）'
  }

  const groupTitle = (index: number, optionList: ChatCommandOption[]): string => {
    if (index > 0 && optionList[index].source_code === optionList[index - 1].source_code) return ''
    return SOURCE_TITLE_MAP[optionList[index].source_code]
  }

  /** 引用来源标签：聚合页签下必须标出来源，否则同名条目无法区分 */
  const REF_SOURCE_LABEL_MAP: Record<ChatRefTabCode, string> = {
    all: '全部',
    doc: '文档',
    chapter: '章节',
    summary: '摘要',
    prompt: '提示词',
    message: '消息'
  }
</script>

<template>
  <div v-if="commandKind" class="composer__popup">
    <p class="composer__popup-title">
      {{ commandKind === 'slash' ? '↑↓ 选择，Enter/Tab 确认，Esc 关闭' : '↑↓ 选择工具，可多选' }}
    </p>
    <template v-for="(option, index) in commandOptionList" :key="option.source_code + option.code">
      <p v-if="groupTitle(index, commandOptionList)" class="composer__group">
        {{ groupTitle(index, commandOptionList) }}
      </p>
      <button
        type="button"
        :class="[
          'composer__item',
          { 'is-active': index === commandActiveIndex, 'is-picked': option.active }
        ]"
        @mouseenter="emit('hover-command', index)"
        @click="emit('select-command', option)"
      >
        <span class="composer__item-head">
          <strong>{{ commandKind === 'slash' ? '/' : '#' }}{{ option.code }}</strong>
          <span class="composer__badge" :class="`is-${option.source_code}`">
            {{ sourceLabel(option) }}
          </span>
          <span class="composer__item-title">{{ option.title }}</span>
        </span>
        <small>{{ option.desc }}</small>
      </button>
    </template>
    <p v-if="commandOptionList.length === 0" class="composer__empty">
      {{ commandKind === 'slash' ? '没有匹配的命令或提示词' : '没有匹配的工具' }}
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
      <span class="composer__tabs-hint">← → 切页签，↑ ↓ 选择，Enter 确认</span>
    </div>
    <button
      v-for="(item, index) in refCandidateList"
      :key="item.ref_ulid"
      type="button"
      :class="[
        'composer__item',
        { 'is-active': index === refActiveIndex, 'is-picked': refSelectedUlidList.includes(item.ref_ulid) }
      ]"
      @mouseenter="emit('hover-ref', index)"
      @click="emit('select-ref', item)"
    >
      <span class="composer__item-head">
        <strong>{{ item.ref_name }}</strong>
        <span v-if="refPickerTab === 'all'" class="composer__badge">{{ REF_SOURCE_LABEL_MAP[item.ref_tab_code] }}</span>
        <span v-if="refSelectedUlidList.includes(item.ref_ulid)" class="composer__badge is-picked">已选</span>
      </span>
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
    max-height: 300px;
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

  .composer__group {
    padding: 8px 8px 3px;
    font-size: 10.5px;
    font-weight: 600;
    color: var(--el-text-color-placeholder);
    letter-spacing: 0.02em;
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

  .composer__item-head {
    display: flex;
    gap: 6px;
    align-items: center;
    min-width: 0;
  }

  .composer__item-title {
    overflow: hidden;
    font-size: 12px;
    color: var(--el-text-color-regular);
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .composer__badge {
    flex: none;
    padding: 0 5px;
    font-size: 10px;
    line-height: 15px;
    color: var(--el-text-color-secondary);
    background: var(--nw-surface-subtle);
    border-radius: 999px;

    &.is-action {
      color: var(--el-color-success);
      background: var(--el-color-success-light-9);
    }

    &.is-tool {
      color: var(--el-color-warning);
      background: var(--el-color-warning-light-9);
    }

    &.is-prompt {
      color: var(--el-color-primary);
      background: var(--nw-primary-soft);
    }

    &.is-picked {
      color: var(--el-color-primary);
      background: var(--nw-primary-soft);
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
    flex-wrap: wrap;
    gap: 4px;
    align-items: center;
    padding: 2px 2px 6px;
    border-bottom: 1px solid var(--nw-border-soft);
  }

  .composer__tabs-hint {
    margin-left: auto;
    font-size: 10px;
    color: var(--el-text-color-placeholder);
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

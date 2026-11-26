<!-- 输入区弹层：斜杠命令（本地动作 + 指令提示词）与 @ 引用选择器，同一定位层互斥展示。 -->
<script setup lang="ts">
  import { ref, watch } from 'vue'
  import type { ChatCommandOption, ChatRefItem, ChatRefTabCode } from './types'

  const props = defineProps<{
    commandVisible: boolean
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

  const commandPopupRef = ref<HTMLDivElement | null>(null)
  const refPopupRef = ref<HTMLDivElement | null>(null)

  /**
   * 键盘移动选中项时把该项滚进可视区：弹层最高 300px，候选多时键盘选中的项会落到视野外，
   * 用户只能靠鼠标滚轮找，这正是「上下键选不动」的根因。
   */
  const scrollActiveIntoView = (host: HTMLDivElement | null, activeIndex: number) => {
    if (!host) return
    const itemList = host.querySelectorAll<HTMLElement>(".composer__item")
    const target = itemList[activeIndex]
    if (!target) return
    const hostRect = host.getBoundingClientRect()
    const targetRect = target.getBoundingClientRect()
    if (targetRect.top < hostRect.top) {
      host.scrollTop -= hostRect.top - targetRect.top + 6
    } else if (targetRect.bottom > hostRect.bottom) {
      host.scrollTop += targetRect.bottom - hostRect.bottom + 6
    }
  }

  watch(
    () => [props.commandVisible, props.commandActiveIndex] as const,
    () => {
      if (props.commandVisible) scrollActiveIntoView(commandPopupRef.value, props.commandActiveIndex)
    },
    { flush: "post" }
  )

  watch(
    () => [props.refPickerVisible, props.refActiveIndex, props.refPickerTab] as const,
    () => {
      if (props.refPickerVisible) scrollActiveIntoView(refPopupRef.value, props.refActiveIndex)
    },
    { flush: "post" }
  )

  /** 命令来源标签：让用户一眼看出哪条会耗模型、哪条只是本地开关 */
  const sourceLabel = (option: ChatCommandOption): string =>
    option.source_code === 'action' ? '本地' : '注入'

  /** 来源标题：动作与提示词混排时只在切换处插一条分隔 */
  const SOURCE_TITLE_MAP: Record<ChatCommandOption['source_code'], string> = {
    action: '本地动作（立即执行，不消耗模型）',
    prompt: '指令提示词（以系统级消息注入本轮）'
  }

  const groupTitle = (index: number, optionList: ChatCommandOption[]): string => {
    if (index > 0 && optionList[index].source_code === optionList[index - 1].source_code) return ''
    return SOURCE_TITLE_MAP[optionList[index].source_code]
  }

  /** 引用来源标签：聚合页签下必须标出来源，否则同名条目无法区分 */
  const REF_SOURCE_LABEL_MAP: Record<ChatRefTabCode, string> = {
    all: '全部',
    character: '角色',
    lore: '设定',
    chapter: '章节',
    summary: '摘要',
    doc: '文档',
    session: '会话'
  }
</script>

<template>
  <div v-if="commandVisible" ref="commandPopupRef" class="composer__popup">
    <p class="composer__popup-title">↑↓ 选择，Enter/Tab 确认，Esc 关闭</p>
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
          <strong>/{{ option.code }}</strong>
          <span class="composer__badge" :class="`is-${option.source_code}`">
            {{ sourceLabel(option) }}
          </span>
          <span class="composer__item-title">{{ option.title }}</span>
        </span>
        <small>{{ option.desc }}</small>
      </button>
    </template>
    <p v-if="commandOptionList.length === 0" class="composer__empty">没有匹配的命令或提示词</p>
  </div>

  <div v-if="refPickerVisible" ref="refPopupRef" class="composer__popup">
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
    margin: 0;
    font-size: 11.5px;
    color: var(--el-text-color-secondary);
  }

  .composer__group {
    padding: 8px 8px 2px;
    margin: 0;
    font-size: 11px;
    font-weight: 600;
    color: var(--el-text-color-secondary);
  }

  .composer__empty {
    padding: 10px 8px;
    margin: 0;
    font-size: 12px;
    color: var(--el-text-color-secondary);
  }

  .composer__item {
    display: block;
    width: 100%;
    padding: 6px 8px;
    text-align: left;
    cursor: pointer;
    background: transparent;
    border: 0;
    border-radius: var(--nw-radius-sm);
    transition: background 0.15s ease;

    &:hover,
    &.is-active {
      background: var(--nw-surface-subtle);
    }

    &.is-picked {
      background: var(--nw-primary-soft);
    }
  }

  .composer__item-head {
    display: flex;
    gap: 6px;
    align-items: center;
    font-size: 12.5px;

    strong {
      font-family: var(--nw-font-mono);
      font-weight: 600;
      color: var(--el-color-primary);
    }
  }

  .composer__item-title {
    color: var(--nw-ink);
  }

  .composer__item small {
    display: block;
    margin-top: 2px;
    font-size: 11.5px;
    line-height: 1.5;
    color: var(--el-text-color-secondary);
  }

  .composer__badge {
    flex: none;
    padding: 0 5px;
    font-size: 10.5px;
    line-height: 16px;
    color: var(--el-text-color-secondary);
    background: var(--nw-surface-subtle);
    border: 1px solid var(--nw-border-soft);
    border-radius: 4px;

    &.is-action {
      color: var(--el-color-success);
    }

    &.is-prompt {
      color: var(--el-color-primary);
    }

    &.is-picked {
      color: var(--el-color-primary);
      background: var(--nw-primary-soft);
    }
  }

  .composer__tabs {
    display: flex;
    gap: 4px;
    align-items: center;
    padding: 2px 4px 6px;
    overflow-x: auto;
  }

  .composer__tab {
    flex: none;
    padding: 2px 8px;
    font-size: 11.5px;
    color: var(--el-text-color-secondary);
    cursor: pointer;
    background: transparent;
    border: 1px solid var(--nw-border-soft);
    border-radius: 10px;
    transition: all 0.15s ease;

    &.is-active {
      color: var(--el-color-primary);
      background: var(--nw-primary-soft);
      border-color: var(--el-color-primary);
    }
  }

  .composer__tabs-hint {
    margin-left: auto;
    font-size: 10.5px;
    color: var(--el-text-color-placeholder);
    white-space: nowrap;
  }
</style>

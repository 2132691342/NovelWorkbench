<!-- 对话输入区：/ 唤起斜杠命令（本地动作 + 指令提示词），@ 引用素材库与历史会话。
     工具由模型自行决定是否调用，界面不提供手动点名入口。 -->
<script setup lang="ts">
  import { computed, nextTick, ref, watch } from 'vue'
  import type { NWPromptVo } from '@/api/contracts'
  import type { ChatCommandOption, ChatRefItem, ChatSendPayload } from './types'
  import { useChatRefs, REF_TAB_LIST } from './useChatRefs'
  import ComposerPopup from './ComposerPopup.vue'
  import {
    filterActions,
    matchActionByCode,
    parseLeadingSlashToken
  } from './composerCommands'
  import type { NWChapterBriefVo } from '@/api/contracts'

  const props = defineProps<{
    projectUlid: string
    sessionUlid: string
    chapterList: NWChapterBriefVo[]
    promptList: NWPromptVo[]
    systemPromptList: NWPromptVo[]
    streaming: boolean
    canSend: boolean
  }>()

  const emit = defineEmits<{
    send: [payload: ChatSendPayload]
    stop: []
    /** 斜杠动作：本地动作交由外层执行（面板或写作台），不进入模型调用 */
    action: [actionCode: string]
  }>()

  const inputText = ref('')
  const promptCode = ref('')
  const systemPromptCode = ref('')
  const inputRef = ref<HTMLTextAreaElement | null>(null)
  const commandVisible = ref(false)
  const commandFilter = ref('')
  const commandActiveIndex = ref(0)
  const refActiveIndex = ref(0)
  /** @ 触发词是否仍在光标前：弹层收起与方向键归属都依赖它 */
  const atTokenActive = ref(false)

  const {
    refCharacterList,
    refLoreList,
    refChapterList,
    refSummaryList,
    refDocList,
    refSessionList,
    refPickerVisible,
    refPickerTab,
    refPickerFilter,
    refPickerPinned,
    refCandidateList,
    refSelectedUlidList,
    switchTab,
    togglePicker,
    closePicker,
    toggleRefItem,
    removeRefItem,
    clearRefs,
    toRefPayload,
    pushChapterRef
  } = useChatRefs({
    projectUlid: () => props.projectUlid,
    chapterList: () => props.chapterList,
    sessionUlid: () => props.sessionUlid
  })

  /** 斜杠候选：本地动作在前、指令提示词在后，同一弹层里按来源分组 */
  const slashOptionList = computed<ChatCommandOption[]>(() => {
    const actionOptions = filterActions(commandFilter.value).map((action) => ({
      code: action.action_code,
      title: action.action_name,
      desc: action.action_desc,
      active: false,
      source_code: 'action' as const,
      icon: action.action_icon
    }))
    const promptOptions = props.promptList
      .filter(
        (prompt) =>
          prompt.prompt_code.toLowerCase().includes(commandFilter.value.toLowerCase()) ||
          prompt.prompt_name.toLowerCase().includes(commandFilter.value.toLowerCase())
      )
      .map((prompt) => ({
        code: prompt.prompt_code,
        title: prompt.prompt_name,
        desc: prompt.description_text,
        active: promptCode.value === prompt.prompt_code,
        source_code: 'prompt' as const,
        icon: ''
      }))
    return [...actionOptions, ...promptOptions]
  })

  const chipList = computed(() => [
    ...refCharacterList.value.map((item) => ({ tab: 'character' as const, label: '角色', ...item })),
    ...refLoreList.value.map((item) => ({ tab: 'lore' as const, label: '设定', ...item })),
    ...refChapterList.value.map((item) => ({ tab: 'chapter' as const, label: '章节', ...item })),
    ...refSummaryList.value.map((item) => ({ tab: 'summary' as const, label: '摘要', ...item })),
    ...refDocList.value.map((item) => ({ tab: 'doc' as const, label: '文档', ...item })),
    ...refSessionList.value.map((item) => ({ tab: 'session' as const, label: '会话', ...item }))
  ])

  const refTabs = REF_TAB_LIST

  const SLASH_TOKEN_REGEXP = /(^|\s)\/([^\s]*)$/
  const AT_TOKEN_REGEXP = /(^|\s)@([^\s]*)$/

  /** 删除光标前的 /xxx 或 @xxx 触发词 */
  const removeTriggerToken = (tokenRegexp: RegExp) => {
    const cursorPos = inputRef.value?.selectionStart ?? inputText.value.length
    const beforeCursor = inputText.value.slice(0, cursorPos)
    inputText.value = beforeCursor.replace(tokenRegexp, '$1') + inputText.value.slice(cursorPos)
  }

  const handleInput = () => {
    const cursorPos = inputRef.value?.selectionStart ?? inputText.value.length
    const beforeCursor = inputText.value.slice(0, cursorPos)

    const slashMatch = SLASH_TOKEN_REGEXP.exec(beforeCursor)
    if (slashMatch) {
      commandVisible.value = true
      commandFilter.value = slashMatch[2]
    } else {
      commandVisible.value = false
      commandFilter.value = ''
    }
    commandActiveIndex.value = 0

    const atMatch = AT_TOKEN_REGEXP.exec(beforeCursor)
    if (atMatch) {
      refPickerFilter.value = atMatch[2]
      refPickerVisible.value = true
      atTokenActive.value = true
      refActiveIndex.value = 0
      switchTab(refPickerTab.value)
    } else {
      // 触发词消失即收起：否则弹层会一直盖在输入框上方，键盘导航还会抢走方向键
      atTokenActive.value = false
      if (!refPickerPinned.value) refPickerVisible.value = false
      refPickerFilter.value = ''
    }
  }

  /** 选中斜杠候选项：动作立即执行，提示词记为徽标 */
  const selectCommand = (option: ChatCommandOption) => {
    commandVisible.value = false
    commandFilter.value = ''
    if (option.source_code === 'action') {
      removeTriggerToken(SLASH_TOKEN_REGEXP)
      runAction(option.code)
      inputRef.value?.focus()
      return
    }
    promptCode.value = option.code
    removeTriggerToken(SLASH_TOKEN_REGEXP)
    inputRef.value?.focus()
  }

  /** 本地动作：能就地处理的就地处理，其余交给外层（面板/写作台） */
  const runAction = (actionCode: string) => {
    if (actionCode === 'clear') {
      clearComposer()
      return
    }
    emit('action', actionCode)
  }

  const selectRef = (item: ChatRefItem) => {
    toggleRefItem(item)
    removeTriggerToken(AT_TOKEN_REGEXP)
    atTokenActive.value = false
    refPickerFilter.value = ''
    // 由 @ 触发词唤起且未钉住时，选完即收起；钉住的弹层留着方便连续多选
    if (!refPickerPinned.value) refPickerVisible.value = false
    inputRef.value?.focus()
  }

  const clearComposer = () => {
    inputText.value = ''
    promptCode.value = ''
    commandVisible.value = false
    commandFilter.value = ''
    atTokenActive.value = false
    clearRefs()
    closePicker()
  }

  const submit = () => {
    const contentText = inputText.value.trim()
    // 正文以 /动作 开头时直接执行动作（不经过模型）；提示词编码则照常走发送链路
    const slashToken = parseLeadingSlashToken(contentText)
    if (slashToken && matchActionByCode(slashToken.actionCode)) {
      inputText.value = slashToken.remainderText
      runAction(slashToken.actionCode)
      return
    }
    if (!props.canSend) return
    emit('send', {
      content_text: contentText,
      prompt_code: promptCode.value,
      tool_code_list: [],
      ...toRefPayload(),
      system_prompt_code: systemPromptCode.value
    })
    // 发送后只清正文与引用，身份提示词与模型是会话级偏好，保持不变
    inputText.value = ''
    promptCode.value = ''
    commandVisible.value = false
    clearRefs()
    closePicker()
  }

  /** 引用选择器的键盘操作：仅当光标前仍有 @ 触发词时接管方向键，
   * 否则（点「@ 引用」钉住的弹层）方向键应留给正文光标。 */
  const handleRefKeydown = (event: KeyboardEvent): boolean => {
    if (!refPickerVisible.value || !atTokenActive.value) return false
    if (event.key === 'ArrowRight' || event.key === 'ArrowLeft') {
      event.preventDefault()
      const currentIndex = refTabs.findIndex((tab) => tab.code === refPickerTab.value)
      const delta = event.key === 'ArrowRight' ? 1 : -1
      const nextIndex = (currentIndex + delta + refTabs.length) % refTabs.length
      switchTab(refTabs[nextIndex].code)
      refActiveIndex.value = 0
      return true
    }
    const candidateList = refCandidateList.value
    if (candidateList.length === 0) return false
    if (event.key === 'ArrowDown') {
      event.preventDefault()
      refActiveIndex.value = (refActiveIndex.value + 1) % candidateList.length
      return true
    }
    if (event.key === 'ArrowUp') {
      event.preventDefault()
      refActiveIndex.value = (refActiveIndex.value - 1 + candidateList.length) % candidateList.length
      return true
    }
    if (event.key === 'Enter' || event.key === 'Tab') {
      event.preventDefault()
      const target = candidateList[Math.min(refActiveIndex.value, candidateList.length - 1)]
      if (target) selectRef(target)
      return true
    }
    return false
  }

  const handleKeydown = (event: KeyboardEvent) => {
    if (commandVisible.value && slashOptionList.value.length > 0) {
      const optionCount = slashOptionList.value.length
      if (event.key === 'ArrowDown') {
        event.preventDefault()
        commandActiveIndex.value = (commandActiveIndex.value + 1) % optionCount
        return
      }
      if (event.key === 'ArrowUp') {
        event.preventDefault()
        commandActiveIndex.value = (commandActiveIndex.value - 1 + optionCount) % optionCount
        return
      }
      if (event.key === 'Enter' || event.key === 'Tab') {
        event.preventDefault()
        selectCommand(slashOptionList.value[commandActiveIndex.value])
        return
      }
    }
    if (handleRefKeydown(event)) return
    if (event.key === 'Escape') {
      commandVisible.value = false
      closePicker()
      return
    }
    if (event.key === 'Enter' && (event.ctrlKey || event.metaKey)) {
      event.preventDefault()
      submit()
      return
    }
    if (event.key === 'Backspace' && inputText.value.length === 0 && promptCode.value) {
      promptCode.value = ''
    }
  }

  /** 外部预置提示词与章节引用（编辑器 AI 续写/润色/梗概入口） */
  const presetPrompt = (newPromptCode: string, chapterUlidList: string[]) => {
    promptCode.value = newPromptCode
    for (const chapterUlid of chapterUlidList) {
      const chapter = props.chapterList.find((item) => item.chapter_ulid === chapterUlid)
      if (chapter) pushChapterRef(chapterUlid, chapter.chapter_name, `第 ${chapter.chapter_order} 章`)
    }
    inputRef.value?.focus()
  }

  /** 外部执行斜杠动作（写作台顶栏与面板按钮共用同一套动作语义） */
  const executeAction = (actionCode: string) => runAction(actionCode)

  watch(
    () => props.projectUlid,
    () => {
      systemPromptCode.value = ''
      clearRefs()
    }
  )

  watch(refCandidateList, () => {
    refActiveIndex.value = 0
  })

  watch(commandFilter, () => {
    commandActiveIndex.value = 0
  })

  defineExpose({
    presetPrompt,
    executeAction,
    focus: () => inputRef.value?.focus(),
    setInputText: (text: string) => {
      inputText.value = text
      void nextTick(() => inputRef.value?.focus())
    }
  })
</script>

<template>
  <div class="composer">
    <div v-if="promptCode || chipList.length > 0" class="composer__chips">
      <ElTag v-if="promptCode" closable size="small" @close="promptCode = ''">/{{ promptCode }}</ElTag>
      <ElTag
        v-for="chip in chipList"
        :key="`${chip.tab}-${chip.ref_ulid}`"
        closable
        size="small"
        type="info"
        @close="removeRefItem(chip.tab, chip.ref_ulid)"
      >
        {{ chip.label }}: {{ chip.ref_name }}
      </ElTag>
    </div>

    <div class="composer__input-wrap">
      <ComposerPopup
        :command-visible="commandVisible"
        :command-option-list="slashOptionList"
        :command-active-index="commandActiveIndex"
        :ref-picker-visible="refPickerVisible"
        :ref-tabs="refTabs"
        :ref-picker-tab="refPickerTab"
        :ref-candidate-list="refCandidateList"
        :ref-selected-ulid-list="refSelectedUlidList"
        :ref-active-index="refActiveIndex"
        @select-command="selectCommand"
        @hover-command="commandActiveIndex = $event"
        @switch-tab="switchTab"
        @select-ref="selectRef"
        @hover-ref="refActiveIndex = $event"
      />

      <textarea
        ref="inputRef"
        v-model="inputText"
        class="composer__textarea"
        rows="3"
        :disabled="streaming"
        placeholder="写下你的要求；/ 命令与提示词，@ 引用素材或历史会话"
        @input="handleInput"
        @keydown="handleKeydown"
      />
    </div>

    <div class="composer__toolbar">
      <ElSelect
        v-model="systemPromptCode"
        size="small"
        clearable
        placeholder="身份提示词"
        class="composer__profile"
      >
        <ElOption
          v-for="item in systemPromptList"
          :key="item.prompt_code"
          :label="item.prompt_name"
          :value="item.prompt_code"
        />
      </ElSelect>
      <ElButton size="small" plain @click="togglePicker">@ 引用</ElButton>
      <ElButton size="small" plain @click="runAction('help')">/ 速查</ElButton>
      <div class="composer__spacer" />
      <ElButton v-if="streaming" size="small" plain type="danger" @click="emit('stop')">
        <Icon icon="mdi:stop" :size="14" />
        <span class="ml-4px">停止</span>
      </ElButton>
      <ElButton v-else size="small" type="primary" plain :disabled="!canSend" @click="submit">
        <Icon icon="mdi:send" :size="14" />
        <span class="ml-4px">发送</span>
      </ElButton>
    </div>
  </div>
</template>

<style lang="less" scoped>
  .composer {
    padding: 8px 10px 10px;
    border-top: 1px solid var(--nw-border-soft);

    &__chips {
      display: flex;
      flex-wrap: wrap;
      gap: 4px;
      margin-bottom: 6px;
    }

    &__input-wrap {
      position: relative;
    }

    &__textarea {
      width: 100%;
      padding: 8px 10px;
      font-family: inherit;
      font-size: 13px;
      line-height: 1.6;
      color: var(--nw-ink);
      resize: vertical;
      background: var(--nw-surface-subtle);
      border: 1px solid var(--nw-border);
      border-radius: var(--nw-radius-md);
      outline: none;

      &:focus {
        border-color: var(--el-color-primary);
        box-shadow: 0 0 0 3px var(--nw-primary-soft);
      }
    }

    &__toolbar {
      display: flex;
      flex-wrap: wrap;
      gap: 6px;
      align-items: center;
      margin-top: 8px;
    }

    &__profile {
      width: 132px;
    }

    &__history {
      width: 112px;
    }

    &__spacer {
      flex: 1;
    }
  }
</style>

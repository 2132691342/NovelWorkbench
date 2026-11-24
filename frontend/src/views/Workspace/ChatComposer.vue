<!-- 对话输入区：斜杠命令唤起技能、井号唤起工具、@ 引用与上下文开关。 -->
<script setup lang="ts">
  import { computed, ref, watch } from 'vue'
  import type { NWPromptProfileVo, NWSkillVo } from '@/api/contracts'
  import type { NWToolVo } from '@/api/tool'
  import type {
    ChatCommandOption,
    ChatRefItem,
    ChatRefTabCode,
    ChatSendPayload
  } from './types'
  import { useChatRefs } from './useChatRefs'
  import ComposerPopup from './ComposerPopup.vue'
  import type { NWChapterBriefVo } from '@/api/contracts'
  import type { ChatDisplayMessage } from './types'

  const props = defineProps<{
    projectUlid: string
    chapterList: NWChapterBriefVo[]
    messageList: ChatDisplayMessage[]
    skillList: NWSkillVo[]
    toolList: NWToolVo[]
    profileList: NWPromptProfileVo[]
    streaming: boolean
    canSend: boolean
  }>()

  const emit = defineEmits<{
    send: [payload: ChatSendPayload]
    stop: []
  }>()

  /** 行内命令触发符：/ 选技能，# 选工具（两者都是单选列表，共用一个弹层） */
  type CommandKind = 'slash' | 'hash'

  const inputText = ref('')
  const skillCode = ref('')
  const toolCodeList = ref<string[]>([])
  const historyCount = ref(0)
  const promptProfileUlid = ref('')
  const inputRef = ref<HTMLTextAreaElement | null>(null)
  const commandKind = ref<CommandKind | ''>('')
  const commandFilter = ref('')
  const commandActiveIndex = ref(0)

  const {
    refDocList,
    refChapterList,
    refSummaryList,
    refPromptList,
    refMessageList,
    refPickerVisible,
    refPickerTab,
    refPickerFilter,
    refCandidateList,
    refSelectedUlidList,
    loadKnowledgeList,
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
    messageList: () => props.messageList,
    profileList: () => props.profileList
  })

  const commandOptionList = computed<ChatCommandOption[]>(() => {
    const filterText = commandFilter.value.toLowerCase()
    if (commandKind.value === 'slash') {
      return props.skillList
        .filter(
          (skill) =>
            skill.skill_code.toLowerCase().includes(filterText) ||
            skill.skill_name.toLowerCase().includes(filterText)
        )
        .map((skill) => ({
          code: skill.skill_code,
          title: skill.skill_name,
          desc: skill.description_text,
          active: skillCode.value === skill.skill_code
        }))
    }
    if (commandKind.value === 'hash') {
      return props.toolList
        .filter(
          (tool) =>
            tool.tool_code.toLowerCase().includes(filterText) ||
            tool.tool_name.toLowerCase().includes(filterText)
        )
        .map((tool) => ({
          code: tool.tool_code,
          title: tool.tool_name,
          desc: tool.description_text,
          active: toolCodeList.value.includes(tool.tool_code)
        }))
    }
    return []
  })

  const chipList = computed(() => [
    ...refDocList.value.map((item) => ({ tab: 'doc' as const, label: '文档', ...item })),
    ...refChapterList.value.map((item) => ({ tab: 'chapter' as const, label: '章节', ...item })),
    ...refSummaryList.value.map((item) => ({ tab: 'summary' as const, label: '摘要', ...item })),
    ...refPromptList.value.map((item) => ({ tab: 'prompt' as const, label: '档案', ...item })),
    ...refMessageList.value.map((item) => ({ tab: 'message' as const, label: '消息', ...item }))
  ])

  const refTabs: Array<{ code: ChatRefTabCode; label: string }> = [
    { code: 'doc', label: '文档' },
    { code: 'chapter', label: '章节' },
    { code: 'summary', label: '摘要' },
    { code: 'prompt', label: '档案' },
    { code: 'message', label: '消息' }
  ]

  const SLASH_TOKEN_REGEXP = /(^|\s)\/([^\s]*)$/
  const HASH_TOKEN_REGEXP = /(^|\s)#([^\s]*)$/
  const AT_TOKEN_REGEXP = /(^|\s)@([^\s]*)$/

  /** 删除光标前的 /xxx、#xxx 或 @xxx 触发词 */
  const removeTriggerToken = (tokenRegexp: RegExp) => {
    const cursorPos = inputRef.value?.selectionStart ?? inputText.value.length
    const beforeCursor = inputText.value.slice(0, cursorPos)
    inputText.value = beforeCursor.replace(tokenRegexp, '$1') + inputText.value.slice(cursorPos)
  }

  const handleInput = () => {
    const cursorPos = inputRef.value?.selectionStart ?? inputText.value.length
    const beforeCursor = inputText.value.slice(0, cursorPos)

    const slashMatch = SLASH_TOKEN_REGEXP.exec(beforeCursor)
    const hashMatch = HASH_TOKEN_REGEXP.exec(beforeCursor)
    if (slashMatch) {
      commandKind.value = 'slash'
      commandFilter.value = slashMatch[2]
    } else if (hashMatch) {
      commandKind.value = 'hash'
      commandFilter.value = hashMatch[2]
    } else {
      commandKind.value = ''
      commandFilter.value = ''
    }
    commandActiveIndex.value = 0

    const atMatch = AT_TOKEN_REGEXP.exec(beforeCursor)
    if (atMatch) {
      refPickerFilter.value = atMatch[2]
      refPickerVisible.value = true
      void loadKnowledgeList()
    }
  }

  const selectCommand = (option: ChatCommandOption) => {
    if (commandKind.value === 'slash') {
      skillCode.value = option.code
    } else if (!toolCodeList.value.includes(option.code)) {
      toolCodeList.value.push(option.code)
    }
    removeTriggerToken(commandKind.value === 'slash' ? SLASH_TOKEN_REGEXP : HASH_TOKEN_REGEXP)
    commandKind.value = ''
    inputRef.value?.focus()
  }

  const removeToolCode = (toolCode: string) => {
    toolCodeList.value = toolCodeList.value.filter((code) => code !== toolCode)
  }

  const selectRef = (item: ChatRefItem) => {
    toggleRefItem(item)
    removeTriggerToken(AT_TOKEN_REGEXP)
    inputRef.value?.focus()
  }

  const submit = () => {
    if (!props.canSend) return
    emit('send', {
      content_text: inputText.value.trim(),
      skill_code: skillCode.value,
      tool_code_list: [...toolCodeList.value],
      ...toRefPayload(),
      history_count: historyCount.value,
      prompt_profile_ulid: promptProfileUlid.value
    })
    inputText.value = ''
    skillCode.value = ''
    toolCodeList.value = []
    commandKind.value = ''
    clearRefs()
    closePicker()
  }

  const handleKeydown = (event: KeyboardEvent) => {
    if (commandKind.value && commandOptionList.value.length > 0) {
      const optionCount = commandOptionList.value.length
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
        selectCommand(commandOptionList.value[commandActiveIndex.value])
        return
      }
    }
    if (event.key === 'Escape') {
      commandKind.value = ''
      closePicker()
      return
    }
    if (event.key === 'Enter' && (event.ctrlKey || event.metaKey)) {
      event.preventDefault()
      submit()
      return
    }
    if (event.key === 'Backspace' && inputText.value.length === 0) {
      if (skillCode.value) {
        skillCode.value = ''
      } else if (toolCodeList.value.length > 0) {
        toolCodeList.value.pop()
      }
    }
  }

  /** 外部预置技能与章节引用（编辑器 AI 续写/润色/梗概入口） */
  const presetSkill = (newSkillCode: string, chapterUlidList: string[]) => {
    skillCode.value = newSkillCode
    for (const chapterUlid of chapterUlidList) {
      const chapter = props.chapterList.find((item) => item.chapter_ulid === chapterUlid)
      if (chapter) pushChapterRef(chapterUlid, chapter.chapter_name, `第 ${chapter.chapter_order} 章`)
    }
    inputRef.value?.focus()
  }

  watch(
    () => props.projectUlid,
    () => {
      promptProfileUlid.value = ''
      toolCodeList.value = []
    }
  )

  defineExpose({
    presetSkill,
    focus: () => inputRef.value?.focus(),
    setInputText: (text: string) => {
      inputText.value = text
      inputRef.value?.focus()
    }
  })
</script>

<template>
  <div class="composer">
    <div v-if="skillCode || toolCodeList.length > 0 || chipList.length > 0" class="composer__chips">
      <ElTag v-if="skillCode" closable size="small" @close="skillCode = ''">/{{ skillCode }}</ElTag>
      <ElTag
        v-for="toolCode in toolCodeList"
        :key="`tool-${toolCode}`"
        closable
        size="small"
        type="warning"
        @close="removeToolCode(toolCode)"
      >
        #{{ toolCode }}
      </ElTag>
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
        :command-kind="commandKind"
        :command-option-list="commandOptionList"
        :command-active-index="commandActiveIndex"
        :ref-picker-visible="refPickerVisible"
        :ref-tabs="refTabs"
        :ref-picker-tab="refPickerTab"
        :ref-candidate-list="refCandidateList"
        :ref-selected-ulid-list="refSelectedUlidList"
        @select-command="selectCommand"
        @hover-command="commandActiveIndex = $event"
        @switch-tab="switchTab"
        @select-ref="selectRef"
      />

      <textarea
        ref="inputRef"
        v-model="inputText"
        class="composer__textarea"
        rows="3"
        :disabled="streaming"
        placeholder="写下你的要求；/ 唤起技能，# 唤起工具，@ 引用素材"
        @input="handleInput"
        @keydown="handleKeydown"
      />
    </div>

    <div class="composer__toolbar">
      <ElSelect
        v-model="promptProfileUlid"
        size="small"
        clearable
        placeholder="提示词档案"
        class="composer__profile"
      >
        <ElOption
          v-for="profile in profileList"
          :key="profile.prompt_profile_ulid"
          :label="profile.prompt_profile_name"
          :value="profile.prompt_profile_ulid"
        />
      </ElSelect>
      <ElSelect v-model="historyCount" size="small" class="composer__history">
        <ElOption :value="0" label="历史：默认" />
        <ElOption :value="3" label="历史：3 条" />
        <ElOption :value="5" label="历史：5 条" />
        <ElOption :value="10" label="历史：10 条" />
      </ElSelect>
      <ElButton size="small" plain @click="togglePicker">@ 引用</ElButton>
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

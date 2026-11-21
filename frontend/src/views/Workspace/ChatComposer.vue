<!-- 对话输入区：斜杠命令、@ 引用与上下文开关。 -->
<script setup lang="ts">
  import { computed, ref, watch } from 'vue'
  import type { NWPromptProfileVo, NWSkillVo } from '@/api/contracts'
  import type { ChatRefItem, ChatRefTabCode, ChatSendPayload } from './types'
  import { useChatRefs } from './useChatRefs'
  import type { NWChapterBriefVo } from '@/api/contracts'
  import type { ChatDisplayMessage } from './types'

  const props = defineProps<{
    projectUlid: string
    chapterList: NWChapterBriefVo[]
    messageList: ChatDisplayMessage[]
    skillList: NWSkillVo[]
    profileList: NWPromptProfileVo[]
    streaming: boolean
    canSend: boolean
  }>()

  const emit = defineEmits<{
    send: [payload: ChatSendPayload]
    stop: []
  }>()

  const inputText = ref('')
  const skillCode = ref('')
  const historyCount = ref(0)
  const promptProfileUlid = ref('')
  const inputRef = ref<HTMLTextAreaElement | null>(null)
  const slashVisible = ref(false)
  const slashFilter = ref('')
  const slashActiveIndex = ref(0)

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

  const filteredSkillList = computed(() => {
    const filterText = slashFilter.value.toLowerCase()
    return props.skillList.filter(
      (skill) =>
        skill.skill_code.toLowerCase().includes(filterText) ||
        skill.skill_name.toLowerCase().includes(filterText)
    )
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
    slashFilter.value = slashMatch ? slashMatch[2] : ''
    slashVisible.value = Boolean(slashMatch)
    slashActiveIndex.value = 0

    const atMatch = AT_TOKEN_REGEXP.exec(beforeCursor)
    if (atMatch) {
      refPickerFilter.value = atMatch[2]
      refPickerVisible.value = true
      void loadKnowledgeList()
    }
  }

  const selectSkill = (skill: NWSkillVo) => {
    skillCode.value = skill.skill_code
    removeTriggerToken(SLASH_TOKEN_REGEXP)
    slashVisible.value = false
    inputRef.value?.focus()
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
      ...toRefPayload(),
      history_count: historyCount.value,
      prompt_profile_ulid: promptProfileUlid.value
    })
    inputText.value = ''
    skillCode.value = ''
    slashVisible.value = false
    clearRefs()
    closePicker()
  }

  const handleKeydown = (event: KeyboardEvent) => {
    if (slashVisible.value && filteredSkillList.value.length > 0) {
      const optionCount = filteredSkillList.value.length
      if (event.key === 'ArrowDown') {
        event.preventDefault()
        slashActiveIndex.value = (slashActiveIndex.value + 1) % optionCount
        return
      }
      if (event.key === 'ArrowUp') {
        event.preventDefault()
        slashActiveIndex.value = (slashActiveIndex.value - 1 + optionCount) % optionCount
        return
      }
      if (event.key === 'Enter' || event.key === 'Tab') {
        event.preventDefault()
        selectSkill(filteredSkillList.value[slashActiveIndex.value])
        return
      }
    }
    if (event.key === 'Escape') {
      slashVisible.value = false
      closePicker()
      return
    }
    if (event.key === 'Enter' && (event.ctrlKey || event.metaKey)) {
      event.preventDefault()
      submit()
      return
    }
    if (event.key === 'Backspace' && inputText.value.length === 0 && skillCode.value) {
      skillCode.value = ''
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
    }
  )

  defineExpose({ presetSkill, focus: () => inputRef.value?.focus() })
</script>

<template>
  <div class="composer">
    <div v-if="skillCode || chipList.length > 0" class="composer__chips">
      <ElTag v-if="skillCode" closable size="small" @close="skillCode = ''">/{{ skillCode }}</ElTag>
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
      <div v-if="slashVisible" class="composer__popup">
        <button
          v-for="(skill, index) in filteredSkillList"
          :key="skill.skill_ulid"
          type="button"
          :class="['composer__item', { 'is-active': index === slashActiveIndex }]"
          @mouseenter="slashActiveIndex = index"
          @click="selectSkill(skill)"
        >
          <strong>/{{ skill.skill_code }}</strong>
          <span>{{ skill.skill_name }}</span>
          <small>{{ skill.description_text }}</small>
        </button>
        <p v-if="filteredSkillList.length === 0" class="composer__empty">没有匹配的技能</p>
      </div>

      <div v-if="refPickerVisible" class="composer__popup">
        <div class="composer__tabs">
          <button
            v-for="tab in refTabs"
            :key="tab.code"
            type="button"
            :class="['composer__tab', { 'is-active': refPickerTab === tab.code }]"
            @click="switchTab(tab.code)"
          >
            {{ tab.label }}
          </button>
        </div>
        <button
          v-for="item in refCandidateList"
          :key="item.ref_ulid"
          type="button"
          :class="['composer__item', { 'is-active': refSelectedUlidList.includes(item.ref_ulid) }]"
          @click="selectRef(item)"
        >
          <strong>{{ item.ref_name }}</strong>
          <small>{{ item.ref_desc }}</small>
        </button>
        <p v-if="refCandidateList.length === 0" class="composer__empty">暂无可引用内容</p>
      </div>

      <textarea
        ref="inputRef"
        v-model="inputText"
        class="composer__textarea"
        rows="3"
        :disabled="streaming"
        placeholder="写下你的要求；/ 唤起技能，@ 引用素材"
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

    &__popup {
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

    &__item {
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
    }

    &__empty {
      padding: 10px;
      font-size: 12px;
      color: var(--el-text-color-placeholder);
      text-align: center;
    }

    &__tabs {
      display: flex;
      gap: 4px;
      padding: 2px 2px 6px;
      border-bottom: 1px solid var(--nw-border-soft);
    }

    &__tab {
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

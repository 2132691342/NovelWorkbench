<!--
  AI 输入区：正文输入 + 技能斜杠菜单（/ 触发）+ @引用选择器（文档/章节/摘要/提示词/历史消息五页签）+
  提示词档案选择 + 历史条数 + 发送/停止生成。子组件负责弹层渲染，本组件只维护输入态。
-->
<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { listPromptProfiles } from '@/api/prompt'
import { listSkills } from '@/api/skill'
import type { NWChapterBriefVo, NWPromptProfileVo, NWSkillVo } from '@/api/contracts'
import NWButton from '@/components/NWButton.vue'
import NWIcon from '@/components/NWIcon.vue'
import type { NWChatDisplayMessage, NWChatRefItem, NWChatSendPayload } from './chatTypes'
import { useChatRefs } from './useChatRefs'
import ComposerToolbar from './ComposerToolbar.vue'
import RefPicker from './RefPicker.vue'
import SlashMenu from './SlashMenu.vue'

const props = defineProps<{
  projectUlid: string
  chapterList: NWChapterBriefVo[]
  messageList: NWChatDisplayMessage[]
  disabledFlag: boolean
}>()

const emit = defineEmits<{
  send: [payload: NWChatSendPayload]
  stop: []
}>()

const { t } = useI18n()

/** 技能触发词（行首或空白后 /xxx） */
const SLASH_TOKEN_REGEXP = /(^|\s)\/([^\s]*)$/
/** 引用触发词（行首或空白后 @xxx） */
const AT_TOKEN_REGEXP = /(^|\s)@([^\s]*)$/

const inputRef = ref<HTMLTextAreaElement | null>(null)
const inputText = ref('')
const cursorPos = ref(0)
const skillCode = ref('')
const historyCount = ref(0)
const promptProfileUlid = ref('')
const slashList = ref<NWSkillVo[]>([])
const profileList = ref<NWPromptProfileVo[]>([])
const slashPopupVisible = ref(false)
const slashFilter = ref('')
const slashActiveIndex = ref(0)

// @ 引用（文档/章节/摘要/提示词/历史消息）的状态与候选装配
const {
  refDocList,
  refChapterList,
  refSummaryList,
  refPromptList,
  refMessageList,
  refPickerVisible,
  refPickerPinned,
  refPickerTab,
  refPickerFilter,
  refCandidateList,
  refSelectedUlidList,
  refEmptyHint,
  loadKnowledgeList,
  switchTab,
  togglePicker,
  closePicker,
  toggleRefItem,
  removeRefItem,
  clearRefs,
  toRefPayload,
  pushChapterRef,
} = useChatRefs({
  projectUlid: () => props.projectUlid,
  chapterList: () => props.chapterList,
  messageList: () => props.messageList,
  profileList,
})

/** 按输入过滤的技能列表 */
const filteredSlashList = computed(() => {
  const filterText = slashFilter.value.toLowerCase()
  return slashList.value.filter(
    (item) =>
      item.skill_code.toLowerCase().includes(filterText) ||
      item.skill_name.toLowerCase().includes(filterText),
  )
})




const canSend = computed(
  () => inputText.value.trim().length > 0 && !props.disabledFlag,
)

/** 已选中的提示词档案名（无档案时不展示徽标） */
const selectedProfileName = computed(
  () => profileList.value.find((item) => item.prompt_profile_ulid === promptProfileUlid.value)?.prompt_profile_name ?? '',
)

/** 输入事件：跟踪光标并按 / 与 @ 触发弹层 */
function handleInput(event: Event): void {
  const target = event.target as HTMLTextAreaElement
  cursorPos.value = target.selectionStart ?? inputText.value.length
  const beforeCursor = inputText.value.slice(0, cursorPos.value)

  const slashMatch = SLASH_TOKEN_REGEXP.exec(beforeCursor)
  if (slashMatch) {
    slashFilter.value = slashMatch[2]
    slashActiveIndex.value = 0
    slashPopupVisible.value = true
  } else {
    slashPopupVisible.value = false
  }

  const atMatch = AT_TOKEN_REGEXP.exec(beforeCursor)
  if (atMatch) {
    refPickerFilter.value = atMatch[2]
    refPickerPinned.value = true
    refPickerVisible.value = true
  } else if (!refPickerPinned.value) {
    refPickerVisible.value = false
  }
}

/** 移除光标前的触发词（/xxx 或 @xxx） */
function removeTriggerTokenBeforeCursor(tokenRegexp: RegExp): void {
  const beforeCursor = inputText.value.slice(0, cursorPos.value)
  const afterCursor = inputText.value.slice(cursorPos.value)
  inputText.value = beforeCursor.replace(tokenRegexp, '$1') + afterCursor
}

/** 键盘交互：弹层导航与技能徽标移除 */
function handleKeydown(event: KeyboardEvent): void {
  if (slashPopupVisible.value) {
    const optionCount = filteredSlashList.value.length
    if (event.key === 'ArrowDown' && optionCount > 0) {
      event.preventDefault()
      slashActiveIndex.value = (slashActiveIndex.value + 1) % optionCount
      return
    }
    if (event.key === 'ArrowUp' && optionCount > 0) {
      event.preventDefault()
      slashActiveIndex.value = (slashActiveIndex.value - 1 + optionCount) % optionCount
      return
    }
    if ((event.key === 'Enter' || event.key === 'Tab') && optionCount > 0) {
      event.preventDefault()
      selectSlashOption(filteredSlashList.value[slashActiveIndex.value])
      return
    }
    if (event.key === 'Escape') {
      event.preventDefault()
      slashPopupVisible.value = false
      return
    }
  }
  if (refPickerVisible.value && event.key === 'Escape') {
    event.preventDefault()
    closePicker()
    return
  }
  if (event.key === 'Enter' && (event.ctrlKey || event.metaKey)) {
    event.preventDefault()
    if (canSend.value) {
      handleSend()
    }
    return
  }
  // 输入为空时 Backspace 移除技能徽标
  if (event.key === 'Backspace' && inputText.value.length === 0 && skillCode.value) {
    skillCode.value = ''
  }
}

/** 选中引用：切换选中态并清掉光标前的 @ 触发词（选择器保持打开可继续多选） */
function handleSelectRef(item: NWChatRefItem): void {
  toggleRefItem(item)
  removeTriggerTokenBeforeCursor(AT_TOKEN_REGEXP)
}

/** 选中技能：填入 skill_code 徽标并清掉触发词 */
function selectSlashOption(item: NWSkillVo): void {
  skillCode.value = item.skill_code
  removeTriggerTokenBeforeCursor(SLASH_TOKEN_REGEXP)
  slashPopupVisible.value = false
}

/** 移除技能徽标 */
function removeSkill(): void {
  skillCode.value = ''
}


/** 切换引用选择器（@ 按钮） */

/** 关闭引用选择器 */

/** 按页签取对应的已选列表 */

/** 页签切换：摘要页签按需加载 */

/** 多选引用：加入 chips 并清掉触发词（选择器保持打开可继续多选） */

/** 移除引用 chip */

/** 组装载荷并发送，随后清空输入态（保留历史条数与档案选择） */
/** 组装载荷并发送，随后清空输入态（保留历史条数与档案选择） */
function handleSend(): void {
  if (!canSend.value) {
    return
  }
  emit('send', {
    content_text: inputText.value.trim(),
    skill_code: skillCode.value,
    ...toRefPayload(),
    history_count: historyCount.value,
    prompt_profile_ulid: promptProfileUlid.value,
  })
  inputText.value = ''
  cursorPos.value = 0
  skillCode.value = ''
  clearRefs()
  slashPopupVisible.value = false
  closePicker()
}

/** 外部预置技能与章节引用（AI 续写 / AI 润色入口） */
/** 外部预置技能与章节引用（AI 续写 / AI 润色入口） */
function presetSkill(newSkillCode: string, refChapterUlidList: string[]): void {
  skillCode.value = newSkillCode
  for (const chapterUlid of refChapterUlidList) {
    const chapterItem = props.chapterList.find((item) => item.chapter_ulid === chapterUlid)
    if (!chapterItem) {
      continue
    }
    pushChapterRef(
      chapterUlid,
      chapterItem.chapter_name,
      t('workspace.ai.composer.chapterOrder', { order: chapterItem.chapter_order }),
    )
  }
}

/** 外部填入输入框（快捷提示词入口） */
function setInputText(text: string): void {
  inputText.value = text
  void nextTick(() => {
    inputRef.value?.focus()
    inputRef.value?.setSelectionRange(text.length, text.length)
  })
}

/** 聚焦输入框 */
function focus(): void {
  inputRef.value?.focus()
}

defineExpose({ presetSkill, setInputText, focus })


/** 加载技能候选（含当前项目技能与全局技能） */
async function loadSkillList(): Promise<void> {
  if (!props.projectUlid) {
    slashList.value = []
    return
  }
  try {
    const listData = await listSkills({ project_ulid: props.projectUlid, enabled_only: true })
    slashList.value = Array.isArray(listData) ? listData : []
  } catch {
    slashList.value = []
  }
}

/** 加载提示词档案（含全局档案；失败降级为空，不影响对话） */
async function loadProfileList(): Promise<void> {
  try {
    const listData = await listPromptProfiles({ project_ulid: props.projectUlid, enabled_only: true })
    profileList.value = Array.isArray(listData) ? listData : []
  } catch {
    profileList.value = []
  }
}

watch(
  () => props.projectUlid,
  () => {
    promptProfileUlid.value = ''
    void loadKnowledgeList()
    void loadSkillList()
    void loadProfileList()
  },
  { immediate: true },
)

onMounted(() => {
  void loadSkillList()
})
</script>

<template>
  <div class="chat-composer">
    <!-- 技能与档案徽标 -->
    <div v-if="skillCode || selectedProfileName" class="chat-composer__badge">
      <span v-if="skillCode" class="chat-composer__badge-text">/{{ skillCode }}</span>
      <NWButton v-if="skillCode" type="text" size="small" class="chat-composer__badge-close" @click="removeSkill">
        {{ t('workspace.ai.composer.removeSkill') }}
      </NWButton>
      <span v-if="selectedProfileName" class="chat-composer__badge-profile">
        <NWIcon icon_code="sparkles" :size_px="11" />
        {{ selectedProfileName }}
      </span>
    </div>

    <!-- 引用 chips -->
    <div
      v-if="refDocList.length + refChapterList.length + refSummaryList.length + refPromptList.length + refMessageList.length > 0"
      class="chat-composer__chips"
    >
      <span v-for="refItem in refDocList" :key="`doc-${refItem.ref_ulid}`" class="chat-composer__chip">
        {{ t('workspace.ai.composer.atDocs') }}: {{ refItem.ref_name }}
        <NWButton type="text" size="small" class="chat-composer__chip-close" @click="removeRefItem('doc', refItem.ref_ulid)">x</NWButton>
      </span>
      <span v-for="refItem in refChapterList" :key="`chp-${refItem.ref_ulid}`" class="chat-composer__chip">
        {{ t('workspace.ai.composer.atChapters') }}: {{ refItem.ref_name }}
        <NWButton type="text" size="small" class="chat-composer__chip-close" @click="removeRefItem('chapter', refItem.ref_ulid)">x</NWButton>
      </span>
      <span v-for="refItem in refSummaryList" :key="`sum-${refItem.ref_ulid}`" class="chat-composer__chip">
        {{ t('workspace.ai.composer.atSummaries') }}: {{ refItem.ref_name }}
        <NWButton type="text" size="small" class="chat-composer__chip-close" @click="removeRefItem('summary', refItem.ref_ulid)">x</NWButton>
      </span>
      <span v-for="refItem in refPromptList" :key="`prm-${refItem.ref_ulid}`" class="chat-composer__chip">
        {{ t('workspace.ai.composer.atPrompts') }}: {{ refItem.ref_name }}
        <NWButton type="text" size="small" class="chat-composer__chip-close" @click="removeRefItem('prompt', refItem.ref_ulid)">x</NWButton>
      </span>
      <span v-for="refItem in refMessageList" :key="`msg-${refItem.ref_ulid}`" class="chat-composer__chip">
        {{ t('workspace.ai.composer.atMessages') }}: {{ refItem.ref_name }}
        <NWButton type="text" size="small" class="chat-composer__chip-close" @click="removeRefItem('message', refItem.ref_ulid)">x</NWButton>
      </span>
    </div>

    <div class="chat-composer__input-wrap">
      <!-- 技能斜杠菜单 -->
      <div v-if="slashPopupVisible" class="chat-composer__popup chat-composer__popup--slash">
        <SlashMenu
          :option-list="filteredSlashList"
          :active-index="slashActiveIndex"
          @select="selectSlashOption"
          @hover="slashActiveIndex = $event"
        />
      </div>

      <!-- @ 引用选择器 -->
      <div v-if="refPickerVisible" class="chat-composer__popup chat-composer__popup--ref">
        <RefPicker
          :tab-code="refPickerTab"
          :filter-text="refPickerFilter"
          :candidate-list="refCandidateList"
          :selected-ulid-list="refSelectedUlidList"
          :empty-hint-text="refEmptyHint"
          @update:tab-code="switchTab"
          @update:filter-text="refPickerFilter = $event"
          @select="handleSelectRef"
          @close="closePicker"
        />
      </div>

      <textarea
        ref="inputRef"
        v-model="inputText"
        class="chat-composer__textarea"
        rows="3"
        :disabled="disabledFlag"
        :placeholder="t('workspace.ai.composer.placeholder')"
        @input="handleInput"
        @keydown="handleKeydown"
      />
    </div>

    <ComposerToolbar
      v-model:prompt-profile-ulid="promptProfileUlid"
      v-model:history-count="historyCount"
      :profile-list="profileList"
      :can-send-flag="canSend"
      :streaming-flag="disabledFlag"
      @open-ref-picker="togglePicker"
      @send="handleSend"
      @stop="emit('stop')"
    />

  </div>
</template>

<style scoped src="./ChatComposer.css"></style>

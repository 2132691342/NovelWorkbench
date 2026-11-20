<!--
  AI 输入区：正文输入 + 技能斜杠菜单（/ 触发）+ @引用选择器（文档/章节/历史消息三页签）+
  提示词档案选择 + 历史条数 + 发送/停止生成。子组件负责弹层渲染，本组件只维护输入态。
-->
<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { listKnowledgeDocuments } from '@/api/knowledge'
import { listPromptProfiles } from '@/api/prompt'
import { listSkills } from '@/api/skill'
import type {
  NWChapterBriefVo,
  NWKnowledgeDocumentVo,
  NWPromptProfileVo,
  NWSkillVo,
} from '@/api/contracts'
import NWButton from '@/components/NWButton.vue'
import NWIcon from '@/components/NWIcon.vue'
import type { NWChatDisplayMessage, NWChatRefItem, NWChatRefTabCode, NWChatSendPayload } from './chatTypes'
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
/** @ 候选里的历史消息最大条数（过长会话只取最近若干条，避免弹层卡顿） */
const AT_MESSAGE_LIMIT = 40

const inputRef = ref<HTMLTextAreaElement | null>(null)
const inputText = ref('')
const cursorPos = ref(0)
const skillCode = ref('')
const refDocList = ref<NWChatRefItem[]>([])
const refChapterList = ref<NWChatRefItem[]>([])
const refMessageList = ref<NWChatRefItem[]>([])
const historyCount = ref(0)
const promptProfileUlid = ref('')
const slashList = ref<NWSkillVo[]>([])
const knowledgeList = ref<NWKnowledgeDocumentVo[]>([])
const profileList = ref<NWPromptProfileVo[]>([])
const slashPopupVisible = ref(false)
const slashFilter = ref('')
const slashActiveIndex = ref(0)
const refPickerVisible = ref(false)
const refPickerPinned = ref(false)
const refPickerTab = ref<NWChatRefTabCode>('doc')
const refPickerFilter = ref('')

/** 按输入过滤的技能列表 */
const filteredSlashList = computed(() => {
  const filterText = slashFilter.value.toLowerCase()
  return slashList.value.filter(
    (item) =>
      item.skill_code.toLowerCase().includes(filterText) ||
      item.skill_name.toLowerCase().includes(filterText),
  )
})

/** @ 候选：文档 / 章节 / 历史消息三选一 */
const refCandidateList = computed<NWChatRefItem[]>(() => {
  switch (refPickerTab.value) {
    case 'doc':
      return knowledgeList.value.map((docItem) => ({
        ref_ulid: docItem.doc_ulid,
        ref_name: docItem.doc_name,
        ref_desc: docItem.doc_type_code ?? '',
      }))
    case 'chapter':
      return props.chapterList.map((chapterItem) => ({
        ref_ulid: chapterItem.chapter_ulid,
        ref_name: chapterItem.chapter_name,
        ref_desc: t('workspace.ai.composer.chapterOrder', { order: chapterItem.chapter_order }),
      }))
    default:
      return props.messageList
        .filter((message) => message.content_text.length > 0 && !message.streaming_flag)
        .slice(-AT_MESSAGE_LIMIT)
        .map((message) => ({
          ref_ulid: message.message_ulid,
          ref_name: message.content_text.replace(/\s+/g, ' ').slice(0, 40),
          ref_desc: message.role_code === 'user' ? t('workspace.ai.roleUser') : t('workspace.ai.roleAssistant'),
        }))
        .filter((item) => item.ref_ulid.length > 0)
  }
})

/** 当前页签已选中的 ULID 列表 */
const refSelectedUlidList = computed<string[]>(() => {
  switch (refPickerTab.value) {
    case 'doc':
      return refDocList.value.map((item) => item.ref_ulid)
    case 'chapter':
      return refChapterList.value.map((item) => item.ref_ulid)
    default:
      return refMessageList.value.map((item) => item.ref_ulid)
  }
})

/** @ 候选为空时的提示 */
const refEmptyHint = computed(() => {
  if (refPickerTab.value === 'message') {
    return t('workspace.ai.composer.atMessagesEmpty')
  }
  return t('workspace.ai.composer.atEmpty')
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
    closeRefPicker()
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
function toggleRefPicker(): void {
  if (refPickerVisible.value) {
    closeRefPicker()
  } else {
    refPickerFilter.value = ''
    refPickerPinned.value = true
    refPickerVisible.value = true
  }
}

/** 关闭引用选择器 */
function closeRefPicker(): void {
  refPickerVisible.value = false
  refPickerPinned.value = false
}

/** 按页签取对应的已选列表 */
function refListOf(tabCode: NWChatRefTabCode): typeof refDocList {
  if (tabCode === 'doc') {
    return refDocList
  }
  if (tabCode === 'chapter') {
    return refChapterList
  }
  return refMessageList
}

/** 多选引用：加入 chips 并清掉触发词（选择器保持打开可继续多选） */
function selectRefItem(item: NWChatRefItem): void {
  const targetList = refListOf(refPickerTab.value)
  const existingIndex = targetList.value.findIndex((refItem) => refItem.ref_ulid === item.ref_ulid)
  if (existingIndex >= 0) {
    targetList.value.splice(existingIndex, 1)
  } else {
    targetList.value.push(item)
  }
  removeTriggerTokenBeforeCursor(AT_TOKEN_REGEXP)
}

/** 移除引用 chip */
function removeRefItem(tabCode: NWChatRefTabCode, refUlid: string): void {
  const targetList = refListOf(tabCode)
  targetList.value = targetList.value.filter((item) => item.ref_ulid !== refUlid)
}

/** 组装载荷并发送，随后清空输入态（保留历史条数与档案选择） */
function handleSend(): void {
  if (!canSend.value) {
    return
  }
  emit('send', {
    content_text: inputText.value.trim(),
    skill_code: skillCode.value,
    ref_doc_ulid_list: refDocList.value.map((item) => item.ref_ulid),
    ref_chapter_ulid_list: refChapterList.value.map((item) => item.ref_ulid),
    ref_message_ulid_list: refMessageList.value.map((item) => item.ref_ulid),
    history_count: historyCount.value,
    prompt_profile_ulid: promptProfileUlid.value,
  })
  inputText.value = ''
  cursorPos.value = 0
  skillCode.value = ''
  refDocList.value = []
  refChapterList.value = []
  refMessageList.value = []
  slashPopupVisible.value = false
  closeRefPicker()
}

/** 外部预置技能与章节引用（AI 续写 / AI 润色入口） */
function presetSkill(newSkillCode: string, refChapterUlidList: string[]): void {
  skillCode.value = newSkillCode
  for (const chapterUlid of refChapterUlidList) {
    if (refChapterList.value.some((item) => item.ref_ulid === chapterUlid)) {
      continue
    }
    const chapterItem = props.chapterList.find((item) => item.chapter_ulid === chapterUlid)
    if (chapterItem) {
      refChapterList.value.push({
        ref_ulid: chapterUlid,
        ref_name: chapterItem.chapter_name,
        ref_desc: t('workspace.ai.composer.chapterOrder', { order: chapterItem.chapter_order }),
      })
    }
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

/** 加载知识库文档（供 @ 引用；接口未就绪降级为空） */
async function loadKnowledgeList(): Promise<void> {
  if (!props.projectUlid) {
    knowledgeList.value = []
    return
  }
  try {
    const pageData = await listKnowledgeDocuments({
      project_ulid: props.projectUlid,
      page_num: 1,
      page_size: 100,
    })
    knowledgeList.value = pageData.items ?? []
  } catch {
    knowledgeList.value = []
  }
}

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
      v-if="refDocList.length + refChapterList.length + refMessageList.length > 0"
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
          @update:tab-code="refPickerTab = $event"
          @update:filter-text="refPickerFilter = $event"
          @select="selectRefItem"
          @close="closeRefPicker"
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
      @open-ref-picker="toggleRefPicker"
      @send="handleSend"
      @stop="emit('stop')"
    />

  </div>
</template>

<style scoped src="./ChatComposer.css"></style>

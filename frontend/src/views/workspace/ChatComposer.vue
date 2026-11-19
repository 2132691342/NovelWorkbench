<!--
  AI 输入区：正文输入 + 技能自动补全（/ 触发，选中填入 skill_code 徽标）+
  @引用选择器（知识库文档/本项目章节，多选 chips）+ 历史条数 + 发送/停止生成。
-->
<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { listKnowledgeDocuments } from '@/api/knowledge'
import { listSkills } from '@/api/skill'
import type {
  NWChapterBriefVo,
  NWKnowledgeDocumentVo,
  NWSkillVo,
} from '@/api/contracts'
import NWButton from '@/components/NWButton.vue'
import NWIcon from '@/components/NWIcon.vue'
import type { NWChatRefItem, NWChatSendPayload } from './chatTypes'

const props = defineProps<{
  projectUlid: string
  chapterList: NWChapterBriefVo[]
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
const refDocList = ref<NWChatRefItem[]>([])
const refChapterList = ref<NWChatRefItem[]>([])
const historyCount = ref(0)
const slashList = ref<NWSkillVo[]>([])
const knowledgeList = ref<NWKnowledgeDocumentVo[]>([])
const slashPopupVisible = ref(false)
const slashFilter = ref('')
const slashActiveIndex = ref(0)
const refPickerVisible = ref(false)
const refPickerPinned = ref(false)
const refPickerTab = ref<'doc' | 'chapter'>('doc')
const refPickerFilter = ref('')

/** 历史条数选项（0 = 使用全局默认） */
const historyOptions = computed(() => [
  { value: 0, label: t('workspace.ai.composer.historyDefault') },
  { value: 1, label: '1' },
  { value: 3, label: '3' },
  { value: 5, label: '5' },
  { value: 10, label: '10' },
])

/** 按输入过滤的技能列表 */
const filteredSlashList = computed(() => {
  const filterText = slashFilter.value.toLowerCase()
  return slashList.value.filter(
    (item) =>
      item.skill_code.toLowerCase().includes(filterText) ||
      item.skill_name.toLowerCase().includes(filterText),
  )
})

/** 按输入过滤的引用候选（知识库文档 / 章节二选一） */
const filteredRefList = computed<NWChatRefItem[]>(() => {
  const sourceList: NWChatRefItem[] =
    refPickerTab.value === 'doc'
      ? knowledgeList.value.map((docItem) => ({
          ref_ulid: docItem.doc_ulid,
          ref_name: docItem.doc_name,
        }))
      : props.chapterList.map((chapterItem) => ({
          ref_ulid: chapterItem.chapter_ulid,
          ref_name: chapterItem.chapter_name,
        }))
  const filterText = refPickerFilter.value.toLowerCase()
  return sourceList.filter((item) => item.ref_name.toLowerCase().includes(filterText))
})

const canSend = computed(
  () => inputText.value.trim().length > 0 && !props.disabledFlag,
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

/** 判断引用是否已选中 */
function isSelectedRef(refUlid: string): boolean {
  return (
    refDocList.value.some((item) => item.ref_ulid === refUlid) ||
    refChapterList.value.some((item) => item.ref_ulid === refUlid)
  )
}

/** 多选引用：加入 chips 并清掉触发词（选择器保持打开可继续多选） */
function selectRefItem(item: NWChatRefItem): void {
  const targetList = refPickerTab.value === 'doc' ? refDocList : refChapterList
  const existingIndex = targetList.value.findIndex((refItem) => refItem.ref_ulid === item.ref_ulid)
  if (existingIndex >= 0) {
    targetList.value.splice(existingIndex, 1)
  } else {
    targetList.value.push(item)
  }
  removeTriggerTokenBeforeCursor(AT_TOKEN_REGEXP)
}

/** 移除引用 chip */
function removeRefItem(kindCode: 'doc' | 'chapter', refUlid: string): void {
  const targetList = kindCode === 'doc' ? refDocList : refChapterList
  targetList.value = targetList.value.filter((item) => item.ref_ulid !== refUlid)
}

/** 组装载荷并发送，随后清空输入态（保留历史条数） */
function handleSend(): void {
  if (!canSend.value) {
    return
  }
  emit('send', {
    content_text: inputText.value.trim(),
    skill_code: skillCode.value,
    ref_doc_ulid_list: refDocList.value.map((item) => item.ref_ulid),
    ref_chapter_ulid_list: refChapterList.value.map((item) => item.ref_ulid),
    history_count: historyCount.value,
  })
  inputText.value = ''
  cursorPos.value = 0
  skillCode.value = ''
  refDocList.value = []
  refChapterList.value = []
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

watch(() => props.projectUlid, loadKnowledgeList, { immediate: true })

/** 加载技能候选（含当前项目技能与全局技能） */
function loadSkillList(): void {
  if (!props.projectUlid) {
    slashList.value = []
    return
  }
  void listSkills({ project_ulid: props.projectUlid, enabled_only: true })
    .then((listData) => {
      slashList.value = Array.isArray(listData) ? listData : []
    })
    .catch(() => {
      slashList.value = []
    })
}

watch(() => props.projectUlid, loadSkillList, { immediate: true })

onMounted(() => {
  loadSkillList()
})
</script>

<template>
  <div class="chat-composer">
    <!-- 技能徽标 -->
    <div v-if="skillCode" class="chat-composer__badge">
      <span class="chat-composer__badge-text">/{{ skillCode }}</span>
      <NWButton type="text" size="small" class="chat-composer__badge-close" @click="removeSkill">
        {{ t('workspace.ai.composer.removeSkill') }}
      </NWButton>
    </div>

    <!-- 引用 chips -->
    <div v-if="refDocList.length + refChapterList.length > 0" class="chat-composer__chips">
      <span v-for="refItem in refDocList" :key="refItem.ref_ulid" class="chat-composer__chip">
        {{ t('workspace.ai.composer.atDocs') }}: {{ refItem.ref_name }}
        <NWButton type="text" size="small" class="chat-composer__chip-close" @click="removeRefItem('doc', refItem.ref_ulid)">x</NWButton>
      </span>
      <span
        v-for="refItem in refChapterList"
        :key="refItem.ref_ulid"
        class="chat-composer__chip"
      >
        {{ t('workspace.ai.composer.atChapters') }}: {{ refItem.ref_name }}
        <NWButton type="text" size="small" class="chat-composer__chip-close" @click="removeRefItem('chapter', refItem.ref_ulid)">x</NWButton>
      </span>
    </div>

    <div class="chat-composer__input-wrap">
      <!-- 技能自动补全 -->
      <ul v-if="slashPopupVisible" class="chat-composer__popup chat-composer__popup--slash">
        <li v-if="filteredSlashList.length === 0" class="chat-composer__popup-empty">
          {{ t('workspace.ai.composer.slashEmpty') }}
        </li>
        <li
          v-for="(optionItem, optionIndex) in filteredSlashList"
          :key="optionItem.skill_ulid"
          class="chat-slash-option"
          :class="{ 'is-active': optionIndex === slashActiveIndex }"
          @mouseenter="slashActiveIndex = optionIndex"
          @click="selectSlashOption(optionItem)"
        >
          <span class="chat-slash-option__code">/{{ optionItem.skill_code }}</span>
          <span class="chat-slash-option__name">{{ optionItem.skill_name }}</span>
          <span class="chat-slash-option__desc">{{ optionItem.description_text }}</span>
        </li>
      </ul>

      <!-- 引用选择器 -->
      <div v-if="refPickerVisible" class="chat-composer__popup chat-composer__popup--ref">
        <div class="chat-ref-picker__tabs">
          <NWButton
            type="text"
            size="small"
            class="chat-ref-picker__tab"
            :class="{ 'is-active': refPickerTab === 'doc' }"
            @click="refPickerTab = 'doc'"
          >
            {{ t('workspace.ai.composer.atDocs') }}
          </NWButton>
          <NWButton
            type="text"
            size="small"
            class="chat-ref-picker__tab"
            :class="{ 'is-active': refPickerTab === 'chapter' }"
            @click="refPickerTab = 'chapter'"
          >
            {{ t('workspace.ai.composer.atChapters') }}
          </NWButton>
          <NWButton type="text" size="small" class="chat-ref-picker__close" @click="closeRefPicker">x</NWButton>
        </div>
        <ul class="chat-ref-picker__list">
          <li v-if="filteredRefList.length === 0" class="chat-composer__popup-empty">
            {{ t('workspace.ai.composer.atEmpty') }}
          </li>
          <li
            v-for="refItem in filteredRefList"
            :key="refItem.ref_ulid"
            class="chat-ref-picker__item"
            :class="{ 'is-selected': isSelectedRef(refItem.ref_ulid) }"
            @click="selectRefItem(refItem)"
          >
            <span class="chat-ref-picker__check">
              {{ isSelectedRef(refItem.ref_ulid) ? 'x' : '' }}
            </span>
            <span class="chat-ref-picker__name" :title="refItem.ref_name">
              {{ refItem.ref_name }}
            </span>
          </li>
        </ul>
      </div>

      <textarea
        v-model="inputText"
        class="chat-composer__textarea"
        rows="3"
        :disabled="disabledFlag"
        :placeholder="t('workspace.ai.composer.placeholder')"
        @input="handleInput"
        @keydown="handleKeydown"
      />
    </div>

    <div class="chat-composer__toolbar">
      <el-tooltip :content="t('workspace.ai.composer.historyHint')" placement="top">
        <label class="chat-composer__history">
          <NWIcon icon_code="history" :size_px="13" />
          <el-select v-model="historyCount" size="small" class="chat-composer__history-select">
            <el-option
              v-for="optionItem in historyOptions"
              :key="optionItem.value"
              :label="optionItem.label"
              :value="optionItem.value"
            />
          </el-select>
        </label>
      </el-tooltip>

      <button type="button" class="chat-composer__tool-btn" :disabled="disabledFlag" @click="toggleRefPicker">
        <NWIcon icon_code="atSign" :size_px="14" />
        {{ t('workspace.ai.composer.atButton') }}
      </button>

      <span class="chat-composer__hint">Ctrl + Enter</span>

      <button
        v-if="!disabledFlag"
        type="button"
        class="chat-composer__send-btn"
        :disabled="!canSend"
        @click="handleSend"
      >
        <NWIcon icon_code="send" :size_px="14" />
        {{ t('workspace.ai.composer.send') }}
      </button>
      <button v-else type="button" class="chat-composer__send-btn is-stop" @click="emit('stop')">
        <span class="chat-composer__stop-icon" />
        {{ t('workspace.ai.composer.stop') }}
      </button>
    </div>
  </div>
</template>

<style scoped src="./ChatComposer.css"></style>

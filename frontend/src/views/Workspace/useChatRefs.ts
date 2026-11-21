// @ 引用数据源：按页签拉取并缓存可选引用条目。
import { computed, ref, type Ref } from 'vue'
import { listSummaries, type NWChapterSummaryVo } from '@/api/continuity'
import { listKnowledgeDocuments } from '@/api/knowledge'
import type { NWChapterBriefVo, NWKnowledgeDocumentVo, NWPromptProfileVo } from '@/api/contracts'
import type { ChatDisplayMessage, ChatRefItem, ChatRefTabCode } from './types'

/** @ 候选里的历史消息上限（长会话只取最近若干条，避免弹层卡顿） */
const AT_MESSAGE_LIMIT = 40

export interface UseChatRefsOptions {
  projectUlid: () => string
  chapterList: () => NWChapterBriefVo[]
  messageList: () => ChatDisplayMessage[]
  profileList: () => NWPromptProfileVo[]
}

/** @ 引用状态：五类来源的候选、选中态与载荷装配（文档/章节/摘要/提示词/历史消息） */
export const useChatRefs = (options: UseChatRefsOptions) => {
  const refDocList = ref<ChatRefItem[]>([])
  const refChapterList = ref<ChatRefItem[]>([])
  const refSummaryList = ref<ChatRefItem[]>([])
  const refPromptList = ref<ChatRefItem[]>([])
  const refMessageList = ref<ChatRefItem[]>([])
  const knowledgeList = ref<NWKnowledgeDocumentVo[]>([])
  const summaryList = ref<NWChapterSummaryVo[]>([])
  const refPickerVisible = ref(false)
  const refPickerPinned = ref(false)
  const refPickerTab = ref<ChatRefTabCode>('doc')
  const refPickerFilter = ref('')

  const refListOf = (tabCode: ChatRefTabCode): Ref<ChatRefItem[]> => {
    if (tabCode === 'chapter') return refChapterList
    if (tabCode === 'summary') return refSummaryList
    if (tabCode === 'prompt') return refPromptList
    if (tabCode === 'message') return refMessageList
    return refDocList
  }

  /** 当前页签的候选：按关键词过滤 */
  const refCandidateList = computed<ChatRefItem[]>(() => {
    const filterText = refPickerFilter.value.toLowerCase()
    let candidates: ChatRefItem[]
    if (refPickerTab.value === 'doc') {
      candidates = knowledgeList.value.map((doc) => ({
        ref_ulid: doc.doc_ulid,
        ref_name: doc.doc_name,
        ref_desc: doc.doc_type_code
      }))
    } else if (refPickerTab.value === 'chapter') {
      candidates = options.chapterList().map((chapter) => ({
        ref_ulid: chapter.chapter_ulid,
        ref_name: chapter.chapter_name,
        ref_desc: `第 ${chapter.chapter_order} 章`
      }))
    } else if (refPickerTab.value === 'summary') {
      candidates = summaryList.value
        .filter((summary) => summary.summary_text.length > 0)
        .map((summary) => ({
          ref_ulid: summary.summary_ulid,
          ref_name:
            options
              .chapterList()
              .find((chapter) => chapter.chapter_ulid === summary.chapter_ulid)?.chapter_name ??
            '章节摘要',
          ref_desc: summary.summary_text.replace(/\s+/g, ' ').slice(0, 40)
        }))
    } else if (refPickerTab.value === 'prompt') {
      candidates = options.profileList().map((profile) => ({
        ref_ulid: profile.prompt_profile_ulid,
        ref_name: profile.prompt_profile_name,
        ref_desc: profile.description_text
      }))
    } else {
      candidates = options
        .messageList()
        .filter((message) => message.content_text.length > 0 && !message.streaming_flag)
        .slice(-AT_MESSAGE_LIMIT)
        .map((message) => ({
          ref_ulid: message.message_ulid,
          ref_name: message.content_text.replace(/\s+/g, ' ').slice(0, 40),
          ref_desc: message.role_code === 'user' ? '我' : 'AI'
        }))
        .filter((item) => item.ref_ulid.length > 0)
    }
    if (!filterText) return candidates
    return candidates.filter((item) => item.ref_name.toLowerCase().includes(filterText))
  })

  const refSelectedUlidList = computed(() =>
    refListOf(refPickerTab.value).value.map((item) => item.ref_ulid)
  )

  const loadSummaryList = async () => {
    if (!options.projectUlid() || summaryList.value.length > 0) return
    try {
      summaryList.value = await listSummaries({ project_ulid: options.projectUlid() })
    } catch {
      summaryList.value = []
    }
  }

  const loadKnowledgeList = async () => {
    if (!options.projectUlid()) {
      knowledgeList.value = []
      return
    }
    try {
      const page = await listKnowledgeDocuments({
        project_ulid: options.projectUlid(),
        page_num: 1,
        page_size: 100
      })
      knowledgeList.value = page.items ?? []
    } catch {
      knowledgeList.value = []
    }
  }

  const switchTab = (tabCode: ChatRefTabCode) => {
    refPickerTab.value = tabCode
    if (tabCode === 'summary') void loadSummaryList()
  }

  const closePicker = () => {
    refPickerVisible.value = false
    refPickerPinned.value = false
  }

  const togglePicker = () => {
    if (refPickerVisible.value) {
      closePicker()
      return
    }
    refPickerFilter.value = ''
    refPickerPinned.value = true
    refPickerVisible.value = true
  }

  const toggleRefItem = (item: ChatRefItem) => {
    const targetList = refListOf(refPickerTab.value)
    const existingIndex = targetList.value.findIndex((refItem) => refItem.ref_ulid === item.ref_ulid)
    if (existingIndex >= 0) targetList.value.splice(existingIndex, 1)
    else targetList.value.push(item)
  }

  const removeRefItem = (tabCode: ChatRefTabCode, refUlid: string) => {
    refListOf(tabCode).value = refListOf(tabCode).value.filter((item) => item.ref_ulid !== refUlid)
  }

  const clearRefs = () => {
    refDocList.value = []
    refChapterList.value = []
    refSummaryList.value = []
    refPromptList.value = []
    refMessageList.value = []
  }

  const toRefPayload = () => ({
    ref_doc_ulid_list: refDocList.value.map((item) => item.ref_ulid),
    ref_chapter_ulid_list: refChapterList.value.map((item) => item.ref_ulid),
    ref_summary_ulid_list: refSummaryList.value.map((item) => item.ref_ulid),
    ref_prompt_ulid_list: refPromptList.value.map((item) => item.ref_ulid),
    ref_message_ulid_list: refMessageList.value.map((item) => item.ref_ulid)
  })

  const pushChapterRef = (chapterUlid: string, refName: string, refDesc: string) => {
    if (refChapterList.value.some((item) => item.ref_ulid === chapterUlid)) return
    refChapterList.value.push({ ref_ulid: chapterUlid, ref_name: refName, ref_desc: refDesc })
  }

  return {
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
    loadSummaryList,
    switchTab,
    togglePicker,
    closePicker,
    toggleRefItem,
    removeRefItem,
    clearRefs,
    toRefPayload,
    pushChapterRef
  }
}

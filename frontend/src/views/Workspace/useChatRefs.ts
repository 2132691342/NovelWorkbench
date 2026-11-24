// @ 引用数据源：按页签拉取并缓存可选引用条目，「全部」页签跨来源聚合检索。
import { computed, ref, type Ref } from 'vue'
import { listSummaries, type NWChapterSummaryVo } from '@/api/continuity'
import { listKnowledgeDocuments } from '@/api/knowledge'
import type { NWChapterBriefVo, NWKnowledgeDocumentVo, NWPromptVo } from '@/api/contracts'
import type { ChatDisplayMessage, ChatRefItem, ChatRefTabCode } from './types'

/** @ 候选里的历史消息上限（长会话只取最近若干条，避免弹层卡顿） */
const AT_MESSAGE_LIMIT = 40

/** 聚合页签最多展示条数（够翻即可，再多靠关键词收窄） */
const AT_ALL_LIMIT = 30

export interface UseChatRefsOptions {
  projectUlid: () => string
  chapterList: () => NWChapterBriefVo[]
  messageList: () => ChatDisplayMessage[]
  promptList: () => NWPromptVo[]
}

/** 页签展示顺序（「全部」置首，作为默认检索入口） */
export const REF_TAB_LIST: Array<{ code: ChatRefTabCode; label: string }> = [
  { code: 'all', label: '全部' },
  { code: 'doc', label: '文档' },
  { code: 'chapter', label: '章节' },
  { code: 'summary', label: '摘要' },
  { code: 'prompt', label: '提示词' },
  { code: 'message', label: '消息' }
]

/** 单条候选的匹配分：名称前缀命中 3、包含命中 2、说明包含命中 1，未命中 0 */
export function refMatchScore(item: ChatRefItem, keywordText: string): number {
  const keyword = keywordText.trim().toLowerCase()
  if (!keyword) return 1
  const name = item.ref_name.toLowerCase()
  if (name.startsWith(keyword)) return 3
  if (name.includes(keyword)) return 2
  return item.ref_desc.toLowerCase().includes(keyword) ? 1 : 0
}

/** 按关键词过滤并按匹配分降序（同分保持原始顺序） */
export function rankRefItems(candidateList: ChatRefItem[], keywordText: string): ChatRefItem[] {
  const scored = candidateList
    .map((item, index) => ({ item, score: refMatchScore(item, keywordText), index }))
    .filter((entry) => entry.score > 0)
  scored.sort((left, right) => right.score - left.score || left.index - right.index)
  return scored.map((entry) => entry.item)
}

/** @ 引用状态：六类页签的候选、选中态与载荷装配（全部/文档/章节/摘要/提示词/历史消息） */
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
  const refPickerTab = ref<ChatRefTabCode>('all')
  const refPickerFilter = ref('')

  const refListOf = (tabCode: ChatRefTabCode): Ref<ChatRefItem[]> => {
    if (tabCode === 'chapter') return refChapterList
    if (tabCode === 'summary') return refSummaryList
    if (tabCode === 'prompt') return refPromptList
    if (tabCode === 'message') return refMessageList
    return refDocList
  }

  /** 各来源的完整候选（未过滤），聚合页签据此合并 */
  const sourceCandidateMap = computed<Record<Exclude<ChatRefTabCode, 'all'>, ChatRefItem[]>>(() => ({
    doc: knowledgeList.value.map((doc) => ({
      ref_ulid: doc.doc_ulid,
      ref_name: doc.doc_name,
      ref_desc: doc.doc_type_code,
      ref_tab_code: 'doc' as const
    })),
    chapter: options.chapterList().map((chapter) => ({
      ref_ulid: chapter.chapter_ulid,
      ref_name: chapter.chapter_name,
      ref_desc: `第 ${chapter.chapter_order} 章`,
      ref_tab_code: 'chapter' as const
    })),
    summary: summaryList.value
      .filter((summary) => summary.summary_text.length > 0)
      .map((summary) => ({
        ref_ulid: summary.summary_ulid,
        ref_name:
          options
            .chapterList()
            .find((chapter) => chapter.chapter_ulid === summary.chapter_ulid)?.chapter_name ??
          '章节摘要',
        ref_desc: summary.summary_text.replace(/\s+/g, ' ').slice(0, 40),
        ref_tab_code: 'summary' as const
      })),
    prompt: options.promptList().map((prompt) => ({
      ref_ulid: prompt.prompt_ulid,
      ref_name: prompt.prompt_name,
      ref_desc: prompt.description_text,
      ref_tab_code: 'prompt' as const
    })),
    message: options
      .messageList()
      .filter((message) => message.content_text.length > 0 && !message.streaming_flag)
      .slice(-AT_MESSAGE_LIMIT)
      .map((message) => ({
        ref_ulid: message.message_ulid,
        ref_name: message.content_text.replace(/\s+/g, ' ').slice(0, 40),
        ref_desc: message.role_code === 'user' ? '我' : 'AI',
        ref_tab_code: 'message' as const
      }))
      .filter((item) => item.ref_ulid.length > 0)
  }))

  /** 当前页签的候选：按关键词过滤，「全部」跨来源聚合后截断 */
  const refCandidateList = computed<ChatRefItem[]>(() => {
    if (refPickerTab.value === 'all') {
      const merged: ChatRefItem[] = []
      for (const tabCode of ['chapter', 'doc', 'summary', 'prompt', 'message'] as const) {
        merged.push(...sourceCandidateMap.value[tabCode])
      }
      return rankRefItems(merged, refPickerFilter.value).slice(0, AT_ALL_LIMIT)
    }
    return rankRefItems(sourceCandidateMap.value[refPickerTab.value], refPickerFilter.value)
  })

  /** 当前页签已选中的 ULID（「全部」页签按条目自身来源判断） */
  const refSelectedUlidList = computed(() =>
    refPickerTab.value === 'all'
      ? [...refDocList.value, ...refChapterList.value, ...refSummaryList.value,
        ...refPromptList.value, ...refMessageList.value].map((item) => item.ref_ulid)
      : refListOf(refPickerTab.value).value.map((item) => item.ref_ulid)
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

  /** 切页签：摘要需要额外拉取，「全部」同时把摘要备好 */
  const switchTab = (tabCode: ChatRefTabCode) => {
    refPickerTab.value = tabCode
    if (tabCode === 'summary' || tabCode === 'all') void loadSummaryList()
    if (tabCode === 'doc' || tabCode === 'all') void loadKnowledgeList()
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
    void loadKnowledgeList()
    void loadSummaryList()
  }

  /** 选中/取消：按条目自身来源写回对应列表，聚合页签下也能正确落位 */
  const toggleRefItem = (item: ChatRefItem) => {
    const targetList = refListOf(item.ref_tab_code)
    const existingIndex = targetList.value.findIndex((refItem) => refItem.ref_ulid === item.ref_ulid)
    if (existingIndex >= 0) targetList.value.splice(existingIndex, 1)
    else targetList.value.push(item)
  }

  const removeRefItem = (tabCode: ChatRefTabCode, refUlid: string) => {
    if (tabCode === 'all') return
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
    refChapterList.value.push({
      ref_ulid: chapterUlid,
      ref_name: refName,
      ref_desc: refDesc,
      ref_tab_code: 'chapter'
    })
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
    refPickerPinned,
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

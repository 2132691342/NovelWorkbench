// @ 引用数据源：只认两类来源——素材库（角色/设定/章节/摘要/文档）与历史会话。
// 「全部」页签跨来源聚合检索，选中时按条目自身来源写回对应列表。
import { computed, ref, type Ref } from 'vue'
import { listCharacters, type NWCharacterVo } from '@/api/character'
import { listLore, type NWLoreVo } from '@/api/lore'
import { listSummaries, type NWChapterSummaryVo } from '@/api/continuity'
import { listKnowledgeDocuments } from '@/api/knowledge'
import { listChatSessions } from '@/api/chat'
import type { NWChapterBriefVo, NWKnowledgeDocumentVo, NWChatSessionVo } from '@/api/contracts'
import type { ChatRefItem, ChatRefTabCode } from './types'

/** 聚合页签最多展示条数（够翻即可，再多靠关键词收窄） */
const AT_ALL_LIMIT = 30

export interface UseChatRefsOptions {
  projectUlid: () => string
  chapterList: () => NWChapterBriefVo[]
  sessionUlid: () => string
}

/** 页签展示顺序（「全部」置首，作为默认检索入口） */
export const REF_TAB_LIST: Array<{ code: ChatRefTabCode; label: string }> = [
  { code: 'all', label: '全部' },
  { code: 'character', label: '角色' },
  { code: 'lore', label: '设定' },
  { code: 'chapter', label: '章节' },
  { code: 'summary', label: '摘要' },
  { code: 'doc', label: '文档' },
  { code: 'session', label: '会话' }
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

/** @ 引用状态：七类页签的候选、选中态与载荷装配 */
export const useChatRefs = (options: UseChatRefsOptions) => {
  const refCharacterList = ref<ChatRefItem[]>([])
  const refLoreList = ref<ChatRefItem[]>([])
  const refChapterList = ref<ChatRefItem[]>([])
  const refSummaryList = ref<ChatRefItem[]>([])
  const refDocList = ref<ChatRefItem[]>([])
  const refSessionList = ref<ChatRefItem[]>([])
  const characterSourceList = ref<NWCharacterVo[]>([])
  const loreSourceList = ref<NWLoreVo[]>([])
  const knowledgeList = ref<NWKnowledgeDocumentVo[]>([])
  const summaryList = ref<NWChapterSummaryVo[]>([])
  const sessionSourceList = ref<NWChatSessionVo[]>([])
  const refPickerVisible = ref(false)
  const refPickerPinned = ref(false)
  const refPickerTab = ref<ChatRefTabCode>('all')
  const refPickerFilter = ref('')

  const refListOf = (tabCode: ChatRefTabCode): Ref<ChatRefItem[]> => {
    if (tabCode === 'character') return refCharacterList
    if (tabCode === 'lore') return refLoreList
    if (tabCode === 'chapter') return refChapterList
    if (tabCode === 'summary') return refSummaryList
    if (tabCode === 'session') return refSessionList
    return refDocList
  }

  /** 各来源的完整候选（未过滤），聚合页签据此合并 */
  const sourceCandidateMap = computed<Record<Exclude<ChatRefTabCode, 'all'>, ChatRefItem[]>>(() => ({
    character: characterSourceList.value.map((character) => ({
      ref_ulid: character.character_ulid,
      ref_name: character.character_name,
      ref_desc: character.personality_text || character.role_type_code,
      ref_tab_code: 'character' as const
    })),
    lore: loreSourceList.value.map((lore) => ({
      ref_ulid: lore.lore_ulid,
      ref_name: lore.lore_name,
      ref_desc: lore.content_text.replace(/\s+/g, ' ').slice(0, 40),
      ref_tab_code: 'lore' as const
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
    doc: knowledgeList.value.map((doc) => ({
      ref_ulid: doc.doc_ulid,
      ref_name: doc.doc_name,
      ref_desc: doc.doc_type_code,
      ref_tab_code: 'doc' as const
    })),
    session: sessionSourceList.value
      .filter((session) => session.session_ulid !== options.sessionUlid())
      .map((session) => ({
        ref_ulid: session.session_ulid,
        ref_name: session.session_name || '未命名会话',
        ref_desc: `${session.message_count} 条消息 · 注入时压缩`,
        ref_tab_code: 'session' as const
      }))
  }))

  /** 当前页签的候选：按关键词过滤，「全部」跨来源聚合后截断 */
  const refCandidateList = computed<ChatRefItem[]>(() => {
    if (refPickerTab.value === 'all') {
      const merged: ChatRefItem[] = []
      for (const tabCode of ['character', 'lore', 'chapter', 'summary', 'doc', 'session'] as const) {
        merged.push(...sourceCandidateMap.value[tabCode])
      }
      return rankRefItems(merged, refPickerFilter.value).slice(0, AT_ALL_LIMIT)
    }
    return rankRefItems(sourceCandidateMap.value[refPickerTab.value], refPickerFilter.value)
  })

  /** 当前页签已选中的 ULID（「全部」页签按条目自身来源判断） */
  const refSelectedUlidList = computed(() =>
    refPickerTab.value === 'all'
      ? [
          ...refCharacterList.value,
          ...refLoreList.value,
          ...refChapterList.value,
          ...refSummaryList.value,
          ...refDocList.value,
          ...refSessionList.value
        ].map((item) => item.ref_ulid)
      : refListOf(refPickerTab.value).value.map((item) => item.ref_ulid)
  )

  const loadCharacterList = async () => {
    if (!options.projectUlid()) {
      characterSourceList.value = []
      return
    }
    try {
      characterSourceList.value = await listCharacters({ project_ulid: options.projectUlid() })
    } catch {
      characterSourceList.value = []
    }
  }

  const loadLoreList = async () => {
    if (!options.projectUlid()) {
      loreSourceList.value = []
      return
    }
    try {
      const allList = await listLore({ project_ulid: options.projectUlid() })
      // 创作笔记严禁进入 AI 上下文，连候选都不该出现
      loreSourceList.value = allList.filter((lore) => lore.lore_type_code !== 'note')
    } catch {
      loreSourceList.value = []
    }
  }

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

  const loadSessionSourceList = async () => {
    if (!options.projectUlid()) {
      sessionSourceList.value = []
      return
    }
    try {
      sessionSourceList.value = await listChatSessions(options.projectUlid())
    } catch {
      sessionSourceList.value = []
    }
  }

  /** 切页签：只拉当前页签需要的数据，「全部」把六类都备好 */
  const switchTab = (tabCode: ChatRefTabCode) => {
    refPickerTab.value = tabCode
    if (tabCode === 'character' || tabCode === 'all') void loadCharacterList()
    if (tabCode === 'lore' || tabCode === 'all') void loadLoreList()
    if (tabCode === 'summary' || tabCode === 'all') void loadSummaryList()
    if (tabCode === 'doc' || tabCode === 'all') void loadKnowledgeList()
    if (tabCode === 'session' || tabCode === 'all') void loadSessionSourceList()
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
    void loadCharacterList()
    void loadLoreList()
    void loadKnowledgeList()
    void loadSummaryList()
    void loadSessionSourceList()
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
    refCharacterList.value = []
    refLoreList.value = []
    refChapterList.value = []
    refSummaryList.value = []
    refDocList.value = []
    refSessionList.value = []
  }

  const toRefPayload = () => ({
    ref_character_ulid_list: refCharacterList.value.map((item) => item.ref_ulid),
    ref_lore_ulid_list: refLoreList.value.map((item) => item.ref_ulid),
    ref_chapter_ulid_list: refChapterList.value.map((item) => item.ref_ulid),
    ref_summary_ulid_list: refSummaryList.value.map((item) => item.ref_ulid),
    ref_doc_ulid_list: refDocList.value.map((item) => item.ref_ulid),
    ref_session_ulid_list: refSessionList.value.map((item) => item.ref_ulid)
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
    togglePicker,
    closePicker,
    toggleRefItem,
    removeRefItem,
    clearRefs,
    toRefPayload,
    pushChapterRef,
    switchTab
  }
}

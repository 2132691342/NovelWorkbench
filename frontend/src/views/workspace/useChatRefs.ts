// @ 引用的状态与候选装配：五类引用（文档/章节/摘要/提示词/历史消息）的选中态、
// 候选列表、页签切换与按需加载，从 ChatComposer 中独立出来便于复用与测试。

import { computed, ref, type Ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { listSummaries, type NWChapterSummaryVo } from '@/api/continuity'
import { listKnowledgeDocuments } from '@/api/knowledge'
import type {
  NWChapterBriefVo,
  NWKnowledgeDocumentVo,
  NWPromptProfileVo,
} from '@/api/contracts'
import type { NWChatDisplayMessage, NWChatRefItem, NWChatRefTabCode } from './chatTypes'

/** @ 候选里的历史消息最大条数（过长会话只取最近若干条，避免弹层卡顿） */
const AT_MESSAGE_LIMIT = 40

export interface NWUseChatRefsOptions {
  projectUlid: () => string
  chapterList: () => NWChapterBriefVo[]
  messageList: () => NWChatDisplayMessage[]
  profileList: Ref<NWPromptProfileVo[]>
}

/** 引用选择器的状态与派生数据 */
export function useChatRefs(options: NWUseChatRefsOptions) {
  const { t } = useI18n()

  const refDocList = ref<NWChatRefItem[]>([])
  const refChapterList = ref<NWChatRefItem[]>([])
  const refSummaryList = ref<NWChatRefItem[]>([])
  const refPromptList = ref<NWChatRefItem[]>([])
  const refMessageList = ref<NWChatRefItem[]>([])
  const knowledgeList = ref<NWKnowledgeDocumentVo[]>([])
  const summaryList = ref<NWChapterSummaryVo[]>([])
  const refPickerVisible = ref(false)
  const refPickerPinned = ref(false)
  const refPickerTab = ref<NWChatRefTabCode>('doc')
  const refPickerFilter = ref('')

  /** @ 候选：按当前页签取一类来源 */
  const refCandidateList = computed<NWChatRefItem[]>(() => {
    switch (refPickerTab.value) {
      case 'doc':
        return knowledgeList.value.map((docItem) => ({
          ref_ulid: docItem.doc_ulid,
          ref_name: docItem.doc_name,
          ref_desc: docItem.doc_type_code ?? '',
        }))
      case 'chapter':
        return options.chapterList().map((chapterItem) => ({
          ref_ulid: chapterItem.chapter_ulid,
          ref_name: chapterItem.chapter_name,
          ref_desc: t('workspace.ai.composer.chapterOrder', { order: chapterItem.chapter_order }),
        }))
      case 'summary':
        return summaryList.value
          .filter((summaryItem) => summaryItem.summary_text.length > 0)
          .map((summaryItem) => {
            const chapterItem = options
              .chapterList()
              .find((candidateItem) => candidateItem.chapter_ulid === summaryItem.chapter_ulid)
            return {
              ref_ulid: summaryItem.summary_ulid,
              ref_name: chapterItem?.chapter_name ?? t('workspace.ai.composer.summaryUntitled'),
              ref_desc: summaryItem.summary_text.replace(/\s+/g, ' ').slice(0, 40),
            }
          })
      case 'prompt':
        return options.profileList.value.map((profileItem) => ({
          ref_ulid: profileItem.prompt_profile_ulid,
          ref_name: profileItem.prompt_profile_name,
          ref_desc: profileItem.description_text ?? '',
        }))
      default:
        return options
          .messageList()
          .filter((message) => message.content_text.length > 0 && !message.streaming_flag)
          .slice(-AT_MESSAGE_LIMIT)
          .map((message) => ({
            ref_ulid: message.message_ulid,
            ref_name: message.content_text.replace(/\s+/g, ' ').slice(0, 40),
            ref_desc:
              message.role_code === 'user'
                ? t('workspace.ai.roleUser')
                : t('workspace.ai.roleAssistant'),
          }))
          .filter((item) => item.ref_ulid.length > 0)
    }
  })

  /** 当前页签已选中的 ULID 列表 */
  const refSelectedUlidList = computed<string[]>(() => refListOf(refPickerTab.value).value.map((item) => item.ref_ulid))

  /** @ 候选为空时的提示 */
  const refEmptyHint = computed(() => {
    if (refPickerTab.value === 'message') {
      return t('workspace.ai.composer.atMessagesEmpty')
    }
    if (refPickerTab.value === 'summary') {
      return t('workspace.ai.composer.atSummariesEmpty')
    }
    return t('workspace.ai.composer.atEmpty')
  })

  /** 按页签取对应的已选列表 */
  function refListOf(tabCode: NWChatRefTabCode): Ref<NWChatRefItem[]> {
    if (tabCode === 'chapter') {
      return refChapterList
    }
    if (tabCode === 'summary') {
      return refSummaryList
    }
    if (tabCode === 'prompt') {
      return refPromptList
    }
    if (tabCode === 'message') {
      return refMessageList
    }
    return refDocList
  }

  /** 加载本项目章节摘要（切到摘要页签时按需拉取一次） */
  async function loadSummaryList(): Promise<void> {
    if (!options.projectUlid() || summaryList.value.length > 0) {
      return
    }
    try {
      summaryList.value = await listSummaries({ project_ulid: options.projectUlid() })
    } catch {
      // 失败提示由 http 层统一弹出；摘要为空时页签显示空态
    }
  }

  /** 加载知识库文档（供 @ 引用；接口未就绪降级为空） */
  async function loadKnowledgeList(): Promise<void> {
    if (!options.projectUlid()) {
      knowledgeList.value = []
      return
    }
    try {
      const pageData = await listKnowledgeDocuments({
        project_ulid: options.projectUlid(),
        page_num: 1,
        page_size: 100,
      })
      knowledgeList.value = pageData.items ?? []
    } catch {
      knowledgeList.value = []
    }
  }

  /** 页签切换：摘要页签按需加载 */
  function switchTab(tabCode: NWChatRefTabCode): void {
    refPickerTab.value = tabCode
    if (tabCode === 'summary') {
      void loadSummaryList()
    }
  }

  /** 打开/关闭选择器（@ 按钮入口） */
  function togglePicker(): void {
    if (refPickerVisible.value) {
      closePicker()
      return
    }
    refPickerFilter.value = ''
    refPickerPinned.value = true
    refPickerVisible.value = true
  }

  /** 关闭选择器 */
  function closePicker(): void {
    refPickerVisible.value = false
    refPickerPinned.value = false
  }

  /** 多选切换：已在列表则移除，否则加入（选择器保持打开可继续多选） */
  function toggleRefItem(item: NWChatRefItem): void {
    const targetList = refListOf(refPickerTab.value)
    const existingIndex = targetList.value.findIndex(
      (refItem) => refItem.ref_ulid === item.ref_ulid,
    )
    if (existingIndex >= 0) {
      targetList.value.splice(existingIndex, 1)
      return
    }
    targetList.value.push(item)
  }

  /** 移除引用 chip */
  function removeRefItem(tabCode: NWChatRefTabCode, refUlid: string): void {
    const targetList = refListOf(tabCode)
    targetList.value = targetList.value.filter((item) => item.ref_ulid !== refUlid)
  }

  /** 清空全部引用（发送后调用） */
  function clearRefs(): void {
    refDocList.value = []
    refChapterList.value = []
    refSummaryList.value = []
    refPromptList.value = []
    refMessageList.value = []
  }

  /** 组装发送载荷里的引用字段 */
  function toRefPayload() {
    return {
      ref_doc_ulid_list: refDocList.value.map((item) => item.ref_ulid),
      ref_chapter_ulid_list: refChapterList.value.map((item) => item.ref_ulid),
      ref_summary_ulid_list: refSummaryList.value.map((item) => item.ref_ulid),
      ref_prompt_ulid_list: refPromptList.value.map((item) => item.ref_ulid),
      ref_message_ulid_list: refMessageList.value.map((item) => item.ref_ulid),
    }
  }

  /** 追加章节引用（外部预置技能入口用） */
  function pushChapterRef(chapterUlid: string, refName: string, refDesc: string): void {
    if (refChapterList.value.some((item) => item.ref_ulid === chapterUlid)) {
      return
    }
    refChapterList.value.push({ ref_ulid: chapterUlid, ref_name: refName, ref_desc: refDesc })
  }

  return {
    refDocList,
    refChapterList,
    refSummaryList,
    refPromptList,
    refMessageList,
    knowledgeList,
    summaryList,
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
  }
}

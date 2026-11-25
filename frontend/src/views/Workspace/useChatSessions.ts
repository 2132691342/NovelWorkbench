// 会话与历史的装配逻辑：会话列表、当前会话、消息列表的加载与增删。
// 与面板的流式渲染分离，本文件只管「有哪些会话、当前是哪个、消息有哪些」。

import { ref, type Ref } from 'vue'
import { ElMessageBox } from 'element-plus'
import {
  createChatSession,
  deleteChatSession,
  getChatHistory,
  listChatSessions
} from '@/api/chat'
import type { NWChatSessionVo } from '@/api/contracts'
import type { ChatDisplayMessage } from './types'

/** 会话与历史的管理器 */
export interface ChatSessionManager {
  sessionList: Ref<NWChatSessionVo[]>
  currentSessionUlid: Ref<string>
  messageList: Ref<ChatDisplayMessage[]>
  historyLoading: Ref<boolean>
  loadHistory: (sessionUlid: string) => Promise<void>
  loadSessions: () => Promise<void>
  createSession: () => Promise<void>
  deleteSession: () => Promise<void>
}

/** 把后端消息视图对象转成面板展示态 */
const toDisplayMessage = (message: {
  message_ulid: string
  role_code: string
  content_text: string
  reasoning_text?: string
  prompt_code: string
  tool_code_list?: string[]
  created_at: string
}): ChatDisplayMessage => ({
  message_ulid: message.message_ulid,
  role_code: message.role_code,
  content_text: message.content_text,
  reasoning_text: message.reasoning_text ?? '',
  prompt_code: message.prompt_code,
  tool_code_list: message.tool_code_list ?? [],
  created_at: message.created_at,
  reference_list: [],
  error_message: '',
  streaming_flag: false,
  waited_ms: 0,
  retry_payload: null
})

/**
 * useChatSessions 构造会话管理器。
 * @param projectUlid 所属项目 ULID
 * @param onLoaded 历史加载完成后的回调（用于滚动到底部）
 */
export function useChatSessions(projectUlid: string, onLoaded?: () => void): ChatSessionManager {
  const sessionList = ref<NWChatSessionVo[]>([])
  const currentSessionUlid = ref('')
  const messageList = ref<ChatDisplayMessage[]>([])
  const historyLoading = ref(false)

  const loadHistory = async (sessionUlid: string): Promise<void> => {
    currentSessionUlid.value = sessionUlid
    historyLoading.value = true
    try {
      const page = await getChatHistory({ session_ulid: sessionUlid, page_num: 1, page_size: 50 })
      messageList.value = (page.items ?? []).map(toDisplayMessage)
    } catch {
      messageList.value = []
    } finally {
      historyLoading.value = false
      onLoaded?.()
    }
  }

  const loadSessions = async (): Promise<void> => {
    try {
      sessionList.value = await listChatSessions(projectUlid)
      if (!currentSessionUlid.value && sessionList.value.length > 0) {
        await loadHistory(sessionList.value[0].session_ulid)
      }
      if (sessionList.value.length === 0) {
        currentSessionUlid.value = ''
        messageList.value = []
      }
    } catch {
      sessionList.value = []
    }
  }

  const createSession = async (): Promise<void> => {
    const session = await createChatSession({ project_ulid: projectUlid })
    sessionList.value.unshift(session)
    await loadHistory(session.session_ulid)
  }

  const deleteSession = async (): Promise<void> => {
    if (!currentSessionUlid.value) return
    try {
      await ElMessageBox.confirm('将删除该会话及其全部消息，确定吗？', '删除会话', { type: 'warning' })
    } catch {
      return
    }
    await deleteChatSession(currentSessionUlid.value)
    messageList.value = []
    currentSessionUlid.value = ''
    await loadSessions()
  }

  return {
    sessionList,
    currentSessionUlid,
    messageList,
    historyLoading,
    loadHistory,
    loadSessions,
    createSession,
    deleteSession
  }
}

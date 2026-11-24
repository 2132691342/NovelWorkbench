// AI 会话 / 消息接口封装（SSE 发送走 composables/useSSE.ts，这里只管普通请求）
import { post } from '@/api/http'
import type {
  NWChatCompactVo,
  NWChatHistoryReqVo,
  NWChatMessageVo,
  NWChatSessionCreateReqVo,
  NWChatSessionVo,
  NWPageDto,
} from './contracts'

/** 新建 AI 会话 */
export function createChatSession(req: NWChatSessionCreateReqVo): Promise<NWChatSessionVo> {
  return post<NWChatSessionVo>('/chat/session/create', req)
}

/** 会话列表 */
export function listChatSessions(projectUlid: string): Promise<NWChatSessionVo[]> {
  return post<NWChatSessionVo[]>('/chat/session/list', { project_ulid: projectUlid })
}

/** 删除会话（级联删消息） */
export async function deleteChatSession(sessionUlid: string): Promise<void> {
  await post('/chat/session/delete', { session_ulid: sessionUlid })
}

/** 压缩会话历史（本地实现，不耗模型） */
export function compactChatSession(sessionUlid: string): Promise<NWChatCompactVo> {
  return post<NWChatCompactVo>('/chat/session/compact', { session_ulid: sessionUlid })
}

/** 会话消息分页历史 */
export function getChatHistory(req: NWChatHistoryReqVo): Promise<NWPageDto<NWChatMessageVo>> {
  return post<NWPageDto<NWChatMessageVo>>('/chat/history', req)
}

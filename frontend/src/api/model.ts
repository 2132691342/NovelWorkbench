// 模型供应商 / 用量 / 系统状态接口封装（字段对齐后端 domain 契约）
import { get, post } from '@/api/http'

/** 协议编码 */
export type NWProtocolCode = 'openai' | 'anthropic'

/** 供应商视图对象（字段对齐后端 ModelProviderVO） */
export interface NWModelProviderVo {
  provider_ulid: string
  provider_name: string
  protocol_code: NWProtocolCode
  base_url: string
  api_key_masked: string
  api_key_configured: boolean
  chat_model_name: string
  temperature: number
  max_tokens: number
  enabled_flag: boolean
  default_flag: boolean
  sort_order: number
  remark_text: string
  last_test_status_code: string
  last_test_message: string
  last_test_at_ms: number
  updated_at_ms: number
}

/** 供应商保存请求（字段对齐后端 ModelProviderREQ） */
export interface NWModelProviderSaveReqVo {
  provider_ulid?: string
  provider_name: string
  protocol_code: NWProtocolCode
  base_url: string
  api_key_text?: string
  chat_model_name: string
  temperature?: number
  max_tokens?: number
  enabled_flag?: boolean
  default_flag?: boolean
  sort_order?: number
  remark_text?: string
}

/** 供应商连通性测试结果（字段对齐后端 ModelProviderTestDTO） */
export interface NWModelProviderTestVo {
  status_code: string
  message_text: string
  latency_ms: number
  reply_text: string
}

/** AI 能力探测结果（字段对齐后端 ModelCapabilityVO） */
export interface NWModelCapabilityVo {
  chat_ready: boolean
  default_provider_ulid: string
  default_provider_name: string
  default_chat_model_name: string
  hint_text: string
}

/** AI 面板选项（供应商列表 + 能力） */
export interface NWChatOptionsVo {
  capability_vo: NWModelCapabilityVo
  provider_list: NWModelProviderVo[]
}

/** 用量趋势点 */
export interface NWModelUsageDailyVo {
  stat_date: string
  call_count: number
  fail_count: number
  total_tokens: number
  completion_tokens: number
  avg_latency_ms: number
}

/** 用量分组项 */
export interface NWModelUsageGroupVo {
  group_name: string
  call_count: number
  fail_count: number
  total_tokens: number
  avg_latency_ms: number
  success_rate_value: number
}

/** 用量总览（字段对齐后端 ModelUsageOverviewVO） */
export interface NWModelUsageOverviewVo {
  call_count_total: number
  fail_count_total: number
  total_tokens_total: number
  avg_latency_ms: number
  success_rate_value: number
  daily_list: NWModelUsageDailyVo[]
  provider_list: NWModelUsageGroupVo[]
  model_list: NWModelUsageGroupVo[]
}

/** 依赖健康项 */
export interface NWDependencyHealthVo {
  dependency_code: string
  dependency_name: string
  status_code: string
  message_text: string
  latency_ms: number
}

/** 系统运行状态 */
export interface NWSystemStatusVo {
  server_started_at_ms: number
  server_uptime_sec: number
  go_version_text: string
  build_version_text: string
  dependency_list: NWDependencyHealthVo[]
  chat_ready: boolean
  hint_text: string
}

/** 供应商列表 */
export function listModelProviders(): Promise<NWModelProviderVo[]> {
  return post<NWModelProviderVo[]>('/model/provider/list', {})
}

/** 保存供应商（有 provider_ulid 为更新） */
export function saveModelProvider(req: NWModelProviderSaveReqVo): Promise<NWModelProviderVo> {
  return post<NWModelProviderVo>('/model/provider/save', req)
}

/** 删除供应商 */
export async function deleteModelProvider(providerUlid: string): Promise<void> {
  await post('/model/provider/delete', { provider_ulid: providerUlid })
}

/** 连通性测试 */
export function testModelProvider(providerUlid: string): Promise<NWModelProviderTestVo> {
  return post<NWModelProviderTestVo>('/model/provider/test', { provider_ulid: providerUlid })
}

/** 当前 AI 能力 */
export function getModelCapability(): Promise<NWModelCapabilityVo> {
  return get<NWModelCapabilityVo>('/model/capability')
}

/** AI 面板选项（供应商列表 + 能力） */
export function getChatOptions(): Promise<NWChatOptionsVo> {
  return get<NWChatOptionsVo>('/model/chat_options')
}

/** 用量总览 */
export function getModelUsageOverview(
  fromDate?: string,
  toDate?: string,
): Promise<NWModelUsageOverviewVo> {
  const query = new URLSearchParams()
  if (fromDate) {
    query.set('from_date', fromDate)
  }
  if (toDate) {
    query.set('to_date', toDate)
  }
  const suffix = query.toString() ? `?${query.toString()}` : ''
  return get<NWModelUsageOverviewVo>(`/model/usage/overview${suffix}`)
}

/** 系统运行状态 */
export function getSystemStatus(): Promise<NWSystemStatusVo> {
  return get<NWSystemStatusVo>('/system/status')
}

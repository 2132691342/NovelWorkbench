// 模型生态接口封装：供应商、模型目录、角色分配、连通性测试、能力探测与用量。
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
  enabled_flag: boolean
  sort_order: number
  remark_text: string
  model_count: number
  enabled_model_count: number
  last_test_status_code: string
  last_test_message: string
  last_test_at: string
  updated_at: string
}

/** 供应商保存请求（字段对齐后端 ModelProviderREQ） */
export interface NWModelProviderSaveReqVo {
  provider_ulid?: string
  provider_name: string
  protocol_code: NWProtocolCode
  base_url: string
  api_key_text?: string
  enabled_flag?: boolean
  sort_order?: number
  remark_text?: string
}

/** 模型视图对象（字段对齐后端 ModelCatalogVO） */
export interface NWModelCatalogVo {
  model_ulid: string
  provider_ulid: string
  provider_name: string
  protocol_code: NWProtocolCode
  base_url: string
  provider_enabled: boolean
  model_name: string
  model_label: string
  temperature: number
  max_tokens: number
  context_window_count: number
  enabled_flag: boolean
  default_flag: boolean
  sort_order: number
  remark_text: string
  role_ulid_list: string[]
  role_name_list: string[]
  user_ulid_list: string[]
  user_label_list: string[]
  last_test_status_code: string
  last_test_message: string
  last_test_at: string
  updated_at: string
}

/** 模型保存请求（字段对齐后端 ModelCatalogREQ） */
export interface NWModelCatalogSaveReqVo {
  model_ulid?: string
  provider_ulid: string
  model_name: string
  model_label?: string
  temperature?: number
  max_tokens?: number
  context_window_count?: number
  enabled_flag?: boolean
  default_flag?: boolean
  sort_order?: number
  remark_text?: string
  role_ulid_list?: string[]
  user_ulid_list?: string[]
}

/** 连通性测试结果 */
export interface NWModelTestVo {
  status_code: string
  message_text: string
  latency_ms: number
  reply_text: string
  probe_model_id?: string
}

/** 对话可用模型选项 */
export interface NWModelOptionVo {
  model_ulid: string
  model_name: string
  model_label: string
  provider_name: string
  protocol_code: NWProtocolCode
  default_flag: boolean
  max_tokens: number
  context_window: number
  last_test_status: string
}

/** 当前用户的 AI 能力探测结果 */
export interface NWModelCapabilityVo {
  chat_ready: boolean
  default_model_ulid: string
  default_model_label: string
  default_model_name: string
  default_provider_name: string
  option_list: NWModelOptionVo[]
  hint_text: string
}

/** 用量趋势点 */
export interface NWModelUsageDailyVo {
  stat_date: string
  call_count: number
  fail_count: number
  /** 网关未返回 token 的调用数 */
  unknown_usage_count?: number
  total_tokens: number
  completion_tokens: number
  avg_latency_ms: number
}

/** 用量分组项 */
export interface NWModelUsageGroupVo {
  group_name: string
  call_count: number
  fail_count: number
  /** 网关未返回 token 的调用数 */
  unknown_usage_count?: number
  total_tokens: number
  avg_latency_ms: number
  success_rate_value: number
}

/** 用量总览 */
export interface NWModelUsageOverviewVo {
  call_count_total: number
  fail_count_total: number
  /** 网关未返回 token 的调用数 */
  unknown_usage_count?: number
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
  server_started_at: string
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

/** 供应商连通性测试 */
export function testModelProvider(providerUlid: string): Promise<NWModelTestVo> {
  return post<NWModelTestVo>('/model/provider/test', { provider_ulid: providerUlid })
}

/** 模型目录（provider_ulid 为空查全部） */
export function listModelCatalogs(providerUlid = ''): Promise<NWModelCatalogVo[]> {
  return post<NWModelCatalogVo[]>('/model/catalog/list', { provider_ulid: providerUlid })
}

/** 保存模型（含角色授权整体覆盖） */
export function saveModelCatalog(req: NWModelCatalogSaveReqVo): Promise<NWModelCatalogVo> {
  return post<NWModelCatalogVo>('/model/catalog/save', req)
}

/** 删除模型 */
export async function deleteModelCatalog(modelUlid: string): Promise<void> {
  await post('/model/catalog/delete', { model_ulid: modelUlid })
}

/** 模型连通性测试 */
export function testModelCatalog(modelUlid: string): Promise<NWModelTestVo> {
  return post<NWModelTestVo>('/model/catalog/test', { model_ulid: modelUlid })
}

/** 设为系统默认模型 */
export async function setDefaultModelCatalog(modelUlid: string): Promise<void> {
  await post('/model/catalog/set_default', { model_ulid: modelUlid })
}

/** 整体覆盖模型的角色授权 */
export async function grantModelCatalog(
  modelUlid: string,
  roleUlidList: string[],
  userUlidList: string[]
): Promise<void> {
  await post('/model/catalog/grant', {
    model_ulid: modelUlid,
    role_ulid_list: roleUlidList,
    user_ulid_list: userUlidList
  })
}

/** 当前用户的 AI 能力（含可用模型选项） */
export function getModelCapability(): Promise<NWModelCapabilityVo> {
  return get<NWModelCapabilityVo>('/model/capability')
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

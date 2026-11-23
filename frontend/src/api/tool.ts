// 工具接口封装：列表、保存、删除与试运行（字段对齐后端 ToolVO / ToolInvokeDTO）。
import { get, post } from '@/api/http'

/** 工具参数声明 */
export interface NWToolParamVo {
  param_name: string
  param_label: string
  required_flag: boolean
  default_value: string
  description_text: string
}

/** 工具视图对象（字段对齐后端 ToolVO） */
export interface NWToolVo {
  tool_ulid: string
  tool_code: string
  tool_name: string
  tool_type_code: string
  description_text: string
  trigger_keywords: string
  param_list: NWToolParamVo[]
  enabled_flag: boolean
  builtin_flag: boolean
  customized_flag: boolean
  sort_order: number
  created_at: string
  updated_at: string
}

/** 工具保存请求（字段对齐后端 ToolREQ） */
export interface NWToolSaveReqVo {
  tool_ulid?: string
  tool_code: string
  tool_name: string
  description_text?: string
  trigger_keywords?: string
  param_list?: NWToolParamVo[]
  enabled_flag?: boolean
  sort_order?: number
}

/** 工具试运行请求（字段对齐后端 ToolInvokeREQ） */
export interface NWToolInvokeReqVo {
  project_ulid?: string
  tool_code: string
  chapter_ulid?: string
  content_text?: string
  param_value?: Record<string, string>
}

/** 工具试运行结果（字段对齐后端 ToolInvokeDTO） */
export interface NWToolInvokeVo {
  tool_code: string
  tool_name: string
  result_text: string
  metric_map: Record<string, string>
  latency_ms: number
  error_message: string
}

/** 工具列表 */
export function listTools(params?: {
  keyword_text?: string
  enabled_only?: boolean
}): Promise<NWToolVo[]> {
  return get<NWToolVo[]>('/tool/list', { params })
}

/** 保存工具 */
export function saveTool(req: NWToolSaveReqVo): Promise<NWToolVo> {
  return post<NWToolVo>('/tool/save', req)
}

/** 删除工具 */
export async function deleteTool(toolUlid: string): Promise<void> {
  await post<null>('/tool/delete', { tool_ulid: toolUlid })
}

/** 试运行工具 */
export function invokeTool(req: NWToolInvokeReqVo): Promise<NWToolInvokeVo> {
  return post<NWToolInvokeVo>('/tool/invoke', req)
}

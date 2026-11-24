// 提示词接口封装：提示词库 CRUD 与斜杠命令解析。

import { get, post } from '@/api/http'
import type { NWPromptResolveVo, NWPromptVo } from './contracts'

/** 提示词列表查询请求 */
export interface NWPromptListReqVo {
  scope_code?: string
  project_ulid?: string
  prompt_kind_code?: string
  category_code?: string
  keyword_text?: string
  enabled_only?: boolean
}

/** 提示词保存请求（prompt_ulid 为空表示新建） */
export interface NWPromptSaveReqVo {
  prompt_ulid?: string
  prompt_code: string
  prompt_name: string
  prompt_kind_code?: string
  scope_code?: string
  project_ulid?: string
  category_code?: string
  description_text?: string
  prompt_text: string
  enabled_flag?: boolean
  sort_order?: number
}

/** 提示词解析请求（对话前置：把 /编码 解析成提示词正文） */
export interface NWPromptResolveReqVo {
  project_ulid: string
  content_text: string
  prompt_code?: string
}

/** 提示词列表（带 project_ulid 时合并项目提示词与全局提示词） */
export function listPrompts(req: NWPromptListReqVo): Promise<NWPromptVo[]> {
  const query = new URLSearchParams()
  if (req.scope_code) query.set('scope_code', req.scope_code)
  if (req.project_ulid) query.set('project_ulid', req.project_ulid)
  if (req.prompt_kind_code) query.set('prompt_kind_code', req.prompt_kind_code)
  if (req.category_code) query.set('category_code', req.category_code)
  if (req.keyword_text) query.set('keyword_text', req.keyword_text)
  if (req.enabled_only) query.set('enabled_only', 'true')
  const suffix = query.toString()
  return get<NWPromptVo[]>(`/prompt/list${suffix ? `?${suffix}` : ''}`)
}

/** 提示词详情 */
export function getPromptDetail(promptUlid: string): Promise<NWPromptVo> {
  return get<NWPromptVo>(`/prompt/detail/${encodeURIComponent(promptUlid)}`)
}

/** 保存提示词 */
export function savePrompt(req: NWPromptSaveReqVo): Promise<NWPromptVo> {
  return post<NWPromptVo>('/prompt/save', req)
}

/** 删除提示词 */
export function deletePrompt(promptUlid: string): Promise<null> {
  return post<null>('/prompt/delete', { prompt_ulid: promptUlid })
}

/** 解析斜杠命令（对话前置预览） */
export function resolvePrompt(req: NWPromptResolveReqVo): Promise<NWPromptResolveVo> {
  return post<NWPromptResolveVo>('/prompt/resolve', req)
}

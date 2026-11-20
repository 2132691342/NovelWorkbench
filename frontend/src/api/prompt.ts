// 提示词档案接口封装：列表、详情、保存与删除。

import { get, post } from '@/api/http'
import type { NWPromptProfileVo } from './contracts'

/** 档案列表查询请求 */
export interface NWPromptProfileListReqVo {
  project_ulid?: string
  enabled_only?: boolean
}

/** 档案保存请求（prompt_profile_ulid 为空表示新建） */
export interface NWPromptProfileSaveReqVo {
  prompt_profile_ulid?: string
  prompt_profile_code: string
  prompt_profile_name: string
  scope_code?: string
  project_ulid?: string
  description_text?: string
  system_prompt_text: string
  enabled_flag?: boolean
  sort_order?: number
}

/** 档案列表（带 project_ulid 时合并项目档案与全局档案） */
export function listPromptProfiles(req: NWPromptProfileListReqVo = {}): Promise<NWPromptProfileVo[]> {
  return post<NWPromptProfileVo[]>('/prompt/profile/list', req)
}

/** 档案详情 */
export function getPromptProfileDetail(promptProfileUlid: string): Promise<NWPromptProfileVo> {
  return get<NWPromptProfileVo>(`/prompt/profile/detail/${encodeURIComponent(promptProfileUlid)}`)
}

/** 保存档案 */
export function savePromptProfile(req: NWPromptProfileSaveReqVo): Promise<NWPromptProfileVo> {
  return post<NWPromptProfileVo>('/prompt/profile/save', req)
}

/** 删除档案 */
export function deletePromptProfile(promptProfileUlid: string): Promise<null> {
  return post<null>('/prompt/profile/delete', { prompt_profile_ulid: promptProfileUlid })
}

// 技能接口封装：技能 CRUD、变量解析与参考资料管理。

import { get, post } from '@/api/http'
import type {
  NWSkillReferenceVo,
  NWSkillResolveVo,
  NWSkillVo,
  NWSkillVariableVo,
} from './contracts'

// 请求对象

/** 技能列表查询请求 */
export interface NWSkillListReqVo {
  scope_code?: string
  project_ulid?: string
  category_code?: string
  keyword_text?: string
  enabled_only?: boolean
}

/** 技能保存请求（skill_ulid 为空表示新建） */
export interface NWSkillSaveReqVo {
  skill_ulid?: string
  skill_code: string
  skill_name: string
  scope_code?: string
  project_ulid?: string
  category_code?: string
  description_text?: string
  prompt_template: string
  variable_list?: NWSkillVariableVo[]
  enabled_flag?: boolean
  sort_order?: number
}

/** 参考资料保存请求 */
export interface NWSkillReferenceSaveReqVo {
  ref_ulid?: string
  skill_ulid: string
  ref_name: string
  ref_text: string
  keyword_list?: string
  sort_order?: number
}

/** 技能解析请求 */
export interface NWSkillResolveReqVo {
  project_ulid: string
  content_text: string
  skill_code?: string
  variable_value?: Record<string, string>
}

// 接口

/** 技能列表（带 project_ulid 时合并项目技能与全局技能） */
export function listSkills(req: NWSkillListReqVo): Promise<NWSkillVo[]> {
  const query = new URLSearchParams()
  if (req.scope_code) {
    query.set('scope_code', req.scope_code)
  }
  if (req.project_ulid) {
    query.set('project_ulid', req.project_ulid)
  }
  if (req.category_code) {
    query.set('category_code', req.category_code)
  }
  if (req.keyword_text) {
    query.set('keyword_text', req.keyword_text)
  }
  if (req.enabled_only) {
    query.set('enabled_only', 'true')
  }
  const suffix = query.toString()
  return get<NWSkillVo[]>(`/skill/list${suffix ? `?${suffix}` : ''}`)
}

/** 技能详情 */
export function getSkillDetail(skillUlid: string): Promise<NWSkillVo> {
  return get<NWSkillVo>(`/skill/detail/${encodeURIComponent(skillUlid)}`)
}

/** 保存技能 */
export function saveSkill(req: NWSkillSaveReqVo): Promise<NWSkillVo> {
  return post<NWSkillVo>('/skill/save', req)
}

/** 删除技能 */
export function deleteSkill(skillUlid: string): Promise<null> {
  return post<null>('/skill/delete', { skill_ulid: skillUlid })
}

/** 解析技能与变量（对话前置） */
export function resolveSkill(req: NWSkillResolveReqVo): Promise<NWSkillResolveVo> {
  return post<NWSkillResolveVo>('/skill/resolve', req)
}

/** 技能参考资料列表 */
export function listSkillReferences(skillUlid: string): Promise<NWSkillReferenceVo[]> {
  return get<NWSkillReferenceVo[]>(`/skill/reference/list?skill_ulid=${encodeURIComponent(skillUlid)}`)
}

/** 保存技能参考资料 */
export function saveSkillReference(req: NWSkillReferenceSaveReqVo): Promise<NWSkillReferenceVo> {
  return post<NWSkillReferenceVo>('/skill/reference/save', req)
}

/** 删除技能参考资料 */
export function deleteSkillReference(refUlid: string): Promise<null> {
  return post<null>('/skill/reference/delete', { ref_ulid: refUlid })
}

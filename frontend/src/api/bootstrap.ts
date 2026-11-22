// AI 开书接口封装：一句创意生成方案，确认后一次性落库。

import { get, post } from '@/api/http'
import type { NWProjectVo } from './contracts'

/** 开书章节条目 */
export interface NWBootstrapChapterVo {
  chapter_name: string
  summary_text: string
  chapter_order: number
}

/** 开书卷条目 */
export interface NWBootstrapVolumeVo {
  volume_name: string
  summary_text: string
  volume_order: number
  chapter_list: NWBootstrapChapterVo[]
}

/** 开书角色卡 */
export interface NWBootstrapCharacterVo {
  character_name: string
  role_type_code: string
  personality_text: string
  goal_text: string
  background_text: string
}

/** 开书世界观设定 */
export interface NWBootstrapLoreVo {
  lore_name: string
  lore_type_code: string
  content_text: string
}

/** 开书方案（字段对齐后端 ProjectBootstrapPlanVO） */
export interface NWBootstrapPlanVo {
  project_name: string
  genre_code: string
  selling_point_text: string
  synopsis_text: string
  style_text: string
  volume_list: NWBootstrapVolumeVo[]
  character_list: NWBootstrapCharacterVo[]
  lore_list: NWBootstrapLoreVo[]
}

/** 开书落库结果 */
export interface NWBootstrapResultVo {
  project_vo: NWProjectVo
  volume_count: number
  chapter_count: number
  character_count: number
  lore_count: number
}

/** 生成开书方案请求 */
export interface NWBootstrapPlanReqVo {
  idea_text: string
  genre_code?: string
  target_chapter_count?: number
  model_ulid?: string
}

/** 生成开书方案（不落库，供预览与修改）：长结构化输出，超时与后端 300s 对齐 */
export function generateBootstrapPlan(req: NWBootstrapPlanReqVo): Promise<NWBootstrapPlanVo> {
  return post<NWBootstrapPlanVo>('/project/bootstrap/plan', req, { timeout: 300000 })
}

/** 按方案落库 */
export function applyBootstrapPlan(plan: NWBootstrapPlanVo): Promise<NWBootstrapResultVo> {
  return post<NWBootstrapResultVo>('/project/bootstrap/apply', { plan }, { timeout: 60000 })
}

/** 当前账号的可用模型（开书时可选模型） */
export function fetchBootstrapModels(): Promise<{ option_list: { model_ulid: string; model_label: string }[] }> {
  return get('/model/capability')
}

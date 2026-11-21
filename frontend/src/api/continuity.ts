// 连续性看板接口封装（字段对齐后端 domain/continuity.go）
import { post } from '@/api/http'

/** 时间事实状态联合类型 */
export type NWFactStatusCode = 'candidate' | 'confirmed' | 'superseded' | 'stale'

/** 伏笔钩子状态联合类型 */
export type NWHookStatusCode = 'planned' | 'planted' | 'echoed' | 'resolved' | 'abandoned'

/** 重要程度联合类型 */
export type NWImportanceCode = 'high' | 'medium' | 'low'

/** 章节摘要状态联合类型 */
export type NWSummaryStatusCode = 'pending' | 'verified' | 'stale'

/** 时间事实视图对象（对齐 TemporalFactVO） */
export interface NWTemporalFactVo {
  fact_ulid: string
  project_ulid: string
  subject_text: string
  predicate_code: string
  object_text: string
  fact_status_code: NWFactStatusCode
  valid_from_chapter_ulid: string
  valid_until_chapter_ulid: string
  source_chapter_ulid: string
  source_quote_text: string
  note_text: string
  updated_at: string
}

/** 时间事实保存请求（对齐 TemporalFactSaveREQ，有 fact_ulid 为更新） */
export interface NWTemporalFactSaveReqVo {
  fact_ulid?: string
  project_ulid: string
  subject_text: string
  predicate_code: string
  object_text?: string
  fact_status_code?: NWFactStatusCode
  valid_from_chapter_ulid?: string
  valid_until_chapter_ulid?: string
  source_chapter_ulid: string
  source_quote_text?: string
  note_text?: string
}

/** 时间事实复核请求（对齐 TemporalFactReviewREQ） */
export interface NWTemporalFactReviewReqVo {
  fact_ulid: string
  project_ulid: string
  fact_status_code: Extract<NWFactStatusCode, 'confirmed' | 'superseded' | 'stale'>
  note_text?: string
}

/** 时间事实列表查询请求（对齐 TemporalFactListREQ） */
export interface NWTemporalFactListReqVo {
  project_ulid: string
  fact_status_code?: NWFactStatusCode
  chapter_ulid?: string
}

/** 伏笔钩子视图对象（对齐 ForeshadowHookVO） */
export interface NWForeshadowHookVo {
  hook_ulid: string
  project_ulid: string
  hook_status_code: NWHookStatusCode
  description_text: string
  expected_payoff_text: string
  setup_chapter_ulid: string
  payoff_chapter_ulid: string
  importance_code: NWImportanceCode
  note_text: string
  updated_at: string
}

/** 伏笔钩子保存请求（对齐 ForeshadowHookSaveREQ，状态随显式保存推进） */
export interface NWForeshadowHookSaveReqVo {
  hook_ulid?: string
  project_ulid: string
  hook_status_code: NWHookStatusCode
  description_text: string
  expected_payoff_text?: string
  setup_chapter_ulid?: string
  payoff_chapter_ulid?: string
  importance_code?: NWImportanceCode
  note_text?: string
}

/** 伏笔钩子列表查询请求（对齐 ForeshadowHookListREQ） */
export interface NWForeshadowHookListReqVo {
  project_ulid: string
  hook_status_code?: NWHookStatusCode
}

/** 章节摘要视图对象（对齐 ChapterSummaryVO） */
export interface NWChapterSummaryVo {
  summary_ulid: string
  project_ulid: string
  chapter_ulid: string
  summary_text: string
  source_hash_text: string
  summary_status_code: NWSummaryStatusCode
  updated_at: string
}

/** 章节摘要保存请求（对齐 ChapterSummarySaveREQ） */
export interface NWChapterSummarySaveReqVo {
  summary_ulid?: string
  project_ulid: string
  chapter_ulid: string
  summary_text: string
  source_hash_text?: string
  summary_status_code?: NWSummaryStatusCode
}

/** 章节摘要列表查询请求（对齐 ChapterSummaryListREQ） */
export interface NWChapterSummaryListReqVo {
  project_ulid: string
  summary_status_code?: NWSummaryStatusCode
  chapter_ulid?: string
}

/** 新建/更新时间事实 */
export function saveFact(req: NWTemporalFactSaveReqVo): Promise<NWTemporalFactVo> {
  return post<NWTemporalFactVo>('/continuity/fact/save', req)
}

/** 复核时间事实状态 */
export function reviewFact(req: NWTemporalFactReviewReqVo): Promise<NWTemporalFactVo> {
  return post<NWTemporalFactVo>('/continuity/fact/review', req)
}

/** 查询时间事实列表 */
export function listFacts(req: NWTemporalFactListReqVo): Promise<NWTemporalFactVo[]> {
  return post<NWTemporalFactVo[]>('/continuity/fact/list', req)
}

/** 新建/更新伏笔钩子 */
export function saveHook(req: NWForeshadowHookSaveReqVo): Promise<NWForeshadowHookVo> {
  return post<NWForeshadowHookVo>('/continuity/hook/save', req)
}

/** 查询伏笔钩子列表 */
export function listHooks(req: NWForeshadowHookListReqVo): Promise<NWForeshadowHookVo[]> {
  return post<NWForeshadowHookVo[]>('/continuity/hook/list', req)
}

/** 新建/更新章节摘要 */
export function saveSummary(req: NWChapterSummarySaveReqVo): Promise<NWChapterSummaryVo> {
  return post<NWChapterSummaryVo>('/continuity/summary/save', req)
}

/** 查询章节摘要列表 */
export function listSummaries(req: NWChapterSummaryListReqVo): Promise<NWChapterSummaryVo[]> {
  return post<NWChapterSummaryVo[]>('/continuity/summary/list', req)
}

// 工作流接口封装（字段对齐后端 domain/workflow.go）
import { get, post } from '@/api/http'
import type { NWPageDto } from './contracts'

/** 步骤类型（决定图标与说明） */
export type NWWorkflowStepType =
  | 'draft'
  | 'check'
  | 'revise'
  | 'review'
  | 'human_review'
  | 'summary'
  | 'apply'

/** 运行状态 */
export type NWWorkflowRunStatus =
  | 'queued'
  | 'running'
  | 'waiting_review'
  | 'succeeded'
  | 'failed'
  | 'canceled'
  | 'paused'

/** 模板步骤定义（对齐 WorkflowStepDefDTO） */
export interface NWWorkflowStepDefVo {
  step_type_code: NWWorkflowStepType
  step_name: string
  prompt_code: string
  max_attempts: number
  skip_when_pass: boolean
  human_gate_flag: boolean
}

/** 模板视图（对齐 WorkflowTemplateVO） */
export interface NWWorkflowTemplateVo {
  template_ulid: string
  template_code: string
  template_name: string
  description_text: string
  step_list: NWWorkflowStepDefVo[]
  builtin_flag: boolean
  enabled_flag: boolean
  sort_order: number
}

/** 运行视图（对齐 WorkflowRunVO） */
export interface NWWorkflowRunVo {
  run_ulid: string
  project_ulid: string
  project_name: string
  template_code: string
  template_name: string
  run_status_code: NWWorkflowRunStatus
  current_index: number
  total_chapter_count: number
  finished_chapter_count: number
  current_chapter_ulid: string
  current_chapter_name: string
  trace_id: string
  error_message: string
  feedback_text: string
  target_word_count: number
  review_gate_flag: boolean
  created_at: string
  started_at: string
  finished_at: string
}

/** 步骤视图（对齐 WorkflowStepVO） */
export interface NWWorkflowStepVo {
  step_ulid: string
  chapter_ulid: string
  chapter_name: string
  step_index: number
  step_type_code: NWWorkflowStepType
  step_name: string
  step_status_code: string
  prompt_code: string
  output_text: string
  metric_json: string
  attempt_count: number
  latency_ms: number
  error_message: string
  started_at: string
  finished_at: string
}

/** 运行详情（对齐 WorkflowRunDetailVO） */
export interface NWWorkflowRunDetailVo extends NWWorkflowRunVo {
  step_list: NWWorkflowStepVo[]
  chapter_content_text: string
  chapter_word_count: number
  check_score: number
  check_verdict_code: string
  review_verdict_code: string
  review_output_text: string
}

/** 运行概览（对齐 WorkflowRunStatsVO） */
export interface NWWorkflowRunStatsVo {
  running_count: number
  waiting_review_count: number
  finished_count: number
  failed_count: number
}

/** 创建运行请求（对齐 WorkflowRunCreateREQ） */
export interface NWWorkflowRunCreateReqVo {
  project_ulid: string
  template_code: string
  chapter_ulid_list?: string[]
  new_chapter_count?: number
  target_word_count?: number
  model_ulid?: string
  system_prompt_code?: string
  draft_prompt_code?: string
  revise_prompt_code?: string
  review_prompt_code?: string
  gate_score?: number
  review_gate_flag?: boolean
  extra_requirement_text?: string
  auto_start_flag?: boolean
}

/** 运行列表查询请求（对齐 WorkflowRunListREQ） */
export interface NWWorkflowRunListReqVo {
  page_num: number
  page_size: number
  project_ulid?: string
  run_status_code?: string
  keyword_text?: string
}

/** 可用模板列表 */
export function listWorkflowTemplates(): Promise<NWWorkflowTemplateVo[]> {
  return get<NWWorkflowTemplateVo[]>('/workflow/template/list')
}

/** 创建工作流运行 */
export function createWorkflowRun(req: NWWorkflowRunCreateReqVo): Promise<NWWorkflowRunVo> {
  return post<NWWorkflowRunVo>('/workflow/run/create', req)
}

/** 运行分页列表 */
export function listWorkflowRuns(req: NWWorkflowRunListReqVo): Promise<NWPageDto<NWWorkflowRunVo>> {
  return post<NWPageDto<NWWorkflowRunVo>>('/workflow/run/list', req)
}

/** 运行详情（含步骤时间线与当前章正文） */
export function getWorkflowRunDetail(runUlid: string): Promise<NWWorkflowRunDetailVo> {
  return get<NWWorkflowRunDetailVo>(`/workflow/run/detail/${encodeURIComponent(runUlid)}`)
}

/** 运行概览 */
export function fetchWorkflowRunStats(projectUlid?: string): Promise<NWWorkflowRunStatsVo> {
  const suffix = projectUlid ? `?project_ulid=${encodeURIComponent(projectUlid)}` : ''
  return get<NWWorkflowRunStatsVo>(`/workflow/run/stats${suffix}`)
}

/** 审核通过（写入章节并继续） */
export function approveWorkflowRun(runUlid: string, noteText?: string): Promise<NWWorkflowRunVo> {
  return post<NWWorkflowRunVo>('/workflow/run/approve', { run_ulid: runUlid, note_text: noteText })
}

/** 审核退回（带意见重跑当前章） */
export function rejectWorkflowRun(runUlid: string, feedbackText: string): Promise<NWWorkflowRunVo> {
  return post<NWWorkflowRunVo>('/workflow/run/reject', {
    run_ulid: runUlid,
    feedback_text: feedbackText
  })
}

/** 暂停运行 */
export function pauseWorkflowRun(runUlid: string): Promise<NWWorkflowRunVo> {
  return post<NWWorkflowRunVo>('/workflow/run/pause', { run_ulid: runUlid })
}

/** 继续运行 */
export function resumeWorkflowRun(runUlid: string): Promise<NWWorkflowRunVo> {
  return post<NWWorkflowRunVo>('/workflow/run/resume', { run_ulid: runUlid })
}

/** 取消运行 */
export function cancelWorkflowRun(runUlid: string): Promise<NWWorkflowRunVo> {
  return post<NWWorkflowRunVo>('/workflow/run/cancel', { run_ulid: runUlid })
}

/** 重试失败运行 */
export function retryWorkflowRun(runUlid: string): Promise<NWWorkflowRunVo> {
  return post<NWWorkflowRunVo>('/workflow/run/retry', { run_ulid: runUlid })
}

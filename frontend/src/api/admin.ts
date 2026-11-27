// 系统管理接口封装（用户 / 角色 / 审计，字段与路径对齐后端 admin 契约）
import { get, post } from '@/api/http'
import type { NWPageDto } from './contracts'

// ============================================================
// 视图对象
// ============================================================

/** 后台用户视图对象（字段对齐后端 UserAdminVO） */
export interface NWUserAdminVo {
  user_ulid: string
  user_name: string
  user_email: string
  avatar_url: string
  remark_text: string
  user_status_code: string
  role_codes: string[]
  created_at: string
  updated_at: string
}

/** 角色视图对象（字段对齐后端 RoleVO） */
export interface NWRoleVo {
  role_ulid: string
  role_code: string
  role_name: string
  remark_text: string
  perm_codes: string[]
  builtin_flag: boolean
  /** 挂载该角色的用户数（后端实时统计，删除前的判断依据） */
  user_count: number
  created_at: string
  updated_at: string
}

/** 审计日志视图对象（字段对齐后端 AuditVO） */
export interface NWAuditVo {
  audit_ulid: string
  log_level_code: string
  /** span 类型：http / workflow_step / ai_call / job / system */
  span_type_code: string
  trace_id: string
  /** 父 span 的动作码（顶级 span 为空） */
  parent_span_code: string
  user_ulid: string
  user_name: string
  action_code: string
  /** 动作中文名（后端按动作码映射，历史行可能为空） */
  action_name: string
  /** AI 调用的模型名（非 AI 为空） */
  model_name: string
  /** AI 调用消耗 token（非 AI 为 0） */
  token_count: number
  request_path: string
  request_method: string
  target_type: string
  target_ulid: string
  resp_code: string
  resp_message: string
  http_status: number
  latency_ms: number
  ip_addr: string
  user_agent_text: string
  created_at: string
}

/** 审计详情视图（含请求体原文，字段对齐后端 AuditDetailVO） */
export interface NWAuditDetailVo extends NWAuditVo {
  request_body: string
  /** 响应体（脱敏截断；空串表示流式/上传/超限未采样） */
  resp_body: string
}

// ============================================================
// 请求对象
// ============================================================

/** 用户分页查询请求（字段对齐后端 UserAdminListREQ） */
export interface NWUserAdminListReqVo {
  page_num: number
  page_size: number
  keyword_text?: string
  user_status_code?: string
  /** 角色编码过滤（空为全部角色；未知编码返回空页） */
  role_code?: string
}

/** 创建用户请求（字段对齐后端 UserAdminCreateREQ） */
export interface NWUserAdminCreateReqVo {
  user_name: string
  user_email: string
  password_text: string
  role_codes?: string[]
  remark_text?: string
}

/** 更新用户请求（字段对齐后端 UserAdminUpdateREQ；缺省字段不改，空串表示清空） */
export interface NWUserAdminUpdateReqVo {
  user_ulid: string
  user_email?: string
  avatar_url?: string
  remark_text?: string
  user_status_code?: string
  role_codes?: string[]
}

/** 重置用户密码请求（字段对齐后端 UserAdminResetPasswordREQ） */
export interface NWUserAdminResetPasswordReqVo {
  user_ulid: string
  new_password_text: string
}

/** 角色分页查询请求 */
export interface NWRoleListReqVo {
  page_num: number
  page_size: number
}

/** 保存角色请求（role_ulid 为空表示新建） */
export interface NWRoleSaveReqVo {
  role_ulid?: string
  role_code: string
  role_name: string
  remark_text?: string
  perm_codes?: string[]
}

/** 审计日志分页查询请求 */
export interface NWAuditListReqVo {
  page_num: number
  page_size: number
  log_level_code?: string
  /** span 类型过滤（空为全部） */
  span_type_code?: string
  trace_id?: string
  target_type?: string
  action_code?: string
  keyword_text?: string
  user_ulid?: string
  date_from?: string
  date_to?: string
  /** 精确起始时刻 RFC3339（优先于 date_from，快捷范围用） */
  time_from?: string
  /** 精确结束时刻 RFC3339（优先于 date_to） */
  time_to?: string
}

// ============================================================
// 用户管理
// ============================================================

/** 用户分页列表 */
export function listAdminUsers(req: NWUserAdminListReqVo): Promise<NWPageDto<NWUserAdminVo>> {
  return post<NWPageDto<NWUserAdminVo>>('/admin/user/list', req)
}

/** 创建用户 */
export function createAdminUser(req: NWUserAdminCreateReqVo): Promise<NWUserAdminVo> {
  return post<NWUserAdminVo>('/admin/user/create', req)
}

/** 更新用户 */
export function updateAdminUser(req: NWUserAdminUpdateReqVo): Promise<NWUserAdminVo> {
  return post<NWUserAdminVo>('/admin/user/update', req)
}

/** 删除用户 */
export async function deleteAdminUser(userUlid: string): Promise<void> {
  await post<null>('/admin/user/delete', { user_ulid: userUlid })
}

/** 重置用户密码 */
export function resetAdminUserPassword(
  req: NWUserAdminResetPasswordReqVo,
): Promise<null> {
  return post<null>('/admin/user/reset_password', req)
}

// ============================================================
// 角色管理
// ============================================================

/** 角色分页列表 */
export function listAdminRoles(req: NWRoleListReqVo): Promise<NWPageDto<NWRoleVo>> {
  return post<NWPageDto<NWRoleVo>>('/admin/role/list', req)
}

/** 保存角色（role_ulid 为空新建） */
export function saveAdminRole(req: NWRoleSaveReqVo): Promise<NWRoleVo> {
  return post<NWRoleVo>('/admin/role/save', req)
}

/** 删除角色（内置角色由后端拒绝） */
export async function deleteAdminRole(roleUlid: string): Promise<void> {
  await post<null>('/admin/role/delete', { role_ulid: roleUlid })
}

// ============================================================
// 审计日志
// ============================================================

/** 审计日志分页列表 */
export function listAdminAudits(req: NWAuditListReqVo): Promise<NWPageDto<NWAuditVo>> {
  return post<NWPageDto<NWAuditVo>>('/admin/audit/list', req)
}

/** 审计详情（含请求体原文） */
export function getAdminAuditDetail(auditUlid: string): Promise<NWAuditDetailVo> {
  return get<NWAuditDetailVo>(`/admin/audit/detail/${auditUlid}`)
}

// ============================================================
// 登录日志
// ============================================================

/** 登录日志视图对象（字段对齐后端 LoginLogVO） */
export interface NWLoginLogVo {
  login_log_ulid: string
  user_ulid: string
  login_account_text: string
  login_result_code: string
  fail_reason_text: string
  ip_addr: string
  user_agent_text: string
  created_at: string
}

/** 登录日志分页查询请求（字段对齐后端 LoginLogListREQ） */
export interface NWLoginLogListReqVo {
  page_num: number
  page_size: number
  keyword_text?: string
  login_result_code?: string
  date_from?: string
  date_to?: string
}

/** 登录日志分页列表 */
export function listAdminLoginLogs(req: NWLoginLogListReqVo): Promise<NWPageDto<NWLoginLogVo>> {
  return post<NWPageDto<NWLoginLogVo>>('/admin/login_log/list', req)
}

// ============================================================
// 异步任务中心
// ============================================================

/** 异步任务视图对象（字段对齐后端 JobVO） */
export interface NWJobVo {
  job_ulid: string
  job_type_code: string
  job_status_code: string
  doc_ulid: string
  doc_name: string
  retry_count: number
  error_message: string
  created_at: string
  updated_at: string
}

/** 异步任务分页查询请求（字段对齐后端 JobListREQ） */
export interface NWJobListReqVo {
  page_num: number
  page_size: number
  job_type_code?: string
  job_status_code?: string
  keyword_text?: string
}

/** 任务状态概览（字段对齐后端 JobStatsVO） */
export interface NWJobStatsVo {
  queued_count: number
  running_count: number
  success_count: number
  failed_count: number
}

/** 异步任务分页列表 */
export function listAdminJobs(req: NWJobListReqVo): Promise<NWPageDto<NWJobVo>> {
  return post<NWPageDto<NWJobVo>>('/admin/job/list', req)
}

/** 任务状态概览 */
export function fetchAdminJobStats(): Promise<NWJobStatsVo> {
  return get<NWJobStatsVo>('/admin/job/stats')
}

/** 重试失败任务 */
export function retryAdminJob(jobUlid: string): Promise<NWJobVo> {
  return post<NWJobVo>('/admin/job/retry', { job_ulid: jobUlid })
}

// ============================================================
// 审计链路 / 留存（Manticore 全文表）
// ============================================================

/** 动作码筛选项（带条数，字段对齐后端 AuditActionOptionVO） */
export interface NWAuditActionOptionVo {
  action_code: string
  /** 动作中文名（历史行可能为空） */
  action_name: string
  total: number
}

/** 链路时间线（字段对齐后端 AuditTraceVO，含请求/响应摘要） */
export interface NWAuditTraceVo {
  trace_id: string
  total: number
  item_list: NWAuditDetailVo[]
}

/** 审计留存策略（字段对齐 AuditRetentionVO） */
export interface NWAuditRetentionVo {
  retention_days: number
  /** 每日清理时刻（0–23，本地时区整点） */
  purge_hour: number
  total_count: number
  /** 登录日志总条数（与审计同一留存窗口） */
  login_log_total_count: number
  /** 本次清理删除的审计条数 */
  purged_count: number
  /** 本次清理删除的登录日志条数 */
  login_log_purged_count: number
  last_purged_at: string
  /** 下次清理时间（RFC3339，后端按配置真实计算） */
  next_purge_at: string
}

/** 链路摘要（字段对齐 AuditTraceSummaryVO：一行一次入口请求） */
export interface NWAuditTraceSummaryVo {
  trace_id: string
  user_ulid: string
  user_name: string
  root_action_code: string
  root_action_name: string
  root_request_path: string
  log_level_code: string
  http_status: number
  resp_code: string
  resp_message: string
  span_count: number
  ai_call_count: number
  token_count: number
  total_latency_ms: number
  slowest_span_ms: number
  slowest_action_name: string
  started_at: string
  finished_at: string
}

/** 链路列表查询请求（字段对齐 AuditTraceListREQ） */
export interface NWAuditTraceListReqVo {
  page_num: number
  page_size: number
  log_level_code?: string
  keyword_text?: string
  user_ulid?: string
  action_code?: string
  span_type_code?: string
  time_from?: string
  time_to?: string
  date_from?: string
  date_to?: string
}

/** 审计概览统计（字段对齐 AuditStatsVO） */
export interface NWAuditStatsVo {
  trace_count: number
  span_count: number
  error_trace_count: number
  warn_trace_count: number
  ai_call_count: number
  token_count: number
  avg_latency_ms: number
  slow_trace_count: number
}

/** 动作码筛选项 */
export function fetchAuditActionOptions(): Promise<NWAuditActionOptionVo[]> {
  return get<NWAuditActionOptionVo[]>('/admin/audit/action_options')
}

/** 按链路 ID 拉取完整调用时间线 */
export function getAdminAuditTrace(traceId: string): Promise<NWAuditTraceVo> {
  return get<NWAuditTraceVo>(`/admin/audit/trace/${encodeURIComponent(traceId)}`)
}

/** 审计留存策略现状 */
export function fetchAuditRetention(): Promise<NWAuditRetentionVo> {
  return get<NWAuditRetentionVo>('/admin/audit/retention')
}

/** 立即执行一次留存清理 */
export function purgeAdminAudit(): Promise<NWAuditRetentionVo> {
  return post<NWAuditRetentionVo>('/admin/audit/purge', {})
}

/** 链路分页列表（审计页主视图：一行一次入口请求） */
export function listAdminAuditTraces(
  req: NWAuditTraceListReqVo
): Promise<NWPageDto<NWAuditTraceSummaryVo>> {
  return post<NWPageDto<NWAuditTraceSummaryVo>>('/admin/audit/trace/list', req)
}

/** 审计概览统计（顶部卡片） */
export function fetchAdminAuditStats(req: NWAuditTraceListReqVo): Promise<NWAuditStatsVo> {
  return post<NWAuditStatsVo>('/admin/audit/stats', req)
}

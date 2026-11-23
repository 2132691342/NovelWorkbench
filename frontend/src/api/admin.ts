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
  created_at: string
  updated_at: string
}

/** 审计日志视图对象（字段对齐后端 AuditVO） */
export interface NWAuditVo {
  audit_ulid: string
  log_level_code: string
  trace_id: string
  user_ulid: string
  user_name: string
  action_code: string
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
}

/** 创建用户请求（字段对齐后端 UserAdminCreateREQ） */
export interface NWUserAdminCreateReqVo {
  user_name: string
  user_email: string
  password_text: string
  role_codes?: string[]
  remark_text?: string
}

/** 更新用户请求（字段对齐后端 UserAdminUpdateREQ，仅覆盖非空字段） */
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
  trace_id?: string
  target_type?: string
  action_code?: string
  keyword_text?: string
  user_ulid?: string
  date_from?: string
  date_to?: string
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

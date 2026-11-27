// 个人中心接口封装（资料、修改密码；字段与路径对齐后端 profile 契约）
import { get, post } from '@/api/http'

/** 个人中心资料视图（字段对齐后端 UserProfileVO） */
export interface NWProfileVo {
  user_ulid: string
  user_name: string
  user_email: string
  avatar_url: string
  user_status_code: string
  role_codes: string[]
  /** 角色名称列表（标签云展示用） */
  role_name_list: string[]
  /** 权限码并集（"*" 表示全部权限） */
  perm_codes: string[]
  remark_text: string
  created_at: string
  updated_at: string
}

/** 更新资料请求（字段对齐后端 UserProfileUpdateREQ） */
export interface NWProfileUpdateReqVo {
  user_email?: string
  avatar_url?: string
  remark_text?: string
}

/** 修改密码请求（字段对齐后端 UserPasswordChangeREQ） */
export interface NWPasswordChangeReqVo {
  old_password_text: string
  new_password_text: string
}

/** 修改密码结果（后端返回新令牌：旧令牌已因密码版本递增失效） */
export interface NWPasswordChangeVo {
  access_token: string
  expires_in_sec: number
}

/** 查询个人资料 */
export function fetchProfileDetail(): Promise<NWProfileVo> {
  return get<NWProfileVo>('/profile/detail')
}

/** 更新个人资料 */
export function updateProfile(req: NWProfileUpdateReqVo): Promise<NWProfileVo> {
  return post<NWProfileVo>('/profile/update', req)
}

/** 修改本人密码（返回新令牌，调用方必须替换本地令牌） */
export function changePassword(req: NWPasswordChangeReqVo): Promise<NWPasswordChangeVo> {
  return post<NWPasswordChangeVo>('/profile/password/change', req)
}

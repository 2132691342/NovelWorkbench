// 登录接口与请求类型：账号密码换取令牌、用户信息与导航树。
import request from '@/request'
import type { NWLoginResultVo, NWMenuItemVo, NWUserVo } from '@/api/contracts'

/** 登录请求（后端 UserLoginREQ） */
export interface LoginParams {
  account_text: string
  password_text: string
}

/** 登录：返回访问令牌与用户信息 */
export const loginApi = (data: LoginParams) =>
  request.post<NWLoginResultVo>({ url: '/auth/login', data })

/** 登出：吊销当前令牌 */
export const logoutApi = () => request.post<null>({ url: '/auth/logout' })

/** 当前登录用户信息 */
export const profileApi = () => request.get<NWUserVo>({ url: '/auth/profile' })

/** 导航树与权限码 */
export const menuNavApi = () =>
  request.get<{ menu_tree_list: NWMenuItemVo[]; perm_code_list: string[] }>({
    url: '/menu/nav'
  })

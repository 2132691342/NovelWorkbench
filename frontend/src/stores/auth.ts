// 认证状态：访问令牌、当前用户与权限码，持久化并提供登录/登出动作（账号由管理员分配，无注册）。

import { defineStore } from 'pinia'
import { get, post } from '@/api/http'
import type { NWLoginResultVo, NWUserVo } from '@/api/contracts'
import {
  clearToken,
  clearUserVo,
  readToken,
  readUserVo,
  writeToken,
  writeUserVo,
} from '@/utils/storage'

/** 登录请求 */
export interface NWLoginReqVo {
  account_text: string
  password_text: string
}

export const useAuthStore = defineStore('auth', {
  state: () => ({
    access_token: readToken(),
    user_vo: readUserVo(),
    /** 当前用户命中的权限码集合（登录与刷新导航时更新） */
    perm_code_list: [] as string[],
  }),
  getters: {
    is_logged_in(state): boolean {
      return state.access_token.length > 0
    },
    /** 权限判定：'*' 通配全部权限 */
    hasPerm(state): (permCode: string) => boolean {
      return (permCode: string) =>
        state.perm_code_list.includes('*') || state.perm_code_list.includes(permCode)
    },
    /** 是否拥有管理端入口（任一 system: 前缀权限或通配） */
    is_admin_user(state): boolean {
      return (
        state.perm_code_list.includes('*') ||
        state.perm_code_list.some((permCode) => permCode.startsWith('system:'))
      )
    },
  },
  actions: {
    /** 登录：保存 token 与用户信息并持久化 */
    async login(loginReqVo: NWLoginReqVo): Promise<void> {
      const loginResultVo = await post<NWLoginResultVo>('/auth/login', loginReqVo)
      this.access_token = loginResultVo.access_token
      this.user_vo = loginResultVo.user_vo
      writeToken(loginResultVo.access_token)
      writeUserVo(loginResultVo.user_vo)
    },
    /** 拉取当前用户信息 */
    async fetchProfile(): Promise<void> {
      const userVo = await get<NWUserVo>('/auth/profile')
      this.user_vo = userVo
      writeUserVo(userVo)
    },
    /** 设置权限码集合（导航接口返回后同步） */
    setPermCodes(permCodeList: string[]): void {
      this.perm_code_list = permCodeList
    },
    /** 退出登录：尽力调用登出接口后清理本地会话 */
    async logout(): Promise<void> {
      try {
        await post<null>('/auth/logout')
      } catch {
        // 登出接口异常不阻塞本地会话清理
      }
      this.resetSession()
    },
    /** 清理本地会话（token + 用户信息 + 权限码） */
    resetSession(): void {
      this.access_token = ''
      this.user_vo = null
      this.perm_code_list = []
      clearToken()
      clearUserVo()
    },
  },
})

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

/** 注册请求 */
export interface NWRegisterReqVo {
  user_name: string
  user_email: string
  password_text: string
}

export const useAuthStore = defineStore('auth', {
  state: () => ({
    access_token: readToken(),
    user_vo: readUserVo(),
  }),
  getters: {
    is_logged_in(state): boolean {
      return state.access_token.length > 0
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
    /** 注册 */
    async register(registerReqVo: NWRegisterReqVo): Promise<void> {
      await post<null>('/auth/register', registerReqVo)
    },
    /** 拉取当前用户信息 */
    async fetchProfile(): Promise<void> {
      const userVo = await get<NWUserVo>('/auth/profile')
      this.user_vo = userVo
      writeUserVo(userVo)
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
    /** 清理本地会话（token + 用户信息） */
    resetSession(): void {
      this.access_token = ''
      this.user_vo = null
      clearToken()
      clearUserVo()
    },
  },
})

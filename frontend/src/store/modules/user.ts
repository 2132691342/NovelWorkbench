// 用户状态：令牌、用户信息与权限码，负责登录与登出。
import { defineStore } from 'pinia'
import { store } from '../index'
import type { NWUserVo } from '@/api/contracts'
import { loginApi, logoutApi, profileApi, type LoginParams } from '@/api/login'

import { useTagsViewStore } from './tagsView'
import { useMenuStoreWithOut } from './menu'
import router, { resetRouter } from '@/router'
import { clearToken, readToken, writeToken } from '@/utils/storage'

interface UserState {
  userInfo?: NWUserVo
  token: string
  permCodes: string[]
}

export const useUserStore = defineStore('user', {
  state: (): UserState => ({
    token: readToken(),
    userInfo: undefined,
    permCodes: []
  }),
  getters: {
    isAuthenticated(): boolean {
      return Boolean(this.token)
    },
    /** 权限判定：'*' 通配全部权限 */
    hasPerm(): (permCode: string) => boolean {
      return (permCode: string) =>
        this.permCodes.includes('*') || this.permCodes.includes(permCode)
    }
  },
  actions: {
    async login(params: LoginParams) {
      const result = await loginApi(params)
      this.token = result.access_token
      this.userInfo = result.user_vo
      writeToken(result.access_token)
    },
    async fetchProfile() {
      this.userInfo = await profileApi()
    },
    setPermCodes(permCodes: string[]) {
      this.permCodes = permCodes
    },
    clearSession() {
      this.token = ''
      this.userInfo = undefined
      this.permCodes = []
      clearToken()
      useMenuStoreWithOut().reset()
      useTagsViewStore().removeAllViews(false)
      resetRouter()
    },
    async logout() {
      try {
        await logoutApi()
      } catch {
        // 登出接口失败不阻塞本地会话清理
      }
      this.clearSession()
      await router.replace('/login')
    }
  },
  persist: {
    key: 'nw-session-v1',
    pick: ['token']
  }
})

export const useUserStoreWithOut = () => useUserStore(store)

// 用户状态：令牌、用户信息与权限码，负责登录、导航加载与登出。

import { defineStore } from 'pinia'
import { store } from '../index'
import type { NWMenuItemVo, NWUserVo } from '@/api/contracts'
import { loginApi, logoutApi, menuNavApi, profileApi, type LoginParams } from '@/api/login'

import { useTagsViewStore } from './tagsView'
import { usePermissionStoreWithOut } from './permission'
import router, { resetRouter } from '@/router'
import { clearToken, readToken, writeToken } from '@/utils/storage'

interface UserState {
  userInfo?: NWUserVo
  token: string
  permCodes: string[]
  menuTree: NWMenuItemVo[]
  navLoaded: boolean
}

export const useUserStore = defineStore('user', {
  state: (): UserState => ({
    token: readToken(),
    userInfo: undefined,
    permCodes: [],
    menuTree: [],
    navLoaded: false
  }),
  getters: {
    isAuthenticated(): boolean {
      return Boolean(this.token)
    },
    /** 权限判定：'*' 通配全部权限 */
    hasPerm(): (permCode: string) => boolean {
      return (permCode: string) => this.permCodes.includes('*') || this.permCodes.includes(permCode)
    }
  },
  actions: {
    async login(params: LoginParams, remember = true) {
      const result = await loginApi(params)
      this.token = result.access_token
      this.userInfo = result.user_vo
      writeToken(result.access_token, remember)
    },
    async fetchProfile() {
      this.userInfo = await profileApi()
    },
    /** 拉取导航树与权限码（导航可见性、路由注册与页面门禁同源） */
    async loadNav() {
      const nav = await menuNavApi()
      this.menuTree = nav.menu_tree_list ?? []
      this.permCodes = nav.perm_code_list ?? []
      this.navLoaded = true
      return nav
    },
    clearSession() {
      this.token = ''
      this.userInfo = undefined
      this.permCodes = []
      this.menuTree = []
      this.navLoaded = false
      clearToken()
      usePermissionStoreWithOut().reset()
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
    key: 'nw-session-v2',
    pick: ['token']
  }
})

export const useUserStoreWithOut = () => useUserStore(store)

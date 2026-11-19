// 导航状态：后端下发的菜单树与权限码（侧边栏渲染与路由守卫共用一份事实）。

import { defineStore } from 'pinia'
import { fetchMenuNav } from '@/api/menu'
import type { NWMenuItemVo } from '@/api/contracts'
import { useAuthStore } from '@/stores/auth'

export const useMenuStore = defineStore('menu', {
  state: () => ({
    menu_tree_list: [] as NWMenuItemVo[],
    perm_code_list: [] as string[],
    nav_loading: false,
    nav_loaded: false,
  }),
  getters: {
    /** 权限判定（与 auth store 保持同一份数据源） */
    hasPerm(state): (permCode: string) => boolean {
      return (permCode: string) =>
        state.perm_code_list.includes('*') || state.perm_code_list.includes(permCode)
    },
  },
  actions: {
    /** 拉取导航树与权限码（登录后与角色变更后调用） */
    async loadNav(): Promise<void> {
      if (this.nav_loading) {
        return
      }
      this.nav_loading = true
      try {
        const navData = await fetchMenuNav()
        this.menu_tree_list = navData.menu_tree_list ?? []
        this.perm_code_list = navData.perm_code_list ?? []
        this.nav_loaded = true
        useAuthStore().setPermCodes(this.perm_code_list)
      } catch {
        // 失败提示由 http 层统一弹出；保留上一次导航，避免侧边栏闪空
      } finally {
        this.nav_loading = false
      }
    },
    /** 清空导航（登出时调用） */
    resetNav(): void {
      this.menu_tree_list = []
      this.perm_code_list = []
      this.nav_loaded = false
    },
  },
})

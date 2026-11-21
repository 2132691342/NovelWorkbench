// 菜单状态：后端导航树与权限码缓存。
import { defineStore } from 'pinia'
import { menuNavApi } from '@/api/login'
import type { NWMenuItemVo } from '@/api/contracts'
import { store } from '../index'
import { useUserStore } from './user'

export interface MenuState {
  menuTree: NWMenuItemVo[]
  permCodes: string[]
  navLoaded: boolean
}

/**
 * 导航状态：后端下发菜单树（决定侧边栏显示什么）与权限码（决定接口与页面门禁）。
 * 页面组件是静态路由，菜单只提供可见性与排序，因此改菜单不需要前端发版。
 */
export const useMenuStore = defineStore('menu', {
  state: (): MenuState => ({
    menuTree: [],
    permCodes: [],
    navLoaded: false
  }),
  getters: {
    /** 扁平化页面菜单（快捷键跳转、标题回退用） */
    flatPages(state): NWMenuItemVo[] {
      return state.menuTree.flatMap((group) =>
        group.menu_type_code === 'page' ? [group] : (group.child_list ?? [])
      )
    }
  },
  actions: {
    async loadNav() {
      const nav = await menuNavApi()
      this.menuTree = nav.menu_tree_list ?? []
      this.permCodes = nav.perm_code_list ?? []
      this.navLoaded = true
      useUserStore().setPermCodes(this.permCodes)
    },
    reset() {
      this.menuTree = []
      this.permCodes = []
      this.navLoaded = false
    }
  }
})

export const useMenuStoreWithOut = () => useMenuStore(store)

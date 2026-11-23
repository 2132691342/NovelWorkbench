// 权限路由状态：后端导航树生成的路由表（菜单即权限，导航可见性与路由注册同源）。

import { defineStore } from 'pinia'
import { constantRouterMap } from '@/router'
import { buildRoutes, type NavMenuNode } from '@/utils/routerHelper'
import { store } from '../index'

export interface PermissionState {
  routers: AppRouteRecordRaw[] // 常量路由 + 后端生成路由（侧边栏与标签页消费）
  addRouters: AppRouteRecordRaw[] // 仅后端生成路由（登出时逐个移除）
  isAddRouters: boolean // 动态路由是否已注册
}

export const usePermissionStore = defineStore('permission', {
  state: (): PermissionState => ({
    routers: [],
    addRouters: [],
    isAddRouters: false
  }),
  actions: {
    /** 由后端导航树生成路由记录（不注册，注册由 permission.ts 负责） */
    generateRoutes(navList: NavMenuNode[]): AppRouteRecordRaw[] {
      const routeList = buildRoutes(navList)
      this.addRouters = routeList
      this.routers = constantRouterMap.concat(routeList)
      return routeList
    },
    setIsAddRouters(flag: boolean) {
      this.isAddRouters = flag
    },
    reset() {
      this.routers = []
      this.addRouters = []
      this.isAddRouters = false
    }
  }
})

export const usePermissionStoreWithOut = () => usePermissionStore(store)

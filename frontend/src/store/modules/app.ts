// 应用状态：侧栏折叠、明暗主题、页面加载与布局模式。
import { defineStore } from 'pinia'
import { store } from '../index'

export type LayoutMode = 'sidebar' | 'top' | 'mixed' | 'dual'

interface AppState {
  collapse: boolean
  pageLoading: boolean
  layout: LayoutMode
  isDark: boolean
  mobile: boolean
}

export const useAppStore = defineStore('app', {
  state: (): AppState => {
    return {
      mobile: false, // 是否是移动端
      pageLoading: false, // 路由跳转loading
      collapse: false, // 折叠菜单
      layout: 'sidebar', // layout布局
      isDark: false // 是否是暗黑模式
    }
  },
  actions: {
    setLayout(layout: LayoutMode) {
      this.layout = layout
    }
  },
  persist: {
    key: 'nw-app-v1',
    pick: ['collapse', 'layout', 'isDark']
  }
})

export const useAppStoreWithOut = () => {
  return useAppStore(store)
}

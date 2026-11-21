// 多页签状态：记录已访问页面，供 TagsView 展示与切换。

import { defineStore } from 'pinia'
import type { RouteLocationNormalizedLoaded } from 'vue-router'

/** 单个页签 */
export interface NWTagView {
  page_path: string // 路由路径（去重键）
  page_full_path: string // 完整路径（含 query，点击跳转用）
  page_name: string // 路由 name
  page_title: string // 展示标题
  page_icon: string // 图标编码
  affix_flag: boolean // 固定页签（不可关闭，如仪表盘）
}

/** 从路由生成页签；无标题或标记 noTagsView 的页面不生成 */
export function buildTagView(
  route: RouteLocationNormalizedLoaded,
  pageTitle: string,
): NWTagView | null {
  if (!pageTitle || route.meta.no_tags_view === true) {
    return null
  }
  const routeName = typeof route.name === 'string' ? route.name : ''
  return {
    page_path: route.path,
    page_full_path: route.fullPath,
    page_name: routeName,
    page_title: pageTitle,
    page_icon: String(route.meta.menu_code ?? ''),
    affix_flag: route.meta.affix_flag === true,
  }
}

export const useTagsViewStore = defineStore('tagsView', {
  state: () => ({
    tag_list: [] as NWTagView[],
  }),
  actions: {
    /** 新增或刷新页签（同路径视为同一页签，仅更新完整路径与标题） */
    addTag(tagView: NWTagView): void {
      const existingIndex = this.tag_list.findIndex(
        (tagItem) => tagItem.page_path === tagView.page_path,
      )
      if (existingIndex < 0) {
        this.tag_list.push(tagView)
        return
      }
      this.tag_list[existingIndex] = { ...tagView, affix_flag: this.tag_list[existingIndex].affix_flag }
    },
    /** 关闭单个页签，返回关闭后应跳转到的相邻页签路径 */
    removeTag(pagePath: string): string | null {
      const targetIndex = this.tag_list.findIndex((tagItem) => tagItem.page_path === pagePath)
      if (targetIndex < 0) {
        return null
      }
      if (this.tag_list[targetIndex].affix_flag) {
        return null
      }
      this.tag_list.splice(targetIndex, 1)
      const nextTag = this.tag_list[targetIndex] ?? this.tag_list[targetIndex - 1]
      return nextTag?.page_full_path ?? null
    },
    /** 关闭其它页签（保留固定页签与当前页） */
    removeOtherTags(pagePath: string): void {
      this.tag_list = this.tag_list.filter(
        (tagItem) => tagItem.affix_flag || tagItem.page_path === pagePath,
      )
    },
    /** 关闭全部页签（保留固定页签） */
    removeAllTags(): void {
      this.tag_list = this.tag_list.filter((tagItem) => tagItem.affix_flag)
    },
    /** 清空（登出时调用） */
    resetTags(): void {
      this.tag_list = []
    },
  },
})

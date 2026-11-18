// 当前项目上下文：全局项目切换器与项目内页面共用的选中态（持久化到 localStorage）
import { defineStore } from 'pinia'
import { listProjects } from '@/api/project'
import type { NWProjectVo } from '@/api/contracts'

/** 当前项目持久化键 */
const NW_CURRENT_PROJECT_KEY = 'nw_current_project_ulid'

export const useProjectStore = defineStore('project', {
  state: () => ({
    project_list: [] as NWProjectVo[],
    current_project_ulid: localStorage.getItem(NW_CURRENT_PROJECT_KEY) ?? '',
    list_loading: false,
  }),
  getters: {
    /** 当前项目视图对象（未选中或已删除时为 null） */
    current_project_vo(state): NWProjectVo | null {
      return (
        state.project_list.find(
          (projectItem) => projectItem.project_ulid === state.current_project_ulid,
        ) ?? null
      )
    },
    /** 判断项目是否在已加载列表中（用于避免对已删除项目发起请求） */
    hasProject(state): (projectUlid: string) => boolean {
      return (projectUlid: string) =>
        state.project_list.some((projectItem) => projectItem.project_ulid === projectUlid)
    },
  },
  actions: {
    /** 拉取项目列表并校正当前选中项（列表变动时自动收敛到第一项） */
    async loadProjectList(): Promise<void> {
      this.list_loading = true
      try {
        const pageData = await listProjects({ page_num: 1, page_size: 100 })
        this.project_list = pageData.items ?? []
        const exists = this.project_list.some(
          (projectItem) => projectItem.project_ulid === this.current_project_ulid,
        )
        if (!exists) {
          this.setCurrentProject(this.project_list[0]?.project_ulid ?? '')
        }
      } catch {
        this.project_list = []
      } finally {
        this.list_loading = false
      }
    },
    /** 切换当前项目 */
    setCurrentProject(projectUlid: string): void {
      this.current_project_ulid = projectUlid
      if (projectUlid) {
        localStorage.setItem(NW_CURRENT_PROJECT_KEY, projectUlid)
        return
      }
      localStorage.removeItem(NW_CURRENT_PROJECT_KEY)
    },
  },
})

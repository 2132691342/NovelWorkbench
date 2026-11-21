// 项目状态：当前项目与可选项，负责与路由同步。
import { defineStore } from 'pinia'
import { getProjectDetail, listProjects } from '@/api/project'
import type { NWProjectVo } from '@/api/contracts'
import { store } from '../index'

export interface ProjectState {
  currentProjectUlid: string
  currentProject: NWProjectVo | null
  projectOptions: NWProjectVo[]
}

/** 项目选中态：项目作用域页面共用（写作台/大纲/角色/设定/知识库/笔记）。 */
export const useProjectStore = defineStore('project', {
  state: (): ProjectState => ({
    currentProjectUlid: '',
    currentProject: null,
    projectOptions: []
  }),
  actions: {
    /** 拉取可选项目（顶栏切换器与页面选择器共用） */
    async loadProjectOptions() {
      const page = await listProjects({ page_num: 1, page_size: 100 })
      this.projectOptions = page.items ?? []
      if (!this.currentProjectUlid && this.projectOptions.length > 0) {
        this.selectProject(this.projectOptions[0].project_ulid)
      }
    },
    selectProject(projectUlid: string) {
      if (this.currentProjectUlid === projectUlid) return
      this.currentProjectUlid = projectUlid
      this.currentProject =
        this.projectOptions.find((item) => item.project_ulid === projectUlid) ?? null
    },
    /** 深链进入项目页时，用 URL 上的 ULID 补齐选中态与项目详情 */
    async syncFromRoute(projectUlid: string) {
      if (!projectUlid) return
      if (this.currentProjectUlid !== projectUlid) this.currentProjectUlid = projectUlid
      if (this.currentProject?.project_ulid === projectUlid) return
      const cached = this.projectOptions.find((item) => item.project_ulid === projectUlid)
      this.currentProject = cached ?? (await getProjectDetail(projectUlid))
    },
    reset() {
      this.currentProjectUlid = ''
      this.currentProject = null
      this.projectOptions = []
    }
  },
  persist: {
    key: 'nw-project-v1',
    pick: ['currentProjectUlid']
  }
})

export const useProjectStoreWithOut = () => useProjectStore(store)

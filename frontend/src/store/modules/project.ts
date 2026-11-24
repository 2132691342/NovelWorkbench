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
    /** 拉取可选项目（顶栏切换器与页面选择器共用）；已选项目失效时回退到首个可用项目 */
    async loadProjectOptions() {
      const page = await listProjects({ page_num: 1, page_size: 100 })
      this.projectOptions = page.items ?? []
      const stillExists = this.projectOptions.some(
        (item) => item.project_ulid === this.currentProjectUlid
      )
      if (!stillExists) {
        const fallbackUlid = this.projectOptions[0]?.project_ulid ?? ''
        this.currentProjectUlid = fallbackUlid
        this.currentProject =
          this.projectOptions.find((item) => item.project_ulid === fallbackUlid) ?? null
      }
    },
    selectProject(projectUlid: string) {
      if (this.currentProjectUlid === projectUlid) return
      this.currentProjectUlid = projectUlid
      this.currentProject =
        this.projectOptions.find((item) => item.project_ulid === projectUlid) ?? null
    },
    /** 深链进入项目页时，用 URL 上的 ULID 补齐选中态与项目详情；项目已删除返回 false */
    async syncFromRoute(projectUlid: string): Promise<boolean> {
      if (!projectUlid) return false
      if (this.currentProject?.project_ulid === projectUlid) return true
      if (this.projectOptions.length === 0) await this.loadProjectOptions()
      const cached = this.projectOptions.find((item) => item.project_ulid === projectUlid)
      if (cached) {
        this.currentProjectUlid = projectUlid
        this.currentProject = cached
        return true
      }
      try {
        this.currentProject = await getProjectDetail(projectUlid)
        this.currentProjectUlid = projectUlid
        return true
      } catch {
        // 项目已被删除：清掉选中态，由调用方回退到可用项目
        this.currentProject = null
        this.currentProjectUlid = this.projectOptions[0]?.project_ulid ?? ''
        this.currentProject =
          this.projectOptions.find((item) => item.project_ulid === this.currentProjectUlid) ?? null
        return false
      }
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

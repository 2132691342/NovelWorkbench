// 导航树装配：把后端菜单转换为侧边栏节点。
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import type { NWMenuItemVo } from '@/api/contracts'
import { useMenuStore } from '@/store/modules/menu'
import { useProjectStore } from '@/store/modules/project'

/** 侧边栏节点（把后端菜单树转成纯展示结构，菜单组件不感知后端字段） */
export interface NavNode {
  key: string
  label: string
  icon: string
  path: string
  children: NavNode[]
  disabled: boolean
}

/**
 * 导航装配：后端菜单树 → 侧边栏节点。
 * 项目作用域菜单的路由形如 /workspace/:project_ulid，用当前选中项目替换；
 * 未选项目时置灰（点击提示先选项目），不隐藏，避免用户以为功能不存在。
 */
export const useNavMenu = () => {
  const menuStore = useMenuStore()
  const projectStore = useProjectStore()
  const route = useRoute()

  const projectUlid = computed(() => {
    const routeUlid = String(route.params.project_ulid ?? '')
    return routeUlid || projectStore.currentProjectUlid
  })

  const toNode = (item: NWMenuItemVo): NavNode => {
    const projectScoped = item.scope_code === 'project'
    const path = item.route_path.replace(':project_ulid', projectUlid.value)
    const children = (item.child_list ?? []).map(toNode)
    const isPage = item.menu_type_code === 'page'
    return {
      key: item.menu_code,
      label: item.menu_name,
      icon: item.icon_code,
      path: isPage ? path : '',
      children,
      disabled: isPage ? projectScoped && !projectUlid.value : false
    }
  }

  const navNodes = computed<NavNode[]>(() => menuStore.menuTree.map(toNode))

  /** 当前高亮项：路径前缀最长匹配（项目页 URL 带 ULID，不能直接比等） */
  const activeKey = computed(() => {
    const currentPath = route.path
    const flatten = (nodes: NavNode[]): NavNode[] =>
      nodes.flatMap((node) => (node.children.length > 0 ? flatten(node.children) : [node]))
    const pages = flatten(navNodes.value).filter((node) => node.path.length > 0)
    const matched = pages
      .filter((node) => currentPath === node.path || currentPath.startsWith(`${node.path}/`))
      .sort((left, right) => right.path.length - left.path.length)[0]
    return matched?.key ?? ''
  })

  return { navNodes, activeKey, projectUlid }
}

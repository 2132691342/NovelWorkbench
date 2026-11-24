// 动态路由装配：把后端导航树（菜单即权限）转成 vue-router 路由并注册。
// 组件路径来自菜单行的 component_path（如 views/Workspace/index），由 Vite 的 glob 表解析；
// 项目作用域菜单保留 :project_ulid 占位，由导航层在点击时替换为当前项目。

import type { RouteRecordRaw } from 'vue-router'
import { resolveMenuIcon } from '@/layout/menuIcons'

/** Vite 在构建期收集的页面组件表（键形如 ../views/Workspace/index.vue） */
const viewModules = import.meta.glob('../views/**/*.{vue,tsx}')

/** 业务外壳（常量路由的父级） */
export const Layout = () => import('@/layout/Layout.vue')

/** 是否为外部链接 */
export const isUrl = (path: string) => /^(?:https?:|mailto:|tel:)/.test(path)

/** 拼接父子路由路径并消除重复斜杠与结尾斜杠（路由为 strict 模式，结尾斜杠会落到 404） */
export const pathResolve = (parentPath: string, path: string) => {
  if (isUrl(path) || path.startsWith('/')) return path
  const merged = `${parentPath}/${path}`.replace(/\/+/g, '/')
  return merged.length > 1 ? merged.replace(/\/+$/, '') : merged
}

/** 按组件路径取懒加载组件；找不到返回 undefined（路由仍注册，页面显示 404） */
export const resolveView = (componentPath: string) => {
  if (!componentPath) return undefined
  const key = `../${componentPath.replace(/^\/+/, '')}`
  return viewModules[`${key}.vue`] ?? viewModules[`${key}.tsx`]
}

/** 后端导航节点（字段与 domain.MenuTreeNodeVO 对齐） */
export interface NavMenuNode {
  menu_code: string
  menu_name: string
  menu_type_code: string
  scope_code: string
  route_path: string
  component_path?: string
  icon_code: string
  perm_code: string
  visible_flag?: boolean
  child_list: NavMenuNode[]
}

/** 生成路由名：编码转 PascalCase（菜单编码在系统内唯一） */
export const routeNameOf = (menuCode: string) =>
  menuCode
    .split(/[_\-\s]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join('')

/** 导航节点 → 路由记录（分组节点只有 children，页面节点带组件） */
export const toRouteRecord = (node: NavMenuNode, parentPath = ''): AppRouteRecordRaw | null => {
  const isGroup = node.menu_type_code === 'group'
  const isPage = node.menu_type_code === 'page'
  if (!isGroup && !isPage) return null
  const fullPath = isGroup ? pathResolve(parentPath, `/${node.menu_code}`) : node.route_path
  const childList = (node.child_list ?? [])
    .map((child) => toRouteRecord(child, fullPath))
    .filter((child): child is AppRouteRecordRaw => child !== null)
  const record: AppRouteRecordRaw = {
    path: fullPath,
    name: routeNameOf(node.menu_code),
    meta: {
      title: node.menu_name,
      icon: resolveMenuIcon(node.icon_code),
      permCode: node.perm_code,
      projectScope: node.scope_code === 'project',
      hidden: isGroup ? childList.length === 0 : node.visible_flag === false
    }
  }
  if (isGroup) {
    record.children = childList
    record.redirect = childList[0]?.path ?? undefined
    return record
  }
  const view = resolveView(node.component_path ?? '')
  if (view) record.component = view
  return record
}

/** 导航树 → 顶层路由记录（挂到根 Layout 下） */
export const buildRoutes = (navList: NavMenuNode[]): AppRouteRecordRaw[] =>
  navList
    .map((node) => toRouteRecord(node))
    .filter((record): record is AppRouteRecordRaw => record !== null)

/** 项目作用域路由路径补当前项目 ULID */
export const withProject = (path: string, projectUlid: string) =>
  projectUlid ? path.replace(':project_ulid', projectUlid) : path

export type { RouteRecordRaw }

/** 把路由表中的 :project_ulid 占位替换为指定项目（菜单 index 与当前路径才能对上） */
export const substituteProject = (
  routes: AppRouteRecordRaw[],
  projectUlid: string
): AppRouteRecordRaw[] =>
  routes.map((route) => ({
    ...route,
    path: projectUlid ? withProject(route.path, projectUlid) : route.path,
    children: route.children ? substituteProject(route.children, projectUlid) : undefined
  }))

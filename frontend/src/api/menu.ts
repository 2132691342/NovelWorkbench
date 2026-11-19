// 菜单与权限目录接口封装：导航树、菜单 CRUD 与权限目录。

import { get, post } from '@/api/http'
import type { NWMenuItemVo, NWMenuNavVo, NWMenuVo, NWPermGroupVo } from './contracts'

// 请求对象

/** 菜单保存请求（menu_ulid 为空表示新建） */
export interface NWMenuSaveReqVo {
  menu_ulid?: string
  parent_ulid?: string
  menu_code: string
  menu_name: string
  menu_type_code: string
  scope_code?: string
  route_path?: string
  icon_code?: string
  perm_code?: string
  visible_flag?: boolean
  enabled_flag?: boolean
  sort_order?: number
  remark_text?: string
}

/** 菜单列表查询请求 */
export interface NWMenuListReqVo {
  keyword_text?: string
  menu_type_code?: string
  scope_code?: string
}

// 接口

/** 当前用户的导航树与权限码集合 */
export function fetchMenuNav(): Promise<NWMenuNavVo> {
  return get<NWMenuNavVo>('/menu/nav')
}

/** 菜单平铺列表（管理端） */
export function listMenus(req: NWMenuListReqVo): Promise<NWMenuVo[]> {
  const query = new URLSearchParams()
  if (req.keyword_text) {
    query.set('keyword_text', req.keyword_text)
  }
  if (req.menu_type_code) {
    query.set('menu_type_code', req.menu_type_code)
  }
  if (req.scope_code) {
    query.set('scope_code', req.scope_code)
  }
  const suffix = query.toString()
  return get<NWMenuVo[]>(`/menu/list${suffix ? `?${suffix}` : ''}`)
}

/** 菜单详情 */
export function getMenuDetail(menuUlid: string): Promise<NWMenuVo> {
  return get<NWMenuVo>(`/menu/detail/${encodeURIComponent(menuUlid)}`)
}

/** 保存菜单 */
export function saveMenu(req: NWMenuSaveReqVo): Promise<NWMenuVo> {
  return post<NWMenuVo>('/menu/save', req)
}

/** 删除菜单 */
export function deleteMenu(menuUlid: string): Promise<null> {
  return post<null>('/menu/delete', { menu_ulid: menuUlid })
}

/** 权限目录（按分组） */
export function fetchPermCatalog(): Promise<NWPermGroupVo[]> {
  return get<NWPermGroupVo[]>('/menu/permission/catalog')
}

/** 菜单树节点（导航渲染用，类型别名便于组件引用） */
export type NWMenuItem = NWMenuItemVo

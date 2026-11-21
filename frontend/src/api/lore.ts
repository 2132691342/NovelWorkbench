// 世界观设定 / 创作笔记接口封装（笔记为 lore_type_code='note'，字段对齐后端 domain/lore.go）
import { post } from '@/api/http'

/** 创作笔记的设定类型编码 */
export const NW_LORE_TYPE_NOTE = 'note'

/** 设定视图对象（字段对齐后端 LoreVO） */
export interface NWLoreVo {
  lore_ulid: string
  project_ulid: string
  lore_name: string
  lore_type_code: string
  content_text: string
  content_mode_code: string
  sort_order: number
  updated_at: string
}

/** 设定保存请求（字段对齐后端 LoreREQ，有 lore_ulid 为更新） */
export interface NWLoreSaveReqVo {
  lore_ulid?: string
  project_ulid: string
  lore_name: string
  lore_type_code?: string
  content_text?: string
  content_mode_code?: string
  sort_order?: number
}

/** 设定列表查询请求（字段对齐后端 LoreListREQ） */
export interface NWLoreListReqVo {
  project_ulid: string
  lore_type_code?: string
  keyword_text?: string
}

/** 新建/更新设定 */
export function saveLore(req: NWLoreSaveReqVo): Promise<NWLoreVo> {
  return post<NWLoreVo>('/lore/save', req)
}

/** 删除设定 */
export async function deleteLore(loreUlid: string): Promise<void> {
  await post('/lore/delete', { lore_ulid: loreUlid })
}

/** 设定列表（可按类型与关键词过滤，note 为创作笔记） */
export function listLore(req: NWLoreListReqVo): Promise<NWLoreVo[]> {
  return post<NWLoreVo[]>('/lore/list', req)
}

/** 创作笔记列表（lore_type_code 固定 note） */
export function listLoreNotes(projectUlid: string): Promise<NWLoreVo[]> {
  return post<NWLoreVo[]>('/lore/list', {
    project_ulid: projectUlid,
    lore_type_code: NW_LORE_TYPE_NOTE,
  })
}

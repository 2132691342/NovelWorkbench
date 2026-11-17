// 角色 / 角色关系接口封装（字段与路径对齐 doc/api.md 第 3 节与后端 domain/character.go）
import { get, post } from '@/api/http'

/** 角色视图对象（字段对齐后端 CharacterVO） */
export interface NWCharacterVo {
  character_ulid: string
  project_ulid: string
  character_name: string
  alias_text: string
  role_type_code: string
  personality_text: string
  background_text: string
  appearance_text: string
  goal_text: string
}

/** 角色保存请求（字段对齐后端 CharacterREQ，有 character_ulid 为更新） */
export interface NWCharacterSaveReqVo {
  character_ulid?: string
  project_ulid: string
  character_name: string
  alias_text?: string
  role_type_code?: string
  personality_text?: string
  background_text?: string
  appearance_text?: string
  goal_text?: string
}

/** 角色列表查询请求（字段对齐后端 CharacterListREQ） */
export interface NWCharacterListReqVo {
  project_ulid: string
  role_type_code?: string
  keyword_text?: string
}

/** 角色关系边视图对象（字段对齐后端 CharacterEdgeVO） */
export interface NWCharacterEdgeVo {
  relation_ulid: string
  from_character_ulid: string
  to_character_ulid: string
  relation_type_code: string
  relation_note: string
}

/** 关系图节点视图对象（字段对齐后端 CharacterNodeVO） */
export interface NWCharacterNodeVo {
  character_ulid: string
  character_name: string
  role_type_code: string
}

/** 角色关系图（字段对齐后端 CharacterGraphVO） */
export interface NWCharacterGraphVo {
  node_list: NWCharacterNodeVo[]
  edge_list: NWCharacterEdgeVo[]
}

/** 新建关系边请求（字段对齐后端 CharacterRelationREQ） */
export interface NWCharacterRelationSaveReqVo {
  project_ulid: string
  from_character_ulid: string
  to_character_ulid: string
  relation_type_code: string
  relation_note?: string
}

/** 新建/更新角色 */
export function saveCharacter(req: NWCharacterSaveReqVo): Promise<NWCharacterVo> {
  return post<NWCharacterVo>('/character/save', req)
}

/** 删除角色（连带删关系边） */
export async function deleteCharacter(characterUlid: string): Promise<void> {
  await post('/character/delete', { character_ulid: characterUlid })
}

/** 角色列表（类型/关键词过滤） */
export function listCharacters(req: NWCharacterListReqVo): Promise<NWCharacterVo[]> {
  return post<NWCharacterVo[]>('/character/list', req)
}

/** 角色关系图（节点 + 边） */
export function getCharacterGraph(projectUlid: string): Promise<NWCharacterGraphVo> {
  return get<NWCharacterGraphVo>(
    `/character/graph?project_ulid=${encodeURIComponent(projectUlid)}`,
  )
}

/** 新建关系边 */
export function saveCharacterRelation(
  req: NWCharacterRelationSaveReqVo,
): Promise<NWCharacterEdgeVo> {
  return post<NWCharacterEdgeVo>('/character/relation/save', req)
}

/** 删除关系边 */
export async function deleteCharacterRelation(relationUlid: string): Promise<void> {
  await post('/character/relation/delete', { relation_ulid: relationUlid })
}

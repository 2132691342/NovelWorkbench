// 创作规划接口封装（字段与后端 domain/outline.go 对齐）
import { post } from '@/api/http'

/** 大纲节点类型编码 */
export type NWOutlineNodeTypeCode = 'volume' | 'arc' | 'storyBlock' | 'chapter'

/** 大纲节点状态编码 */
export type NWOutlineNodeStatusCode = 'draft' | 'planning' | 'ready' | 'done'

/** 场景节奏编码 */
export type NWOutlinePaceCode = 'slow' | 'medium' | 'fast' | 'climax'

/** 大纲节点视图对象 */
export interface NWOutlineNodeVo {
  outline_ulid: string
  project_ulid: string
  parent_outline_ulid: string
  node_type_code: NWOutlineNodeTypeCode
  title_text: string
  summary_text: string
  node_status_code: NWOutlineNodeStatusCode
  sort_order: number
  updated_at: string
  children?: NWOutlineNodeVo[]
}

/** 大纲节点保存请求（有 outline_ulid 为更新） */
export interface NWOutlineNodeSaveReqVo {
  outline_ulid?: string
  project_ulid: string
  parent_outline_ulid?: string
  node_type_code: NWOutlineNodeTypeCode
  title_text: string
  summary_text?: string
  node_status_code?: NWOutlineNodeStatusCode
  sort_order?: number
}

/** 详细场景卡片视图对象 */
export interface NWDetailedSceneVo {
  scene_ulid: string
  project_ulid: string
  outline_ulid: string
  character_ulid_list: string[]
  location_text: string
  conflict_text: string
  pace_code: NWOutlinePaceCode | ''
  estimated_words: number
  entry_state_text: string
  goal_text: string
  key_choice_text: string
  outcome_text: string
  exit_state_text: string
  sort_order: number
  updated_at: string
}

/** 详细场景卡片保存请求（有 scene_ulid 为更新） */
export interface NWDetailedSceneSaveReqVo {
  scene_ulid?: string
  project_ulid: string
  outline_ulid: string
  character_ulid_list?: string[]
  location_text?: string
  conflict_text?: string
  pace_code?: NWOutlinePaceCode | ''
  estimated_words?: number
  entry_state_text?: string
  goal_text?: string
  key_choice_text?: string
  outcome_text?: string
  exit_state_text?: string
  sort_order?: number
}

/** 保存大纲节点 */
export function saveOutlineNode(req: NWOutlineNodeSaveReqVo): Promise<NWOutlineNodeVo> {
  return post<NWOutlineNodeVo>('/outline/node/save', req)
}

/** 删除大纲节点（有子节点时后端拒绝） */
export async function deleteOutlineNode(projectUlid: string, outlineUlid: string): Promise<void> {
  await post('/outline/node/delete', { project_ulid: projectUlid, outline_ulid: outlineUlid })
}

/** 查询项目大纲树 */
export function listOutlineTree(projectUlid: string): Promise<NWOutlineNodeVo[]> {
  return post<NWOutlineNodeVo[]>('/outline/list', { project_ulid: projectUlid })
}

/** 保存详细场景卡片 */
export function saveDetailedScene(req: NWDetailedSceneSaveReqVo): Promise<NWDetailedSceneVo> {
  return post<NWDetailedSceneVo>('/outline/scene/save', req)
}

/** 删除详细场景卡片 */
export async function deleteDetailedScene(projectUlid: string, sceneUlid: string): Promise<void> {
  await post('/outline/scene/delete', { project_ulid: projectUlid, scene_ulid: sceneUlid })
}

/** 查询场景列表（outlineUlid 为空表示项目下全部场景） */
export function listDetailedScenes(projectUlid: string, outlineUlid?: string): Promise<NWDetailedSceneVo[]> {
  return post<NWDetailedSceneVo[]>('/outline/scene/list', {
    project_ulid: projectUlid,
    outline_ulid: outlineUlid ?? '',
  })
}

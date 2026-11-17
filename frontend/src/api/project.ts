// 项目 / 卷接口封装（字段与路径对齐 doc/api.md 第 3 节）
import { get, post } from '@/api/http'
import type {
  NWPageDto,
  NWProjectCreateReqVo,
  NWProjectListReqVo,
  NWProjectTreeVo,
  NWProjectUpdateReqVo,
  NWProjectVo,
  NWVolumeCreateReqVo,
  NWVolumeUpdateReqVo,
  NWVolumeVo,
} from './contracts'

/** 创建项目 */
export function createProject(req: NWProjectCreateReqVo): Promise<NWProjectVo> {
  return post<NWProjectVo>('/project/create', req)
}

/** 更新项目 */
export function updateProject(req: NWProjectUpdateReqVo): Promise<NWProjectVo> {
  return post<NWProjectVo>('/project/update', req)
}

/** 删除项目（级联删卷/章/设定） */
export async function deleteProject(projectUlid: string): Promise<void> {
  await post('/project/delete', { project_ulid: projectUlid })
}

/** 项目分页列表 */
export function listProjects(req: NWProjectListReqVo): Promise<NWPageDto<NWProjectVo>> {
  return post<NWPageDto<NWProjectVo>>('/project/list', req)
}

/** 项目详情 */
export function getProjectDetail(projectUlid: string): Promise<NWProjectVo> {
  return get<NWProjectVo>(`/project/detail?project_ulid=${encodeURIComponent(projectUlid)}`)
}

/** 项目目录树（卷 + 章节，写作工作台用） */
export function getProjectTree(projectUlid: string): Promise<NWProjectTreeVo> {
  return get<NWProjectTreeVo>(`/project/tree?project_ulid=${encodeURIComponent(projectUlid)}`)
}

/** 新建卷 */
export function createVolume(req: NWVolumeCreateReqVo): Promise<NWVolumeVo> {
  return post<NWVolumeVo>('/volume/create', req)
}

/** 更新卷（仅覆盖非空字段） */
export function updateVolume(req: NWVolumeUpdateReqVo): Promise<NWVolumeVo> {
  return post<NWVolumeVo>('/volume/update', req)
}

/** 删除卷（级联删卷下章节并回补字数） */
export async function deleteVolume(volumeUlid: string): Promise<void> {
  await post('/volume/delete', { volume_ulid: volumeUlid })
}

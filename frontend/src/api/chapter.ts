// 章节 / 快照接口封装（字段与路径对齐 doc/api.md 第 3 节）
import { get, post } from '@/api/http'
import type {
  NWChapterCreateReqVo,
  NWChapterDetailVo,
  NWChapterSnapshotCreateReqVo,
  NWChapterSnapshotRestoreReqVo,
  NWChapterSnapshotVo,
  NWChapterUpdateReqVo,
} from './contracts'

/** 新建章节 */
export function createChapter(req: NWChapterCreateReqVo): Promise<NWChapterDetailVo> {
  return post<NWChapterDetailVo>('/chapter/create', req)
}

/** 更新章节（内容整体覆盖；仅覆盖非空字段） */
export function updateChapter(req: NWChapterUpdateReqVo): Promise<NWChapterDetailVo> {
  return post<NWChapterDetailVo>('/chapter/update', req)
}

/** 删除章节（回补字数） */
export async function deleteChapter(chapterUlid: string): Promise<void> {
  await post('/chapter/delete', { chapter_ulid: chapterUlid })
}

/** 章节详情（含正文） */
export function getChapterDetail(chapterUlid: string): Promise<NWChapterDetailVo> {
  return get<NWChapterDetailVo>(`/chapter/detail/${encodeURIComponent(chapterUlid)}`)
}

/** 章节快照列表 */
export function listChapterSnapshots(chapterUlid: string): Promise<NWChapterSnapshotVo[]> {
  return get<NWChapterSnapshotVo[]>(
    `/chapter/snapshot/list/${encodeURIComponent(chapterUlid)}`,
  )
}

/** 章节快照手动留档 */
export function createChapterSnapshot(
  req: NWChapterSnapshotCreateReqVo,
): Promise<NWChapterSnapshotVo> {
  return post<NWChapterSnapshotVo>('/chapter/snapshot/create', req)
}

/** 回滚章节到快照（回滚前服务端自动留档当前正文） */
export function restoreChapterSnapshot(
  req: NWChapterSnapshotRestoreReqVo,
): Promise<NWChapterDetailVo> {
  return post<NWChapterDetailVo>('/chapter/snapshot/restore', req)
}

/** 章节体检结果（字段对齐后端 ChapterCheckVO） */
export interface NWChapterCheckVo {
  word_count: number
  paragraph_count: number
  dialogue_ratio: number
  dash_count: number
  banned_word_count: number
  warn_word_count: number
  emotion_word_count: number
  bare_tag_count: number
  hook_present: boolean
  score_value: number
  verdict_code: string
  issue_list: {
    issue_code: string
    level_code: string
    title_text: string
    detail_text: string
    hit_count: number
    sample_text: string
  }[]
}

/** 章节体检：正文优先，留空取库内正文 */
export function checkChapter(params: {
  chapter_ulid?: string
  content_text?: string
}): Promise<NWChapterCheckVo> {
  return post<NWChapterCheckVo>('/chapter/check', params)
}

// 创作规划的枚举文案与配色：节点类型、节点状态、场景节奏统一在此维护。
import type {
  NWOutlineNodeStatusCode,
  NWOutlineNodeTypeCode,
  NWOutlinePaceCode
} from '@/api/outline'

/** 下拉选项（值 + 中文名） */
export interface OutlineOption<T extends string> {
  value: T
  label: string
}

/** 节点类型选项 */
export const NODE_TYPE_OPTIONS: OutlineOption<NWOutlineNodeTypeCode>[] = [
  { value: 'volume', label: '卷' },
  { value: 'arc', label: '篇章' },
  { value: 'storyBlock', label: '情节块' },
  { value: 'chapter', label: '章节' }
]

/** 节点状态选项 */
export const NODE_STATUS_OPTIONS: OutlineOption<NWOutlineNodeStatusCode>[] = [
  { value: 'draft', label: '草稿' },
  { value: 'planning', label: '规划中' },
  { value: 'ready', label: '就绪' },
  { value: 'done', label: '完成' }
]

/** 场景节奏选项（空串表示未设定） */
export const PACE_OPTIONS: OutlineOption<NWOutlinePaceCode | ''>[] = [
  { value: '', label: '未设定' },
  { value: 'slow', label: '慢节奏' },
  { value: 'medium', label: '中速' },
  { value: 'fast', label: '快节奏' },
  { value: 'climax', label: '高潮' }
]

/** 节点类型中文名 */
export const nodeTypeLabel = (typeCode: string): string =>
  NODE_TYPE_OPTIONS.find((item) => item.value === typeCode)?.label ?? typeCode

/** 节点状态中文名 */
export const nodeStatusLabel = (statusCode: string): string =>
  NODE_STATUS_OPTIONS.find((item) => item.value === statusCode)?.label ?? statusCode

/** 节点状态徽标配色 */
export const nodeStatusBadge = (statusCode: string): string => {
  if (statusCode === 'done' || statusCode === 'ready') return 'success'
  if (statusCode === 'planning') return 'warning'
  return 'neutral'
}

/** 场景节奏中文名 */
export const paceLabel = (paceCode: string): string =>
  PACE_OPTIONS.find((item) => item.value === paceCode)?.label ?? (paceCode || '未设定')

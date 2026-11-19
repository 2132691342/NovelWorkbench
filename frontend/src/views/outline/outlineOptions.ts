// 创作规划的节点类型/状态/节奏选项与展示文案（标签由调用方传入 i18n t）。

import type {
  NWOutlineNodeTypeCode,
  NWOutlineNodeStatusCode,
  NWOutlinePaceCode,
} from '@/api/outline'

type TranslateFn = (key: string) => string

export const nodeTypeOptionList: Array<{ value: NWOutlineNodeTypeCode; label_key: string }> = [
  { value: 'volume', label_key: 'outline.nodeVolume' },
  { value: 'arc', label_key: 'outline.nodeArc' },
  { value: 'storyBlock', label_key: 'outline.nodeBlock' },
  { value: 'chapter', label_key: 'outline.nodeChapter' },
]

export const nodeStatusOptionList: Array<{ value: NWOutlineNodeStatusCode; label_key: string }> = [
  { value: 'draft', label_key: 'outline.statusDraft' },
  { value: 'planning', label_key: 'outline.statusPlanning' },
  { value: 'ready', label_key: 'outline.statusReady' },
  { value: 'done', label_key: 'outline.statusDone' },
]

export const paceOptionList: Array<{ value: NWOutlinePaceCode; label_key: string }> = [
  { value: 'slow', label_key: 'outline.paceSlow' },
  { value: 'medium', label_key: 'outline.paceMedium' },
  { value: 'fast', label_key: 'outline.paceFast' },
  { value: 'climax', label_key: 'outline.paceClimax' },
]

export function nodeTypeLabel(nodeTypeCode: string, t: TranslateFn): string {
  const option = nodeTypeOptionList.find((item) => item.value === nodeTypeCode)
  return option ? t(option.label_key) : nodeTypeCode
}

export function nodeStatusLabel(statusCode: string, t: TranslateFn): string {
  const option = nodeStatusOptionList.find((item) => item.value === statusCode)
  return option ? t(option.label_key) : statusCode
}

export function paceLabel(paceCode: string, t: TranslateFn): string {
  const option = paceOptionList.find((item) => item.value === paceCode)
  return option ? t(option.label_key) : paceCode
}

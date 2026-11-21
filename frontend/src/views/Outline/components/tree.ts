// 大纲树拍平工具：把父子结构转成带层级的扁平行，便于渲染与折叠控制。
import type { NWOutlineNodeVo } from '@/api/outline'

/** 扁平行：节点本体 + 层级 + 是否有子节点 */
export interface OutlineFlatRow {
  node: NWOutlineNodeVo
  depth: number
  hasChildren: boolean
}

/**
 * 把大纲树拍平为渲染行；collapsedMap 命中的节点会隐藏其整棵子树。
 */
export function flattenOutlineTree(
  nodeList: NWOutlineNodeVo[],
  collapsedMap: Record<string, boolean>,
  depth = 0
): OutlineFlatRow[] {
  const rowList: OutlineFlatRow[] = []
  for (const node of nodeList) {
    const children = node.children ?? []
    rowList.push({ node, depth, hasChildren: children.length > 0 })
    if (children.length > 0 && !collapsedMap[node.outline_ulid]) {
      rowList.push(...flattenOutlineTree(children, collapsedMap, depth + 1))
    }
  }
  return rowList
}

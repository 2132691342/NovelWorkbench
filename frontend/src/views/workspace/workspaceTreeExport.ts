// 写作台目录导出：把卷/章结构写成 XLSX 大纲（按需加载 XLSX，控制首屏体积）。

import { ElMessage } from 'element-plus'
import type { NWProjectTreeVo } from '@/api/contracts'

interface ExportLabels {
  volume: string
  chapter: string
  order: string
  words: string
  status: string
  sheetName: string
  fileName: string
  success: string
  failed: string
  empty: string
}

/** 导出项目目录为 XLSX；treeData 为空或无卷时提示并返回。 */
export async function exportProjectOutline(
  treeData: NWProjectTreeVo | null,
  chapterStatusText: (statusCode: string) => string,
  labels: ExportLabels,
): Promise<void> {
  if (!treeData || treeData.volume_list.length === 0) {
    ElMessage.warning(labels.empty)
    return
  }
  const rowList: Record<string, string | number>[] = []
  for (const volumeNode of treeData.volume_list) {
    if (volumeNode.chapter_list.length === 0) {
      rowList.push({
        [labels.volume]: volumeNode.volume_vo.volume_name,
        [labels.chapter]: '',
        [labels.order]: 0,
        [labels.words]: 0,
        [labels.status]: '',
      })
      continue
    }
    for (const chapter of volumeNode.chapter_list) {
      rowList.push({
        [labels.volume]: volumeNode.volume_vo.volume_name,
        [labels.chapter]: chapter.chapter_name,
        [labels.order]: chapter.chapter_order,
        [labels.words]: chapter.word_count,
        [labels.status]: chapterStatusText(chapter.chapter_status),
      })
    }
  }
  try {
    const XLSX = await import('xlsx')
    const worksheet = XLSX.utils.json_to_sheet(rowList)
    const workbook = XLSX.utils.book_new()
    XLSX.utils.book_append_sheet(workbook, worksheet, labels.sheetName)
    XLSX.writeFile(workbook, `${treeData.project_vo.project_name}-${labels.fileName}.xlsx`)
    ElMessage.success(labels.success)
  } catch {
    ElMessage.error(labels.failed)
  }
}

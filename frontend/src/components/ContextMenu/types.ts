// 右键菜单的数据结构定义。
export interface ContextMenuSchema {
  disabled?: boolean
  divided?: boolean
  icon?: string
  label: string
  command?: () => void
}

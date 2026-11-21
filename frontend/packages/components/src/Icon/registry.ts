// 图标注册表：登记图标名与组件的映射。
import { addIcon, type IconifyIcon } from '@iconify/vue/offline'

export type IconRegistry = Readonly<Record<string, IconifyIcon>>

export const registerIcons = (icons: IconRegistry) => {
  Object.entries(icons).forEach(([name, data]) => addIcon(name, data))
}

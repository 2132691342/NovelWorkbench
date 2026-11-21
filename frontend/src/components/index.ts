// 全局组件注册入口：登记图标组件与图标表。
import type { App } from 'vue'
import { Icon, registerIcons } from '@novelworkbench/components'
import { icons } from '@/icons'

export const setupGlobCom = (app: App<Element>): void => {
  registerIcons(icons)
  app.component('Icon', Icon)
}

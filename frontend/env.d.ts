/// <reference types="vite/client" />

// UnoCSS 虚拟样式模块
declare module 'virtual:uno.css'

// turndown-plugin-gfm 未提供类型声明，仅用到 gfm 插件本体
declare module 'turndown-plugin-gfm' {
  import type TurndownService from 'turndown'
  export const gfm: TurndownService.Plugin
  export const tables: TurndownService.Plugin
  export const strikethrough: TurndownService.Plugin
  export const taskListItems: TurndownService.Plugin
}

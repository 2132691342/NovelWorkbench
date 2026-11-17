// @wangeditor/editor-for-vue 的 package.json exports 缺少 types 条件，
// moduleResolution=bundler 解析不到官方 d.ts（dist/src/index.d.ts），这里补充最小模块声明。
declare module '@wangeditor/editor-for-vue' {
  import type { Component } from 'vue'

  export const Editor: Component
  export const Toolbar: Component
}

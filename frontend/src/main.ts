// 应用入口：装配 Pinia、路由、鉴权、i18n、组件库与样式后挂载。
import 'vue/jsx'
import 'virtual:uno.css'

import { createApp } from 'vue'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'

import App from './App.vue'

import { setupI18n } from '@/plugins/vueI18n'
import { setupStore } from '@/store'
import { setupGlobCom } from '@/components'
import { setupRouter } from './router'
import { setupPermission } from './permission'

import '@novelworkbench/styles'

// 装配顺序：状态 → 鉴权/导航 → i18n → 全局组件 → 组件库 → 路由 → 挂载
const setupAll = async () => {
  const app = createApp(App)

  setupStore(app)

  await setupPermission()

  setupI18n(app)

  setupGlobCom(app)

  // 全量注册：业务页直接写 <ElButton>，不再逐页维护 import 清单
  app.use(ElementPlus)

  setupRouter(app)

  app.mount('#app')
}

setupAll()

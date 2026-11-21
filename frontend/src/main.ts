// 应用入口：挂载 Vue 实例、注册 Pinia、路由、Element Plus 与 UnoCSS。

import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import 'element-plus/theme-chalk/dark/css-vars.css'
import 'virtual:uno.css'
import '@/styles/variables.css'
import '@/styles/theme-dark.css'
import '@/styles/element-theme.css'
import '@/styles/button-states.css'
import '@/styles/page.css'
import '@/styles/global.css'
import App from './App.vue'
import router from './router'
import i18n from './i18n'
import { useAppStore } from '@/stores/app'

const app = createApp(App)

app.use(createPinia())
app.use(router)
app.use(i18n)
app.use(ElementPlus)

// 主题必须在挂载前应用，否则首帧会闪亮色
useAppStore().initTheme()

app.mount('#app')

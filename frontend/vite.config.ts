import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import UnoCSS from 'unocss/vite'

/**
 * 手动分包：把体积大的三方库拆成独立 chunk，避免首屏主包过大。
 * - element-plus：组件库
 * - echarts：图表（连同其渲染引擎 zrender）
 * - x6：图编辑引擎（连同 @antv 系列依赖）
 */
function manualChunks(moduleId: string): string | undefined {
  if (moduleId.includes('node_modules/element-plus')) {
    return 'element-plus'
  }
  if (moduleId.includes('node_modules/echarts') || moduleId.includes('node_modules/zrender')) {
    return 'echarts'
  }
  if (moduleId.includes('node_modules/@antv')) {
    return 'x6'
  }
  return undefined
}

export default defineConfig({
  plugins: [vue(), UnoCSS()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks,
      },
    },
  },
})

import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vitest/config'

/** 前端单元测试：只跑纯逻辑（流式报文解析与事件归并），不渲染组件。 */
export default defineConfig({
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  test: {
    environment: 'node',
    include: ['src/**/*.test.ts']
  }
})

import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vitest/config'

/** 前端单元测试：只跑纯逻辑（流式解析与归并、表单校验、请求客户端），不渲染组件。 */
export default defineConfig({
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  test: {
    environment: 'node',
    include: ['src/**/*.test.ts', 'packages/*/test/**/*.test.ts']
  }
})

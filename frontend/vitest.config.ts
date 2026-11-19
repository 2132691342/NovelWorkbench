import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vitest/config'

/** 前端单元测试配置：只跑纯逻辑单测，复用 vite 的 @ 别名。 */
export default defineConfig({
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  test: {
    environment: 'node',
    include: ['src/**/*.test.ts'],
  },
})
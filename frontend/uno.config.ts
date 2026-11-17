import { defineConfig, presetUno } from 'unocss'
import transformerDirectives from '@unocss/transformer-directives'

// UnoCSS 配置：原子化工具类 + @apply/@variants 指令支持
export default defineConfig({
  presets: [presetUno()],
  transformers: [transformerDirectives()],
})

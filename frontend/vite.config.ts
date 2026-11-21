import { resolve } from 'path'
import { loadEnv } from 'vite'
import type { UserConfig, ConfigEnv } from 'vite'
import Vue from '@vitejs/plugin-vue'
import VueJsx from '@vitejs/plugin-vue-jsx'
import VueI18nPlugin from '@intlify/unplugin-vue-i18n/vite'
import UnoCSS from 'unocss/vite'

const root = process.cwd()

export default ({ mode }: ConfigEnv): UserConfig => {
  const env = loadEnv(mode, root)
  return {
    base: env.VITE_BASE_PATH,
    plugins: [
      Vue(),
      VueJsx(),
      VueI18nPlugin({
        runtimeOnly: true,
        compositionOnly: true,
        include: [resolve(import.meta.dirname, 'src/locales/**')]
      }),
      UnoCSS()
    ],

    resolve: {
      alias: [
        {
          find: /@\//,
          replacement: `${resolve(root, 'src')}/`
        }
      ]
    },
    build: {
      target: 'es2015',
      outDir: env.VITE_OUT_DIR || 'dist',
      sourcemap: env.VITE_SOURCEMAP === 'true',
      cssCodeSplit: !(env.VITE_USE_CSS_SPLIT === 'false'),
      cssTarget: ['chrome31'],
      rolldownOptions: {
        output: {
          codeSplitting: {
            // 大体积三方库单独成块：首屏不背编辑器与画布，访问对应页面时才下载
            groups: [
              { name: 'vendor-vue', test: /node_modules[\\/](vue|vue-router|pinia|@vue)[\\/]/ },
              { name: 'vendor-element', test: /node_modules[\\/](element-plus|@floating-ui)[\\/]/ },
              {
                name: 'vendor-editor',
                test: /node_modules[\\/](@codemirror|@lezer|mavon-editor|@wangeditor|turndown)[\\/]/
              },
              { name: 'vendor-canvas', test: /node_modules[\\/](echarts|zrender|@antv)[\\/]/ },
              {
                name: 'vendor-doc',
                test: /node_modules[\\/](docx-preview|vue-pdf-embed|pdfjs-dist|marked|xlsx)[\\/]/
              }
            ]
          }
        }
      }
    },
    server: {
      port: 4000,
      proxy: {
        '/api': {
          target: process.env.NW_DEV_PROXY_TARGET || 'http://127.0.0.1:8080',
          changeOrigin: true
        }
      },
      hmr: {
        overlay: false
      },
      host: '0.0.0.0'
    }
  }
}

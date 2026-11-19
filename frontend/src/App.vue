<script setup lang="ts">

// 应用根组件：装配 Element Plus 国际化与主题，负责全局过渡与布局渲染。
import { computed } from 'vue'
import type { Language } from 'element-plus/es/locale'
import zhCnLocale from 'element-plus/es/locale/lang/zh-cn'
import enLocale from 'element-plus/es/locale/lang/en'
import { useAppStore } from '@/stores/app'

const appStore = useAppStore()

// 语言包字面量结构与 Element Plus 的 Language 形状兼容，这里做边界转换
const elementPlusLocale = computed(
  () => (appStore.locale_code === 'en-US' ? enLocale : zhCnLocale) as unknown as Language,
)
</script>

<template>
  <el-config-provider :locale="elementPlusLocale">
    <router-view v-slot="{ Component }">
      <transition name="fade-slide" mode="out-in">
        <component :is="Component" />
      </transition>
    </router-view>
  </el-config-provider>
</template>

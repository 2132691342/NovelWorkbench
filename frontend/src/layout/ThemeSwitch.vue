<!--
  主题切换：亮色 / 暗色 / 跟随系统三态，切换时同步 html[data-theme] 与 Element Plus 暗色变量。
  选择持久化到 localStorage，刷新不闪回。
-->
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import NWIcon from '@/components/NWIcon.vue'
import { useAppStore, type NWThemeCode } from '@/stores/app'

const appStore = useAppStore()
const { t } = useI18n()

/** 三态选项 */
const themeOptionList = computed<{ theme_code: NWThemeCode; theme_name: string; icon_code: string }[]>(() => [
  { theme_code: 'light', theme_name: t('layout.themeLight'), icon_code: 'sparkles' },
  { theme_code: 'dark', theme_name: t('layout.themeDark'), icon_code: 'lock' },
  { theme_code: 'system', theme_name: t('layout.themeSystem'), icon_code: 'sliders' },
])

/** 当前图标：跟随系统时按实际生效主题显示 */
const currentIconCode = computed(() => {
  const matchedOption = themeOptionList.value.find(
    (optionItem) => optionItem.theme_code === appStore.theme_code,
  )
  if (appStore.theme_code === 'system') {
    return appStore.resolved_theme_code === 'dark' ? 'lock' : 'sparkles'
  }
  return matchedOption?.icon_code ?? 'sparkles'
})

/** 当前提示文案 */
const currentThemeLabel = computed(() => {
  const matchedOption = themeOptionList.value.find(
    (optionItem) => optionItem.theme_code === appStore.theme_code,
  )
  return matchedOption?.theme_name ?? t('layout.themeLight')
})

function handleCommand(themeCode: string | number | object): void {
  appStore.setTheme(String(themeCode) as NWThemeCode)
}
</script>

<template>
  <el-dropdown trigger="click" @command="handleCommand">
    <button
      type="button"
      class="theme-switch"
      :aria-label="currentThemeLabel"
      :title="currentThemeLabel"
    >
      <NWIcon :icon_code="currentIconCode" :size_px="18" />
    </button>
    <template #dropdown>
      <el-dropdown-menu>
        <el-dropdown-item
          v-for="optionItem in themeOptionList"
          :key="optionItem.theme_code"
          :command="optionItem.theme_code"
        >
          <span class="theme-switch__option">
            <NWIcon :icon_code="optionItem.icon_code" :size_px="14" />
            <span>{{ optionItem.theme_name }}</span>
            <NWIcon
              v-if="appStore.theme_code === optionItem.theme_code"
              icon_code="check"
              :size_px="13"
              class="theme-switch__check"
            />
          </span>
        </el-dropdown-item>
      </el-dropdown-menu>
    </template>
  </el-dropdown>
</template>

<style scoped src="./ThemeSwitch.css"></style>

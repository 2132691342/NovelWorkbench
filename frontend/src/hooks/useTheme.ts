// 主题切换：按应用状态切换根节点的明暗 class。
import { watch } from 'vue'
import { useAppStore } from '@/store/modules/app'

export const useTheme = () => {
  const appStore = useAppStore()

  watch(
    () => appStore.isDark,
    (isDark) => {
      document.documentElement.classList.toggle('dark', isDark)
      document.documentElement.classList.toggle('light', !isDark)
    },
    { immediate: true }
  )
}

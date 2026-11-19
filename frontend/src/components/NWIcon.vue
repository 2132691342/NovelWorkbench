<script setup lang="ts">

// 图标组件：按后端下发的 icon_code 渲染线性图标（统一 24×24 网格，避免各页面各写一份 SVG）。

import { computed } from 'vue'

/** 图标路径表：key 为后端的 icon_code，value 为 <path d> 列表 */
const NW_ICON_PATH_MAP: Record<string, string[]> = {
  dashboard: ['M4 4h7v7H4z', 'M13 4h7v7h-7z', 'M4 13h7v7H4z', 'M13 13h7v7h-7z'],
  book: ['M5 4h9l5 5v11H5z', 'M14 4v5h5', 'M8.5 13h7', 'M8.5 16.5h4'],
  edit: ['M4 20h4l10-10-4-4L4 16z', 'M14 6l4 4'],
  tree: ['M5 4h14v16H5z', 'M9 4v16', 'M13 8h3', 'M13 12h3', 'M13 16h2'],
  shield: ['M12 3l7 3v5c0 4.5-2.9 8.3-7 10-4.1-1.7-7-5.5-7-10V6z', 'M9 12l2 2 4-4'],
  user: ['M12 11a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7z', 'M5 20c0-3.3 3.1-5.5 7-5.5s7 2.2 7 5.5'],
  globe: ['M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18z', 'M3.5 12h17', 'M12 3c2.4 2.4 3.6 5.4 3.6 9s-1.2 6.6-3.6 9'],
  database: ['M12 8c4.4 0 8-1.3 8-3s-3.6-3-8-3-8 1.3-8 3 3.6 3 8 3z', 'M4 5v14c0 1.7 3.6 3 8 3s8-1.3 8-3V5', 'M4 12c0 1.7 3.6 3 8 3s8-1.3 8-3'],
  note: ['M6 3.5h9l3 3v14H6z', 'M9 9h6', 'M9 12.5h6', 'M9 16h4'],
  layers: ['M12 3l8 4-8 4-8-4z', 'M4 12l8 4 8-4', 'M4 17l8 4 8-4'],
  model: ['M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6z', 'M12 3.5v2', 'M12 18.5v2', 'M3.5 12h2', 'M18.5 12h2', 'M6 6l1.4 1.4', 'M16.6 16.6L18 18', 'M18 6l-1.4 1.4', 'M7.4 16.6L6 18'],
  wand: ['M5 19l9-9', 'M14 4l1 2 2 1-2 1-1 2-1-2-2-1 2-1z', 'M19 12l.7 1.3 1.3.7-1.3.7-.7 1.3-.7-1.3-1.3-.7 1.3-.7z'],
  settings: ['M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6z', 'M4 12h2', 'M18 12h2', 'M12 4v2', 'M12 18v2'],
  users: ['M9 11a3 3 0 1 0 0-6 3 3 0 0 0 0 6z', 'M3 20c0-3 2.7-5 6-5s6 2 6 5', 'M16 11a2.6 2.6 0 1 0 0-5.2', 'M17 15c2.2.5 4 2.4 4 5'],
  key: ['M14 4a6 6 0 1 0-4.2 10.2L4 20v-3h2v-2h2v-2h2l1.8-1.8A6 6 0 0 0 14 4z', 'M16.5 7.5h.01'],
  menu: ['M4 6h16', 'M4 12h16', 'M4 18h10'],
  lock: ['M6 11h12v10H6z', 'M9 11V8a3 3 0 0 1 6 0v3'],
  history: ['M4 12a8 8 0 1 0 3-6.2', 'M4 4v4h4', 'M12 8v4l3 2'],
  chart: ['M4 20V9', 'M9.3 20V4', 'M14.7 20v-7', 'M20 20V7'],
  pulse: ['M3 12h4l2-5 3 10 2-5h5'],
  sliders: ['M4 7h10', 'M18 7h2', 'M4 12h4', 'M12 12h8', 'M4 17h12', 'M20 17h0'],
  folder: ['M4 6h6l2 3h8v11H4z'],
  plus: ['M12 5v14', 'M5 12h14'],
  search: ['M11 19a8 8 0 1 0 0-16 8 8 0 0 0 0 16z', 'M21 21l-4.3-4.3'],
  bell: ['M18 15V9a6 6 0 1 0-12 0v6l-2 3h16z', 'M10 21h4'],
  fullscreen: ['M4 9V4h5', 'M20 9V4h-5', 'M4 15v5h5', 'M20 15v5h-5'],
  collapse: ['M4 6h16', 'M4 12h16', 'M4 18h16'],
  check: ['M5 13l4 4L19 7'],
  close: ['M6 6l12 12', 'M18 6L6 18'],
  refresh: ['M20 12a8 8 0 1 1-2.3-5.6', 'M20 4v5h-5'],
}

const props = withDefaults(
  defineProps<{
    icon_code: string
    size_px?: number
  }>(),
  { size_px: 18 },
)

const pathList = computed(() => NW_ICON_PATH_MAP[props.icon_code] ?? NW_ICON_PATH_MAP.folder)

</script>

<template>
  <svg
    class="nw-icon"
    :width="size_px"
    :height="size_px"
    viewBox="0 0 24 24"
    fill="none"
    aria-hidden="true"
  >
    <path
      v-for="(pathItem, pathIndex) in pathList"
      :key="pathIndex"
      :d="pathItem"
      stroke="currentColor"
      stroke-width="1.8"
      stroke-linecap="round"
      stroke-linejoin="round"
    />
  </svg>
</template>

<style scoped>
.nw-icon {
  flex-shrink: 0;
  display: block;
}
</style>

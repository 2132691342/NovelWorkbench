<!--
  侧边栏菜单：后端菜单树渲染为分组 + 页面项，折叠时只留图标并出悬浮提示。
  所有状态（激活、禁用、悬停、按下）都在此组件内闭合，页面不再各写一份。
-->
<script setup lang="ts">
import { computed } from 'vue'
import NWIcon from '@/components/NWIcon.vue'
import type { NWMenuItemVo } from '@/api/contracts'

const props = defineProps<{
  menu_tree_list: NWMenuItemVo[]
  active_menu_code: string
  collapsed_flag: boolean
  is_menu_disabled: (menuItem: NWMenuItemVo) => boolean
}>()

const emit = defineEmits<{ select: [menuItem: NWMenuItemVo] }>()

/** 过滤掉没有可见子项的分组 */
const visibleMenuList = computed(() =>
  props.menu_tree_list
    .map((menuItem) => ({
      ...menuItem,
      child_list: (menuItem.child_list ?? []).filter(
        (childItem) => childItem.menu_type_code === 'page',
      ),
    }))
    .filter(
      (menuItem) => menuItem.menu_type_code === 'page' || menuItem.child_list.length > 0,
    ),
)

/** 点击菜单项 */
function handleClick(menuItem: NWMenuItemVo): void {
  if (props.is_menu_disabled(menuItem)) {
    return
  }
  emit('select', menuItem)
}
</script>

<template>
  <nav class="nw-menu" :class="{ 'is-collapsed': collapsed_flag }">
    <template v-for="menuItem in visibleMenuList" :key="menuItem.menu_code">
      <!-- 单页菜单（仪表盘 / 系统设置） -->
      <el-tooltip
        v-if="menuItem.menu_type_code === 'page'"
        :content="menuItem.menu_name"
        placement="right"
        :disabled="!collapsed_flag"
        :show-after="120"
      >
        <button
          type="button"
          class="nw-menu__item"
          :class="{ 'is-active': active_menu_code === menuItem.menu_code }"
          @click="handleClick(menuItem)"
        >
          <NWIcon :icon_code="menuItem.icon_code" :size_px="18" />
          <span v-show="!collapsed_flag" class="nw-menu__text">{{ menuItem.menu_name }}</span>
          <span
            v-if="!collapsed_flag && active_menu_code === menuItem.menu_code"
            class="nw-menu__dot"
          />
        </button>
      </el-tooltip>

      <!-- 分组菜单 -->
      <div v-else class="nw-menu__group">
        <div v-show="!collapsed_flag" class="nw-menu__group-title">
          {{ menuItem.menu_name }}
        </div>
        <el-tooltip
          v-for="childItem in menuItem.child_list"
          :key="childItem.menu_code"
          :content="childItem.menu_name"
          placement="right"
          :disabled="!collapsed_flag"
          :show-after="120"
        >
          <button
            type="button"
            class="nw-menu__item"
            :class="{
              'is-active': active_menu_code === childItem.menu_code,
              'is-disabled': is_menu_disabled(childItem),
            }"
            @click="handleClick(childItem)"
          >
            <NWIcon :icon_code="childItem.icon_code" :size_px="18" />
            <span v-show="!collapsed_flag" class="nw-menu__text">{{ childItem.menu_name }}</span>
            <span
              v-if="!collapsed_flag && active_menu_code === childItem.menu_code"
              class="nw-menu__dot"
            />
          </button>
        </el-tooltip>
      </div>
    </template>
  </nav>
</template>

<style scoped src="./SidebarMenu.css"></style>

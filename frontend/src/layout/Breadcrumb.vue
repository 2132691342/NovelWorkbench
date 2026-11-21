<!--
  面包屑：展示「分组 / 页面」两级路径，首级为应用名，末级为当前页。
  数据来源与侧边栏一致（后端菜单树），保证标题永远与菜单文案同步。
-->
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { NWMenuItemVo } from '@/api/contracts'

const props = defineProps<{
  menu_tree_list: NWMenuItemVo[]
  active_menu_code: string
}>()

const { t } = useI18n()

/** 面包屑项：分组名（可空）+ 页面名 */
const breadcrumbItemList = computed<{ group_name: string; page_name: string }[]>(() => {
  if (!props.active_menu_code) {
    return [{ group_name: '', page_name: t('common.appName') }]
  }
  for (const menuItem of props.menu_tree_list) {
    if (menuItem.menu_type_code === 'page' && menuItem.menu_code === props.active_menu_code) {
      return [{ group_name: '', page_name: menuItem.menu_name }]
    }
    for (const childItem of menuItem.child_list ?? []) {
      if (childItem.menu_code === props.active_menu_code) {
        return [{ group_name: menuItem.menu_name, page_name: childItem.menu_name }]
      }
    }
  }
  return [{ group_name: '', page_name: t('common.appName') }]
})
</script>

<template>
  <nav class="nw-breadcrumb" :aria-label="t('layout.breadcrumbLabel')">
    <template v-for="(crumbItem, crumbIndex) in breadcrumbItemList" :key="crumbIndex">
      <span v-if="crumbItem.group_name" class="nw-breadcrumb__item is-muted">
        {{ crumbItem.group_name }}
      </span>
      <span v-if="crumbItem.group_name" class="nw-breadcrumb__separator">/</span>
      <span class="nw-breadcrumb__item is-current">{{ crumbItem.page_name }}</span>
    </template>
  </nav>
</template>

<style scoped src="./Breadcrumb.css"></style>

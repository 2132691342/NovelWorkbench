<!--
  快速跳转弹层：搜索后端下发的页面菜单，Esc 关闭，点击进入。
  菜单过滤与分组在组件内完成，父级只负责开关与导航。
-->
<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { NWMenuItemVo } from '@/api/contracts'
import NWIcon from '@/components/NWIcon.vue'

const props = defineProps<{
  visible: boolean
  menu_tree_list: NWMenuItemVo[]
  is_disabled: (menuItem: NWMenuItemVo) => boolean
}>()

const emit = defineEmits<{
  'update:visible': [visible: boolean]
  select: [menuItem: NWMenuItemVo]
}>()

const { t } = useI18n()
const keyword = ref('')
const inputRef = ref<HTMLInputElement | null>(null)

watch(
  () => props.visible,
  (visibleFlag) => {
    if (!visibleFlag) {
      return
    }
    keyword.value = ''
    void nextTick(() => inputRef.value?.focus())
  },
)

/** 按关键词过滤并保留分组结构 */
const groupList = computed(() => {
  const searchText = keyword.value.trim().toLowerCase()
  const matched = (menuItem: NWMenuItemVo): boolean =>
    !searchText ||
    menuItem.menu_name.toLowerCase().includes(searchText) ||
    menuItem.menu_code.toLowerCase().includes(searchText) ||
    menuItem.route_path.toLowerCase().includes(searchText)
  return props.menu_tree_list
    .map((menuItem) => {
      if (menuItem.menu_type_code === 'page') {
        return { group_name: t('layout.commandGroupPage'), items: matched(menuItem) ? [menuItem] : [] }
      }
      return {
        group_name: menuItem.menu_name,
        items: (menuItem.child_list ?? []).filter(matched),
      }
    })
    .filter((groupItem) => groupItem.items.length > 0)
})

function handleSelect(menuItem: NWMenuItemVo): void {
  if (props.is_disabled(menuItem)) {
    return
  }
  emit('select', menuItem)
}
</script>

<template>
  <el-dialog
    :model-value="visible"
    width="580px"
    class="admin-layout__command-dialog"
    :show-close="false"
    align-center
    @update:model-value="emit('update:visible', $event)"
  >
    <template #header>
      <div class="admin-layout__command-head">
        <NWIcon icon_code="search" :size_px="18" />
        <input
          ref="inputRef"
          v-model="keyword"
          class="admin-layout__command-input"
          :placeholder="t('layout.quickJumpPlaceholder')"
        />
        <button type="button" class="admin-layout__command-close" @click="emit('update:visible', false)">
          Esc
        </button>
      </div>
    </template>
    <div class="admin-layout__command-list">
      <section v-for="groupItem in groupList" :key="groupItem.group_name" class="admin-layout__command-group">
        <h4 class="admin-layout__command-group-title">{{ groupItem.group_name }}</h4>
        <button
          v-for="menuItem in groupItem.items"
          :key="menuItem.menu_code"
          type="button"
          class="admin-layout__command-item"
          :disabled="is_disabled(menuItem)"
          @click="handleSelect(menuItem)"
        >
          <span class="admin-layout__command-item-icon">
            <NWIcon :icon_code="menuItem.icon_code" :size_px="16" />
          </span>
          <span class="admin-layout__command-item-text">
            <strong>{{ menuItem.menu_name }}</strong>
            <small>{{ menuItem.menu_code }}</small>
          </span>
          <NWIcon icon_code="arrowRight" :size_px="14" class="admin-layout__command-item-arrow" />
        </button>
      </section>
      <p v-if="!groupList.length" class="admin-layout__command-empty">
        {{ t('layout.quickJumpEmpty') }}
      </p>
    </div>
  </el-dialog>
</template>

<style scoped src="./CommandPalette.css"></style>

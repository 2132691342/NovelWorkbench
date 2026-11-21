<!--
  多页签栏：显示已访问页面，支持点击切换、关闭单个、右键批量关闭。
  固定页签（仪表盘）不可关闭；关闭当前页签时自动跳到相邻页签。
-->
<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import NWIcon from '@/components/NWIcon.vue'
import { useTagsViewStore, type NWTagView } from '@/stores/tagsView'

const route = useRoute()
const router = useRouter()
const tagsViewStore = useTagsViewStore()
const { t } = useI18n()

/** 右键菜单状态 */
const menuVisible = ref(false)
const menuLeftPx = ref(0)
const menuTopPx = ref(0)
const menuTargetTag = ref<NWTagView | null>(null)

/** 当前激活页签路径 */
const activePath = computed(() => route.path)

/** 点击页签跳转 */
function handleTagClick(tagView: NWTagView): void {
  if (tagView.page_full_path === route.fullPath) {
    return
  }
  void router.push(tagView.page_full_path)
}

/** 关闭页签：若关闭的是当前页则跳转到相邻页签 */
function handleTagClose(tagView: NWTagView): void {
  const nextPath = tagsViewStore.removeTag(tagView.page_path)
  if (tagView.page_path !== activePath.value) {
    return
  }
  if (nextPath) {
    void router.push(nextPath)
    return
  }
  void router.push('/')
}

/** 打开右键菜单 */
function handleContextMenu(event: MouseEvent, tagView: NWTagView): void {
  event.preventDefault()
  menuTargetTag.value = tagView
  menuLeftPx.value = event.clientX
  menuTopPx.value = event.clientY
  menuVisible.value = true
}

/** 关闭右键菜单 */
function closeContextMenu(): void {
  menuVisible.value = false
}

/** 右键菜单动作 */
function handleMenuCommand(commandCode: string): void {
  const targetTag = menuTargetTag.value
  closeContextMenu()
  if (!targetTag) {
    return
  }
  if (commandCode === 'refresh') {
    void router.replace({ path: '/redirect', query: { target_path: targetTag.page_full_path } })
    return
  }
  if (commandCode === 'close_other') {
    tagsViewStore.removeOtherTags(targetTag.page_path)
    void router.push(targetTag.page_full_path)
    return
  }
  if (commandCode === 'close_all') {
    tagsViewStore.removeAllTags()
    void router.push('/')
  }
}

onMounted(() => {
  document.addEventListener('click', closeContextMenu)
})

onUnmounted(() => {
  document.removeEventListener('click', closeContextMenu)
})
</script>

<template>
  <div class="tags-view">
    <div class="tags-view__scroll">
      <button
        v-for="tagItem in tagsViewStore.tag_list"
        :key="tagItem.page_path"
        type="button"
        class="tags-view__tag"
        :class="{ 'is-active': tagItem.page_path === activePath }"
        @click="handleTagClick(tagItem)"
        @contextmenu="handleContextMenu($event, tagItem)"
      >
        <span class="tags-view__dot" />
        <span class="tags-view__label">{{ tagItem.page_title }}</span>
        <span
          v-if="!tagItem.affix_flag"
          class="tags-view__close"
          role="button"
          :aria-label="t('layout.closeTag')"
          @click.stop="handleTagClose(tagItem)"
        >
          <NWIcon icon_code="close" :size_px="11" />
        </span>
      </button>
    </div>

    <div
      v-if="menuVisible"
      class="tags-view__menu"
      :style="{ left: `${menuLeftPx}px`, top: `${menuTopPx}px` }"
      @click.stop
    >
      <button type="button" class="tags-view__menu-item" @click="handleMenuCommand('refresh')">
        <NWIcon icon_code="refresh" :size_px="14" />
        <span>{{ t('layout.refreshPage') }}</span>
      </button>
      <button type="button" class="tags-view__menu-item" @click="handleMenuCommand('close_other')">
        <NWIcon icon_code="close" :size_px="14" />
        <span>{{ t('layout.closeOtherTags') }}</span>
      </button>
      <button type="button" class="tags-view__menu-item" @click="handleMenuCommand('close_all')">
        <NWIcon icon_code="trash" :size_px="14" />
        <span>{{ t('layout.closeAllTags') }}</span>
      </button>
    </div>
  </div>
</template>

<style scoped src="./TagsView.css"></style>

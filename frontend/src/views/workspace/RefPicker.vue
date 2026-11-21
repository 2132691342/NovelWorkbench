<!--
  @ 引用选择器：五个页签——知识库文档 / 本项目章节 / 章节摘要 / 提示词档案 / 本会话历史消息。
  支持关键词过滤、多选（再点取消）、键盘 Esc 关闭；选中项以 chips 形式回显在输入区。
-->
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import NWIcon from '@/components/NWIcon.vue'
import type { NWChatRefItem, NWChatRefTabCode } from './chatTypes'

const props = defineProps<{
  tabCode: NWChatRefTabCode
  filterText: string
  candidateList: NWChatRefItem[]
  selectedUlidList: string[]
  emptyHintText: string
}>()

const emit = defineEmits<{
  'update:tabCode': [tabCode: NWChatRefTabCode]
  'update:filterText': [filterText: string]
  select: [item: NWChatRefItem]
  close: []
}>()

const { t } = useI18n()

/** 页签定义（顺序即展示顺序） */
const tabList = computed<{ tab_code: NWChatRefTabCode; tab_name: string; tab_icon: string }[]>(() => [
  { tab_code: 'doc', tab_name: t('workspace.ai.composer.atDocs'), tab_icon: 'database' },
  { tab_code: 'chapter', tab_name: t('workspace.ai.composer.atChapters'), tab_icon: 'book' },
  { tab_code: 'summary', tab_name: t('workspace.ai.composer.atSummaries'), tab_icon: 'fileText' },
  { tab_code: 'prompt', tab_name: t('workspace.ai.composer.atPrompts'), tab_icon: 'sparkles' },
  { tab_code: 'message', tab_name: t('workspace.ai.composer.atMessages'), tab_icon: 'message' },
])

/** 过滤后的候选（大小写不敏感，名称与说明都参与匹配） */
const filteredList = computed(() => {
  const keywordText = props.filterText.trim().toLowerCase()
  if (!keywordText) {
    return props.candidateList
  }
  return props.candidateList.filter(
    (item) =>
      item.ref_name.toLowerCase().includes(keywordText) ||
      item.ref_desc.toLowerCase().includes(keywordText),
  )
})

/** 是否已选中 */
function isSelected(refUlid: string): boolean {
  return props.selectedUlidList.includes(refUlid)
}
</script>

<template>
  <div class="chat-ref-picker">
    <div class="chat-ref-picker__tabs">
      <button
        v-for="tabItem in tabList"
        :key="tabItem.tab_code"
        type="button"
        class="chat-ref-picker__tab"
        :class="{ 'is-active': tabCode === tabItem.tab_code }"
        @click="emit('update:tabCode', tabItem.tab_code)"
      >
        <NWIcon :icon_code="tabItem.tab_icon" :size_px="12" />
        {{ tabItem.tab_name }}
      </button>
      <button type="button" class="chat-ref-picker__close" @click="emit('close')">
        <NWIcon icon_code="close" :size_px="12" />
      </button>
    </div>

    <input
      :value="filterText"
      class="chat-ref-picker__filter"
      :placeholder="t('workspace.ai.composer.atFilterPlaceholder')"
      @input="emit('update:filterText', ($event.target as HTMLInputElement).value)"
    />

    <ul class="chat-ref-picker__list">
      <li v-if="filteredList.length === 0" class="chat-ref-picker__empty">
        {{ emptyHintText }}
      </li>
      <li
        v-for="refItem in filteredList"
        :key="refItem.ref_ulid"
        class="chat-ref-picker__item"
        :class="{ 'is-selected': isSelected(refItem.ref_ulid) }"
        @click="emit('select', refItem)"
      >
        <span class="chat-ref-picker__check">
          <NWIcon v-if="isSelected(refItem.ref_ulid)" icon_code="check" :size_px="11" />
        </span>
        <span class="chat-ref-picker__name" :title="refItem.ref_name">{{ refItem.ref_name }}</span>
        <span v-if="refItem.ref_desc" class="chat-ref-picker__desc">{{ refItem.ref_desc }}</span>
      </li>
    </ul>
  </div>
</template>

<style scoped src="./RefPicker.css"></style>

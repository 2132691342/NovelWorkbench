<!--
  技能斜杠菜单：按分类分组展示，键盘上下选择 + Enter/Tab 确认 + Esc 关闭。
  列表来自 /skill/list（项目技能覆盖同编码全局技能，后端已标记 overridden_flag）。
-->
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { NWSkillVo } from '@/api/contracts'

const props = defineProps<{
  optionList: NWSkillVo[]
  activeIndex: number
}>()

const emit = defineEmits<{
  select: [option: NWSkillVo]
  hover: [index: number]
}>()

const { t } = useI18n()

/** 分类展示顺序（对齐后端 SkillCategory* 常量） */
const CATEGORY_ORDER = ['write', 'revise', 'outline', 'setting', 'review', 'other'] as const

/** 分类名（未知分类归入「其他」） */
function categoryLabel(categoryCode: string): string {
  switch (categoryCode) {
    case 'write':
      return t('skills.categoryWrite')
    case 'revise':
      return t('skills.categoryRevise')
    case 'outline':
      return t('skills.categoryOutline')
    case 'setting':
      return t('skills.categorySetting')
    case 'review':
      return t('skills.categoryReview')
    default:
      return t('skills.categoryOther')
  }
}

/**
 * 分组后的选项：保持 props 顺序，同时记录每项在原始列表中的下标（键盘选择依赖该下标）。
 * 分组只影响展示，不改变 activeIndex 的语义。
 */
const groupList = computed(() => {
  const indexByCode = new Map<string, number>()
  props.optionList.forEach((option, index) => indexByCode.set(option.skill_ulid, index))
  const groups = CATEGORY_ORDER.map((categoryCode) => ({
    category_code: categoryCode,
    category_name: categoryLabel(categoryCode),
    item_list: props.optionList
      .map((option, index) => ({ option, index }))
      .filter((entry) => (CATEGORY_ORDER as readonly string[]).includes(entry.option.category_code)
        ? entry.option.category_code === categoryCode
        : categoryCode === 'other'),
  })).filter((group) => group.item_list.length > 0)
  return groups
})
</script>

<template>
  <div class="chat-slash-menu">
    <div v-if="optionList.length === 0" class="chat-slash-menu__empty">
      {{ t('workspace.ai.composer.slashEmpty') }}
    </div>
    <template v-else>
      <div v-for="group in groupList" :key="group.category_code" class="chat-slash-menu__group">
        <div class="chat-slash-menu__group-title">{{ group.category_name }}</div>
        <button
          v-for="entry in group.item_list"
          :key="entry.option.skill_ulid"
          type="button"
          class="chat-slash-option"
          :class="{ 'is-active': entry.index === activeIndex }"
          @mouseenter="emit('hover', entry.index)"
          @click="emit('select', entry.option)"
        >
          <span class="chat-slash-option__code">/{{ entry.option.skill_code }}</span>
          <span class="chat-slash-option__name">{{ entry.option.skill_name }}</span>
          <span class="chat-slash-option__desc">{{ entry.option.description_text }}</span>
          <span v-if="entry.option.scope_code === 'project'" class="chat-slash-option__badge">
            {{ t('workspace.ai.composer.projectSkill') }}
          </span>
        </button>
      </div>
      <div class="chat-slash-menu__hint">
        {{ t('workspace.ai.composer.slashHint') }}
      </div>
    </template>
  </div>
</template>

<style scoped src="./SlashMenu.css"></style>

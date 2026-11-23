<!-- 素材库：角色、设定、笔记、文档四个入口合到一处，写作时只认这一个页面。
     页签做成胶囊分段控件，与子页面内部的表格页签区分层级，避免两层下划线页签叠在一起。 -->
<script setup lang="ts">
  import { computed, defineAsyncComponent, ref, watch } from 'vue'
  import { ElEmpty } from 'element-plus'
  import { useRoute } from 'vue-router'
  import { useProjectStore } from '@/store/modules/project'

  const projectStore = useProjectStore()
  const route = useRoute()
  const projectUlid = computed(() => projectStore.currentProjectUlid)

  /** 四个子页面按需加载：首次点开才拉数据，之后保留状态 */
  const CharacterPage = defineAsyncComponent(() => import('@/views/Character/index.vue'))
  const LorePage = defineAsyncComponent(() => import('@/views/Lore/index.vue'))
  const NotesPage = defineAsyncComponent(() => import('@/views/Notes/index.vue'))
  const KnowledgePage = defineAsyncComponent(() => import('@/views/Knowledge/index.vue'))

  const activeTab = ref('character')

  /** 入口元信息：图标 + 一句话说明，让用户一眼知道去哪找什么 */
  const tabList = [
    { name: 'character', label: '角色', icon: 'mdi:account-group', hint: '人物卡与关系图' },
    { name: 'lore', label: '设定', icon: 'mdi:earth', hint: '世界观、地点、物品' },
    { name: 'notes', label: '笔记', icon: 'mdi:notebook-outline', hint: '随手记录与灵感' },
    { name: 'knowledge', label: '文档', icon: 'mdi:database-outline', hint: '资料上传与检索' }
  ]

  const visitedTabSet = ref<string[]>(['character'])
  const selectTab = (tabName: string) => {
    activeTab.value = tabName
    if (!visitedTabSet.value.includes(tabName)) visitedTabSet.value.push(tabName)
  }

  /** 深链 ?tab=notes 直达对应页签（写作台与旧页面跳转都用它） */
  watch(
    () => route.query.tab,
    (tabName) => {
      const target = String(tabName ?? '')
      if (target && tabList.some((tab) => tab.name === target)) selectTab(target)
    },
    { immediate: true }
  )
</script>

<template>
  <div class="material">
    <ElEmpty
      v-if="!projectUlid"
      description="请先在顶栏选择项目，再整理素材"
      :image-size="90"
    />

    <template v-else>
      <nav class="material__switch" aria-label="素材分类">
        <button
          v-for="tab in tabList"
          :key="tab.name"
          type="button"
          :class="['material__pill', { 'is-active': activeTab === tab.name }]"
          @click="selectTab(tab.name)"
        >
          <Icon :icon="tab.icon" :size="14" />
          <span class="material__pill-label">{{ tab.label }}</span>
          <span class="material__pill-hint">{{ tab.hint }}</span>
        </button>
      </nav>

      <div class="material__body">
        <CharacterPage
          v-if="visitedTabSet.includes('character')"
          v-show="activeTab === 'character'"
        />
        <LorePage v-if="visitedTabSet.includes('lore')" v-show="activeTab === 'lore'" />
        <NotesPage v-if="visitedTabSet.includes('notes')" v-show="activeTab === 'notes'" />
        <KnowledgePage
          v-if="visitedTabSet.includes('knowledge')"
          v-show="activeTab === 'knowledge'"
        />
      </div>
    </template>
  </div>
</template>

<style lang="less" scoped>
  .material {
    display: flex;
    flex-direction: column;
    gap: 12px;
    width: 100%;

    /* 分段控件占位：子页面按 100vh 算自身高度时要扣掉这一层 */
    --material-tabs-offset: 46px;

    &__switch {
      display: flex;
      flex-wrap: wrap;
      flex: none;
      gap: 6px;
      padding: 4px;
      background: var(--nw-surface-subtle);
      border: 1px solid var(--nw-border-soft);
      border-radius: var(--nw-radius-lg);
    }

    &__pill {
      display: inline-flex;
      gap: 6px;
      align-items: center;
      padding: 6px 14px;
      font-size: 13px;
      color: var(--el-text-color-regular);
      cursor: pointer;
      background: transparent;
      border: 0;
      border-radius: var(--nw-radius-base);
      transition:
        color 0.16s ease,
        background 0.16s ease,
        transform 0.16s ease;

      &:hover {
        color: var(--el-color-primary);
        background: var(--nw-primary-soft);
        transform: scale(1.02);
      }

      &:active {
        transform: scale(0.98);
      }

      &.is-active {
        color: var(--el-color-primary);
        background: var(--nw-surface);
        box-shadow: 0 1px 3px rgb(15 23 42 / 10%);
      }
    }

    &__pill-label {
      font-weight: 600;
    }

    &__pill-hint {
      font-size: 11px;
      color: var(--el-text-color-placeholder);
    }

    &__body {
      flex: 1;
      min-width: 0;
    }
  }
</style>

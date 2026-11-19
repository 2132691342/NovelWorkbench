<!--
  项目上下文头：项目身份 + 关键指标 + 项目内导航。
  项目内所有页面复用，保证“我在哪本书、能去哪”始终清晰。
-->
<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { getProjectDetail } from '@/api/project'
import { useProjectStore } from '@/stores/project'
import type { NWProjectVo } from '@/api/contracts'
import NWIcon from '@/components/NWIcon.vue'

const props = defineProps<{
  projectUlid: string
}>()

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const projectStore = useProjectStore()

const projectDetail = ref<NWProjectVo | null>(null)
const loading = ref(false)

const projectName = computed(
  () =>
    projectDetail.value?.project_name ||
    projectStore.current_project_vo?.project_name ||
    '',
)

const projectStatusText = computed(() => {
  const statusCode = projectDetail.value?.project_status ?? projectStore.current_project_vo?.project_status
  switch (statusCode) {
    case 'serial':
      return t('dashboard.statusSerial')
    case 'completed':
      return t('dashboard.statusCompleted')
    default:
      return t('dashboard.statusDraft')
  }
})

const wordCount = computed(
  () => projectDetail.value?.total_word_count ?? projectStore.current_project_vo?.total_word_count ?? 0,
)

const chapterCount = computed(
  () => projectDetail.value?.chapter_count ?? projectStore.current_project_vo?.chapter_count ?? 0,
)

const coverLetter = computed(() => projectName.value.slice(0, 1) || 'N')

/** 项目内导航（route path 不包含 project_ulid，跳转时补齐） */
const projectNavList = computed(() => [
  { nav_code: 'workspace', nav_label: t('projectPage.navWorkspace'), icon_code: 'edit', path: '/workspace' },
  { nav_code: 'outline', nav_label: t('projectPage.navOutline'), icon_code: 'tree', path: '/outline' },
  { nav_code: 'continuity', nav_label: t('projectPage.navContinuity'), icon_code: 'shield', path: '/continuity' },
  { nav_code: 'character', nav_label: t('projectPage.navCharacter'), icon_code: 'user', path: '/character' },
  { nav_code: 'lore', nav_label: t('projectPage.navLore'), icon_code: 'globe', path: '/lore' },
  { nav_code: 'knowledge', nav_label: t('projectPage.navKnowledge'), icon_code: 'database', path: '/knowledge' },
  { nav_code: 'notes', nav_label: t('projectPage.navNotes'), icon_code: 'note', path: '/notes' },
])

const activeNavCode = computed(() => {
  const currentPath = route.path
  const matched = projectNavList.value.find((navItem) => currentPath.startsWith(navItem.path))
  return matched?.nav_code ?? ''
})

/** 加载项目详情（列表已有数据时先用缓存，再静默刷新） */
async function loadProjectDetail(): Promise<void> {
  if (!props.projectUlid) {
    projectDetail.value = null
    return
  }
  if (projectStore.hasProject(props.projectUlid)) {
    projectDetail.value = projectStore.project_list.find(
      (projectItem) => projectItem.project_ulid === props.projectUlid,
    ) ?? null
  }
  loading.value = true
  try {
    projectDetail.value = await getProjectDetail(props.projectUlid)
  } catch {
    // 失败提示由 http 层统一弹出；保留缓存数据
  } finally {
    loading.value = false
  }
}

function handleBack(): void {
  void router.push(`/workspace/${props.projectUlid}`)
}

function handleNav(navPath: string): void {
  void router.push(`${navPath}/${props.projectUlid}`)
}

watch(
  () => props.projectUlid,
  () => {
    void loadProjectDetail()
  },
)

onMounted(() => {
  void loadProjectDetail()
})

defineExpose({ loading })
</script>

<template>
  <header class="project-context">
    <div class="project-context__top">
      <button type="button" class="project-context__back" @click="handleBack">
        <NWIcon icon_code="arrowLeft" :size_px="15" />
        <span>{{ t('projectPage.backToWorkspace') }}</span>
      </button>

      <span class="project-context__cover">{{ coverLetter }}</span>
      <div class="project-context__identity">
        <div class="project-context__name-row">
          <h2 class="project-context__name" :title="projectName">{{ projectName }}</h2>
          <span class="nw-badge nw-badge--primary">{{ projectStatusText }}</span>
        </div>
        <div class="project-context__meta">
          <span>{{ t('projectPage.wordCount', { count: wordCount.toLocaleString() }) }}</span>
          <span class="project-context__dot" />
          <span>{{ t('projectPage.chapterCount', { count: chapterCount }) }}</span>
        </div>
      </div>

      <div v-if="$slots.actions" class="project-context__actions">
        <slot name="actions" />
      </div>
    </div>

    <nav class="project-context__nav">
      <button
        v-for="navItem in projectNavList"
        :key="navItem.nav_code"
        type="button"
        class="project-context__nav-item"
        :class="{ 'is-active': activeNavCode === navItem.nav_code }"
        @click="handleNav(navItem.path)"
      >
        <NWIcon :icon_code="navItem.icon_code" :size_px="15" />
        <span>{{ navItem.nav_label }}</span>
      </button>
    </nav>
  </header>
</template>

<style scoped>
.project-context {
  display: flex;
  flex-direction: column;
  border: 1px solid var(--border-light);
  border-radius: var(--radius-lg);
  background: var(--bg-card);
  box-shadow: var(--shadow-card);
  overflow: hidden;
}

.project-context__top {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 18px 12px;
}

.project-context__back {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 5px 8px;
  border: 1px solid var(--border-light);
  border-radius: var(--radius-base);
  background: var(--bg-subtle);
  color: var(--text-secondary);
  font-size: 12px;
  cursor: pointer;
  transition: all var(--transition-base);
}

.project-context__back:hover {
  border-color: var(--primary-hover);
  background: var(--primary-soft);
  color: var(--primary);
  transform: translateX(-2px);
}

.project-context__cover {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 42px;
  flex-shrink: 0;
  border-radius: 9px;
  background: linear-gradient(160deg, var(--primary), var(--primary-hover));
  color: var(--text-inverse);
  font-size: 17px;
  font-weight: 700;
  box-shadow: 0 4px 12px rgba(99, 102, 241, 0.26);
}

.project-context__identity {
  min-width: 0;
  flex: 1;
}

.project-context__name-row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.project-context__name {
  margin: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text-strong);
  font-size: 17px;
  font-weight: 700;
  letter-spacing: -0.01em;
}

.project-context__meta {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 3px;
  color: var(--text-placeholder);
  font-size: 11.5px;
}

.project-context__dot {
  width: 3px;
  height: 3px;
  border-radius: 50%;
  background: var(--border-strong);
}

.project-context__actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.project-context__nav {
  display: flex;
  align-items: center;
  gap: 2px;
  padding: 0 12px;
  overflow-x: auto;
  border-top: 1px solid var(--border-light);
  background: linear-gradient(180deg, var(--bg-card), #fdfdff);
}

.project-context__nav-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 40px;
  padding: 0 12px;
  border: none;
  border-bottom: 2px solid transparent;
  background: transparent;
  color: var(--text-secondary);
  font-size: 12.8px;
  font-weight: 500;
  white-space: nowrap;
  cursor: pointer;
  transition: color var(--transition-base), border-color var(--transition-base),
    background-color var(--transition-base);
}

.project-context__nav-item:hover {
  color: var(--primary);
  background: var(--primary-soft);
}

.project-context__nav-item.is-active {
  color: var(--primary);
  border-bottom-color: var(--primary);
  font-weight: 600;
}

@media (max-width: 900px) {
  .project-context__top {
    flex-wrap: wrap;
  }

  .project-context__actions {
    width: 100%;
    margin-left: 0;
  }
}
</style>

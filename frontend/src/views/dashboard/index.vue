<!--
  仪表盘：创作概览 + 趋势 + 今日目标 + 项目卡片。
  项目卡片是主入口，点击进入写作台；新建/编辑复用独立抽屉组件。
-->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { get } from '@/api/http'
import { deleteProject, listProjects } from '@/api/project'
import type { NWProjectVo, NWSettingItemVo, NWStatsOverviewVo } from '@/api/contracts'
import NWButton from '@/components/NWButton.vue'
import NWEmpty from '@/components/NWEmpty.vue'
import NWIcon from '@/components/NWIcon.vue'
import NWPageHeader from '@/components/NWPageHeader.vue'
import NWStatTile from '@/components/NWStatTile.vue'
import { useAuthStore } from '@/stores/auth'
import { useProjectStore } from '@/stores/project'
import DashboardProjectCard from './DashboardProjectCard.vue'
import DashboardProjectDrawer from './DashboardProjectDrawer.vue'
import DashboardTrendCard from './DashboardTrendCard.vue'

const { t, locale } = useI18n()
const router = useRouter()
const authStore = useAuthStore()
const projectStore = useProjectStore()

const NW_PROJECT_PAGE_SIZE = 12

const statsLoading = ref(false)
const projectLoading = ref(false)
const projectList = ref<NWProjectVo[]>([])
const projectTotal = ref(0)
const projectPageNum = ref(1)
const statusFilter = ref('')
const keywordText = ref('')
const dailyGoal = ref(2000)
const drawerVisible = ref(false)
const editingProject = ref<NWProjectVo | null>(null)

const statsOverview = ref<NWStatsOverviewVo>({
  project_count: 0,
  total_word_count: 0,
  today_word_delta: 0,
  ai_word_total: 0,
  trend_list: [],
})

const projectStatusOptions = computed(() => [
  { value: '', label: t('dashboard.statusAll') },
  { value: 'draft', label: t('dashboard.statusDraft') },
  { value: 'serial', label: t('dashboard.statusSerial') },
  { value: 'completed', label: t('dashboard.statusCompleted') },
])

const greetText = computed(() => {
  const hour = new Date().getHours()
  if (hour < 6) return t('dashboard.greetNight')
  if (hour < 12) return t('dashboard.greetMorning')
  if (hour < 18) return t('dashboard.greetAfternoon')
  return t('dashboard.greetEvening')
})

const todayDateText = computed(() =>
  new Date().toLocaleDateString(locale.value, { month: 'long', day: 'numeric', weekday: 'long' }),
)

const goalPercent = computed(() => {
  if (dailyGoal.value <= 0) return 0
  return Math.min(100, Math.round((statsOverview.value.today_word_delta / dailyGoal.value) * 100))
})

const continueProject = computed(() => {
  const currentUlid = projectStore.current_project_ulid
  return (
    projectList.value.find((item) => item.project_ulid === currentUlid) ??
    projectList.value[0] ??
    null
  )
})

function genreLabel(genreCode: string): string {
  const genreKeyMap: Record<string, string> = {
    fantasy: t('dashboard.genreFantasy'),
    scifi: t('dashboard.genreScifi'),
    romance: t('dashboard.genreRomance'),
    suspense: t('dashboard.genreSuspense'),
    wuxia: t('dashboard.genreWuxia'),
    urban: t('dashboard.genreUrban'),
    history: t('dashboard.genreHistory'),
  }
  return genreKeyMap[genreCode] ?? (genreCode || t('dashboard.genreUnset'))
}

function statusLabel(statusCode: string): string {
  return projectStatusOptions.value.find((option) => option.value === statusCode)?.label ?? statusCode
}

function statusBadgeClass(statusCode: string): string {
  if (statusCode === 'serial') return 'nw-badge--success'
  if (statusCode === 'completed') return 'nw-badge--info'
  return 'nw-badge--warning'
}

/** 封面色（按书名稳定散列，刷新不跳变） */
function coverGradient(projectName: string): string {
  const paletteList = [
    ['#6366f1', '#818cf8'],
    ['#0f172a', '#475569'],
    ['#0ea5e9', '#38bdf8'],
    ['#059669', '#34d399'],
    ['#d97706', '#fbbf24'],
    ['#7c3aed', '#a78bfa'],
  ]
  let hashValue = 0
  for (const charText of projectName) {
    hashValue = (hashValue * 31 + (charText.codePointAt(0) ?? 0)) % 997
  }
  const [fromColor, toColor] = paletteList[hashValue % paletteList.length]
  return `linear-gradient(150deg, ${fromColor}, ${toColor})`
}

function formatRelativeTime(timeMs: number): string {
  const diffMs = Date.now() - timeMs
  const minuteMs = 60 * 1000
  const hourMs = 60 * minuteMs
  const dayMs = 24 * hourMs
  if (diffMs < minuteMs) return t('dashboard.timeJustNow')
  if (diffMs < hourMs) return t('dashboard.timeMinutesAgo', { count: Math.floor(diffMs / minuteMs) })
  if (diffMs < dayMs) return t('dashboard.timeHoursAgo', { count: Math.floor(diffMs / hourMs) })
  if (diffMs < 7 * dayMs) return t('dashboard.timeDaysAgo', { count: Math.floor(diffMs / dayMs) })
  return new Date(timeMs).toLocaleDateString()
}

async function loadStatsOverview(): Promise<void> {
  statsLoading.value = true
  try {
    const statsData = await get<NWStatsOverviewVo>('/stats/overview')
    statsOverview.value = {
      project_count: statsData.project_count ?? 0,
      total_word_count: statsData.total_word_count ?? 0,
      today_word_delta: statsData.today_word_delta ?? 0,
      ai_word_total: statsData.ai_word_total ?? 0,
      trend_list: Array.isArray(statsData.trend_list) ? statsData.trend_list : [],
    }
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    statsLoading.value = false
  }
}

async function loadDailyGoal(): Promise<void> {
  try {
    const settingList = await get<NWSettingItemVo[]>('/setting/list')
    const goalValue = Number(
      settingList.find((item) => item.setting_key === 'daily_word_goal')?.setting_value ?? '',
    )
    dailyGoal.value = Number.isFinite(goalValue) && goalValue > 0 ? goalValue : 2000
  } catch {
    dailyGoal.value = 2000
  }
}

async function loadProjectList(): Promise<void> {
  projectLoading.value = true
  try {
    const pageData = await listProjects({
      page_num: projectPageNum.value,
      page_size: NW_PROJECT_PAGE_SIZE,
      keyword_text: keywordText.value.trim() || undefined,
      project_status: statusFilter.value || undefined,
    })
    projectList.value = pageData.items ?? []
    projectTotal.value = pageData.total ?? 0
  } catch {
    projectList.value = []
    projectTotal.value = 0
  } finally {
    projectLoading.value = false
  }
}

function handleSearch(): void {
  projectPageNum.value = 1
  void loadProjectList()
}

function handleStatusChange(statusCode: string): void {
  statusFilter.value = statusCode
  handleSearch()
}

function openProjectDrawer(projectVo?: NWProjectVo): void {
  editingProject.value = projectVo ?? null
  drawerVisible.value = true
}

async function handleProjectSaved(): Promise<void> {
  await Promise.all([loadProjectList(), loadStatsOverview(), projectStore.loadProjectList()])
}

async function handleDeleteProject(projectVo: NWProjectVo): Promise<void> {
  try {
    await ElMessageBox.confirm(
      t('dashboard.deleteProjectConfirm'),
      t('dashboard.deleteProject'),
      { type: 'warning', confirmButtonText: t('common.confirm'), cancelButtonText: t('common.cancel') },
    )
  } catch {
    return
  }
  try {
    await deleteProject(projectVo.project_ulid)
    ElMessage.success(t('dashboard.deleteSuccess'))
    await Promise.all([loadProjectList(), loadStatsOverview(), projectStore.loadProjectList()])
  } catch {
    // 失败提示由 http 层统一弹出
  }
}

function openProject(projectVo: NWProjectVo): void {
  projectStore.setCurrentProject(projectVo.project_ulid)
  void router.push(`/workspace/${projectVo.project_ulid}`)
}

function handlePageChange(pageNum: number): void {
  projectPageNum.value = pageNum
  void loadProjectList()
}

onMounted(() => {
  void loadStatsOverview()
  void loadDailyGoal()
  void loadProjectList()
})
</script>

<template>
  <div class="nw-page dashboard-page">
    <NWPageHeader
      :eyebrow_text="todayDateText"
      :title_text="t('dashboard.greetTitle', { name: authStore.user_vo?.user_name || t('dashboard.greetWriter'), greet: greetText })"
      :subtitle_text="t('dashboard.greetSubtitle')"
      icon_code="sparkles"
    >
      <template #actions>
        <NWButton @click="openProjectDrawer()">
          <NWIcon icon_code="plus" :size_px="15" />
          {{ t('dashboard.createProject') }}
        </NWButton>
        <NWButton v-if="continueProject" type="primary" @click="openProject(continueProject)">
          <NWIcon icon_code="play" :size_px="14" />
          {{ t('dashboard.continueWriting') }}
        </NWButton>
      </template>
    </NWPageHeader>

    <div class="nw-stat-grid">
      <NWStatTile :label_text="t('dashboard.projectCount')" :value_number="statsOverview.project_count" :hint_text="t('dashboard.projectCountHint')" icon_code="bookOpen" tone_code="primary" />
      <NWStatTile :label_text="t('dashboard.totalWords')" :value_number="statsOverview.total_word_count" :hint_text="t('dashboard.totalWordsHint')" icon_code="pen" tone_code="neutral" />
      <NWStatTile :label_text="t('dashboard.todayWords')" :value_number="statsOverview.today_word_delta" :hint_text="t('dashboard.todayWordsHint')" icon_code="flame" tone_code="warning" />
      <NWStatTile :label_text="t('dashboard.aiWords')" :value_number="statsOverview.ai_word_total" :hint_text="t('dashboard.aiWordsHint')" icon_code="sparkles" tone_code="info" />
    </div>

    <div class="dashboard-page__main-grid">
      <DashboardTrendCard :overview="statsOverview" :loading="statsLoading" />

      <section class="nw-panel dashboard-page__goal-panel">
        <header class="nw-panel__header">
          <div>
            <h3 class="nw-panel__title">{{ t('dashboard.todayGoal') }}</h3>
            <p class="nw-panel__subtitle">{{ t('dashboard.todayGoalSubtitle') }}</p>
          </div>
        </header>
        <div class="nw-panel__body">
          <div class="dashboard-page__goal-value">
            <strong>{{ statsOverview.today_word_delta }}</strong>
            <span>/ {{ dailyGoal.toLocaleString() }} {{ t('dashboard.goalUnit') }}</span>
          </div>
          <el-progress :percentage="goalPercent" :stroke-width="9" :show-text="false" class="dashboard-page__goal-progress" />
          <p class="nw-hint">
            {{ goalPercent >= 100 ? t('dashboard.goalDone') : t('dashboard.goalRemain', { count: Math.max(0, dailyGoal - statsOverview.today_word_delta) }) }}
          </p>

          <div class="dashboard-page__quick-list">
            <button v-if="continueProject" type="button" class="dashboard-page__quick-item" @click="openProject(continueProject)">
              <span class="dashboard-page__quick-icon"><NWIcon icon_code="edit" :size_px="16" /></span>
              <span class="dashboard-page__quick-text">
                <strong>{{ t('dashboard.quickContinue') }}</strong>
                <small>{{ continueProject.project_name }}</small>
              </span>
              <NWIcon icon_code="chevronRight" :size_px="14" />
            </button>
            <button type="button" class="dashboard-page__quick-item" @click="openProjectDrawer()">
              <span class="dashboard-page__quick-icon"><NWIcon icon_code="plus" :size_px="16" /></span>
              <span class="dashboard-page__quick-text">
                <strong>{{ t('dashboard.quickNewProject') }}</strong>
                <small>{{ t('dashboard.quickNewProjectHint') }}</small>
              </span>
              <NWIcon icon_code="chevronRight" :size_px="14" />
            </button>
            <button v-if="continueProject" type="button" class="dashboard-page__quick-item" @click="router.push(`/knowledge/${continueProject.project_ulid}`)">
              <span class="dashboard-page__quick-icon"><NWIcon icon_code="database" :size_px="16" /></span>
              <span class="dashboard-page__quick-text">
                <strong>{{ t('dashboard.quickKnowledge') }}</strong>
                <small>{{ t('dashboard.quickKnowledgeHint') }}</small>
              </span>
              <NWIcon icon_code="chevronRight" :size_px="14" />
            </button>
          </div>
        </div>
      </section>
    </div>

    <section class="nw-panel dashboard-page__project-panel">
      <header class="nw-panel__header">
        <div>
          <h3 class="nw-panel__title">{{ t('dashboard.myProjects') }}</h3>
          <p class="nw-panel__subtitle">{{ t('dashboard.myProjectsSubtitle', { count: projectTotal }) }}</p>
        </div>
        <div class="dashboard-page__project-actions">
          <el-input v-model="keywordText" class="dashboard-page__search" clearable :placeholder="t('dashboard.searchProject')" @keyup.enter="handleSearch" @clear="handleSearch">
            <template #prefix><NWIcon icon_code="search" :size_px="14" /></template>
          </el-input>
          <div class="dashboard-page__status-tabs">
            <button
              v-for="statusOption in projectStatusOptions"
              :key="statusOption.value"
              type="button"
              class="dashboard-page__status-tab"
              :class="{ 'is-active': statusFilter === statusOption.value }"
              @click="handleStatusChange(statusOption.value)"
            >
              {{ statusOption.label }}
            </button>
          </div>
          <NWButton size="small" type="primary" @click="openProjectDrawer()">
            <NWIcon icon_code="plus" :size_px="14" />
            {{ t('dashboard.createProject') }}
          </NWButton>
        </div>
      </header>

      <div class="nw-panel__body">
        <div v-if="projectLoading" class="dashboard-page__project-skeleton">
          <div v-for="index in 3" :key="index" class="dashboard-page__project-skeleton-item" />
        </div>
        <NWEmpty
          v-else-if="projectList.length === 0"
          icon_code="bookOpen"
          :title_text="t('dashboard.projectEmptyTitle')"
          :description_text="t('dashboard.projectEmptyDesc')"
        >
          <template #actions>
            <NWButton type="primary" @click="openProjectDrawer()">
              <NWIcon icon_code="plus" :size_px="14" />
              {{ t('dashboard.createProject') }}
            </NWButton>
          </template>
        </NWEmpty>
        <template v-else>
          <div class="dashboard-page__project-grid">
            <DashboardProjectCard
              v-for="projectVo in projectList"
              :key="projectVo.project_ulid"
              :project="projectVo"
              :genre_text="genreLabel(projectVo.genre_code)"
              :status_text="statusLabel(projectVo.project_status)"
              :status_class="statusBadgeClass(projectVo.project_status)"
              :cover_style="coverGradient(projectVo.project_name)"
              :relative_text="formatRelativeTime(projectVo.updated_at_ms)"
              @open="openProject(projectVo)"
              @edit="openProjectDrawer(projectVo)"
              @remove="handleDeleteProject(projectVo)"
            />
          </div>

          <el-pagination
            v-if="projectTotal > NW_PROJECT_PAGE_SIZE"
            class="dashboard-page__pagination"
            layout="total, prev, pager, next"
            :total="projectTotal"
            :page-size="NW_PROJECT_PAGE_SIZE"
            :current-page="projectPageNum"
            background
            @current-change="handlePageChange"
          />
        </template>
      </div>
    </section>

    <DashboardProjectDrawer v-model:visible="drawerVisible" :project="editingProject" @saved="handleProjectSaved" />
  </div>
</template>

<style scoped src="./dashboard.css"></style>

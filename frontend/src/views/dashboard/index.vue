<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import * as echarts from 'echarts'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { get } from '@/api/http'
import { createProject, deleteProject, listProjects, updateProject } from '@/api/project'
import type { NWProjectVo, NWStatsOverviewVo } from '@/api/contracts'
import NWButton from '@/components/NWButton.vue'
import NWCard from '@/components/NWCard.vue'
import NWEmpty from '@/components/NWEmpty.vue'
import NWLoading from '@/components/NWLoading.vue'

const { t } = useI18n()
const router = useRouter()

// 趋势图配色（对齐设计系统品牌色）
const NW_TREND_BAR_COLOR = '#6366f1'
const NW_TREND_LINE_COLOR = '#818cf8'

// 项目封面色块取色板（设计系统五色 + 语义色派生）
const NW_COVER_COLOR_LIST = ['#6366f1', '#818cf8', '#3b82f6', '#10b981', '#f59e0b', '#1e293b']

/** 项目列表分页大小 */
const NW_PROJECT_PAGE_SIZE = 10

const statsLoading = ref(false)
const projectLoading = ref(false)
const projectList = ref<NWProjectVo[]>([])
const projectTotal = ref(0)
const projectPageNum = ref(1)
const createDialogVisible = ref(false)
const createSaving = ref(false)
const editingProjectUlid = ref('')
const createFormRef = ref<FormInstance>()
const statsOverview = ref<NWStatsOverviewVo>({
  project_count: 0,
  total_word_count: 0,
  today_word_delta: 0,
  ai_word_total: 0,
  trend_list: [],
})

/** 新建项目表单状态 */
interface ProjectFormState {
  project_name: string
  genre_code: string
  style_text: string
  synopsis_text: string
  project_status: string
}

const createForm = reactive<ProjectFormState>({
  project_name: '',
  genre_code: '',
  style_text: '',
  synopsis_text: '',
  project_status: 'draft',
})

const projectEditMode = computed(() => editingProjectUlid.value !== '')

const createRules = computed<FormRules>(() => ({
  project_name: [
    { required: true, message: t('dashboard.ruleProjectNameRequired'), trigger: 'blur' },
    { max: 128, message: t('dashboard.ruleProjectNameMax'), trigger: 'blur' },
  ],
}))

const trendChartElement = ref<HTMLDivElement | null>(null)
let trendChartInstance: echarts.ECharts | null = null

const statCardList = computed(() => [
  {
    stat_key: 'project_count',
    stat_label: t('dashboard.projectCount'),
    stat_value: statsOverview.value.project_count,
    stat_class: 'stat-card--indigo',
  },
  {
    stat_key: 'total_word_count',
    stat_label: t('dashboard.totalWords'),
    stat_value: statsOverview.value.total_word_count,
    stat_class: 'stat-card--slate',
  },
  {
    stat_key: 'today_word_delta',
    stat_label: t('dashboard.todayWords'),
    stat_value: statsOverview.value.today_word_delta,
    stat_class: 'stat-card--violet',
  },
  {
    stat_key: 'ai_word_total',
    stat_label: t('dashboard.aiWords'),
    stat_value: statsOverview.value.ai_word_total,
    stat_class: 'stat-card--blue',
  },
])

/** 渲染近 30 天趋势图（双系列：柱状 word_delta + 折线 ai_word_delta） */
function renderTrendChart(): void {
  if (!trendChartElement.value) {
    return
  }
  if (!trendChartInstance) {
    trendChartInstance = echarts.init(trendChartElement.value)
  }
  const trendList = statsOverview.value.trend_list
  trendChartInstance.setOption({
    color: [NW_TREND_BAR_COLOR, NW_TREND_LINE_COLOR],
    tooltip: { trigger: 'axis' },
    legend: {
      data: [t('dashboard.trendWordDelta'), t('dashboard.trendAiWordDelta')],
      top: 0,
    },
    grid: { left: 56, right: 24, top: 44, bottom: 36 },
    xAxis: {
      type: 'category',
      data: trendList.map((trendItem) => trendItem.stat_date),
      boundaryGap: true,
    },
    yAxis: { type: 'value' },
    series: [
      {
        name: t('dashboard.trendWordDelta'),
        type: 'bar',
        data: trendList.map((trendItem) => trendItem.word_delta),
        itemStyle: { color: NW_TREND_BAR_COLOR, borderRadius: [4, 4, 0, 0] },
        barMaxWidth: 18,
      },
      {
        name: t('dashboard.trendAiWordDelta'),
        type: 'line',
        smooth: true,
        data: trendList.map((trendItem) => trendItem.ai_word_delta),
        itemStyle: { color: NW_TREND_LINE_COLOR },
        lineStyle: { width: 2 },
        symbolSize: 6,
      },
    ],
  })
}

function handleWindowResize(): void {
  trendChartInstance?.resize()
}

/** 加载统计概览 */
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
    renderTrendChart()
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    statsLoading.value = false
  }
}

/** 加载项目分页列表（接口未就绪时静默失败，仅展示空态） */
async function loadProjectList(): Promise<void> {
  projectLoading.value = true
  try {
    const pageData = await listProjects({
      page_num: projectPageNum.value,
      page_size: NW_PROJECT_PAGE_SIZE,
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

/** 项目状态转展示文案 */
function projectStatusText(projectStatus: string): string {
  switch (projectStatus) {
    case 'draft':
      return t('dashboard.statusDraft')
    case 'serial':
      return t('dashboard.statusSerial')
    case 'completed':
      return t('dashboard.statusCompleted')
    default:
      return projectStatus
  }
}

/** 封面色块取色（按书名散列稳定取色） */
function coverColor(projectName: string): string {
  let hashValue = 0
  for (const charText of projectName) {
    hashValue = (hashValue * 31 + charText.codePointAt(0)!) % 997
  }
  return NW_COVER_COLOR_LIST[hashValue % NW_COVER_COLOR_LIST.length]
}

/** 毫秒时间戳转本地时间文案 */
function formatTimestamp(timestampMs: number): string {
  return new Date(timestampMs).toLocaleString()
}

/** 行点击进入写作台 */
function handleProjectRowClick(row: NWProjectVo): void {
  void router.push(`/workspace/${row.project_ulid}`)
}

function handleCreateProject(): void {
  editingProjectUlid.value = ''
  createForm.project_name = ''
  createForm.genre_code = ''
  createForm.style_text = ''
  createForm.synopsis_text = ''
  createForm.project_status = 'draft'
  createDialogVisible.value = true
}

/** 编辑项目：复用新建弹窗，回填可编辑字段 */
function handleEditProject(row: NWProjectVo): void {
  editingProjectUlid.value = row.project_ulid
  createForm.project_name = row.project_name
  createForm.genre_code = row.genre_code
  createForm.style_text = row.style_text
  createForm.synopsis_text = row.synopsis_text
  createForm.project_status = row.project_status || 'draft'
  createDialogVisible.value = true
}

/** 保存新项目 */
async function handleCreateSubmit(): Promise<void> {
  if (!createFormRef.value) {
    return
  }
  try {
    await createFormRef.value.validate()
  } catch {
    return
  }
  createSaving.value = true
  try {
    if (editingProjectUlid.value) {
      await updateProject({
        project_ulid: editingProjectUlid.value,
        project_name: createForm.project_name,
        genre_code: createForm.genre_code,
        style_text: createForm.style_text,
        synopsis_text: createForm.synopsis_text,
        project_status: createForm.project_status,
      })
      ElMessage.success(t('dashboard.updateSuccess'))
    } else {
      await createProject({
        project_name: createForm.project_name,
        genre_code: createForm.genre_code || undefined,
        style_text: createForm.style_text || undefined,
        synopsis_text: createForm.synopsis_text || undefined,
      })
      ElMessage.success(t('dashboard.createSuccess'))
    }
    createDialogVisible.value = false
    projectPageNum.value = 1
    await loadProjectList()
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    createSaving.value = false
  }
}

/** 删除项目（二次确认，级联删卷/章/设定） */
async function handleDeleteProject(row: NWProjectVo): Promise<void> {
  try {
    await ElMessageBox.confirm(
      t('dashboard.deleteProjectConfirm'),
      t('dashboard.deleteProject'),
      { type: 'warning' },
    )
  } catch {
    return
  }
  try {
    await deleteProject(row.project_ulid)
    ElMessage.success(t('dashboard.deleteSuccess'))
    await loadProjectList()
  } catch {
    // 失败提示由 http 层统一弹出
  }
}

function handleProjectPageChange(pageNum: number): void {
  projectPageNum.value = pageNum
  void loadProjectList()
}

onMounted(() => {
  void loadStatsOverview()
  void loadProjectList()
  window.addEventListener('resize', handleWindowResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleWindowResize)
  trendChartInstance?.dispose()
  trendChartInstance = null
})
</script>

<template>
  <div class="dashboard-page">
    <h2 class="dashboard-page__title">{{ t('dashboard.title') }}</h2>

    <!-- 统计卡片 -->
    <NWLoading v-if="statsLoading" />
    <div v-else class="dashboard-page__stats">
      <NWCard
        v-for="statItem in statCardList"
        :key="statItem.stat_key"
        class="dashboard-page__stat-card"
      >
        <div class="stat-card" :class="statItem.stat_class">
          <span class="stat-card__label">{{ statItem.stat_label }}</span>
          <span class="stat-card__value">{{ statItem.stat_value }}</span>
        </div>
      </NWCard>
    </div>

    <!-- 近 30 天趋势 -->
    <NWCard :title="t('dashboard.trendTitle')" class="dashboard-page__trend-card">
      <div ref="trendChartElement" class="dashboard-page__trend-chart" />
    </NWCard>

    <!-- 项目列表 -->
    <NWCard class="dashboard-page__project-card">
      <template #header>
        <h3 class="nw-card__title">{{ t('dashboard.projectList') }}</h3>
        <NWButton type="primary" size="small" @click="handleCreateProject">
          {{ t('dashboard.createProject') }}
        </NWButton>
      </template>

      <NWLoading v-if="projectLoading" />
      <template v-else>
        <el-table :data="projectList" class="dashboard-page__project-table" @row-click="handleProjectRowClick">
          <el-table-column :label="t('dashboard.projectCover')" width="72">
            <template #default="{ row }">
              <div
                class="project-cover"
                :style="{ backgroundColor: coverColor(row.project_name) }"
              >
                {{ (row.project_name || '?').slice(0, 1) }}
              </div>
            </template>
          </el-table-column>
          <el-table-column prop="project_name" :label="t('dashboard.projectName')" min-width="180" />
          <el-table-column :label="t('dashboard.projectStatus')" width="110">
            <template #default="{ row }">
              <span class="project-status">{{ projectStatusText(row.project_status) }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="total_word_count" :label="t('dashboard.projectWordCount')" width="120" />
          <el-table-column prop="chapter_count" :label="t('dashboard.projectChapterCount')" width="100" />
          <el-table-column :label="t('dashboard.projectUpdatedAt')" width="180">
            <template #default="{ row }">
              {{ formatTimestamp(row.updated_at_ms) }}
            </template>
          </el-table-column>
          <el-table-column :label="t('common.action')" width="160" fixed="right">
            <template #default="{ row }">
              <NWButton size="small" @click.stop="handleEditProject(row)">
                {{ t('dashboard.editProject') }}
              </NWButton>
              <NWButton
                type="danger"
                size="small"
                @click.stop="handleDeleteProject(row)"
              >
                {{ t('common.delete') }}
              </NWButton>
            </template>
          </el-table-column>
          <template #empty>
            <NWEmpty :description_text="t('dashboard.projectEmptyTip')" />
          </template>
        </el-table>

        <el-pagination
          v-if="projectTotal > NW_PROJECT_PAGE_SIZE"
          class="dashboard-page__pagination"
          layout="prev, pager, next"
          :total="projectTotal"
          :page-size="NW_PROJECT_PAGE_SIZE"
          :current-page="projectPageNum"
          @current-change="handleProjectPageChange"
        />
      </template>
    </NWCard>

    <!-- 新建项目对话框 -->
    <el-dialog
      v-model="createDialogVisible"
      :title="projectEditMode ? t('dashboard.editProjectTitle') : t('dashboard.createProjectTitle')"
      width="520px"
    >
      <el-form ref="createFormRef" :model="createForm" :rules="createRules" label-width="88px">
        <el-form-item :label="t('dashboard.projectName')" prop="project_name">
          <el-input
            v-model="createForm.project_name"
            maxlength="128"
            :placeholder="t('dashboard.projectNamePlaceholder')"
          />
        </el-form-item>
        <el-form-item v-if="projectEditMode" :label="t('dashboard.projectStatus')">
          <el-select v-model="createForm.project_status">
            <el-option value="draft" :label="t('dashboard.statusDraft')" />
            <el-option value="serial" :label="t('dashboard.statusSerial')" />
            <el-option value="completed" :label="t('dashboard.statusCompleted')" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('dashboard.genreCode')" prop="genre_code">
          <el-input
            v-model="createForm.genre_code"
            maxlength="32"
            :placeholder="t('dashboard.genrePlaceholder')"
          />
        </el-form-item>
        <el-form-item :label="t('dashboard.styleText')" prop="style_text">
          <el-input
            v-model="createForm.style_text"
            type="textarea"
            :rows="3"
            maxlength="1024"
            :placeholder="t('dashboard.stylePlaceholder')"
          />
        </el-form-item>
        <el-form-item :label="t('dashboard.synopsisText')" prop="synopsis_text">
          <el-input
            v-model="createForm.synopsis_text"
            type="textarea"
            :rows="3"
            maxlength="4096"
            :placeholder="t('dashboard.synopsisPlaceholder')"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <NWButton @click="createDialogVisible = false">{{ t('common.cancel') }}</NWButton>
        <NWButton type="primary" :loading="createSaving" @click="handleCreateSubmit">
          {{ t('common.confirm') }}
        </NWButton>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped src="./dashboard.css"></style>

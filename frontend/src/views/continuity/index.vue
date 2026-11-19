<!--
  连续性看板：时间事实复核、伏笔钩子跟踪、章节摘要管理。
  章节相关筛选与表单统一使用章节选择器，避免手填 ULID。
-->
<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { useProjectStore } from '@/stores/project'
import { getProjectTree } from '@/api/project'
import {
  listFacts,
  listHooks,
  listSummaries,
  reviewFact,
  type NWTemporalFactVo,
  type NWForeshadowHookVo,
  type NWChapterSummaryVo,
  type NWFactStatusCode,
  type NWHookStatusCode,
  type NWSummaryStatusCode,
  type NWImportanceCode,
} from '@/api/continuity'
import type { NWChapterBriefVo } from '@/api/contracts'
import NWButton from '@/components/NWButton.vue'
import NWEmpty from '@/components/NWEmpty.vue'
import NWIcon from '@/components/NWIcon.vue'
import NWPageHeader from '@/components/NWPageHeader.vue'
import NWStatTile from '@/components/NWStatTile.vue'
import ProjectPageHeader from '@/components/ProjectPageHeader.vue'
import ContinuityDialogs from './ContinuityDialogs.vue'

const { t } = useI18n()
const route = useRoute()
const projectStore = useProjectStore()
const projectUlid = computed(
  () => String(route.params.project_ulid ?? '') || projectStore.current_project_ulid,
)

const FACT_STATUS_TEXT = computed<Record<NWFactStatusCode, string>>(() => ({
  candidate: t('continuity.candidate'),
  confirmed: t('continuity.confirmed'),
  superseded: t('continuity.superseded'),
  stale: t('continuity.stale'),
}))
const HOOK_STATUS_TEXT = computed<Record<NWHookStatusCode, string>>(() => ({
  planned: t('continuity.planned'),
  planted: t('continuity.planted'),
  echoed: t('continuity.echoed'),
  resolved: t('continuity.resolved'),
  abandoned: t('continuity.abandoned'),
}))
const SUMMARY_STATUS_TEXT = computed<Record<NWSummaryStatusCode, string>>(() => ({
  pending: t('continuity.pending'),
  verified: t('continuity.verified'),
  stale: t('continuity.stale'),
}))
const IMPORTANCE_TEXT = computed<Record<NWImportanceCode, string>>(() => ({
  high: t('continuity.high'),
  medium: t('continuity.medium'),
  low: t('continuity.low'),
}))

const activeTab = ref('fact')
const factList = ref<NWTemporalFactVo[]>([])
const hookList = ref<NWForeshadowHookVo[]>([])
const summaryList = ref<NWChapterSummaryVo[]>([])
const chapterList = ref<NWChapterBriefVo[]>([])
const factLoading = ref(false)
const hookLoading = ref(false)
const summaryLoading = ref(false)
const factStatusFilter = ref<NWFactStatusCode | ''>('')
const factChapterFilter = ref('')
const hookStatusFilter = ref<NWHookStatusCode | ''>('')
const summaryStatusFilter = ref<NWSummaryStatusCode | ''>('')
const summaryChapterFilter = ref('')
const dialogsRef = ref<InstanceType<typeof ContinuityDialogs>>()

const chapterNameMap = computed(() => {
  const map = new Map<string, string>()
  for (const chapterItem of chapterList.value) {
    map.set(chapterItem.chapter_ulid, chapterItem.chapter_name)
  }
  return map
})

const activeHookCount = computed(
  () => hookList.value.filter((hookItem) => hookItem.hook_status_code === 'planted' || hookItem.hook_status_code === 'echoed').length,
)
const candidateFactCount = computed(
  () => factList.value.filter((factItem) => factItem.fact_status_code === 'candidate').length,
)
const pendingSummaryCount = computed(
  () => summaryList.value.filter((summaryItem) => summaryItem.summary_status_code !== 'verified').length,
)

function chapterName(chapterUlid: string): string {
  if (!chapterUlid) return '—'
  return chapterNameMap.value.get(chapterUlid) ?? chapterUlid
}

function formatTime(timeMs: number): string {
  return timeMs ? new Date(timeMs).toLocaleString() : '—'
}

/** 加载章节选项（项目目录树） */
async function loadChapterList(): Promise<void> {
  if (!projectUlid.value) return
  try {
    const treeData = await getProjectTree(projectUlid.value)
    const list: NWChapterBriefVo[] = []
    for (const volumeNode of treeData.volume_list ?? []) {
      list.push(...volumeNode.chapter_list)
    }
    chapterList.value = list
  } catch {
    chapterList.value = []
  }
}

async function loadFacts(): Promise<void> {
  if (!projectUlid.value) return
  factLoading.value = true
  try {
    const list = await listFacts({
      project_ulid: projectUlid.value,
      fact_status_code: factStatusFilter.value || undefined,
      chapter_ulid: factChapterFilter.value || undefined,
    })
    factList.value = Array.isArray(list) ? list : []
  } catch {
    factList.value = []
  } finally {
    factLoading.value = false
  }
}

async function loadHooks(): Promise<void> {
  if (!projectUlid.value) return
  hookLoading.value = true
  try {
    const list = await listHooks({
      project_ulid: projectUlid.value,
      hook_status_code: hookStatusFilter.value || undefined,
    })
    hookList.value = Array.isArray(list) ? list : []
  } catch {
    hookList.value = []
  } finally {
    hookLoading.value = false
  }
}

async function loadSummaries(): Promise<void> {
  if (!projectUlid.value) return
  summaryLoading.value = true
  try {
    const list = await listSummaries({
      project_ulid: projectUlid.value,
      summary_status_code: summaryStatusFilter.value || undefined,
      chapter_ulid: summaryChapterFilter.value || undefined,
    })
    summaryList.value = Array.isArray(list) ? list : []
  } catch {
    summaryList.value = []
  } finally {
    summaryLoading.value = false
  }
}

async function reviewFactStatus(
  item: NWTemporalFactVo,
  statusCode: Extract<NWFactStatusCode, 'confirmed' | 'superseded' | 'stale'>,
): Promise<void> {
  try {
    await reviewFact({
      fact_ulid: item.fact_ulid,
      project_ulid: projectUlid.value,
      fact_status_code: statusCode,
    })
    ElMessage.success(t('continuity.statusChanged', { status: FACT_STATUS_TEXT.value[statusCode] }))
    await loadFacts()
  } catch {
    // 失败提示由 http 层统一弹出
  }
}

function loadAll(): void {
  void loadChapterList()
  void loadFacts()
  void loadHooks()
  void loadSummaries()
}

watch(projectUlid, loadAll)
onMounted(loadAll)
</script>

<template>
  <div class="nw-page continuity-page">
    <ProjectPageHeader :project-ulid="projectUlid" />

    <NWPageHeader
       :title_text="t('continuity.title')"
       :subtitle_text="t('continuity.subtitle')"
      icon_code="shield"
    >
      <template #actions>
        <NWButton @click="loadAll">
          <NWIcon icon_code="refresh" :size_px="15" />
          {{ t('continuity.refresh') }}
        </NWButton>
        <NWButton type="primary" @click="dialogsRef?.openFact()">
          <NWIcon icon_code="plus" :size_px="15" />
          {{ t('continuity.createFact') }}
        </NWButton>
      </template>
    </NWPageHeader>

    <div class="nw-stat-grid">
      <NWStatTile :label_text="t('continuity.statFacts')" :value_number="factList.length" :hint_text="t('continuity.statFactsHint')" icon_code="clock" tone_code="primary" />
      <NWStatTile :label_text="t('continuity.statPending')" :value_number="candidateFactCount" :hint_text="t('continuity.statPendingHint')" icon_code="alertCircle" tone_code="warning" />
      <NWStatTile :label_text="t('continuity.statHooks')" :value_number="activeHookCount" :hint_text="t('continuity.statHooksHint')" icon_code="target" tone_code="info" />
      <NWStatTile :label_text="t('continuity.statSummaries')" :value_number="pendingSummaryCount" :hint_text="t('continuity.statSummariesHint')" icon_code="note" tone_code="success" />
    </div>

    <section class="nw-panel">
      <div class="continuity-tabs">
        <button type="button" class="continuity-tab" :class="{ 'is-active': activeTab === 'fact' }" @click="activeTab = 'fact'">
          {{ t('continuity.tabFacts') }}
          <span class="continuity-tab__count">{{ factList.length }}</span>
        </button>
        <button type="button" class="continuity-tab" :class="{ 'is-active': activeTab === 'hook' }" @click="activeTab = 'hook'">
          {{ t('continuity.tabHooks') }}
          <span class="continuity-tab__count">{{ hookList.length }}</span>
        </button>
        <button type="button" class="continuity-tab" :class="{ 'is-active': activeTab === 'summary' }" @click="activeTab = 'summary'">
          {{ t('continuity.tabSummaries') }}
          <span class="continuity-tab__count">{{ summaryList.length }}</span>
        </button>
      </div>

      <!-- 时间事实 -->
      <div v-if="activeTab === 'fact'" class="nw-panel__body">
        <div class="continuity-toolbar">
          <el-select v-model="factStatusFilter" class="continuity-filter" :placeholder="t('continuity.statusAll')" clearable @change="loadFacts">
            <el-option v-for="(text, code) in FACT_STATUS_TEXT" :key="code" :label="text" :value="code" />
          </el-select>
          <el-select v-model="factChapterFilter" class="continuity-filter is-wide" :placeholder="t('continuity.chapterAll')" clearable @change="loadFacts">
            <el-option v-for="chapterItem in chapterList" :key="chapterItem.chapter_ulid" :label="chapterItem.chapter_name" :value="chapterItem.chapter_ulid" />
          </el-select>
        </div>

        <NWEmpty v-if="!factLoading && factList.length === 0" icon_code="clock" :title_text="t('continuity.emptyFactsTitle')" :description_text="t('continuity.emptyFactsDesc')" compact_flag>
          <template #actions><NWButton size="small" type="primary" @click="dialogsRef?.openFact()">{{ t('continuity.createFact') }}</NWButton></template>
        </NWEmpty>
        <el-table v-else v-loading="factLoading" :data="factList" stripe>
          <el-table-column :label="t('continuity.colFact')" min-width="280">
            <template #default="{ row }">
              <div class="continuity-fact">
                <strong>{{ row.subject_text }}</strong>
                <span class="continuity-fact__predicate">{{ row.predicate_code }}</span>
                <span>{{ row.object_text }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column :label="t('continuity.colStatus')" width="100">
            <template #default="{ row }">
              <span class="nw-badge" :class="`nw-badge--${row.fact_status_code === 'confirmed' ? 'success' : row.fact_status_code === 'candidate' ? 'warning' : 'danger'}`">
                {{ FACT_STATUS_TEXT[row.fact_status_code as NWFactStatusCode] || row.fact_status_code }}
              </span>
            </template>
          </el-table-column>
          <el-table-column :label="t('continuity.colSourceChapter')" min-width="160">
            <template #default="{ row }">{{ chapterName(row.source_chapter_ulid) }}</template>
          </el-table-column>
          <el-table-column :label="t('continuity.colUpdatedAt')" width="160">
            <template #default="{ row }">{{ formatTime(row.updated_at_ms) }}</template>
          </el-table-column>
          <el-table-column :label="t('continuity.colActions')" width="170" fixed="right">
            <template #default="{ row }">
              <div class="continuity-row-actions">
                <button v-if="row.fact_status_code !== 'confirmed'" type="button" class="continuity-text-btn" @click="reviewFactStatus(row, 'confirmed')">{{ t('continuity.confirm') }}</button>
                <el-dropdown trigger="click">
                  <button type="button" class="continuity-text-btn is-icon"><NWIcon icon_code="more" :size_px="15" /></button>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item @click="dialogsRef?.openFact(row)">{{ t('continuity.edit') }}</el-dropdown-item>
                      <el-dropdown-item v-if="row.fact_status_code !== 'superseded'" @click="reviewFactStatus(row, 'superseded')">{{ t('continuity.replace') }}</el-dropdown-item>
                      <el-dropdown-item v-if="row.fact_status_code !== 'stale'" @click="reviewFactStatus(row, 'stale')">{{ t('continuity.markStale') }}</el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
              </div>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <!-- 伏笔钩子 -->
      <div v-else-if="activeTab === 'hook'" class="nw-panel__body">
        <div class="continuity-toolbar">
          <el-select v-model="hookStatusFilter" class="continuity-filter" :placeholder="t('continuity.statusAll')" clearable @change="loadHooks">
            <el-option v-for="(text, code) in HOOK_STATUS_TEXT" :key="code" :label="text" :value="code" />
          </el-select>
        </div>

        <NWEmpty v-if="!hookLoading && hookList.length === 0" icon_code="target" :title_text="t('continuity.emptyHooksTitle')" :description_text="t('continuity.emptyHooksDesc')" compact_flag>
          <template #actions><NWButton size="small" type="primary" @click="dialogsRef?.openHook()">{{ t('continuity.createHook') }}</NWButton></template>
        </NWEmpty>
        <el-table v-else v-loading="hookLoading" :data="hookList" stripe>
          <el-table-column :label="t('continuity.colHookDesc')" min-width="260">
            <template #default="{ row }">
              <div class="continuity-hook">
                <strong>{{ row.description_text }}</strong>
                <small v-if="row.expected_payoff_text">{{ t('continuity.expectedPrefix') }}{{ row.expected_payoff_text }}</small>
              </div>
            </template>
          </el-table-column>
          <el-table-column :label="t('continuity.colStatus')" width="100">
            <template #default="{ row }">
              <span class="nw-badge" :class="`nw-badge--${row.hook_status_code === 'resolved' ? 'success' : row.hook_status_code === 'abandoned' ? 'danger' : 'warning'}`">
                {{ HOOK_STATUS_TEXT[row.hook_status_code as NWHookStatusCode] || row.hook_status_code }}
              </span>
            </template>
          </el-table-column>
          <el-table-column :label="t('continuity.colHookImportance')" width="90">
            <template #default="{ row }">{{ IMPORTANCE_TEXT[row.importance_code as NWImportanceCode] || row.importance_code }}</template>
          </el-table-column>
          <el-table-column :label="t('continuity.colHookChapters')" min-width="200">
            <template #default="{ row }">
              <span class="continuity-chapter-pair">{{ chapterName(row.setup_chapter_ulid) }} → {{ chapterName(row.payoff_chapter_ulid) }}</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('continuity.colActions')" width="80" fixed="right">
            <template #default="{ row }">
              <button type="button" class="continuity-text-btn" @click="dialogsRef?.openHook(row)">{{ t('continuity.edit') }}</button>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <!-- 章节摘要 -->
      <div v-else class="nw-panel__body">
        <div class="continuity-toolbar">
          <el-select v-model="summaryStatusFilter" class="continuity-filter" :placeholder="t('continuity.statusAll')" clearable @change="loadSummaries">
            <el-option v-for="(text, code) in SUMMARY_STATUS_TEXT" :key="code" :label="text" :value="code" />
          </el-select>
          <el-select v-model="summaryChapterFilter" class="continuity-filter is-wide" :placeholder="t('continuity.chapterAll')" clearable @change="loadSummaries">
            <el-option v-for="chapterItem in chapterList" :key="chapterItem.chapter_ulid" :label="chapterItem.chapter_name" :value="chapterItem.chapter_ulid" />
          </el-select>
        </div>

        <NWEmpty v-if="!summaryLoading && summaryList.length === 0" icon_code="note" :title_text="t('continuity.emptySummariesTitle')" :description_text="t('continuity.emptySummariesDesc')" compact_flag>
          <template #actions><NWButton size="small" type="primary" @click="dialogsRef?.openSummary()">{{ t('continuity.createSummary') }}</NWButton></template>
        </NWEmpty>
        <el-table v-else v-loading="summaryLoading" :data="summaryList" stripe>
          <el-table-column :label="t('continuity.colSummaryChapter')" width="180">
            <template #default="{ row }">{{ chapterName(row.chapter_ulid) }}</template>
          </el-table-column>
          <el-table-column :label="t('continuity.colStatus')" width="100">
            <template #default="{ row }">
              <span class="nw-badge" :class="`nw-badge--${row.summary_status_code === 'verified' ? 'success' : row.summary_status_code === 'stale' ? 'danger' : 'warning'}`">
                {{ SUMMARY_STATUS_TEXT[row.summary_status_code as NWSummaryStatusCode] || row.summary_status_code }}
              </span>
            </template>
          </el-table-column>
          <el-table-column :label="t('continuity.colSummaryContent')" min-width="300">
            <template #default="{ row }">
              <span class="continuity-summary-text">{{ row.summary_text }}</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('continuity.colUpdatedAt')" width="160">
            <template #default="{ row }">{{ formatTime(row.updated_at_ms) }}</template>
          </el-table-column>
          <el-table-column :label="t('continuity.colActions')" width="80" fixed="right">
            <template #default="{ row }">
              <button type="button" class="continuity-text-btn" @click="dialogsRef?.openSummary(row)">{{ t('continuity.edit') }}</button>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </section>

    <ContinuityDialogs
      ref="dialogsRef"
      :project-ulid="projectUlid"
      :chapter-list="chapterList"
      :fact-status-text="FACT_STATUS_TEXT"
      :hook-status-text="HOOK_STATUS_TEXT"
      :summary-status-text="SUMMARY_STATUS_TEXT"
      :importance-text="IMPORTANCE_TEXT"
      @fact-saved="loadFacts"
      @hook-saved="loadHooks"
      @summary-saved="loadSummaries"
    />
  </div>
</template>

<style scoped src="./continuity.css"></style>

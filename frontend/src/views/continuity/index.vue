<!-- 连续性看板：时间事实复核、伏笔钩子跟踪、章节摘要管理三个页签。 -->
<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useProjectStore } from '@/stores/project'
import {
  listFacts, listHooks, listSummaries, reviewFact,
  type NWTemporalFactVo, type NWForeshadowHookVo, type NWChapterSummaryVo,
  type NWFactStatusCode, type NWHookStatusCode, type NWSummaryStatusCode, type NWImportanceCode,
} from '@/api/continuity'
import NWButton from '@/components/NWButton.vue'
import NWCard from '@/components/NWCard.vue'
import NWEmpty from '@/components/NWEmpty.vue'
import NWLoading from '@/components/NWLoading.vue'
import ProjectPageHeader from '@/components/ProjectPageHeader.vue'
import ContinuityDialogs from './ContinuityDialogs.vue'

const route = useRoute()
const projectStore = useProjectStore()
/** 路由参数优先，缺省回退到 Pinia 当前项目 */
const projectUlid = computed(() => String(route.params.project_ulid ?? '') || projectStore.current_project_ulid)

const FACT_STATUS_TEXT: Record<NWFactStatusCode, string> = {
  candidate: '候选', confirmed: '已确认', superseded: '已取代', stale: '已过期',
}
const HOOK_STATUS_TEXT: Record<NWHookStatusCode, string> = {
  planned: '计划中', planted: '已埋设', echoed: '已呼应', resolved: '已回收', abandoned: '已放弃',
}
const SUMMARY_STATUS_TEXT: Record<NWSummaryStatusCode, string> = {
  pending: '待核验', verified: '已核验', stale: '已过期',
}
const IMPORTANCE_TEXT: Record<NWImportanceCode, string> = { high: '高', medium: '中', low: '低' }

const activeTab = ref('fact')
const factList = ref<NWTemporalFactVo[]>([])
const hookList = ref<NWForeshadowHookVo[]>([])
const summaryList = ref<NWChapterSummaryVo[]>([])
const factLoading = ref(false)
const hookLoading = ref(false)
const summaryLoading = ref(false)
const factError = ref(false)
const hookError = ref(false)
const summaryError = ref(false)
const factStatusFilter = ref<NWFactStatusCode | ''>('')
const factChapterFilter = ref('')
const hookStatusFilter = ref<NWHookStatusCode | ''>('')
const summaryStatusFilter = ref<NWSummaryStatusCode | ''>('')
const summaryChapterFilter = ref('')
const dialogsRef = ref<InstanceType<typeof ContinuityDialogs>>()

/** 毫秒时间戳转本地时间 */
function formatTime(timeMs: number): string {
  return timeMs ? new Date(timeMs).toLocaleString() : '—'
}

/** 加载时间事实列表 */
async function loadFacts(): Promise<void> {
  if (!projectUlid.value) return
  factLoading.value = true
  factError.value = false
  try {
    const list = await listFacts({
      project_ulid: projectUlid.value,
      fact_status_code: factStatusFilter.value || undefined,
      chapter_ulid: factChapterFilter.value.trim() || undefined,
    })
    factList.value = Array.isArray(list) ? list : []
  } catch {
    factList.value = []
    factError.value = true
    ElMessage.error('时间事实加载失败，请重试')
  } finally {
    factLoading.value = false
  }
}

/** 加载伏笔钩子列表 */
async function loadHooks(): Promise<void> {
  if (!projectUlid.value) return
  hookLoading.value = true
  hookError.value = false
  try {
    const list = await listHooks({
      project_ulid: projectUlid.value,
      hook_status_code: hookStatusFilter.value || undefined,
    })
    hookList.value = Array.isArray(list) ? list : []
  } catch {
    hookList.value = []
    hookError.value = true
    ElMessage.error('伏笔钩子加载失败，请重试')
  } finally {
    hookLoading.value = false
  }
}

/** 加载章节摘要列表 */
async function loadSummaries(): Promise<void> {
  if (!projectUlid.value) return
  summaryLoading.value = true
  summaryError.value = false
  try {
    const list = await listSummaries({
      project_ulid: projectUlid.value,
      summary_status_code: summaryStatusFilter.value || undefined,
      chapter_ulid: summaryChapterFilter.value.trim() || undefined,
    })
    summaryList.value = Array.isArray(list) ? list : []
  } catch {
    summaryList.value = []
    summaryError.value = true
    ElMessage.error('章节摘要加载失败，请重试')
  } finally {
    summaryLoading.value = false
  }
}

/** 复核时间事实状态 */
async function reviewFactStatus(item: NWTemporalFactVo, statusCode: Extract<NWFactStatusCode, 'confirmed' | 'superseded' | 'stale'>): Promise<void> {
  try {
    await reviewFact({ fact_ulid: item.fact_ulid, project_ulid: projectUlid.value, fact_status_code: statusCode })
    ElMessage.success(`已标记为${FACT_STATUS_TEXT[statusCode]}`)
    await loadFacts()
  } catch {
    // 失败提示由 http 层统一弹出
  }
}

/** 项目切换或首次挂载时加载全部列表 */
function loadAll(): void {
  void loadFacts()
  void loadHooks()
  void loadSummaries()
}
watch(projectUlid, loadAll)
onMounted(loadAll)
</script>

<template>
  <div class="continuity-page">
    <ProjectPageHeader :project-ulid="projectUlid" />
    <NWCard title="连续性看板" :hoverable="false">
      <el-tabs v-model="activeTab" class="continuity-tabs">
        <!-- 时间事实 -->
        <el-tab-pane label="时间事实" name="fact">
          <div class="continuity-toolbar">
            <el-select v-model="factStatusFilter" class="continuity-filter" placeholder="全部状态" clearable @change="loadFacts">
              <el-option v-for="(text, code) in FACT_STATUS_TEXT" :key="code" :label="text" :value="code" />
            </el-select>
            <el-input v-model="factChapterFilter" class="continuity-filter" placeholder="章节 ULID" clearable @keyup.enter="loadFacts" />
            <NWButton size="small" @click="loadFacts">刷新</NWButton>
            <NWButton size="small" type="primary" @click="dialogsRef?.openFact()">新建事实</NWButton>
          </div>
          <NWLoading v-if="factLoading && factList.length === 0" />
          <div v-else-if="factError" class="continuity-error">
            <NWEmpty description_text="时间事实加载失败" />
            <NWButton size="small" type="primary" @click="loadFacts">重试</NWButton>
          </div>
          <NWEmpty v-else-if="factList.length === 0" description_text="暂无时间事实" />
          <el-table v-else :data="factList" border stripe size="small" class="continuity-table">
            <el-table-column prop="subject_text" label="主体" min-width="110" show-overflow-tooltip />
            <el-table-column prop="predicate_code" label="谓词" min-width="100" show-overflow-tooltip />
            <el-table-column prop="object_text" label="客体" min-width="120" show-overflow-tooltip />
            <el-table-column label="状态" width="100">
              <template #default="{ row }">
                <span class="continuity-chip" :class="`continuity-chip--${row.fact_status_code}`">
                  {{ FACT_STATUS_TEXT[row.fact_status_code as NWFactStatusCode] || row.fact_status_code }}
                </span>
              </template>
            </el-table-column>
            <el-table-column prop="source_chapter_ulid" label="来源章节" min-width="130" show-overflow-tooltip />
            <el-table-column label="更新时间" width="150">
              <template #default="{ row }">{{ formatTime(row.updated_at_ms) }}</template>
            </el-table-column>
            <el-table-column label="操作" width="230" fixed="right">
              <template #default="{ row }">
                <NWButton v-if="row.fact_status_code !== 'confirmed'" size="small" type="primary" @click="reviewFactStatus(row, 'confirmed')">确认</NWButton>
                <NWButton v-if="row.fact_status_code !== 'superseded'" size="small" @click="reviewFactStatus(row, 'superseded')">取代</NWButton>
                <NWButton v-if="row.fact_status_code !== 'stale'" size="small" @click="reviewFactStatus(row, 'stale')">过期</NWButton>
                <NWButton size="small" @click="dialogsRef?.openFact(row)">编辑</NWButton>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>

        <!-- 伏笔钩子 -->
        <el-tab-pane label="伏笔钩子" name="hook">
          <div class="continuity-toolbar">
            <el-select v-model="hookStatusFilter" class="continuity-filter" placeholder="全部状态" clearable @change="loadHooks">
              <el-option v-for="(text, code) in HOOK_STATUS_TEXT" :key="code" :label="text" :value="code" />
            </el-select>
            <NWButton size="small" @click="loadHooks">刷新</NWButton>
            <NWButton size="small" type="primary" @click="dialogsRef?.openHook()">新建伏笔</NWButton>
          </div>
          <NWLoading v-if="hookLoading && hookList.length === 0" />
          <div v-else-if="hookError" class="continuity-error">
            <NWEmpty description_text="伏笔钩子加载失败" />
            <NWButton size="small" type="primary" @click="loadHooks">重试</NWButton>
          </div>
          <NWEmpty v-else-if="hookList.length === 0" description_text="暂无伏笔钩子" />
          <el-table v-else :data="hookList" border stripe size="small" class="continuity-table">
            <el-table-column prop="description_text" label="伏笔描述" min-width="180" show-overflow-tooltip />
            <el-table-column label="状态" width="100">
              <template #default="{ row }">
                <span class="continuity-chip" :class="`continuity-chip--${row.hook_status_code}`">
                  {{ HOOK_STATUS_TEXT[row.hook_status_code as NWHookStatusCode] || row.hook_status_code }}
                </span>
              </template>
            </el-table-column>
            <el-table-column label="重要程度" width="90">
              <template #default="{ row }">{{ IMPORTANCE_TEXT[row.importance_code as NWImportanceCode] || row.importance_code }}</template>
            </el-table-column>
            <el-table-column prop="setup_chapter_ulid" label="埋设章节" min-width="130" show-overflow-tooltip />
            <el-table-column prop="payoff_chapter_ulid" label="回收章节" min-width="130" show-overflow-tooltip />
            <el-table-column prop="expected_payoff_text" label="预期回收" min-width="160" show-overflow-tooltip />
            <el-table-column label="更新时间" width="150">
              <template #default="{ row }">{{ formatTime(row.updated_at_ms) }}</template>
            </el-table-column>
            <el-table-column label="操作" width="90" fixed="right">
              <template #default="{ row }"><NWButton size="small" @click="dialogsRef?.openHook(row)">编辑</NWButton></template>
            </el-table-column>
          </el-table>
        </el-tab-pane>

        <!-- 章节摘要 -->
        <el-tab-pane label="章节摘要" name="summary">
          <div class="continuity-toolbar">
            <el-select v-model="summaryStatusFilter" class="continuity-filter" placeholder="全部状态" clearable @change="loadSummaries">
              <el-option v-for="(text, code) in SUMMARY_STATUS_TEXT" :key="code" :label="text" :value="code" />
            </el-select>
            <el-input v-model="summaryChapterFilter" class="continuity-filter" placeholder="章节 ULID" clearable @keyup.enter="loadSummaries" />
            <NWButton size="small" @click="loadSummaries">刷新</NWButton>
            <NWButton size="small" type="primary" @click="dialogsRef?.openSummary()">新建摘要</NWButton>
          </div>
          <NWLoading v-if="summaryLoading && summaryList.length === 0" />
          <div v-else-if="summaryError" class="continuity-error">
            <NWEmpty description_text="章节摘要加载失败" />
            <NWButton size="small" type="primary" @click="loadSummaries">重试</NWButton>
          </div>
          <NWEmpty v-else-if="summaryList.length === 0" description_text="暂无章节摘要" />
          <el-table v-else :data="summaryList" border stripe size="small" class="continuity-table">
            <el-table-column prop="chapter_ulid" label="章节 ULID" min-width="140" show-overflow-tooltip />
            <el-table-column label="状态" width="100">
              <template #default="{ row }">
                <span class="continuity-chip" :class="`continuity-chip--${row.summary_status_code}`">
                  {{ SUMMARY_STATUS_TEXT[row.summary_status_code as NWSummaryStatusCode] || row.summary_status_code }}
                </span>
              </template>
            </el-table-column>
            <el-table-column prop="summary_text" label="摘要内容" min-width="260" show-overflow-tooltip />
            <el-table-column prop="source_hash_text" label="正文指纹" min-width="130" show-overflow-tooltip />
            <el-table-column label="更新时间" width="150">
              <template #default="{ row }">{{ formatTime(row.updated_at_ms) }}</template>
            </el-table-column>
            <el-table-column label="操作" width="90" fixed="right">
              <template #default="{ row }"><NWButton size="small" @click="dialogsRef?.openSummary(row)">编辑</NWButton></template>
            </el-table-column>
          </el-table>
        </el-tab-pane>
      </el-tabs>
    </NWCard>

    <!-- 三个编辑弹窗（时间事实 / 伏笔钩子 / 章节摘要） -->
    <ContinuityDialogs
      ref="dialogsRef"
      :project-ulid="projectUlid"
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

<!-- 连续性看板：时间事实、伏笔台账与章节摘要三本账。 -->
<script setup lang="ts">
  import { computed, onMounted, reactive, ref, watch } from 'vue'
  import { ElMessage } from 'element-plus'
  import {
    listFacts,
    listHooks,
    listSummaries,
    reviewFact,
    saveSummary,
    type NWFactStatusCode,
    type NWForeshadowHookVo,
    type NWHookStatusCode,
    type NWImportanceCode,
    type NWSummaryStatusCode,
    type NWTemporalFactVo
  } from '@/api/continuity'
  import type { NWChapterBriefVo } from '@/api/contracts'
  import { getProjectTree } from '@/api/project'
  import { useProjectStore } from '@/store/modules/project'
  import { formatTime } from '@/utils/time'
  import EntityDialogs from './components/EntityDialogs.vue'

  const projectStore = useProjectStore()
  const projectUlid = computed(() => projectStore.currentProjectUlid)

  type TabCode = 'fact' | 'hook' | 'summary'
  const activeTab = ref<TabCode>('fact')
  const loading = ref(false)

  const chapterList = ref<NWChapterBriefVo[]>([])
  const factList = ref<NWTemporalFactVo[]>([])
  const hookList = ref<NWForeshadowHookVo[]>([])
  const summaryList = ref<Awaited<ReturnType<typeof listSummaries>>[number][]>([])

  const factStatusOptions: Array<{ value: NWFactStatusCode; label: string }> = [
    { value: 'candidate', label: '待确认' },
    { value: 'confirmed', label: '已确认' },
    { value: 'superseded', label: '已推翻' },
    { value: 'stale', label: '待复核' }
  ]
  const hookStatusOptions: Array<{ value: NWHookStatusCode; label: string }> = [
    { value: 'planned', label: '已计划' },
    { value: 'planted', label: '已埋设' },
    { value: 'echoed', label: '已呼应' },
    { value: 'resolved', label: '已回收' },
    { value: 'abandoned', label: '已放弃' }
  ]
  const importanceOptions: Array<{ value: NWImportanceCode; label: string }> = [
    { value: 'high', label: '高' },
    { value: 'medium', label: '中' },
    { value: 'low', label: '低' }
  ]
  const summaryStatusOptions: Array<{ value: NWSummaryStatusCode; label: string }> = [
    { value: 'pending', label: '待复核' },
    { value: 'verified', label: '已确认' },
    { value: 'stale', label: '已过期' }
  ]

  const labelOf = <T extends string>(
    options: Array<{ value: T; label: string }>,
    valueCode: string
  ) => options.find((item) => item.value === valueCode)?.label ?? valueCode

  const chapterName = (chapterUlid: string) =>
    chapterList.value.find((item) => item.chapter_ulid === chapterUlid)?.chapter_name ?? '—'

  const loadChapters = async () => {
    if (!projectUlid.value) return
    const tree = await getProjectTree(projectUlid.value)
    chapterList.value = (tree.volume_list ?? []).flatMap((volume) => volume.chapter_list)
  }

  const loadAll = async () => {
    if (!projectUlid.value) return
    loading.value = true
    try {
      const [facts, hooks, summaries] = await Promise.all([
        listFacts({ project_ulid: projectUlid.value }),
        listHooks({ project_ulid: projectUlid.value }),
        listSummaries({ project_ulid: projectUlid.value })
      ])
      factList.value = facts
      hookList.value = hooks
      summaryList.value = summaries
    } finally {
      loading.value = false
    }
  }

  // 事实与伏笔的表单在 EntityDialogs 中（含校验与保存），这里只保留入口与列表
  const entityDialogs = ref<InstanceType<typeof EntityDialogs> | null>(null)
  const openFact = (fact?: NWTemporalFactVo) => entityDialogs.value?.openFact(fact)
  const openHook = (hook?: NWForeshadowHookVo) => entityDialogs.value?.openHook(hook)

  // ---------------- 摘要 ----------------
  const summaryDialogVisible = ref(false)
  const summarySaving = ref(false)
  const summaryForm = reactive({
    summary_ulid: '',
    chapter_ulid: '',
    summary_text: '',
    summary_status_code: 'pending' as NWSummaryStatusCode
  })

  const openSummary = (summary?: { summary_ulid: string; chapter_ulid: string; summary_text: string; summary_status_code: NWSummaryStatusCode }) => {
    summaryForm.summary_ulid = summary?.summary_ulid ?? ''
    summaryForm.chapter_ulid = summary?.chapter_ulid ?? chapterList.value[0]?.chapter_ulid ?? ''
    summaryForm.summary_text = summary?.summary_text ?? ''
    summaryForm.summary_status_code = summary?.summary_status_code ?? 'pending'
    summaryDialogVisible.value = true
  }

  const submitSummary = async () => {
    if (!summaryForm.chapter_ulid) {
      ElMessage.warning('请选择章节')
      return
    }
    if (!summaryForm.summary_text.trim()) {
      ElMessage.warning('请填写摘要')
      return
    }
    summarySaving.value = true
    try {
      await saveSummary({
        summary_ulid: summaryForm.summary_ulid || undefined,
        project_ulid: projectUlid.value,
        chapter_ulid: summaryForm.chapter_ulid,
        summary_text: summaryForm.summary_text,
        summary_status_code: summaryForm.summary_status_code
      })
      ElMessage.success('摘要已保存')
      summaryDialogVisible.value = false
      await loadAll()
    } finally {
      summarySaving.value = false
    }
  }

  const review = async (fact: NWTemporalFactVo, statusCode: 'confirmed' | 'superseded' | 'stale') => {
    await reviewFact({ fact_ulid: fact.fact_ulid, project_ulid: projectUlid.value, fact_status_code: statusCode })
    ElMessage.success('已复核')
    await loadAll()
  }

  /** 由事实候选一键生成摘要：把待确认事实拼成条目，便于人工复核 */
  const generateSummaryDraft = (chapterUlid: string) => {
    const factLines = factList.value
      .filter((fact) => fact.source_chapter_ulid === chapterUlid)
      .map((fact) => `- ${fact.subject_text} ${fact.predicate_code} ${fact.object_text || '（未填）'}`)
    openSummary()
    summaryForm.chapter_ulid = chapterUlid
    summaryForm.summary_text = factLines.length > 0 ? `本章事实：\n${factLines.join('\n')}` : ''
  }

  const pendingFactCount = computed(() => factList.value.filter((fact) => fact.fact_status_code === 'candidate').length)
  const unresolvedHookCount = computed(
    () => hookList.value.filter((hook) => hook.hook_status_code === 'planned' || hook.hook_status_code === 'planted').length
  )

  onMounted(async () => {
    if (!projectUlid.value) {
      await projectStore.loadProjectOptions()
    }
    await loadChapters()
    await loadAll()
  })

  watch(projectUlid, async () => {
    await loadChapters()
    await loadAll()
  })
</script>

<template>
  <div class="nw-page">
    <ElAlert type="info" :closable="false" show-icon>
      连续性看板记录「已经发生的事实」「埋伏与回收的伏笔」与「章节摘要」。这些条目会随 @ 引用注入 AI 上下文，是长篇不崩设定的依据。
    </ElAlert>

    <div class="cont__stats">
      <ElCard shadow="never" class="cont__stat">
        <span class="cont__stat-label">待确认事实</span>
        <strong class="cont__stat-value">{{ pendingFactCount }}</strong>
      </ElCard>
      <ElCard shadow="never" class="cont__stat">
        <span class="cont__stat-label">未回收伏笔</span>
        <strong class="cont__stat-value is-warning">{{ unresolvedHookCount }}</strong>
      </ElCard>
      <ElCard shadow="never" class="cont__stat">
        <span class="cont__stat-label">章节摘要</span>
        <strong class="cont__stat-value">{{ summaryList.length }}</strong>
      </ElCard>
    </div>

    <ElTabs v-model="activeTab">
      <ElTabPane label="时间事实" name="fact">
        <div class="nw-toolbar">
          <ElButton type="primary" plain size="small" @click="openFact()">
            <Icon icon="mdi:plus" :size="14" /><span class="ml-4px">新建事实</span>
          </ElButton>
          <div class="cont__spacer" />
          <span class="nw-hint">事实 = 主体 + 谓词 + 客体，并绑定来源章节</span>
        </div>

        <ElTable v-loading="loading" :data="factList" class="cont__table">
          <ElTableColumn label="事实" min-width="240">
            <template #default="{ row }">
              <span class="cont__subject">{{ row.subject_text }}</span>
              <span class="cont__predicate">{{ row.predicate_code }}</span>
              <span>{{ row.object_text || '—' }}</span>
            </template>
          </ElTableColumn>
          <ElTableColumn label="状态" width="100">
            <template #default="{ row }">
              <span class="nw-badge" :class="row.fact_status_code === 'confirmed' ? 'nw-badge--success' : row.fact_status_code === 'stale' ? 'nw-badge--danger' : 'nw-badge--warning'">
                {{ labelOf(factStatusOptions, row.fact_status_code) }}
              </span>
            </template>
          </ElTableColumn>
          <ElTableColumn label="来源章节" width="150">
            <template #default="{ row }">{{ chapterName(row.source_chapter_ulid) }}</template>
          </ElTableColumn>
          <ElTableColumn label="原文摘录" min-width="200" show-overflow-tooltip>
            <template #default="{ row }">{{ row.source_quote_text || '—' }}</template>
          </ElTableColumn>
          <ElTableColumn label="操作" width="220" fixed="right">
            <template #default="{ row }">
              <ElButton size="small" text type="primary" @click="openFact(row)">编辑</ElButton>
              <ElButton v-if="row.fact_status_code !== 'confirmed'" size="small" text type="success" @click="review(row, 'confirmed')">确认</ElButton>
              <ElButton v-else size="small" text @click="review(row, 'superseded')">标记推翻</ElButton>
            </template>
          </ElTableColumn>
          <template #empty><span class="nw-hint">还没有记录事实</span></template>
        </ElTable>
      </ElTabPane>

      <ElTabPane label="伏笔台账" name="hook">
        <div class="nw-toolbar">
          <ElButton type="primary" plain size="small" @click="openHook()">
            <Icon icon="mdi:plus" :size="14" /><span class="ml-4px">新建伏笔</span>
          </ElButton>
          <div class="cont__spacer" />
          <span class="nw-hint">状态随显式保存推进：计划 → 埋设 → 呼应 → 回收</span>
        </div>

        <ElTable v-loading="loading" :data="hookList" class="cont__table">
          <ElTableColumn label="伏笔内容" min-width="260" show-overflow-tooltip>
            <template #default="{ row }">{{ row.description_text }}</template>
          </ElTableColumn>
          <ElTableColumn label="预期回收" min-width="200" show-overflow-tooltip>
            <template #default="{ row }">{{ row.expected_payoff_text || '—' }}</template>
          </ElTableColumn>
          <ElTableColumn label="状态" width="110">
            <template #default="{ row }">
              <span class="nw-badge" :class="row.hook_status_code === 'resolved' ? 'nw-badge--success' : row.hook_status_code === 'abandoned' ? 'nw-badge--neutral' : 'nw-badge--warning'">
                {{ labelOf(hookStatusOptions, row.hook_status_code) }}
              </span>
            </template>
          </ElTableColumn>
          <ElTableColumn label="重要度" width="90">
            <template #default="{ row }">{{ labelOf(importanceOptions, row.importance_code) }}</template>
          </ElTableColumn>
          <ElTableColumn label="埋设章" width="140">
            <template #default="{ row }">{{ row.setup_chapter_ulid ? chapterName(row.setup_chapter_ulid) : '—' }}</template>
          </ElTableColumn>
          <ElTableColumn label="操作" width="120" fixed="right">
            <template #default="{ row }">
              <ElButton size="small" text type="primary" @click="openHook(row)">编辑</ElButton>
            </template>
          </ElTableColumn>
          <template #empty><span class="nw-hint">还没有伏笔</span></template>
        </ElTable>
      </ElTabPane>

      <ElTabPane label="章节摘要" name="summary">
        <div class="nw-toolbar">
          <ElButton type="primary" plain size="small" @click="openSummary()">
            <Icon icon="mdi:plus" :size="14" /><span class="ml-4px">新建摘要</span>
          </ElButton>
          <div class="cont__spacer" />
          <span class="nw-hint">摘要可被 AI 对话以 @ 引用，替代整章正文注入</span>
        </div>

        <ElTable v-loading="loading" :data="summaryList" class="cont__table">
          <ElTableColumn label="章节" width="170">
            <template #default="{ row }">{{ chapterName(row.chapter_ulid) }}</template>
          </ElTableColumn>
          <ElTableColumn label="摘要" min-width="320" show-overflow-tooltip>
            <template #default="{ row }">{{ row.summary_text }}</template>
          </ElTableColumn>
          <ElTableColumn label="状态" width="100">
            <template #default="{ row }">
              <span class="nw-badge" :class="row.summary_status_code === 'verified' ? 'nw-badge--success' : row.summary_status_code === 'stale' ? 'nw-badge--danger' : 'nw-badge--warning'">
                {{ labelOf(summaryStatusOptions, row.summary_status_code) }}
              </span>
            </template>
          </ElTableColumn>
          <ElTableColumn label="更新时间" width="180">
            <template #default="{ row }">{{ formatTime(row.updated_at) }}</template>
          </ElTableColumn>
          <ElTableColumn label="操作" width="170" fixed="right">
            <template #default="{ row }">
              <ElButton size="small" text type="primary" @click="openSummary(row)">编辑</ElButton>
              <ElButton size="small" text @click="generateSummaryDraft(row.chapter_ulid)">用事实起草</ElButton>
            </template>
          </ElTableColumn>
          <template #empty><span class="nw-hint">还没有章节摘要</span></template>
        </ElTable>
      </ElTabPane>
    </ElTabs>

    <EntityDialogs
      ref="entityDialogs"
      :project-ulid="projectUlid"
      :chapter-list="chapterList"
      :hook-status-options="hookStatusOptions"
      :importance-options="importanceOptions"
      @saved="loadAll"
    />

    <ElDialog v-model="summaryDialogVisible" :title="summaryForm.summary_ulid ? '编辑摘要' : '新建摘要'" width="560px">
      <ElForm label-position="top">
        <ElFormItem label="章节">
          <ElSelect v-model="summaryForm.chapter_ulid" class="cont__select" filterable>
            <ElOption v-for="chapter in chapterList" :key="chapter.chapter_ulid" :label="chapter.chapter_name" :value="chapter.chapter_ulid" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="摘要内容">
          <ElInput v-model="summaryForm.summary_text" type="textarea" :rows="6" maxlength="2000" show-word-limit />
        </ElFormItem>
        <ElFormItem label="状态">
          <ElSelect v-model="summaryForm.summary_status_code" class="cont__select">
            <ElOption v-for="item in summaryStatusOptions" :key="item.value" :label="item.label" :value="item.value" />
          </ElSelect>
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="summaryDialogVisible = false">取消</ElButton>
        <ElButton type="primary" :loading="summarySaving" @click="submitSummary">保存</ElButton>
      </template>
    </ElDialog>
  </div>
</template>

<style lang="less" scoped>
  .cont {
    &__stats {
      display: grid;
      gap: 14px;
      grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
    }

    &__stat :deep(.el-card__body) {
      display: flex;
      flex-direction: column;
      gap: 6px;
      padding: 14px 18px;
    }

    &__stat-label {
      font-size: 12px;
      color: var(--el-text-color-secondary);
    }

    &__stat-value {
      font-size: 24px;
      font-weight: 700;
      color: var(--nw-ink);

      &.is-warning {
        color: var(--nw-warning);
      }
    }

    &__table {
      margin-top: 10px;
    }

    &__subject {
      font-weight: 600;
      color: var(--nw-ink);
    }

    &__predicate {
      margin: 0 6px;
      font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
      font-size: 11.5px;
      color: var(--el-color-primary);
    }

    &__spacer {
      flex: 1;
    }

    &__row {
      display: grid;
      gap: 12px;
      grid-template-columns: 1fr 1fr;
    }

    &__select {
      width: 100%;
    }
  }
</style>

<!-- 知识库页：文档管理与词法检索两个页签。 -->
<script setup lang="ts">
  import { computed, onMounted, ref, watch } from 'vue'
  import {
    ElButton,
    ElCard,
    ElEmpty,
    ElMessage,
    ElMessageBox,
    ElOption,
    ElPagination,
    ElSelect,
    ElTable,
    ElTableColumn
  } from 'element-plus'
  import {
    deleteKnowledgeDocument,
    getKnowledgeStats,
    listKnowledgeDocuments,
    reindexKnowledgeDocument,
    type NWKnowledgeStatsVo
  } from '@/api/knowledge'
  import type { NWKnowledgeDocumentVo } from '@/api/contracts'
  import { usePageQuery } from '@/composables/usePageQuery'
  import { useProjectStore } from '@/store/modules/project'
  import { formatTime } from '@/utils/time'
  import ChunkDialog from './components/ChunkDialog.vue'
  import KnowledgeSearchPanel from './components/KnowledgeSearchPanel.vue'
  import KnowledgeUpload from './components/KnowledgeUpload.vue'
  import PreviewDrawer from './components/PreviewDrawer.vue'
  import { asRow } from '@/utils/table'

  const projectStore = useProjectStore()
  const projectUlid = computed(() => projectStore.currentProjectUlid)

  const { items, total, pageNum, pageSize, loading, query, load, search, changePage } = usePageQuery(
    listKnowledgeDocuments,
    { project_ulid: '', doc_status_code: '' }
  )

  const stats = ref<NWKnowledgeStatsVo>({
    total_count: 0,
    indexed_count: 0,
    pending_count: 0,
    failed_count: 0,
    chunk_count: 0
  })
  const statsLoading = ref(false)

  const previewDrawer = ref<InstanceType<typeof PreviewDrawer> | null>(null)
  const chunkDialog = ref<InstanceType<typeof ChunkDialog> | null>(null)

  /** 文档状态选项（与后端枚举一致） */
  const statusOptions = [
    { value: '', label: '全部状态' },
    { value: 'pending', label: '待处理' },
    { value: 'indexing', label: '索引中' },
    { value: 'indexed', label: '已索引' },
    { value: 'failed', label: '失败' }
  ]

  /** 状态展示文案 */
  const statusLabel = (statusCode: string) =>
    statusOptions.find((item) => item.value === statusCode)?.label ?? (statusCode || '未知')

  /** 状态徽标配色 */
  const statusBadge = (statusCode: string) => {
    if (statusCode === 'indexed') return 'success'
    if (statusCode === 'failed') return 'danger'
    if (statusCode === 'indexing') return 'warning'
    return 'neutral'
  }

  /** 统计卡片配置（标签 + 取值） */
  const statCards = computed(() => [
    { label: '文档总数', value: stats.value.total_count },
    { label: '已索引', value: stats.value.indexed_count },
    { label: '待处理', value: stats.value.pending_count },
    { label: '索引失败', value: stats.value.failed_count },
    { label: '切片总数', value: stats.value.chunk_count }
  ])

  const loadStats = async () => {
    if (!projectUlid.value) return
    statsLoading.value = true
    try {
      stats.value = await getKnowledgeStats(projectUlid.value)
    } catch {
      stats.value = {
        total_count: 0,
        indexed_count: 0,
        pending_count: 0,
        failed_count: 0,
        chunk_count: 0
      }
    } finally {
      statsLoading.value = false
    }
  }

  const refreshAll = () => Promise.all([load(), loadStats()])

  const handleReindex = async (row: NWKnowledgeDocumentVo) => {
    await reindexKnowledgeDocument(row.doc_ulid)
    ElMessage.success('已重新投递索引任务')
    await refreshAll()
  }

  const handleDelete = async (row: NWKnowledgeDocumentVo) => {
    try {
      await ElMessageBox.confirm(
        `将删除「${row.doc_name}」及其切片与原文，删除后不可恢复。`,
        '删除文档',
        { type: 'warning' }
      )
    } catch {
      return
    }
    await deleteKnowledgeDocument(row.doc_ulid)
    ElMessage.success('已删除')
    await refreshAll()
  }

  const openPreview = (row: NWKnowledgeDocumentVo) => {
    void previewDrawer.value?.open(row)
  }

  const openChunks = (row: NWKnowledgeDocumentVo) => {
    void chunkDialog.value?.open({ docUlid: row.doc_ulid, docName: row.doc_name })
  }

  /** 重索引仅在已索引/失败状态可用 */
  const canReindex = (statusCode: string) => statusCode === 'indexed' || statusCode === 'failed'

  watch(
    projectUlid,
    async (nextUlid) => {
      query.value.project_ulid = nextUlid
      if (!nextUlid) {
        items.value = []
        total.value = 0
        return
      }
      await refreshAll()
    },
    { immediate: true }
  )

  onMounted(async () => {
    if (!projectStore.projectOptions.length) {
      await projectStore.loadProjectOptions()
    }
  })
</script>

<template>
  <div class="nw-page">
    <div class="nw-toolbar">
      <ElSelect v-model="query.doc_status_code" class="knowledge__status" @change="search()">
        <ElOption
          v-for="item in statusOptions"
          :key="item.value"
          :label="item.label"
          :value="item.value"
        />
      </ElSelect>
      <ElButton size="small" plain :loading="loading" @click="refreshAll">
        <Icon icon="mdi:reload" :size="14" />
        <span class="ml-4px">刷新</span>
      </ElButton>
      <KnowledgeUpload :project-ulid="projectUlid" @uploaded="refreshAll" />
      <div class="knowledge__spacer" />
      <span class="nw-hint">共 {{ total }} 个文档</span>
    </div>

    <div v-loading="statsLoading" class="knowledge__stats">
      <ElCard v-for="card in statCards" :key="card.label" shadow="never" class="knowledge__stat">
        <span class="knowledge__stat-label">{{ card.label }}</span>
        <strong class="knowledge__stat-value">{{ card.value.toLocaleString() }}</strong>
      </ElCard>
    </div>

    <ElCard shadow="never" class="knowledge__table-card">
      <ElTable :data="items" v-loading="loading" size="small" row-key="doc_ulid">
        <ElTableColumn prop="doc_name" label="文档名" min-width="200" show-overflow-tooltip />
        <ElTableColumn label="类型" width="80">
          <template #default="{ row }">
            <span class="nw-badge nw-badge--neutral">{{ row.doc_type_code || '—' }}</span>
          </template>
        </ElTableColumn>
        <ElTableColumn label="状态" width="100">
          <template #default="{ row }">
            <span class="nw-badge" :class="`nw-badge--${statusBadge(row.doc_status_code)}`">
              {{ statusLabel(row.doc_status_code) }}
            </span>
          </template>
        </ElTableColumn>
        <ElTableColumn prop="chunk_count" label="切片" width="80" align="right" />
        <ElTableColumn label="Tokens" width="100" align="right">
          <template #default="{ row }">{{ row.total_token_count.toLocaleString() }}</template>
        </ElTableColumn>
        <ElTableColumn label="上传时间" width="170">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </ElTableColumn>
        <ElTableColumn label="操作" width="250" fixed="right">
          <template #default="{ row }">
            <ElButton size="small" text type="primary" @click="openPreview(asRow<NWKnowledgeDocumentVo>(row))">预览</ElButton>
            <ElButton size="small" text @click="openChunks(asRow<NWKnowledgeDocumentVo>(row))">切片</ElButton>
            <ElButton
              size="small"
              text
              :disabled="!canReindex(asRow<NWKnowledgeDocumentVo>(row).doc_status_code)"
              @click="handleReindex(asRow<NWKnowledgeDocumentVo>(row))"
            >
              重索引
            </ElButton>
            <ElButton size="small" text type="danger" @click="handleDelete(asRow<NWKnowledgeDocumentVo>(row))">删除</ElButton>
          </template>
        </ElTableColumn>
        <template #empty>
          <ElEmpty
            :description="projectUlid ? '还没有文档，上传 txt / md / docx / pdf 开始构建知识库' : '请先在顶栏选择项目'"
            :image-size="80"
          />
        </template>
      </ElTable>

      <div v-if="total > pageSize" class="knowledge__pager">
        <ElPagination
          :current-page="pageNum"
          :page-size="pageSize"
          :total="total"
          layout="prev, pager, next"
          background
          @current-change="changePage"
        />
      </div>
    </ElCard>

    <KnowledgeSearchPanel :project-ulid="projectUlid" />

    <PreviewDrawer ref="previewDrawer" />
    <ChunkDialog ref="chunkDialog" />
  </div>
</template>

<style lang="less" scoped>
  .knowledge {
    &__status {
      width: 140px;
    }

    &__spacer {
      flex: 1;
    }

    &__stats {
      display: grid;
      gap: 14px;
      grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
    }

    &__stat {
      :deep(.el-card__body) {
        display: flex;
        flex-direction: column;
        gap: 6px;
        padding: 14px 16px;
      }
    }

    &__stat-label {
      font-size: 12px;
      color: var(--el-text-color-secondary);
    }

    &__stat-value {
      font-size: 24px;
      font-weight: 700;
      letter-spacing: -0.02em;
      color: var(--nw-ink);
      font-variant-numeric: tabular-nums;
    }

    &__table-card {
      :deep(.el-card__body) {
        padding: 6px 10px 12px;
      }
    }

    &__pager {
      display: flex;
      justify-content: flex-end;
      padding-top: 12px;
    }
  }
</style>

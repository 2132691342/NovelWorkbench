<!--
  知识库：文档库（上传/索引状态/预览/切片）与检索测试两个页签。
  文档处理中自动轮询；上传走抽屉，避免挤压列表空间。
-->
<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import {
  deleteKnowledgeDocument,
  getKnowledgeStats,
  listKnowledgeDocuments,
  reindexKnowledgeDocument,
  type NWKnowledgeStatsVo,
} from '@/api/knowledge'
import type { NWKnowledgeDocumentVo } from '@/api/contracts'
import NWButton from '@/components/NWButton.vue'
import NWEmpty from '@/components/NWEmpty.vue'
import NWIcon from '@/components/NWIcon.vue'
import NWPageHeader from '@/components/NWPageHeader.vue'
import NWStatTile from '@/components/NWStatTile.vue'
import ProjectPageHeader from '@/components/ProjectPageHeader.vue'
import ChunkListDialog from './ChunkListDialog.vue'
import KnowledgePreviewDrawer from './KnowledgePreviewDrawer.vue'
import KnowledgeSearchPanel from './KnowledgeSearchPanel.vue'
import KnowledgeUploadPanel from './KnowledgeUploadPanel.vue'

const route = useRoute()
const { t } = useI18n()

const projectUlid = computed(() => String(route.params.project_ulid ?? ''))

/** 轮询间隔（毫秒） */
const NW_POLL_INTERVAL_MS = 3000

const activeTab = ref<'documents' | 'search'>('documents')
const docList = ref<NWKnowledgeDocumentVo[]>([])
const listLoading = ref(false)
const total = ref(0)
const pageNum = ref(1)
const pageSize = ref(10)
const statusFilter = ref('')
const uploadDrawerVisible = ref(false)
const previewVisible = ref(false)
const previewDoc = ref<NWKnowledgeDocumentVo | null>(null)
const chunkDialogVisible = ref(false)
const chunkDoc = ref<NWKnowledgeDocumentVo | null>(null)
const statsVo = ref<NWKnowledgeStatsVo>({
  total_count: 0,
  indexed_count: 0,
  pending_count: 0,
  failed_count: 0,
  chunk_count: 0,
})
const statsLoading = ref(false)

let pollTimer: number | null = null

/** 文档状态筛选 */
const statusFilterList = computed(() => [
  { value: '', label: t('knowledge.statusAll') },
  { value: 'pending', label: t('knowledge.statusPending') },
  { value: 'indexing', label: t('knowledge.statusIndexing') },
  { value: 'indexed', label: t('knowledge.statusIndexed') },
  { value: 'failed', label: t('knowledge.statusFailed') },
])

/** 状态文案 */
function statusText(statusCode: string): string {
  switch (statusCode) {
    case 'pending':
      return t('knowledge.statusPending')
    case 'indexing':
      return t('knowledge.statusIndexing')
    case 'indexed':
      return t('knowledge.statusIndexed')
    case 'failed':
      return t('knowledge.statusFailed')
    default:
      return statusCode
  }
}

function statusBadgeClass(statusCode: string): string {
  switch (statusCode) {
    case 'indexed':
      return 'nw-badge--success'
    case 'indexing':
      return 'nw-badge--warning'
    case 'failed':
      return 'nw-badge--danger'
    default:
      return 'nw-badge--neutral'
  }
}

function docTypeIcon(docTypeCode: string): string {
  if (docTypeCode === 'pdf') return 'fileText'
  if (docTypeCode === 'docx') return 'fileText'
  return 'note'
}

function formatTime(timeMs: number): string {
  return new Date(timeMs).toLocaleString()
}

/** 加载文档分页 */
async function loadDocuments(): Promise<void> {
  if (!projectUlid.value || listLoading.value) {
    return
  }
  listLoading.value = true
  try {
    const pageData = await listKnowledgeDocuments({
      project_ulid: projectUlid.value,
      doc_status_code: statusFilter.value || undefined,
      page_num: pageNum.value,
      page_size: pageSize.value,
    })
    docList.value = pageData.items ?? []
    total.value = pageData.total ?? 0
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    listLoading.value = false
    syncPolling()
  }
}

/** 加载概览 */
async function loadStats(): Promise<void> {
  if (!projectUlid.value) return
  statsLoading.value = true
  try {
    statsVo.value = await getKnowledgeStats(projectUlid.value)
  } catch {
    // 概览失败不阻断列表
  } finally {
    statsLoading.value = false
  }
}

/** 有待处理/索引中文档时启动轮询 */
function syncPolling(): void {
  const needPolling = docList.value.some(
    (item) => item.doc_status_code === 'pending' || item.doc_status_code === 'indexing',
  )
  if (needPolling && pollTimer === null) {
    pollTimer = window.setInterval(() => {
      void loadDocuments()
      void loadStats()
    }, NW_POLL_INTERVAL_MS)
  } else if (!needPolling && pollTimer !== null) {
    window.clearInterval(pollTimer)
    pollTimer = null
  }
}

function stopPolling(): void {
  if (pollTimer !== null) {
    window.clearInterval(pollTimer)
    pollTimer = null
  }
}

function handleUploaded(): void {
  pageNum.value = 1
  void loadDocuments()
  void loadStats()
}

function handlePreview(documentVo: NWKnowledgeDocumentVo): void {
  previewDoc.value = documentVo
  previewVisible.value = true
}

function handleOpenChunks(documentVo: NWKnowledgeDocumentVo): void {
  chunkDoc.value = documentVo
  chunkDialogVisible.value = true
}

async function handleReindex(documentVo: NWKnowledgeDocumentVo): Promise<void> {
  try {
    await reindexKnowledgeDocument(documentVo.doc_ulid)
    ElMessage.success(t('knowledge.reindexSuccess'))
    await loadDocuments()
    await loadStats()
  } catch {
    // 失败提示由 http 层统一弹出
  }
}

async function handleDelete(documentVo: NWKnowledgeDocumentVo): Promise<void> {
  try {
    await ElMessageBox.confirm(t('knowledge.deleteConfirm'), t('common.confirm'), {
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel'),
      type: 'warning',
    })
  } catch {
    return
  }
  try {
    await deleteKnowledgeDocument(documentVo.doc_ulid)
    ElMessage.success(t('knowledge.deleteSuccess'))
    if (docList.value.length === 1 && pageNum.value > 1) {
      pageNum.value -= 1
    }
    await loadDocuments()
    await loadStats()
  } catch {
    // 失败提示由 http 层统一弹出
  }
}

function handleStatusChange(statusCode: string): void {
  statusFilter.value = statusCode
  pageNum.value = 1
  void loadDocuments()
}

function handlePageChange(page: number): void {
  pageNum.value = page
  void loadDocuments()
}

function handleSizeChange(size: number): void {
  pageSize.value = size
  pageNum.value = 1
  void loadDocuments()
}

watch(projectUlid, () => {
  pageNum.value = 1
  statusFilter.value = ''
  void loadDocuments()
  void loadStats()
})

onMounted(() => {
  void loadDocuments()
  void loadStats()
})

onUnmounted(() => {
  stopPolling()
})
</script>

<template>
  <div class="nw-page knowledge-page">
    <ProjectPageHeader :project-ulid="projectUlid" />

    <NWPageHeader
      :title_text="t('knowledge.title')"
      :subtitle_text="t('knowledge.pageSubtitle')"
      icon_code="database"
    >
      <template #actions>
        <NWButton @click="loadDocuments(); loadStats()">
          <NWIcon icon_code="refresh" :size_px="15" />
          {{ t('common.refresh') }}
        </NWButton>
        <NWButton type="primary" @click="uploadDrawerVisible = true">
          <NWIcon icon_code="upload" :size_px="15" />
          {{ t('knowledge.upload.action') }}
        </NWButton>
      </template>
    </NWPageHeader>

    <div class="nw-stat-grid">
      <NWStatTile
        :label_text="t('knowledge.statTotal')"
        :value_number="statsVo.total_count"
        :hint_text="t('knowledge.statTotalHint')"
        icon_code="bookOpen"
        tone_code="primary"
      />
      <NWStatTile
        :label_text="t('knowledge.statIndexed')"
        :value_number="statsVo.indexed_count"
        :hint_text="t('knowledge.statIndexedHint')"
        icon_code="checkCircle"
        tone_code="success"
      />
      <NWStatTile
        :label_text="t('knowledge.statPending')"
        :value_number="statsVo.pending_count"
        :hint_text="t('knowledge.statPendingHint')"
        icon_code="clock"
        tone_code="warning"
      />
      <NWStatTile
        :label_text="t('knowledge.statChunks')"
        :value_number="statsVo.chunk_count"
        :hint_text="t('knowledge.statChunksHint')"
        icon_code="layers"
        tone_code="info"
      />
    </div>

    <section class="nw-panel">
      <div class="knowledge-page__tabs">
        <button
          type="button"
          class="knowledge-page__tab"
          :class="{ 'is-active': activeTab === 'documents' }"
          @click="activeTab = 'documents'"
        >
          <NWIcon icon_code="fileText" :size_px="15" />
          {{ t('knowledge.tabDocuments') }}
          <span class="knowledge-page__tab-count">{{ total }}</span>
        </button>
        <button
          type="button"
          class="knowledge-page__tab"
          :class="{ 'is-active': activeTab === 'search' }"
          @click="activeTab = 'search'"
        >
          <NWIcon icon_code="search" :size_px="15" />
          {{ t('knowledge.tabSearch') }}
        </button>
      </div>

      <!-- 文档库 -->
      <div v-if="activeTab === 'documents'" class="nw-panel__body">
        <div class="knowledge-page__toolbar">
          <div class="knowledge-page__status-tabs">
            <button
              v-for="statusOption in statusFilterList"
              :key="statusOption.value"
              type="button"
              class="knowledge-page__status-tab"
              :class="{ 'is-active': statusFilter === statusOption.value }"
              @click="handleStatusChange(statusOption.value)"
            >
              {{ statusOption.label }}
            </button>
          </div>
          <span class="nw-toolbar__spacer" />
          <span class="knowledge-page__poll-hint">
            <template v-if="statsVo.pending_count > 0">
              <span class="knowledge-page__poll-dot" />
              {{ t('knowledge.pollingHint', { count: statsVo.pending_count }) }}
            </template>
          </span>
        </div>

        <NWEmpty
          v-if="!listLoading && docList.length === 0"
          icon_code="folderOpen"
          :title_text="t('knowledge.emptyTitle')"
          :description_text="t('knowledge.emptyDesc')"
        >
          <template #actions>
            <NWButton type="primary" @click="uploadDrawerVisible = true">
              <NWIcon icon_code="upload" :size_px="14" />
              {{ t('knowledge.upload.action') }}
            </NWButton>
          </template>
        </NWEmpty>
        <el-table v-else v-loading="listLoading" :data="docList" stripe class="knowledge-page__table">
          <el-table-column :label="t('knowledge.docName')" min-width="260">
            <template #default="{ row }">
              <div class="knowledge-page__doc-cell">
                <span class="knowledge-page__doc-icon">
                  <NWIcon :icon_code="docTypeIcon(row.doc_type_code)" :size_px="16" />
                </span>
                <div class="knowledge-page__doc-text">
                  <span class="knowledge-page__doc-name" :title="row.doc_name">{{ row.doc_name }}</span>
                  <span class="knowledge-page__doc-type">{{ row.doc_type_code.toUpperCase() }}</span>
                </div>
              </div>
            </template>
          </el-table-column>
          <el-table-column :label="t('knowledge.docStatus')" width="120">
            <template #default="{ row }">
              <el-tooltip :content="row.error_message" :disabled="!row.error_message" placement="top">
                <span class="nw-badge" :class="statusBadgeClass(row.doc_status_code)">
                  <span class="nw-badge__dot" />
                  {{ statusText(row.doc_status_code) }}
                </span>
              </el-tooltip>
            </template>
          </el-table-column>
          <el-table-column :label="t('knowledge.chunkCount')" width="90" align="right">
            <template #default="{ row }">{{ row.chunk_count.toLocaleString() }}</template>
          </el-table-column>
          <el-table-column :label="t('knowledge.tokenCount')" width="110" align="right">
            <template #default="{ row }">{{ row.total_token_count.toLocaleString() }}</template>
          </el-table-column>
          <el-table-column :label="t('knowledge.createdAt')" width="170">
            <template #default="{ row }">{{ formatTime(row.created_at_ms) }}</template>
          </el-table-column>
          <el-table-column :label="t('common.action')" width="190" fixed="right">
            <template #default="{ row }">
              <div class="knowledge-page__row-actions">
                <button type="button" class="knowledge-page__text-btn" @click="handlePreview(row)">
                  {{ t('knowledge.preview.action') }}
                </button>
                <button type="button" class="knowledge-page__text-btn" @click="handleOpenChunks(row)">
                  {{ t('knowledge.chunk.action') }}
                </button>
                <el-dropdown trigger="click">
                  <button type="button" class="knowledge-page__text-btn is-icon" :aria-label="t('common.action')">
                    <NWIcon icon_code="more" :size_px="15" />
                  </button>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item
                        v-if="row.doc_status_code === 'indexed' || row.doc_status_code === 'failed'"
                        @click="handleReindex(row)"
                      >
                        {{ t('knowledge.reindex') }}
                      </el-dropdown-item>
                      <el-dropdown-item divided @click="handleDelete(row)">
                        <span class="knowledge-page__danger-text">{{ t('common.delete') }}</span>
                      </el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
              </div>
            </template>
          </el-table-column>
          <template #empty>
            <NWEmpty :description_text="t('knowledge.emptyTip')" />
          </template>
        </el-table>

        <el-pagination
          v-if="total > 0"
          class="knowledge-page__pagination"
          layout="total, prev, pager, next, sizes"
          :total="total"
          :current-page="pageNum"
          :page-size="pageSize"
          :page-sizes="[10, 20, 50]"
          background
          @current-change="handlePageChange"
          @size-change="handleSizeChange"
        />
      </div>

      <!-- 检索测试 -->
      <div v-else class="nw-panel__body">
        <KnowledgeSearchPanel :project-ulid="projectUlid" />
      </div>
    </section>

    <!-- 上传抽屉 -->
    <el-drawer v-model="uploadDrawerVisible" :title="t('knowledge.upload.action')" size="500px">
      <p class="knowledge-page__upload-intro">{{ t('knowledge.upload.drawerIntro') }}</p>
      <KnowledgeUploadPanel :project-ulid="projectUlid" @uploaded="handleUploaded" />
      <div class="knowledge-page__upload-tips">
        <h4>{{ t('knowledge.upload.tipsTitle') }}</h4>
        <ul>
          <li>{{ t('knowledge.upload.tipFormat') }}</li>
          <li>{{ t('knowledge.upload.tipChunk') }}</li>
          <li>{{ t('knowledge.upload.tipRag') }}</li>
        </ul>
      </div>
      <template #footer>
        <div class="knowledge-page__drawer-footer">
          <NWButton @click="uploadDrawerVisible = false">{{ t('knowledge.upload.done') }}</NWButton>
        </div>
      </template>
    </el-drawer>

    <ChunkListDialog
      v-model:visible="chunkDialogVisible"
      :doc-ulid="chunkDoc?.doc_ulid ?? ''"
      :doc-name="chunkDoc?.doc_name ?? ''"
    />
    <KnowledgePreviewDrawer v-model:visible="previewVisible" :document-vo="previewDoc" />
  </div>
</template>

<style scoped src="./knowledge.css"></style>

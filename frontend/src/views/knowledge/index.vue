<!--
  知识库页：上传区 + 文档表格（状态徽标/重新索引/删除）+ 切片弹窗 + 预览抽屉 + 检索区。
  索引中/待处理文档按 3s 轮询刷新，离开页面自动停止。
-->
<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox, ElTooltip } from 'element-plus'
import { useI18n } from 'vue-i18n'
import {
  deleteKnowledgeDocument,
  listKnowledgeDocuments,
  reindexKnowledgeDocument,
} from '@/api/knowledge'
import type { NWKnowledgeDocumentVo } from '@/api/contracts'
import NWButton from '@/components/NWButton.vue'
import NWCard from '@/components/NWCard.vue'
import NWEmpty from '@/components/NWEmpty.vue'
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

const docList = ref<NWKnowledgeDocumentVo[]>([])
const listLoading = ref(false)
const total = ref(0)
const pageNum = ref(1)
const pageSize = ref(10)
const previewVisible = ref(false)
const previewDoc = ref<NWKnowledgeDocumentVo | null>(null)
const chunkDialogVisible = ref(false)
const chunkDoc = ref<NWKnowledgeDocumentVo | null>(null)

let pollTimer: number | null = null

/** 状态徽标文案 */
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

/** 毫秒时间戳转本地时间 */
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
      page_num: pageNum.value,
      page_size: pageSize.value,
    })
    docList.value = pageData.items ?? []
    total.value = pageData.total ?? 0
  } catch {
    // 失败提示由 http 层统一弹出（静默），保持当前列表
  } finally {
    listLoading.value = false
    syncPolling()
  }
}

/** 有待处理/索引中文档时启动轮询，否则停止 */
function syncPolling(): void {
  const needPolling = docList.value.some(
    (item) => item.doc_status_code === 'pending' || item.doc_status_code === 'indexing',
  )
  if (needPolling && pollTimer === null) {
    pollTimer = window.setInterval(() => {
      void loadDocuments()
    }, NW_POLL_INTERVAL_MS)
  } else if (!needPolling && pollTimer !== null) {
    window.clearInterval(pollTimer)
    pollTimer = null
  }
}

/** 停止轮询 */
function stopPolling(): void {
  if (pollTimer !== null) {
    window.clearInterval(pollTimer)
    pollTimer = null
  }
}

/** 上传成功：回到首页刷新 */
function handleUploaded(): void {
  pageNum.value = 1
  void loadDocuments()
}

/** 打开预览抽屉 */
function handlePreview(documentVo: NWKnowledgeDocumentVo): void {
  previewDoc.value = documentVo
  previewVisible.value = true
}

/** 打开切片列表弹窗 */
function handleOpenChunks(documentVo: NWKnowledgeDocumentVo): void {
  chunkDoc.value = documentVo
  chunkDialogVisible.value = true
}

/** 重新索引（仅 indexed/failed 可重投） */
async function handleReindex(documentVo: NWKnowledgeDocumentVo): Promise<void> {
  try {
    await reindexKnowledgeDocument(documentVo.doc_ulid)
    ElMessage.success(t('knowledge.reindexSuccess'))
    await loadDocuments()
  } catch {
    // 失败提示由 http 层统一弹出
  }
}

/** 删除文档（二次确认，连带删原文/切片/索引） */
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
  } catch {
    // 失败提示由 http 层统一弹出
  }
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
  void loadDocuments()
})

onMounted(() => {
  void loadDocuments()
})

onUnmounted(() => {
  stopPolling()
})
</script>

<template>
  <div class="knowledge-page">
    <ProjectPageHeader :project-ulid="projectUlid" />

    <!-- 上传区 -->
    <NWCard :title="t('knowledge.upload.title')" :hoverable="false">
      <KnowledgeUploadPanel :project-ulid="projectUlid" @uploaded="handleUploaded" />
    </NWCard>

    <!-- 文档表格 -->
    <NWCard :title="t('knowledge.docListTitle')" :hoverable="false">
      <NWEmpty v-if="!listLoading && docList.length === 0" :description_text="t('knowledge.emptyTip')" />
      <template v-else>
        <el-table :data="docList" v-loading="listLoading" stripe>
          <el-table-column :label="t('knowledge.docName')" min-width="200">
            <template #default="{ row }">
              <span class="knowledge-page__doc-name" :title="row.doc_name">{{ row.doc_name }}</span>
              <span class="knowledge-page__doc-type">{{ row.doc_type_code }}</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('knowledge.docStatus')" width="110">
            <template #default="{ row }">
              <el-tooltip
                :content="row.error_message"
                :disabled="!row.error_message"
                placement="top"
              >
                <span class="knowledge-page__status" :class="`is-${row.doc_status_code}`">
                  {{ statusText(row.doc_status_code) }}
                </span>
              </el-tooltip>
            </template>
          </el-table-column>
          <el-table-column :label="t('knowledge.chunkCount')" prop="chunk_count" width="90" />
          <el-table-column :label="t('knowledge.tokenCount')" prop="total_token_count" width="110" />
          <el-table-column :label="t('knowledge.createdAt')" width="170">
            <template #default="{ row }">
              {{ formatTime(row.created_at_ms) }}
            </template>
          </el-table-column>
          <el-table-column :label="t('common.action')" width="260" fixed="right">
            <template #default="{ row }">
              <NWButton size="small" @click="handlePreview(row)">{{ t('knowledge.preview.action') }}</NWButton>
              <NWButton size="small" @click="handleOpenChunks(row)">{{ t('knowledge.chunk.action') }}</NWButton>
              <NWButton
                v-if="row.doc_status_code === 'indexed' || row.doc_status_code === 'failed'"
                size="small"
                @click="handleReindex(row)"
              >
                {{ t('knowledge.reindex') }}
              </NWButton>
              <NWButton size="small" type="danger" @click="handleDelete(row)">
                {{ t('common.delete') }}
              </NWButton>
            </template>
          </el-table-column>
        </el-table>
        <el-pagination
          v-if="total > 0"
          class="knowledge-page__pagination"
          layout="total, prev, pager, next, sizes"
          :total="total"
          :current-page="pageNum"
          :page-size="pageSize"
          :page-sizes="[10, 20, 50]"
          @current-change="handlePageChange"
          @size-change="handleSizeChange"
        />
      </template>
    </NWCard>

    <!-- 检索区 -->
    <NWCard :title="t('knowledge.search.title')" :hoverable="false">
      <KnowledgeSearchPanel :project-ulid="projectUlid" />
    </NWCard>

    <KnowledgePreviewDrawer
      v-model:visible="previewVisible"
      :document-vo="previewDoc"
    />
    <ChunkListDialog
      v-model:visible="chunkDialogVisible"
      :doc-ulid="chunkDoc?.doc_ulid ?? ''"
      :doc-name="chunkDoc?.doc_name ?? ''"
    />
  </div>
</template>

<style scoped src="./knowledge.css"></style>

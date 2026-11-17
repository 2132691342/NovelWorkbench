<!--
  文档切片列表弹窗：GET /knowledge/chunk/list 分页，展示切片序号与文本。
-->
<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { listKnowledgeChunks } from '@/api/knowledge'
import type { NWKnowledgeChunkVo } from '@/api/contracts'
import NWEmpty from '@/components/NWEmpty.vue'
import NWLoading from '@/components/NWLoading.vue'

const props = defineProps<{
  visible: boolean
  docUlid: string
  docName: string
}>()

const emit = defineEmits<{
  'update:visible': [visible: boolean]
}>()

const { t } = useI18n()

const loading = ref(false)
const chunkList = ref<NWKnowledgeChunkVo[]>([])
const pageNum = ref(1)
const pageSize = ref(10)
const total = ref(0)

/** 加载当前页切片 */
async function loadChunks(): Promise<void> {
  if (!props.docUlid) {
    chunkList.value = []
    total.value = 0
    return
  }
  loading.value = true
  try {
    const pageData = await listKnowledgeChunks({
      docUlid: props.docUlid,
      pageNum: pageNum.value,
      pageSize: pageSize.value,
    })
    chunkList.value = pageData.items ?? []
    total.value = pageData.total ?? 0
  } catch {
    // 失败提示由 http 层统一弹出
    chunkList.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function handlePageChange(page: number): void {
  pageNum.value = page
  void loadChunks()
}

function handleSizeChange(size: number): void {
  pageSize.value = size
  pageNum.value = 1
  void loadChunks()
}

function handleClose(): void {
  emit('update:visible', false)
}

watch(
  () => props.visible,
  (visibleFlag) => {
    if (visibleFlag) {
      pageNum.value = 1
      void loadChunks()
    }
  },
)
</script>

<template>
  <el-dialog
    :model-value="visible"
    :title="t('knowledge.chunk.title', { name: docName })"
    width="680px"
    @close="handleClose"
    @update:model-value="handleClose"
  >
    <NWLoading v-if="loading" />
    <NWEmpty
      v-else-if="chunkList.length === 0"
      :description_text="t('knowledge.chunk.empty')"
    />
    <div v-else class="chunk-dialog__list">
      <article v-for="chunkVo in chunkList" :key="chunkVo.chunk_ulid" class="chunk-dialog__item">
        <div class="chunk-dialog__item-head">
          <span class="chunk-dialog__index">#{{ chunkVo.chunk_index }}</span>
        </div>
        <p class="chunk-dialog__text">{{ chunkVo.chunk_text }}</p>
      </article>
    </div>
    <el-pagination
      v-if="total > 0"
      class="chunk-dialog__pagination"
      layout="total, prev, pager, next, sizes"
      :total="total"
      :current-page="pageNum"
      :page-size="pageSize"
      :page-sizes="[10, 20, 50]"
      @current-change="handlePageChange"
      @size-change="handleSizeChange"
    />
  </el-dialog>
</template>

<style scoped>
.chunk-dialog__list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  max-height: 56vh;
  overflow-y: auto;
}

.chunk-dialog__item {
  padding: 12px 14px;
  background-color: var(--bg-page);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-base);
}

.chunk-dialog__item-head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 6px;
}

.chunk-dialog__index {
  font-size: 12px;
  font-weight: 600;
  color: var(--primary);
}

.chunk-dialog__text {
  margin: 0;
  font-size: 13px;
  line-height: 1.7;
  color: var(--text-secondary);
  white-space: pre-wrap;
  word-break: break-word;
}

.chunk-dialog__pagination {
  margin-top: 14px;
  justify-content: flex-end;
}
</style>

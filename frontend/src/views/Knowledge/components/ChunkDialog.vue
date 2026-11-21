<!-- 切片弹窗：展示文档的切片明细。 -->
<script setup lang="ts">
  import { ref } from 'vue'
  import { ElButton, ElDialog, ElEmpty, ElPagination, ElTag } from 'element-plus'
  import { listKnowledgeChunks } from '@/api/knowledge'
  import type { NWKnowledgeChunkVo } from '@/api/contracts'

  const visible = ref(false)
  const docUlid = ref('')
  const docName = ref('')
  const chunkList = ref<NWKnowledgeChunkVo[]>([])
  const total = ref(0)
  const pageNum = ref(1)
  const pageSize = ref(10)
  const loading = ref(false)

  const loadChunks = async () => {
    if (!docUlid.value) return
    loading.value = true
    try {
      const page = await listKnowledgeChunks({
        docUlid: docUlid.value,
        pageNum: pageNum.value,
        pageSize: pageSize.value
      })
      chunkList.value = page.items ?? []
      total.value = page.total ?? 0
    } catch {
      chunkList.value = []
      total.value = 0
    } finally {
      loading.value = false
    }
  }

  /** 打开弹窗：切换文档时重置到第一页 */
  const open = async (payload: { docUlid: string; docName: string }) => {
    docUlid.value = payload.docUlid
    docName.value = payload.docName
    pageNum.value = 1
    chunkList.value = []
    total.value = 0
    visible.value = true
    await loadChunks()
  }

  const changePage = async (nextPageNum: number) => {
    pageNum.value = nextPageNum
    await loadChunks()
  }

  defineExpose({ open })
</script>

<template>
  <ElDialog v-model="visible" :title="`切片预览 · ${docName}`" width="720px" append-to-body>
    <div v-loading="loading" class="chunk-dialog">
      <div v-for="chunk in chunkList" :key="chunk.chunk_ulid" class="chunk-dialog__item">
        <div class="chunk-dialog__head">
          <ElTag size="small" type="info">#{{ chunk.chunk_index + 1 }}</ElTag>
          <span class="chunk-dialog__ulid">{{ chunk.chunk_ulid }}</span>
        </div>
        <p class="chunk-dialog__text">{{ chunk.chunk_text }}</p>
      </div>

      <ElEmpty
        v-if="!loading && chunkList.length === 0"
        description="该文档还没有切片，索引完成后可查看"
        :image-size="70"
      />
    </div>

    <div v-if="total > pageSize" class="chunk-dialog__pager">
      <ElPagination
        :current-page="pageNum"
        :page-size="pageSize"
        :total="total"
        layout="prev, pager, next"
        background
        small
        @current-change="changePage"
      />
    </div>

    <template #footer>
      <ElButton size="small" @click="visible = false">关闭</ElButton>
    </template>
  </ElDialog>
</template>

<style lang="less" scoped>
  .chunk-dialog {
    display: flex;
    flex-direction: column;
    gap: 10px;
    max-height: 460px;
    overflow-y: auto;

    &__item {
      padding: 10px 12px;
      border: 1px solid var(--nw-border-soft);
      border-radius: var(--nw-radius-md);
      background: var(--nw-surface-subtle);
    }

    &__head {
      display: flex;
      gap: 8px;
      align-items: center;
      margin-bottom: 6px;
    }

    &__ulid {
      font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
      font-size: 11px;
      color: var(--nw-text-muted);
    }

    &__text {
      font-size: 12.5px;
      line-height: 1.7;
      color: var(--el-text-color-regular);
      white-space: pre-wrap;
      word-break: break-word;
    }

    &__pager {
      display: flex;
      justify-content: flex-end;
      margin-top: 12px;
    }
  }
</style>

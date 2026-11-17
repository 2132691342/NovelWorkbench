<!--
  文档预览抽屉：txt/md 纯文本 <pre>（切片文本），docx 走 docx-preview，pdf 走 vue-pdf-embed。
  原文经 GET /knowledge/content 获取；取不到原文时按合同仅展示失败提示，不提供假下载按钮。
--->
<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { renderAsync } from 'docx-preview'
import VuePdfEmbed from 'vue-pdf-embed'
import { listKnowledgeChunks, fetchKnowledgeContent } from '@/api/knowledge'
import type { NWKnowledgeDocumentVo } from '@/api/contracts'
import NWButton from '@/components/NWButton.vue'
import NWLoading from '@/components/NWLoading.vue'

const props = defineProps<{
  visible: boolean
  documentVo: NWKnowledgeDocumentVo | null
}>()

const emit = defineEmits<{
  'update:visible': [visible: boolean]
}>()

const { t } = useI18n()

/** 预览形态：docx / pdf / text（txt、md 纯文本） */
type NWPreviewKind = 'docx' | 'pdf' | 'text' | 'unknown'

/** 文本预览最多拉取的切片数 */
const NW_TEXT_PREVIEW_CHUNK_LIMIT = 500

const previewKind = computed<NWPreviewKind>(() => {
  switch (props.documentVo?.doc_type_code) {
    case 'docx':
      return 'docx'
    case 'pdf':
      return 'pdf'
    case 'txt':
    case 'md':
      return 'text'
    default:
      return 'unknown'
  }
})

const previewLoading = ref(false)
const previewError = ref(false)
const previewText = ref('')
const previewBlobUrl = ref('')
const docxHost = ref<HTMLDivElement | null>(null)

/** 读取文档原文二进制（docx/pdf 预览用，失败返回 null 走失败提示） */
async function loadOriginalBlob(): Promise<Blob | null> {
  try {
    if (!props.documentVo?.doc_ulid) {
      return null
    }
    return await fetchKnowledgeContent(props.documentVo.doc_ulid)
  } catch {
    return null
  }
}

/** 文本预览：按切片序号拼接索引文本 */
async function loadTextPreview(docUlid: string): Promise<string> {
  const chunkTextList: string[] = []
  const pageSize = 100
  let pageNum = 1
  while (chunkTextList.length < NW_TEXT_PREVIEW_CHUNK_LIMIT) {
    const pageData = await listKnowledgeChunks({ docUlid, pageNum, pageSize })
    const items = pageData.items ?? []
    if (items.length === 0) {
      break
    }
    for (const chunkVo of items) {
      chunkTextList.push(chunkVo.chunk_text)
    }
    if (chunkTextList.length >= (pageData.total ?? 0)) {
      break
    }
    pageNum += 1
  }
  return chunkTextList.join('\n\n')
}

/** 清理上一次预览产物（含 docx-preview 注入的 DOM 与 pdf 预览 URL） */
function cleanupPreview(): void {
  previewText.value = ''
  previewError.value = false
  if (previewBlobUrl.value) {
    URL.revokeObjectURL(previewBlobUrl.value)
    previewBlobUrl.value = ''
  }
  if (docxHost.value) {
    docxHost.value.innerHTML = ''
  }
}

/** 按文档类型加载并渲染预览；失败仅提示 */
async function loadPreview(): Promise<void> {
  cleanupPreview()
  const docUlid = props.documentVo?.doc_ulid ?? ''
  if (!docUlid) {
    return
  }
  previewLoading.value = true
  try {
    if (previewKind.value === 'text') {
      previewText.value = await loadTextPreview(docUlid)
      previewError.value = previewText.value.length === 0
      return
    }
    const blob = await loadOriginalBlob()
    if (!blob) {
      previewError.value = true
      return
    }
    if (previewKind.value === 'docx') {
      await nextTick()
      if (!docxHost.value) {
        previewError.value = true
        return
      }
      await renderAsync(blob, docxHost.value, undefined, { inWrapper: true })
      return
    }
    if (previewKind.value === 'pdf') {
      previewBlobUrl.value = URL.createObjectURL(blob)
      return
    }
    previewError.value = true
  } catch {
    // 渲染失败按合同仅提示
    previewError.value = true
  } finally {
    previewLoading.value = false
  }
}

function handleClose(): void {
  emit('update:visible', false)
}

watch(
  () => props.visible,
  (visibleFlag) => {
    if (visibleFlag) {
      void loadPreview()
    } else {
      cleanupPreview()
    }
  },
)
</script>

<template>
  <el-drawer
    :model-value="visible"
    :title="t('knowledge.preview.title', { name: documentVo?.doc_name ?? '' })"
    size="60%"
    @close="handleClose"
    @update:model-value="handleClose"
  >
    <NWLoading v-if="previewLoading" />
    <template v-else-if="previewError">
      <div class="preview-drawer__error">
        <p class="preview-drawer__error-text">{{ t('knowledge.preview.failedTip') }}</p>
      </div>
    </template>
    <template v-else>
      <!-- txt / md：纯文本预览 -->
      <pre v-if="previewKind === 'text'" class="preview-drawer__text">{{ previewText }}</pre>
      <!-- docx：docx-preview 渲染容器 -->
      <div v-else-if="previewKind === 'docx'" ref="docxHost" class="preview-drawer__docx" />
      <!-- pdf：vue-pdf-embed 渲染 -->
      <VuePdfEmbed
        v-else-if="previewKind === 'pdf' && previewBlobUrl"
        class="preview-drawer__pdf"
        :src="previewBlobUrl"
      />
    </template>
    <template #footer>
      <NWButton @click="handleClose">{{ t('common.cancel') }}</NWButton>
    </template>
  </el-drawer>
</template>

<style scoped>
.preview-drawer__text {
  margin: 0;
  padding: 16px;
  font-size: 13px;
  line-height: 1.8;
  color: var(--text-main);
  background-color: var(--bg-page);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-base);
  white-space: pre-wrap;
  word-break: break-word;
}

.preview-drawer__docx {
  overflow: auto;
  border: 1px solid var(--border-light);
  border-radius: var(--radius-base);
}

.preview-drawer__pdf {
  width: 100%;
}

.preview-drawer__error {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 48px 0;
}

.preview-drawer__error-text {
  margin: 0;
  font-size: 13px;
  color: var(--text-placeholder);
}
</style>

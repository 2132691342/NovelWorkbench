<!--
  文档预览抽屉：宽抽屉（72%）+ 工具栏（字号缩放、原文下载）。
  txt/md 直接渲染（md 走 marked），docx 走 docx-preview，pdf 走 vue-pdf-embed；取不到原文时明确提示。
-->
<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import { renderAsync } from 'docx-preview'
import VuePdfEmbed from 'vue-pdf-embed'
import { listKnowledgeChunks, fetchKnowledgeContent } from '@/api/knowledge'
import type { NWKnowledgeDocumentVo } from '@/api/contracts'
import NWButton from '@/components/NWButton.vue'
import NWIcon from '@/components/NWIcon.vue'
import NWLoading from '@/components/NWLoading.vue'

const props = defineProps<{
  visible: boolean
  documentVo: NWKnowledgeDocumentVo | null
}>()

const emit = defineEmits<{
  'update:visible': [visible: boolean]
}>()

const { t } = useI18n()

/** 预览形态：docx / pdf / markdown / text / unknown */
type NWPreviewKind = 'docx' | 'pdf' | 'markdown' | 'text' | 'unknown'

/** 文本预览最多拉取的切片数 */
const NW_TEXT_PREVIEW_CHUNK_LIMIT = 500
/** 字号缩放档位 */
const NW_FONT_SCALE_LIST = [0.85, 1, 1.15, 1.3]

const previewKind = computed<NWPreviewKind>(() => {
  switch (props.documentVo?.doc_type_code) {
    case 'docx':
      return 'docx'
    case 'pdf':
      return 'pdf'
    case 'md':
      return 'markdown'
    case 'txt':
      return 'text'
    default:
      return 'unknown'
  }
})

const previewLoading = ref(false)
const previewError = ref(false)
const previewText = ref('')
const previewHtml = ref('')
const previewBlobUrl = ref('')
const docxHost = ref<HTMLDivElement | null>(null)
const fontScale = ref(1)

/** 缩放档位下标（决定按钮禁用态） */
const fontScaleIndex = computed(() => NW_FONT_SCALE_LIST.indexOf(fontScale.value))

/** 正文渲染样式（字号随档位缩放，行高保持可读） */
const previewBodyStyle = computed(() => ({
  fontSize: `${Math.round(14 * fontScale.value)}px`,
  lineHeight: String(1.85),
}))

/** 读取文档原文二进制（docx/pdf 预览与下载用） */
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

/** 文本兜底：按切片序号拼接索引文本 */
async function loadChunkText(docUlid: string): Promise<string> {
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

/** 清理上一次预览产物（docx 注入的 DOM 与 pdf blob URL） */
function cleanupPreview(): void {
  previewText.value = ''
  previewHtml.value = ''
  previewError.value = false
  if (previewBlobUrl.value) {
    URL.revokeObjectURL(previewBlobUrl.value)
    previewBlobUrl.value = ''
  }
  if (docxHost.value) {
    docxHost.value.innerHTML = ''
  }
}

/** 按文档类型加载并渲染预览 */
async function loadPreview(): Promise<void> {
  cleanupPreview()
  fontScale.value = 1
  const docUlid = props.documentVo?.doc_ulid ?? ''
  if (!docUlid) {
    return
  }
  previewLoading.value = true
  try {
    if (previewKind.value === 'text' || previewKind.value === 'markdown') {
      const blob = await loadOriginalBlob()
      const rawText = blob ? await blob.text() : ''
      previewText.value = rawText || (await loadChunkText(docUlid))
      if (previewKind.value === 'markdown') {
        previewHtml.value = previewText.value
          ? String(marked.parse(previewText.value, { async: false }))
          : ''
      }
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
    previewError.value = true
  } finally {
    previewLoading.value = false
  }
}

/** 字号放大/缩小 */
function handleZoom(delta: number): void {
  const nextIndex = Math.min(
    NW_FONT_SCALE_LIST.length - 1,
    Math.max(0, fontScaleIndex.value + delta),
  )
  fontScale.value = NW_FONT_SCALE_LIST[nextIndex]
}

/** 下载原文（真实取回二进制后落盘，失败明确提示） */
async function handleDownload(): Promise<void> {
  if (!props.documentVo) {
    return
  }
  const blob = await loadOriginalBlob()
  if (!blob) {
    ElMessage.error(t('knowledge.preview.downloadFailed'))
    return
  }
  const downloadUrl = URL.createObjectURL(blob)
  const linkElement = document.createElement('a')
  linkElement.href = downloadUrl
  linkElement.download = props.documentVo.doc_name
  linkElement.click()
  URL.revokeObjectURL(downloadUrl)
  ElMessage.success(t('knowledge.preview.downloadSuccess'))
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
    :title="documentVo?.doc_name ?? ''"
    size="72%"
    class="preview-drawer"
    @close="handleClose"
    @update:model-value="handleClose"
  >
    <template #header>
      <div class="preview-drawer__header">
        <div class="preview-drawer__title-wrap">
          <span class="preview-drawer__title">{{ documentVo?.doc_name ?? '' }}</span>
          <span v-if="documentVo" class="preview-drawer__type">{{ documentVo.doc_type_code }}</span>
        </div>
        <div class="preview-drawer__tools">
          <el-tooltip :content="t('knowledge.preview.zoomOut')" placement="bottom">
            <button
              type="button"
              class="preview-drawer__tool"
              :disabled="fontScaleIndex === 0"
              @click="handleZoom(-1)"
            >
              <NWIcon icon_code="minus" :size_px="14" />
            </button>
          </el-tooltip>
          <span class="preview-drawer__scale">{{ Math.round(fontScale * 100) }}%</span>
          <el-tooltip :content="t('knowledge.preview.zoomIn')" placement="bottom">
            <button
              type="button"
              class="preview-drawer__tool"
              :disabled="fontScaleIndex === NW_FONT_SCALE_LIST.length - 1"
              @click="handleZoom(1)"
            >
              <NWIcon icon_code="plus" :size_px="14" />
            </button>
          </el-tooltip>
          <span class="preview-drawer__divider" />
          <NWButton size="small" @click="handleDownload">
            <NWIcon icon_code="download" :size_px="13" />
            {{ t('knowledge.preview.download') }}
          </NWButton>
        </div>
      </div>
    </template>

    <NWLoading v-if="previewLoading" />
    <div v-else-if="previewError" class="preview-drawer__error">
      <NWIcon icon_code="fileWarning" :size_px="28" />
      <p class="preview-drawer__error-text">{{ t('knowledge.preview.failedTip') }}</p>
      <NWButton size="small" @click="loadPreview">{{ t('common.refresh') }}</NWButton>
    </div>
    <div v-else class="preview-drawer__body" :style="previewBodyStyle">
      <!-- md：渲染后的富文本 -->
      <article
        v-if="previewKind === 'markdown'"
        class="preview-drawer__markdown"
        v-html="previewHtml"
      />
      <!-- txt：等宽纯文本 -->
      <pre v-else-if="previewKind === 'text'" class="preview-drawer__text">{{ previewText }}</pre>
      <!-- docx：docx-preview 渲染容器 -->
      <div v-else-if="previewKind === 'docx'" ref="docxHost" class="preview-drawer__docx" />
      <!-- pdf：vue-pdf-embed 渲染 -->
      <VuePdfEmbed
        v-else-if="previewKind === 'pdf' && previewBlobUrl"
        class="preview-drawer__pdf"
        :src="previewBlobUrl"
      />
    </div>
  </el-drawer>
</template>

<style scoped src="./previewDrawer.css"></style>

<!-- 原文预览抽屉：按格式渲染 docx、pdf、md 与 txt。 -->
<script setup lang="ts">
  import { nextTick, onBeforeUnmount, ref, shallowRef } from 'vue'
  import { ElAlert, ElButton, ElDrawer, ElSkeleton } from 'element-plus'
  import { fetchKnowledgeContent } from '@/api/knowledge'
  import type { NWKnowledgeDocumentVo } from '@/api/contracts'

  type PreviewMode = 'markdown' | 'text' | 'docx' | 'pdf' | 'unsupported'
  type PdfEmbedComponent = typeof import('vue-pdf-embed')['default']

  const visible = ref(false)
  const loading = ref(false)
  const failedFlag = ref(false)
  const docName = ref('')
  const previewMode = ref<PreviewMode>('unsupported')
  const docTypeCode = ref('')
  const textContent = ref('')
  const markdownHtml = ref('')
  const pdfSource = ref('')
  const pdfComponent = shallowRef<PdfEmbedComponent | null>(null)
  const docxHost = ref<HTMLDivElement | null>(null)

  let objectUrl = ''

  const releaseObjectUrl = () => {
    if (objectUrl) {
      URL.revokeObjectURL(objectUrl)
      objectUrl = ''
    }
  }

  const resetState = () => {
    releaseObjectUrl()
    loading.value = false
    failedFlag.value = false
    textContent.value = ''
    markdownHtml.value = ''
    pdfSource.value = ''
    pdfComponent.value = null
    previewMode.value = 'unsupported'
  }

  /** markdown 渲染（marked 动态引入，避免首屏加载解析器） */
  const renderMarkdown = async (rawText: string) => {
    const { marked } = await import('marked')
    markdownHtml.value = await marked.parse(rawText)
  }

  /** docx 渲染：docx-preview 直接输出到容器 */
  const renderDocx = async (blob: Blob) => {
    await nextTick()
    const host = docxHost.value
    if (!host) return
    const { renderAsync } = await import('docx-preview')
    host.innerHTML = ''
    await renderAsync(blob, host, undefined, { inWrapper: true, ignoreWidth: false })
  }

  /** pdf 渲染：vue-pdf-embed 读取对象 URL */
  const renderPdf = async (blob: Blob) => {
    const module = await import('vue-pdf-embed')
    pdfComponent.value = module.default
    objectUrl = URL.createObjectURL(blob)
    pdfSource.value = objectUrl
  }

  /** 打开抽屉并加载原文（按类型分流预览方式） */
  const open = async (doc: NWKnowledgeDocumentVo) => {
    resetState()
    docName.value = doc.doc_name
    docTypeCode.value = doc.doc_type_code
    visible.value = true
    loading.value = true
    try {
      const blob = await fetchKnowledgeContent(doc.doc_ulid)
      if (doc.doc_type_code === 'md') {
        previewMode.value = 'markdown'
        await renderMarkdown(await blob.text())
      } else if (doc.doc_type_code === 'txt') {
        previewMode.value = 'text'
        textContent.value = await blob.text()
      } else if (doc.doc_type_code === 'docx') {
        previewMode.value = 'docx'
        await renderDocx(blob)
      } else if (doc.doc_type_code === 'pdf') {
        previewMode.value = 'pdf'
        await renderPdf(blob)
      } else {
        previewMode.value = 'unsupported'
      }
    } catch {
      failedFlag.value = true
    } finally {
      loading.value = false
    }
  }

  onBeforeUnmount(releaseObjectUrl)

  defineExpose({ open })
</script>

<template>
  <ElDrawer v-model="visible" :title="`原文预览 · ${docName}`" size="62%" @closed="resetState">
    <div v-loading="loading" class="preview">
      <ElSkeleton v-if="loading" :rows="8" animated />

      <ElAlert
        v-else-if="failedFlag"
        type="error"
        :closable="false"
        title="原文读取失败"
        description="文档可能尚未完成存储，请稍后重试。"
        show-icon
      />

      <ElAlert
        v-else-if="previewMode === 'unsupported'"
        type="warning"
        :closable="false"
        :title="`暂不支持在线预览 ${docTypeCode || '该类型'}`"
        description="可下载原文后用本地应用打开；知识库切片检索不受影响。"
        show-icon
      />

      <div v-else-if="previewMode === 'markdown'" class="preview__markdown" v-html="markdownHtml"></div>

      <pre v-else-if="previewMode === 'text'" class="preview__text">{{ textContent }}</pre>

      <div v-else-if="previewMode === 'docx'" ref="docxHost" class="preview__docx"></div>

      <div v-else-if="previewMode === 'pdf'" class="preview__pdf">
        <component :is="pdfComponent" v-if="pdfComponent" :source="pdfSource" />
      </div>
    </div>

    <template #footer>
      <ElButton size="small" @click="visible = false">关闭</ElButton>
    </template>
  </ElDrawer>
</template>

<style lang="less" scoped>
  .preview {
    min-height: 200px;
    color: var(--el-text-color-regular);

    &__markdown {
      font-size: 13.5px;
      line-height: 1.8;

      :deep(h1),
      :deep(h2),
      :deep(h3) {
        margin: 16px 0 8px;
        color: var(--nw-ink);
      }

      :deep(p) {
        margin: 8px 0;
      }

      :deep(code) {
        padding: 1px 5px;
        font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
        font-size: 12px;
        border-radius: var(--nw-radius-sm);
        background: var(--nw-surface-subtle);
      }

      :deep(pre) {
        padding: 10px 12px;
        overflow-x: auto;
        border-radius: var(--nw-radius-md);
        background: var(--nw-surface-subtle);
      }

      :deep(blockquote) {
        padding-left: 12px;
        margin: 10px 0;
        border-left: 3px solid var(--nw-primary-light);
        color: var(--nw-text-secondary);
      }

      :deep(table) {
        border-collapse: collapse;

        th,
        td {
          padding: 4px 10px;
          border: 1px solid var(--nw-border);
        }
      }
    }

    &__text {
      font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
      font-size: 12.5px;
      line-height: 1.75;
      color: var(--el-text-color-regular);
      white-space: pre-wrap;
      word-break: break-word;
    }

    &__docx {
      :deep(.docx-wrapper) {
        padding: 0;
        background: transparent;
      }

      :deep(.docx-wrapper > section.docx) {
        margin-bottom: 12px;
        box-shadow: var(--nw-shadow-card);
      }
    }

    &__pdf {
      display: flex;
      flex-direction: column;
      gap: 12px;
      align-items: center;

      :deep(.vue-pdf-embed) {
        max-width: 100%;
        box-shadow: var(--nw-shadow-card);
      }
    }
  }
</style>

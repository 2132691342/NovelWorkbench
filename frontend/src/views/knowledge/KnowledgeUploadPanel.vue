<!--
  知识库上传区：拖拽 / 点击选择文件，multipart 上传（POST /knowledge/upload）。
  展示大小上限提示与上传进度；仅支持 txt/md/pdf/docx。
-->
<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { uploadKnowledgeDocument } from '@/api/knowledge'
import type { NWKnowledgeDocumentVo } from '@/api/contracts'

const props = defineProps<{
  projectUlid: string
}>()

const emit = defineEmits<{
  uploaded: [documentVo: NWKnowledgeDocumentVo]
}>()

const { t } = useI18n()

/** 上传大小上限（MB，与后端 upload_max_mb 配置一致） */
const NW_UPLOAD_MAX_MB = 20

/** 允许的扩展名 */
const acceptExtensions = ['txt', 'md', 'pdf', 'docx']

const fileInputRef = ref<HTMLInputElement | null>(null)
const draggingFlag = ref(false)
const uploadingFlag = ref(false)
const uploadPercent = ref(0)

function handlePickClick(): void {
  if (uploadingFlag.value) {
    return
  }
  fileInputRef.value?.click()
}

function handleFileChange(event: Event): void {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (file) {
    void uploadFile(file)
  }
}

function handleDragOver(event: DragEvent): void {
  event.preventDefault()
  draggingFlag.value = true
}

function handleDragLeave(): void {
  draggingFlag.value = false
}

function handleDrop(event: DragEvent): void {
  event.preventDefault()
  draggingFlag.value = false
  const file = event.dataTransfer?.files?.[0]
  if (file && !uploadingFlag.value) {
    void uploadFile(file)
  }
}

/** 校验并上传单个文件 */
async function uploadFile(file: File): Promise<void> {
  const extension = file.name.split('.').pop()?.toLowerCase() ?? ''
  if (!acceptExtensions.includes(extension)) {
    ElMessage.warning(t('knowledge.upload.invalidType'))
    return
  }
  if (file.size > NW_UPLOAD_MAX_MB << 20) {
    ElMessage.warning(t('knowledge.upload.tooLarge', { size: NW_UPLOAD_MAX_MB }))
    return
  }
  uploadingFlag.value = true
  uploadPercent.value = 0
  try {
    const documentVo = await uploadKnowledgeDocument({
      file,
      projectUlid: props.projectUlid,
      onUploadProgress: (percent) => {
        uploadPercent.value = percent
      },
    })
    ElMessage.success(t('knowledge.upload.success'))
    emit('uploaded', documentVo)
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    uploadingFlag.value = false
    uploadPercent.value = 0
  }
}
</script>

<template>
  <div
    class="knowledge-upload"
    :class="{ 'is-dragging': draggingFlag, 'is-uploading': uploadingFlag }"
    @click="handlePickClick"
    @dragover="handleDragOver"
    @dragleave="handleDragLeave"
    @drop="handleDrop"
  >
    <input
      ref="fileInputRef"
      type="file"
      class="knowledge-upload__input"
      :accept="acceptExtensions.map((ext) => `.${ext}`).join(',')"
      @change="handleFileChange"
    />
    <template v-if="uploadingFlag">
      <el-progress class="knowledge-upload__progress" :percentage="uploadPercent" />
      <span class="knowledge-upload__hint">{{ t('knowledge.upload.uploading') }}</span>
    </template>
    <template v-else>
      <span class="knowledge-upload__title">{{ t('knowledge.upload.title') }}</span>
      <span class="knowledge-upload__hint">
        {{ t('knowledge.upload.hint', { types: acceptExtensions.join(' / '), size: NW_UPLOAD_MAX_MB }) }}
      </span>
    </template>
  </div>
</template>

<style scoped>
.knowledge-upload {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 24px;
  border: 2px dashed var(--border);
  border-radius: var(--radius-lg);
  cursor: pointer;
  transition: border-color var(--transition-base), background-color var(--transition-base);
  color: var(--text-secondary);
}

.knowledge-upload:hover,
.knowledge-upload.is-dragging {
  border-color: var(--primary);
  background-color: var(--primary-light);
}

.knowledge-upload.is-uploading {
  cursor: default;
  border-color: var(--primary);
}

.knowledge-upload__input {
  display: none;
}

.knowledge-upload__title {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-main);
}

.knowledge-upload__hint {
  font-size: 12px;
  color: var(--text-placeholder);
}

.knowledge-upload__progress {
  width: 60%;
}
</style>

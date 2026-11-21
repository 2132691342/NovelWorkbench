<!-- 上传组件：选择文档并提交索引任务。 -->
<script setup lang="ts">
  import { ref } from 'vue'
  import { ElButton, ElMessage, ElProgress, ElUpload, type UploadRequestOptions } from 'element-plus'
  import { uploadKnowledgeDocument } from '@/api/knowledge'

  const props = defineProps<{ projectUlid: string }>()
  const emit = defineEmits<{ uploaded: [] }>()

  /** 允许上传的扩展名（与后端白名单一致） */
  const ALLOWED_EXT_LIST = ['txt', 'md', 'docx', 'pdf']
  /** 单文件大小上限（字节） */
  const MAX_FILE_SIZE = 20 * 1024 * 1024

  const uploading = ref(false)
  const percent = ref(0)

  const extOf = (fileName: string) => fileName.split('.').pop()?.toLowerCase() ?? ''

  /** 上传前校验：类型白名单 + 大小上限 */
  const beforeUpload = (file: File) => {
    if (!ALLOWED_EXT_LIST.includes(extOf(file.name))) {
      ElMessage.warning('仅支持 txt / md / docx / pdf 格式')
      return false
    }
    if (file.size > MAX_FILE_SIZE) {
      ElMessage.warning('文件不能超过 20MB')
      return false
    }
    return true
  }

  /** 自定义上传：走业务接口并回传进度 */
  const handleUpload = async (options: UploadRequestOptions) => {
    if (!props.projectUlid) {
      ElMessage.warning('请先选择项目')
      return
    }
    uploading.value = true
    percent.value = 0
    try {
      await uploadKnowledgeDocument({
        file: options.file,
        projectUlid: props.projectUlid,
        onUploadProgress: (value) => {
          percent.value = value
        }
      })
      percent.value = 100
      ElMessage.success('上传成功，文档正在后台解析')
      emit('uploaded')
    } catch {
      // 失败提示由请求层统一弹出，这里只复位进度
      percent.value = 0
    } finally {
      uploading.value = false
    }
  }
</script>

<template>
  <div class="knowledge-upload">
    <ElUpload
      :show-file-list="false"
      :before-upload="beforeUpload"
      :http-request="handleUpload"
      accept=".txt,.md,.docx,.pdf"
      :disabled="uploading || !projectUlid"
    >
      <ElButton type="primary" plain size="small" :loading="uploading">
        <Icon icon="mdi:plus" :size="14" />
        <span class="knowledge-upload__label">上传文档</span>
      </ElButton>
    </ElUpload>
    <ElProgress
      v-if="uploading"
      :percentage="percent"
      :stroke-width="4"
      class="knowledge-upload__progress"
    />
  </div>
</template>

<style lang="less" scoped>
  .knowledge-upload {
    display: flex;
    gap: 10px;
    align-items: center;

    &__label {
      margin-left: 4px;
    }

    &__progress {
      width: 120px;
    }
  }
</style>

<!-- 档案预览弹窗：只读展示提示词全文。 -->
<script setup lang="ts">
  import { computed } from 'vue'
  import { ElButton, ElDialog, ElTag } from 'element-plus'
  import type { NWPromptProfileVo } from '@/api/contracts'
  import { formatTime } from '@/utils/time'

  const props = defineProps<{ profile: NWPromptProfileVo | null }>()
  const visible = defineModel<boolean>('visible', { required: true })

  const title = computed(() =>
    props.profile ? `提示词预览 · ${props.profile.prompt_profile_name}` : '提示词预览'
  )

  const charCount = computed(() => props.profile?.system_prompt_text.length ?? 0)
</script>

<template>
  <ElDialog v-model="visible" :title="title" width="720px" append-to-body destroy-on-close>
    <div v-if="profile" class="preview">
      <div class="preview__meta">
        <ElTag size="small" effect="plain">{{ profile.prompt_profile_code }}</ElTag>
        <ElTag v-if="profile.domain_name" size="small" type="success" effect="plain">
          {{ profile.domain_name }}
        </ElTag>
        <span class="nw-hint">{{ charCount }} 字符 · 更新于 {{ formatTime(profile.updated_at) }}</span>
      </div>
      <pre class="preview__body">{{ profile.system_prompt_text }}</pre>
    </div>
    <template #footer>
      <ElButton @click="visible = false">关闭</ElButton>
    </template>
  </ElDialog>
</template>

<style lang="less" scoped>
  .preview {
    display: flex;
    flex-direction: column;
    gap: 10px;

    &__meta {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      align-items: center;
    }

    &__body {
      max-height: 60vh;
      padding: 14px;
      overflow: auto;
      font-family: var(--el-font-family-mono, monospace);
      font-size: 12px;
      line-height: 1.8;
      color: var(--nw-ink);
      white-space: pre-wrap;
      word-break: break-word;
      background: var(--nw-surface-subtle);
      border: 1px solid var(--nw-border-soft);
      border-radius: var(--nw-radius-md);
    }
  }
</style>

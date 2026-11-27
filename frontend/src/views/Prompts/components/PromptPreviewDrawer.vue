<!-- 提示词预览抽屉：完整正文 + 复制，占位符说明按需展示。 -->
<script setup lang="ts">
  import { computed } from 'vue'
  import { ElButton, ElDrawer, ElMessage, ElTag } from 'element-plus'
  import type { NWPromptVo } from '@/api/contracts'
  import { formatTime } from '@/utils/time'

  const props = defineProps<{ prompt: NWPromptVo | null }>()
  const visible = defineModel<boolean>('visible', { required: true })

  const title = computed(() =>
    props.prompt ? `提示词预览 · ${props.prompt.prompt_name}` : '提示词预览'
  )

  const charCount = computed(() => props.prompt?.prompt_text.length ?? 0)
  const kindLabel = computed(() => (props.prompt?.prompt_kind_code === 'system' ? '身份' : '指令'))

  /** 复制完整正文：作者常要贴到别处对照或改写 */
  const copyText = async () => {
    if (!props.prompt) return
    try {
      await navigator.clipboard.writeText(props.prompt.prompt_text)
      ElMessage.success('提示词正文已复制')
    } catch {
      ElMessage.warning('复制失败，请手动选中正文复制')
    }
  }
</script>

<template>
  <ElDrawer v-model="visible" :title="title" size="620px" append-to-body destroy-on-close>
    <div v-if="prompt" class="preview">
      <div class="preview__meta">
        <ElTag size="small" effect="plain">
          {{ prompt.prompt_kind_code === 'command' ? '/' : '' }}{{ prompt.prompt_code }}
        </ElTag>
        <ElTag size="small" type="success" effect="plain">{{ kindLabel }}</ElTag>
        <ElTag v-if="prompt.domain_name" size="small" type="info" effect="plain">
          {{ prompt.domain_name }}
        </ElTag>
        <span class="nw-hint">{{ charCount }} 字符 · 更新于 {{ formatTime(prompt.updated_at) }}</span>
      </div>
      <p class="preview__when">
        <strong>什么时候用：</strong>{{ prompt.description_text || '未填写使用场景' }}
      </p>
      <pre class="preview__body">{{ prompt.prompt_text }}</pre>
      <p class="nw-hint" v-pre>
        注入方式：以系统级消息进入消息列表；用户输入始终是用户级消息。可用占位符
        <code>{{context}}</code>、<code>{{chapter_content}}</code>、<code>{{user_input}}</code>。
      </p>
    </div>
    <template #footer>
      <div class="preview__footer">
        <ElButton plain @click="copyText">复制正文</ElButton>
        <ElButton type="primary" plain @click="visible = false">关闭</ElButton>
      </div>
    </template>
  </ElDrawer>
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

    &__when {
      margin: 0;
      font-size: 12.5px;
      line-height: 1.7;
      color: var(--nw-text-secondary);
    }

    &__body {
      max-height: 62vh;
      padding: 14px;
      overflow: auto;
      font-family: var(--nw-font-mono);
      font-size: 12px;
      line-height: 1.8;
      color: var(--nw-ink);
      white-space: pre-wrap;
      word-break: break-word;
      background: var(--nw-surface-subtle);
      border: 1px solid var(--nw-border-soft);
      border-radius: var(--nw-radius-md);
    }

    &__footer {
      display: flex;
      gap: 8px;
      justify-content: flex-end;
    }
  }
</style>

<!-- 笔记编辑器：双栏编辑预览与防抖自动保存。 -->
<script setup lang="ts">
  import { computed, onBeforeUnmount, ref, watch } from 'vue'
  import { ElButton, ElEmpty, ElInput, ElRadioButton, ElRadioGroup } from 'element-plus'
  import { marked } from 'marked'
  import { NW_LORE_TYPE_NOTE, saveLore } from '@/api/lore'
  import type { NWLoreVo } from '@/api/lore'

  type SaveStatus = 'idle' | 'saving' | 'saved' | 'failed'
  type NoteModeCode = 'rich' | 'markdown'

  const props = defineProps<{ note: NWLoreVo | null; projectUlid: string }>()

  const emit = defineEmits<{ saved: [lore: NWLoreVo] }>()

  /** 自动保存防抖时长（毫秒） */
  const SAVE_DEBOUNCE_MS = 2000

  const titleText = ref('')
  const contentText = ref('')
  const modeCode = ref<NoteModeCode>('markdown')
  const saveStatus = ref<SaveStatus>('saved')
  const dirtyFlag = ref(false)

  let saveTimer: number | null = null
  /** 内容版本号：保存期间再次编辑时避免误判为已保存 */
  let revision = 0

  const statusText = computed(() => {
    if (saveStatus.value === 'saving') return '保存中…'
    if (saveStatus.value === 'failed') return '保存失败'
    if (dirtyFlag.value) return '未保存'
    return '已保存'
  })

  const statusBadge = computed(() => {
    if (saveStatus.value === 'failed') return 'danger'
    if (saveStatus.value === 'saving' || dirtyFlag.value) return 'warning'
    return 'success'
  })

  /** 预览：Markdown 走 marked，HTML 模式直接渲染原文 */
  const previewHtml = computed(() => {
    if (!contentText.value.trim()) return ''
    if (modeCode.value === 'rich') return contentText.value
    const rendered = marked.parse(contentText.value, { async: false })
    return typeof rendered === 'string' ? rendered : ''
  })

  /** 切换内容格式：Markdown 原文 / 富文本 HTML（后端编码为 rich） */
  const handleModeChange = (value: string | number | boolean | undefined) => {
    modeCode.value = value === 'rich' ? 'rich' : 'markdown'
    scheduleSave()
  }

  const cancelTimer = () => {
    if (saveTimer !== null) {
      window.clearTimeout(saveTimer)
      saveTimer = null
    }
  }

  const startTimer = () => {
    cancelTimer()
    saveTimer = window.setTimeout(() => {
      saveTimer = null
      void saveNow()
    }, SAVE_DEBOUNCE_MS)
  }

  /** 内容变化：标记脏状态并重置防抖计时 */
  const scheduleSave = () => {
    revision += 1
    dirtyFlag.value = true
    if (saveStatus.value !== 'saving') saveStatus.value = 'idle'
    startTimer()
  }

  /** 立即落盘当前内容 */
  const saveNow = async () => {
    const loreUlid = props.note?.lore_ulid
    if (!loreUlid || !dirtyFlag.value) return
    cancelTimer()
    const startRevision = revision
    saveStatus.value = 'saving'
    try {
      const savedLore = await saveLore({
        lore_ulid: loreUlid,
        project_ulid: props.projectUlid,
        lore_name: titleText.value.trim() || '未命名笔记',
        lore_type_code: NW_LORE_TYPE_NOTE,
        content_text: contentText.value,
        content_mode_code: modeCode.value,
        sort_order: props.note?.sort_order ?? 0
      })
      if (revision === startRevision) {
        dirtyFlag.value = false
        saveStatus.value = 'saved'
      } else {
        saveStatus.value = 'idle'
        startTimer()
      }
      emit('saved', savedLore)
    } catch {
      saveStatus.value = 'failed'
    }
  }

  /** 切换笔记/离开页面前落盘未保存内容 */
  const flushSave = async () => {
    if (!dirtyFlag.value) return
    await saveNow()
  }

  defineExpose({ flushSave })

  /** 仅在切换到另一篇笔记时重置编辑器，避免保存回写覆盖正在输入的内容 */
  watch(
    () => props.note?.lore_ulid,
    () => {
      cancelTimer()
      revision = 0
      dirtyFlag.value = false
      saveStatus.value = 'saved'
      titleText.value = props.note?.lore_name ?? ''
      contentText.value = props.note?.content_text ?? ''
      modeCode.value = props.note?.content_mode_code === 'rich' ? 'rich' : 'markdown'
    },
    { immediate: true }
  )

  onBeforeUnmount(() => {
    cancelTimer()
    void saveNow()
  })
</script>

<template>
  <section class="note-editor">
    <ElEmpty v-if="!note" description="从左侧选择一篇笔记，或新建一篇" :image-size="90" />

    <template v-else>
      <header class="note-editor__head">
        <ElInput
          v-model="titleText"
          class="note-editor__title"
          placeholder="笔记标题"
          maxlength="128"
          @input="scheduleSave"
        />

        <ElRadioGroup :model-value="modeCode" size="small" @change="handleModeChange">
          <ElRadioButton value="markdown">Markdown</ElRadioButton>
          <ElRadioButton value="rich">HTML</ElRadioButton>
        </ElRadioGroup>

        <span class="nw-badge" :class="`nw-badge--${statusBadge}`">{{ statusText }}</span>
        <ElButton v-if="saveStatus === 'failed'" size="small" text @click="saveNow">
          <Icon icon="mdi:refresh" :size="13" />
          <span class="ml-4px">重试</span>
        </ElButton>
      </header>

      <div class="note-editor__body">
        <div class="note-editor__pane">
          <ElInput
            v-model="contentText"
            class="note-editor__input"
            type="textarea"
            resize="none"
            :placeholder="
              modeCode === 'markdown'
                ? '用 Markdown 记录灵感：# 标题、- 列表、**强调**'
                : '直接写 HTML 片段，例如 <p>段落</p>'
            "
            @input="scheduleSave"
          />
        </div>

        <div class="note-editor__pane is-preview">
          <div v-if="previewHtml" class="note-editor__preview" v-html="previewHtml"></div>
          <p v-else class="nw-hint">预览区：开始输入后实时渲染。</p>
        </div>
      </div>
    </template>
  </section>
</template>

<style lang="less" scoped>
  .note-editor {
    display: flex;
    flex: 1;
    flex-direction: column;
    min-width: 0;
    min-height: 0;

    &__head {
      display: flex;
      flex: none;
      gap: 10px;
      align-items: center;
      padding: 10px 14px;
      border-bottom: 1px solid var(--nw-border-soft);
    }

    &__title {
      flex: 1;
      min-width: 0;

      :deep(.el-input__wrapper) {
        box-shadow: none;
      }

      :deep(.el-input__inner) {
        font-size: 15px;
        font-weight: 650;
        color: var(--nw-ink);
      }
    }

    &__body {
      display: flex;
      flex: 1;
      min-height: 0;
    }

    &__pane {
      display: flex;
      flex: 1;
      min-width: 0;
      min-height: 0;
      padding: 12px 14px;
      overflow: hidden;

      &.is-preview {
        overflow-y: auto;
        background: var(--nw-surface-subtle);
        border-left: 1px solid var(--nw-border-soft);
      }
    }

    &__input {
      flex: 1;
      height: 100%;

      :deep(.el-textarea) {
        height: 100%;
      }

      :deep(.el-textarea__inner) {
        height: 100%;
        padding: 0;
        font-size: 13.5px;
        line-height: 1.85;
        color: var(--el-text-color-primary);
        background: transparent;
        border: none;
        box-shadow: none;
      }
    }

    &__preview {
      width: 100%;
      font-size: 13.5px;
      line-height: 1.85;
      color: var(--el-text-color-regular);
      word-break: break-word;

      :deep(h1),
      :deep(h2),
      :deep(h3) {
        margin: 12px 0 8px;
        font-weight: 700;
        color: var(--nw-ink);
      }

      :deep(h1) {
        font-size: 19px;
      }

      :deep(h2) {
        font-size: 17px;
      }

      :deep(h3) {
        font-size: 15px;
      }

      :deep(p) {
        margin: 6px 0;
      }

      :deep(ul),
      :deep(ol) {
        margin: 6px 0;
        padding-left: 20px;
      }

      :deep(li) {
        margin: 3px 0;
      }

      :deep(code) {
        padding: 1px 5px;
        font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
        font-size: 12.5px;
        background: var(--el-fill-color-light);
        border-radius: var(--nw-radius-sm);
      }

      :deep(pre) {
        padding: 10px 12px;
        overflow-x: auto;
        background: var(--el-fill-color-light);
        border-radius: var(--nw-radius-md);
      }

      :deep(blockquote) {
        margin: 8px 0;
        padding: 4px 12px;
        color: var(--nw-text-secondary);
        border-left: 3px solid var(--nw-primary-light);
      }

      :deep(a) {
        color: var(--el-color-primary);
      }

      :deep(table) {
        width: 100%;
        border-collapse: collapse;
      }

      :deep(th),
      :deep(td) {
        padding: 4px 8px;
        border: 1px solid var(--nw-border);
      }
    }
  }

  @media (width <= 900px) {
    .note-editor__body {
      flex-direction: column;
    }

    .note-editor__pane.is-preview {
      border-top: 1px solid var(--nw-border-soft);
      border-left: none;
    }
  }
</style>

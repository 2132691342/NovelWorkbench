<!-- 章节编辑器：CodeMirror 正文编辑与防抖自动保存。 -->
<script setup lang="ts">
  import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
  import { EditorState } from '@codemirror/state'
  import {
    EditorView,
    drawSelection,
    highlightActiveLine,
    highlightActiveLineGutter,
    keymap,
    lineNumbers
  } from '@codemirror/view'
  import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands'
  import { defaultHighlightStyle, syntaxHighlighting } from '@codemirror/language'
  import { markdown } from '@codemirror/lang-markdown'
  import { updateChapter } from '@/api/chapter'
  import type { NWChapterDetailVo } from '@/api/contracts'
  import { countWords } from './wordCount'

  const props = defineProps<{ chapter: NWChapterDetailVo | null }>()

  const emit = defineEmits<{
    'save-success': [
      payload: {
        chapter_ulid: string
        chapter_name: string
        word_count: number
        chapter_status: string
      }
    ]
    'word-count-change': [wordCount: number]
    'ai-continue': []
    'ai-polish': []
    'ai-summary': []
    'open-snapshot': []
  }>()

  /** 自动保存防抖时长（毫秒） */
  const SAVE_DEBOUNCE_MS = 2000

  type SaveStatus = 'idle' | 'saving' | 'saved' | 'failed'

  const editorHost = ref<HTMLDivElement | null>(null)
  const chapterName = ref('')
  const summaryText = ref('')
  const chapterStatus = ref('draft')
  const contentText = ref('')
  const saveStatus = ref<SaveStatus>('idle')
  const summaryCollapsed = ref(true)
  const dirtyFlag = ref(false)

  let editorView: EditorView | null = null
  let saveTimer: number | null = null
  let applyingContentFlag = false

  const liveWordCount = computed(() => countWords(contentText.value))

  const saveStatusText = computed(() => {
    if (saveStatus.value === 'saving') return '保存中…'
    if (saveStatus.value === 'saved') return '已保存'
    if (saveStatus.value === 'failed') return '保存失败'
    return dirtyFlag.value ? '未保存' : '已保存'
  })

  /** CodeMirror 主题：颜色全部走设计令牌，跟随明暗主题 */
  const editorTheme = EditorView.theme({
    '&': {
      height: '100%',
      backgroundColor: 'var(--nw-surface)',
      color: 'var(--nw-ink)',
      fontSize: '15px',
      lineHeight: '1.9'
    },
    '.cm-content': {
      maxWidth: '820px',
      padding: '20px 8px 80px',
      margin: '0 auto',
      fontFamily: "'PingFang SC', 'Microsoft YaHei', 'Helvetica Neue', Arial, sans-serif"
    },
    '.cm-cursor, .cm-dropCursor': { borderLeftColor: 'var(--el-color-primary)' },
    '&.cm-focused .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection': {
      backgroundColor: 'var(--nw-primary-light)'
    },
    '.cm-activeLine': { backgroundColor: 'var(--el-fill-color-light)' },
    '.cm-gutters': {
      backgroundColor: 'var(--nw-surface-subtle)',
      color: 'var(--el-text-color-placeholder)',
      border: 'none'
    },
    '.cm-activeLineGutter': {
      backgroundColor: 'var(--el-fill-color-light)',
      color: 'var(--el-text-color-secondary)'
    }
  })

  const cancelPendingSave = () => {
    if (saveTimer !== null) {
      window.clearTimeout(saveTimer)
      saveTimer = null
    }
  }

  const saveChapter = async () => {
    const chapterUlid = props.chapter?.chapter_ulid
    if (!chapterUlid) return
    saveStatus.value = 'saving'
    try {
      await updateChapter({
        chapter_ulid: chapterUlid,
        chapter_name: chapterName.value,
        content_text: contentText.value,
        summary_text: summaryText.value,
        chapter_status: chapterStatus.value
      })
      saveStatus.value = 'saved'
      dirtyFlag.value = false
      emit('save-success', {
        chapter_ulid: chapterUlid,
        chapter_name: chapterName.value,
        word_count: liveWordCount.value,
        chapter_status: chapterStatus.value
      })
    } catch {
      saveStatus.value = 'failed'
    }
  }

  const scheduleSave = () => {
    dirtyFlag.value = true
    cancelPendingSave()
    saveTimer = window.setTimeout(() => {
      saveTimer = null
      void saveChapter()
    }, SAVE_DEBOUNCE_MS)
  }

  /** 程序性写入正文（不触发保存） */
  const applyContent = (text: string) => {
    applyingContentFlag = true
    if (editorView) {
      editorView.dispatch({ changes: { from: 0, to: editorView.state.doc.length, insert: text } })
    }
    contentText.value = text
    applyingContentFlag = false
  }

  const insertTextAtCursor = (insertText: string) => {
    if (!editorView) return
    const insertFrom = editorView.state.selection.main.head
    editorView.dispatch({
      changes: { from: insertFrom, insert: insertText },
      selection: { anchor: insertFrom + insertText.length },
      scrollIntoView: true
    })
    editorView.focus()
  }

  const replaceContent = (text: string) => {
    if (!editorView) return
    editorView.dispatch({
      changes: { from: 0, to: editorView.state.doc.length, insert: text },
      scrollIntoView: true
    })
    editorView.focus()
  }

  /** 立即落盘未保存内容（切换章节/打开快照前调用） */
  const flushSave = async () => {
    if (!dirtyFlag.value) return
    cancelPendingSave()
    await saveChapter()
  }

  defineExpose({ insertTextAtCursor, replaceContent, flushSave })

  watch(
    () => props.chapter,
    () => {
      cancelPendingSave()
      dirtyFlag.value = false
      chapterName.value = props.chapter?.chapter_name ?? ''
      summaryText.value = props.chapter?.summary_text ?? ''
      chapterStatus.value = props.chapter?.chapter_status ?? 'draft'
      saveStatus.value = 'saved'
      applyContent(props.chapter?.content_text ?? '')
    },
    { immediate: true }
  )

  watch(
    () => props.chapter?.chapter_name,
    (newName) => {
      if (newName !== undefined && newName !== chapterName.value) chapterName.value = newName
    }
  )

  watch(contentText, (newText) => emit('word-count-change', countWords(newText)))

  /** 创建编辑器实例；章节为空时宿主节点不存在，等章节加载后再建（nextTick 保证 DOM 已渲染） */
  const mountEditor = async () => {
    await nextTick()
    if (editorView || !editorHost.value) return
    editorView = new EditorView({
      state: EditorState.create({
        doc: contentText.value,
        extensions: [
          lineNumbers(),
          history(),
          drawSelection(),
          highlightActiveLine(),
          highlightActiveLineGutter(),
          EditorView.lineWrapping,
          markdown(),
          syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
          keymap.of([...defaultKeymap, ...historyKeymap, indentWithTab]),
          editorTheme,
          EditorView.updateListener.of((update) => {
            if (!update.docChanged) return
            contentText.value = update.state.doc.toString()
            if (!applyingContentFlag) scheduleSave()
          })
        ]
      }),
      parent: editorHost.value
    })
  }

  watch(
    () => props.chapter?.chapter_ulid,
    (chapterUlid) => {
      if (chapterUlid) void mountEditor()
    },
    { immediate: true }
  )

  onMounted(() => void mountEditor())

  onUnmounted(() => {
    cancelPendingSave()
    editorView?.destroy()
    editorView = null
  })
</script>

<template>
  <section class="editor">
    <ElEmpty v-if="!chapter" description="从左侧目录选择章节，或新建一章开始写作" />

    <template v-else>
      <header class="editor__head">
        <ElInput
          v-model="chapterName"
          class="editor__name"
          placeholder="章节名"
          maxlength="128"
          @input="scheduleSave"
        />
        <ElSelect v-model="chapterStatus" class="editor__status" @change="scheduleSave">
          <ElOption label="草稿" value="draft" />
          <ElOption label="已修订" value="revised" />
          <ElOption label="已锁定" value="locked" />
        </ElSelect>
        <span class="editor__words">{{ liveWordCount.toLocaleString() }} 字</span>
        <span :class="['editor__save', `is-${saveStatus}`]">
          {{ saveStatusText }}
          <button v-if="saveStatus === 'failed'" type="button" @click="saveChapter">重试</button>
        </span>
        <div class="editor__spacer" />
        <div class="editor__actions">
          <ElButton size="small" plain @click="emit('ai-continue')">
            <Icon icon="mdi:sparkles" :size="14" />
            <span class="ml-4px">续写</span>
          </ElButton>
          <ElButton size="small" plain @click="emit('ai-polish')">
            <Icon icon="mdi:magic-staff" :size="14" />
            <span class="ml-4px">润色</span>
          </ElButton>
          <ElButton size="small" plain @click="emit('ai-summary')">
            <Icon icon="mdi:script-text-outline" :size="14" />
            <span class="ml-4px">梗概</span>
          </ElButton>
          <ElButton size="small" plain @click="emit('open-snapshot')">
            <Icon icon="mdi:history" :size="14" />
            <span class="ml-4px">快照</span>
          </ElButton>
        </div>
      </header>

      <div class="editor__summary">
        <button type="button" class="editor__summary-toggle" @click="summaryCollapsed = !summaryCollapsed">
          {{ summaryCollapsed ? '▸' : '▾' }} 本章梗概
        </button>
        <ElInput
          v-show="!summaryCollapsed"
          v-model="summaryText"
          type="textarea"
          :rows="2"
          maxlength="2048"
          placeholder="写一句话概括本章，供 AI 与连续性看板引用"
          @input="scheduleSave"
        />
      </div>

      <div ref="editorHost" class="editor__host" />
    </template>
  </section>
</template>

<style lang="less" scoped>
  .editor {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    background: var(--nw-surface);

    &__head {
      display: flex;
      flex: none;
      flex-wrap: wrap;
      gap: 8px;
      align-items: center;
      padding: 8px 12px;
      border-bottom: 1px solid var(--nw-border-soft);
    }

    &__name {
      flex: 1 1 160px;
      min-width: 120px;
    }

    &__status {
      flex: none;
      width: 96px;
    }

    &__actions {
      display: flex;
      flex: none;
      gap: 4px;
      align-items: center;
    }

    &__words,
    &__save {
      font-size: 12px;
      color: var(--el-text-color-secondary);
      white-space: nowrap;
    }

    &__save {
      &.is-failed {
        color: var(--nw-danger);
      }

      button {
        margin-left: 4px;
        color: var(--el-color-primary);
        cursor: pointer;
        background: transparent;
        border: 0;
      }
    }

    &__spacer {
      flex: 1;
    }

    &__summary {
      flex: none;
      padding: 6px 12px;
      background: var(--nw-surface-subtle);
      border-bottom: 1px solid var(--nw-border-soft);
    }

    &__summary-toggle {
      padding: 0;
      font-size: 12px;
      color: var(--el-text-color-secondary);
      cursor: pointer;
      background: transparent;
      border: 0;
    }

    &__host {
      flex: 1;
      min-height: 0;
      overflow: hidden;
    }

    :deep(.cm-editor) {
      height: 100%;
    }

    :deep(.cm-scroller) {
      overflow-y: auto;
    }
  }
</style>

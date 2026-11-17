<!--
  章节编辑器：CodeMirror 6（markdown）+ debounce 2s 自动保存。
  正文/章节名/梗概/状态统一走同一自动保存通道（POST /chapter/update），
  右上角展示保存状态（保存中/已保存/保存失败可重试）。
-->
<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { EditorState } from '@codemirror/state'
import {
  EditorView,
  drawSelection,
  highlightActiveLine,
  highlightActiveLineGutter,
  keymap,
  lineNumbers,
} from '@codemirror/view'
import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands'
import { defaultHighlightStyle, syntaxHighlighting } from '@codemirror/language'
import { markdown } from '@codemirror/lang-markdown'
import { useI18n } from 'vue-i18n'
import { updateChapter } from '@/api/chapter'
import type { NWChapterDetailVo } from '@/api/contracts'
import NWButton from '@/components/NWButton.vue'
import NWEmpty from '@/components/NWEmpty.vue'
import { countWords } from './wordCount'

const props = defineProps<{
  chapter: NWChapterDetailVo | null
}>()

const emit = defineEmits<{
  'save-success': [
    payload: {
      chapter_ulid: string
      chapter_name: string
      word_count: number
      chapter_status: string
    },
  ]
  'word-count-change': [wordCount: number]
  'ai-continue': []
  'ai-polish': []
  'open-snapshot': []
}>()

const { t } = useI18n()

/** 自动保存防抖时长（毫秒） */
const NW_SAVE_DEBOUNCE_MS = 2000

/** 保存状态：idle 未改动 / saving 保存中 / saved 已保存 / failed 保存失败 */
type NWSaveStatus = 'idle' | 'saving' | 'saved' | 'failed'

const editorHost = ref<HTMLDivElement | null>(null)
const chapterName = ref('')
const summaryText = ref('')
const chapterStatus = ref('draft')
const contentText = ref('')
const saveStatus = ref<NWSaveStatus>('idle')
const dirtyFlag = ref(false)

let editorView: EditorView | null = null
let saveTimer: number | null = null
let applyingContentFlag = false

const liveWordCount = computed(() => countWords(contentText.value))

/** CodeMirror 浅色主题：颜色全部贴合设计系统变量 */
const editorTheme = EditorView.theme({
  '&': {
    height: '100%',
    backgroundColor: 'var(--bg-card)',
    color: 'var(--text-main)',
    fontSize: '15px',
  },
  '.cm-content': {
    fontFamily: "'PingFang SC', 'Microsoft YaHei', 'Helvetica Neue', Arial, sans-serif",
    caretColor: 'var(--primary)',
    padding: '12px 0',
  },
  '.cm-cursor, .cm-dropCursor': { borderLeftColor: 'var(--primary)' },
  '&.cm-focused .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection': {
    backgroundColor: 'var(--primary-light)',
  },
  '.cm-activeLine': { backgroundColor: 'var(--bg-hover)' },
  '.cm-gutters': {
    backgroundColor: 'var(--bg-page)',
    color: 'var(--text-placeholder)',
    border: 'none',
  },
  '.cm-activeLineGutter': {
    backgroundColor: 'var(--bg-hover)',
    color: 'var(--text-secondary)',
  },
})

/** 调度 2s 防抖自动保存 */
function scheduleSave(): void {
  dirtyFlag.value = true
  if (saveTimer !== null) {
    window.clearTimeout(saveTimer)
  }
  saveTimer = window.setTimeout(() => {
    saveTimer = null
    void saveChapter()
  }, NW_SAVE_DEBOUNCE_MS)
}

/** 取消未触发的防抖保存（切换章节前由父组件 flushSave 确保存盘） */
function cancelPendingSave(): void {
  if (saveTimer !== null) {
    window.clearTimeout(saveTimer)
    saveTimer = null
  }
}

/** 保存章节（正文 + 名称 + 梗概 + 状态整体提交） */
async function saveChapter(): Promise<void> {
  const chapterUlid = props.chapter?.chapter_ulid
  if (!chapterUlid) {
    return
  }
  saveStatus.value = 'saving'
  try {
    await updateChapter({
      chapter_ulid: chapterUlid,
      chapter_name: chapterName.value,
      content_text: contentText.value,
      summary_text: summaryText.value,
      chapter_status: chapterStatus.value,
    })
    saveStatus.value = 'saved'
    dirtyFlag.value = false
    emit('save-success', {
      chapter_ulid: chapterUlid,
      chapter_name: chapterName.value,
      word_count: liveWordCount.value,
      chapter_status: chapterStatus.value,
    })
  } catch {
    // 失败提示由 http 层统一弹出，这里仅落状态供重试
    saveStatus.value = 'failed'
  }
}

/** 将正文写入编辑器（程序性替换，不触发保存） */
function applyContent(text: string): void {
  applyingContentFlag = true
  if (editorView) {
    editorView.dispatch({
      changes: { from: 0, to: editorView.state.doc.length, insert: text },
    })
  }
  contentText.value = text
  applyingContentFlag = false
}

/** 章节切换：重置本地编辑状态并载入正文 */
function initFromChapter(): void {
  cancelPendingSave()
  dirtyFlag.value = false
  chapterName.value = props.chapter?.chapter_name ?? ''
  summaryText.value = props.chapter?.summary_text ?? ''
  chapterStatus.value = props.chapter?.chapter_status ?? 'draft'
  saveStatus.value = 'idle'
  applyContent(props.chapter?.content_text ?? '')
}

/** 在光标处插入文本（AI 插入到章节，走同一自动保存通道） */
function insertTextAtCursor(insertText: string): void {
  if (!editorView) {
    return
  }
  const insertFrom = editorView.state.selection.main.head
  editorView.dispatch({
    changes: { from: insertFrom, insert: insertText },
    selection: { anchor: insertFrom + insertText.length },
    scrollIntoView: true,
  })
  editorView.focus()
}

/** 立即落盘未保存的防抖内容（切换章节/打开快照前调用） */
async function flushSave(): Promise<void> {
  if (!dirtyFlag.value) {
    return
  }
  cancelPendingSave()
  await saveChapter()
}

defineExpose({ insertTextAtCursor, flushSave })

watch(() => props.chapter, initFromChapter, { immediate: true })

// 目录树重命名同步到编辑器标题
watch(() => props.chapter?.chapter_name, (newName) => {
  if (newName !== undefined && newName !== chapterName.value) {
    chapterName.value = newName
  }
})

// 正文变化实时上报字数（与后端 countWords 同口径）
watch(contentText, (newText) => {
  emit('word-count-change', countWords(newText))
})

onMounted(() => {
  if (!editorHost.value) {
    return
  }
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
          if (update.docChanged) {
            contentText.value = update.state.doc.toString()
            if (!applyingContentFlag) {
              scheduleSave()
            }
          }
        }),
      ],
    }),
    parent: editorHost.value,
  })
})

onUnmounted(() => {
  cancelPendingSave()
  editorView?.destroy()
  editorView = null
})
</script>

<template>
  <div class="chapter-editor">
    <NWEmpty
      v-if="!chapter"
      class="chapter-editor__empty"
      :description_text="t('workspace.editor.emptyTip')"
    />

    <div v-show="chapter" class="chapter-editor__body">
      <header class="chapter-editor__header">
        <el-input
          v-model="chapterName"
          class="chapter-editor__name"
          :placeholder="t('workspace.editor.chapterNameLabel')"
          maxlength="128"
          @input="scheduleSave"
        />
        <el-select
          v-model="chapterStatus"
          class="chapter-editor__status"
          @change="scheduleSave"
        >
          <el-option :label="t('workspace.tree.statusDraft')" value="draft" />
          <el-option :label="t('workspace.tree.statusRevised')" value="revised" />
          <el-option :label="t('workspace.tree.statusLocked')" value="locked" />
        </el-select>

        <span class="chapter-editor__words">
          {{ t('workspace.editor.wordCount') }} {{ liveWordCount }}
        </span>

        <span class="chapter-editor__save-state" :class="`is-${saveStatus}`">
          <template v-if="saveStatus === 'saving'">{{ t('workspace.editor.saving') }}</template>
          <template v-else-if="saveStatus === 'saved'">{{ t('workspace.editor.saved') }}</template>
          <template v-else-if="saveStatus === 'failed'">
            {{ t('workspace.editor.saveFailed') }}
            <NWButton size="small" @click="saveChapter">
              {{ t('workspace.editor.retrySave') }}
            </NWButton>
          </template>
          <template v-else>{{ t('workspace.editor.unsaved') }}</template>
        </span>

        <NWButton size="small" @click="emit('ai-continue')">
          {{ t('workspace.editor.aiContinue') }}
        </NWButton>
        <NWButton size="small" @click="emit('ai-polish')">
          {{ t('workspace.editor.aiPolish') }}
        </NWButton>
        <NWButton size="small" @click="emit('open-snapshot')">
          {{ t('workspace.editor.snapshot') }}
        </NWButton>
      </header>

      <div class="chapter-editor__summary">
        <span class="chapter-editor__summary-label">
          {{ t('workspace.editor.summaryLabel') }}
        </span>
        <el-input
          v-model="summaryText"
          type="textarea"
          :rows="2"
          maxlength="2048"
          :placeholder="t('workspace.editor.summaryPlaceholder')"
          @input="scheduleSave"
        />
      </div>

      <div ref="editorHost" class="chapter-editor__host" />
    </div>
  </div>
</template>

<style scoped>
.chapter-editor {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.chapter-editor__empty {
  margin: auto;
}

.chapter-editor__body {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}

.chapter-editor__header {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--border-light);
}

.chapter-editor__name {
  flex: 1;
  min-width: 180px;
}

.chapter-editor__name :deep(.el-input__wrapper) {
  font-size: 16px;
  font-weight: 600;
}

.chapter-editor__status {
  width: 110px;
}

.chapter-editor__words {
  font-size: 13px;
  color: var(--text-secondary);
  white-space: nowrap;
}

.chapter-editor__save-state {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--text-secondary);
  white-space: nowrap;
}

.chapter-editor__save-state.is-saved {
  color: var(--success);
}

.chapter-editor__save-state.is-saving {
  color: var(--text-placeholder);
}

.chapter-editor__save-state.is-failed {
  color: var(--error);
}

.chapter-editor__summary {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 12px 0;
  border-bottom: 1px solid var(--border-light);
}

.chapter-editor__summary-label {
  padding-top: 6px;
  font-size: 13px;
  color: var(--text-secondary);
  white-space: nowrap;
}

.chapter-editor__host {
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.chapter-editor__host :deep(.cm-editor) {
  height: 100%;
}

.chapter-editor__host :deep(.cm-editor.cm-focused) {
  outline: none;
}
</style>

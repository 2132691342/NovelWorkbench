<!--
  创作笔记页：左侧笔记列表（标题/更新时间/搜索）+ 右侧 WangEditor 富文本编辑。
  标题/正文统一走 debounce 2s 自动保存（POST /lore/save，有 lore_ulid 为更新），
  切换笔记前先落盘；右上角展示保存状态（保存中/已保存/未保存/失败可重试）。
-->
<script setup lang="ts">
import { computed, defineAsyncComponent, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import {
  deleteLore,
  listLoreNotes,
  NW_LORE_TYPE_NOTE,
  saveLore,
  type NWLoreVo,
} from '@/api/lore'
import NWButton from '@/components/NWButton.vue'
import NWIcon from '@/components/NWIcon.vue'
import NWPageHeader from '@/components/NWPageHeader.vue'
import NWEmpty from '@/components/NWEmpty.vue'
import NWLoading from '@/components/NWLoading.vue'
import ProjectPageHeader from '@/components/ProjectPageHeader.vue'
import { convertNoteContent, type NWNoteContentMode } from './noteContent'

// 两种编辑器体积都在 1MB 级，且互斥使用：按模式懒加载，进页面只付当前模式那一份。
const NoteEditor = defineAsyncComponent(() => import('./NoteEditor.vue'))
const MarkdownNoteEditor = defineAsyncComponent(() => import('./MarkdownNoteEditor.vue'))

const route = useRoute()
const { t } = useI18n()

const projectUlid = computed(() => String(route.params.project_ulid ?? ''))

/** 自动保存防抖时长（毫秒，与写作台章节编辑器一致） */
const NW_SAVE_DEBOUNCE_MS = 2000

/** 保存状态：idle 未改动 / saving 保存中 / saved 已保存 / failed 保存失败 */
type NWSaveStatus = 'idle' | 'saving' | 'saved' | 'failed'

const noteList = ref<NWLoreVo[]>([])
const listLoading = ref(false)
const searchText = ref('')
const currentNote = ref<NWLoreVo | null>(null)
const noteName = ref('')
const contentHtml = ref('')
const saveStatus = ref<NWSaveStatus>('idle')
/** 笔记编辑模式：rich 使用 WangEditor，markdown 使用 MavonEditor */
const editorMode = ref<'rich' | 'markdown'>('rich')
const creatingFlag = ref(false)

let saveTimer: number | null = null
let dirtyFlag = false
/** 程序化回填内容期间抑制 change 事件触发自动保存 */
let suppressChangeFlag = false

/** 去掉富文本标签，得到纯文本片段 */
function plainTextOf(htmlText: string): string {
  return htmlText.replace(/<[^>]*>/g, ' ').replace(/&nbsp;/g, ' ').replace(/\s+/g, ' ').trim()
}

/** 笔记正文片段 */
function noteSnippet(noteVo: NWLoreVo): string {
  const plainText = plainTextOf(noteVo.content_text || '')
  return plainText.length > 60 ? plainText.slice(0, 60) + '…' : plainText || t('notes.emptySnippet')
}

/** 过滤后的笔记列表（标题或正文模糊匹配） */
const filteredNoteList = computed<NWLoreVo[]>(() => {
  const keyword = searchText.value.trim().toLowerCase()
  if (!keyword) {
    return noteList.value
  }
  return noteList.value.filter(
    (item) =>
      item.lore_name.toLowerCase().includes(keyword) ||
      plainTextOf(item.content_text || '').toLowerCase().includes(keyword),
  )
})

/** 毫秒时间戳转本地时间 */
function formatTime(timeMs: number): string {
  return new Date(timeMs).toLocaleString()
}

/** 加载笔记列表 */
async function loadNotes(): Promise<void> {
  if (!projectUlid.value) {
    return
  }
  listLoading.value = true
  try {
    const listData = await listLoreNotes(projectUlid.value)
    noteList.value = Array.isArray(listData) ? listData : []
  } catch {
    // 失败提示由 http 层统一弹出
    noteList.value = []
  } finally {
    listLoading.value = false
  }
}

/** 安排防抖自动保存 */
function scheduleSave(): void {
  dirtyFlag = true
  saveStatus.value = 'saved'
  if (saveTimer !== null) {
    window.clearTimeout(saveTimer)
  }
  saveTimer = window.setTimeout(() => {
    saveTimer = null
    void saveNote()
  }, NW_SAVE_DEBOUNCE_MS)
}

/** 取消未触发的防抖 */
function cancelScheduledSave(): void {
  if (saveTimer !== null) {
    window.clearTimeout(saveTimer)
    saveTimer = null
  }
}

/** 保存当前笔记（新建走首次落盘拿 lore_ulid，后续为更新） */
async function saveNote(): Promise<void> {
  const note = currentNote.value
  if (!note) {
    return
  }
  saveStatus.value = 'saving'
  try {
    const savedVo = await saveLore({
      lore_ulid: note.lore_ulid || undefined,
      project_ulid: projectUlid.value,
      lore_name: noteName.value || t('notes.defaultName'),
      lore_type_code: NW_LORE_TYPE_NOTE,
      content_text: contentHtml.value,
      content_mode_code: editorMode.value,
      sort_order: note.sort_order,
    })
    currentNote.value = savedVo
    const listItem = noteList.value.find((item) => item.lore_ulid === savedVo.lore_ulid)
    if (listItem) {
      listItem.lore_name = savedVo.lore_name
      listItem.content_text = savedVo.content_text
      listItem.content_mode_code = savedVo.content_mode_code
      listItem.updated_at_ms = savedVo.updated_at_ms
    }
    dirtyFlag = false
    saveStatus.value = 'saved'
  } catch {
    // 失败提示由 http 层统一弹出；状态置失败可重试
    saveStatus.value = 'failed'
  }
}

/** 立即落盘未保存内容（切换笔记前调用） */
async function flushSave(): Promise<void> {
  cancelScheduledSave()
  if (dirtyFlag) {
    await saveNote()
  }
}

/** 回填编辑器（抑制回环触发保存） */
function applyNoteContent(html: string): void {
  suppressChangeFlag = true
  contentHtml.value = html
  window.setTimeout(() => {
    suppressChangeFlag = false
  }, 0)
}

/** 切换笔记：先落盘当前，再载入目标 */
async function selectNote(noteVo: NWLoreVo): Promise<void> {
  if (currentNote.value?.lore_ulid === noteVo.lore_ulid) {
    return
  }
  await flushSave()
  currentNote.value = noteVo
  noteName.value = noteVo.lore_name
  editorMode.value = noteVo.content_mode_code === 'markdown' ? 'markdown' : 'rich'
  applyNoteContent(noteVo.content_text)
  saveStatus.value = 'saved'
}

/** 新建笔记（立即落库占位，后续编辑走自动保存） */
async function handleCreate(): Promise<void> {
  creatingFlag.value = true
  try {
    const savedVo = await saveLore({
      project_ulid: projectUlid.value,
      lore_name: t('notes.defaultName'),
      lore_type_code: NW_LORE_TYPE_NOTE,
      content_text: '',
    })
    ElMessage.success(t('notes.createSuccess'))
    noteList.value.unshift(savedVo)
    await selectNote(savedVo)
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    creatingFlag.value = false
  }
}

/** 删除笔记（二次确认） */
async function handleDelete(noteVo: NWLoreVo): Promise<void> {
  try {
    await ElMessageBox.confirm(t('notes.deleteConfirm'), t('common.confirm'), {
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel'),
      type: 'warning',
    })
  } catch {
    return
  }
  try {
    await deleteLore(noteVo.lore_ulid)
    ElMessage.success(t('notes.deleteSuccess'))
    if (currentNote.value?.lore_ulid === noteVo.lore_ulid) {
      cancelScheduledSave()
      dirtyFlag = false
      currentNote.value = null
      noteName.value = ''
      applyNoteContent('')
      saveStatus.value = 'idle'
    }
    noteList.value = noteList.value.filter((item) => item.lore_ulid !== noteVo.lore_ulid)
  } catch {
    // 失败提示由 http 层统一弹出
  }
}

function handleNameInput(): void {
  scheduleSave()
}

function handleContentChange(): void {
  if (suppressChangeFlag) {
    return
  }
  scheduleSave()
}

/**
 * 切换编辑器模式：把当前正文转换成目标格式，避免 HTML 标签出现在 Markdown 里（反之亦然）。
 * 转换后立即落盘，保证下次打开笔记仍是同一格式。
 */
function handleModeChange(targetMode: string | number | boolean | undefined): void {
  const nextMode: NWNoteContentMode = targetMode === 'markdown' ? 'markdown' : 'rich'
  if (nextMode === editorMode.value) {
    return
  }
  editorMode.value = nextMode
  applyNoteContent(convertNoteContent(contentHtml.value, nextMode))
  scheduleSave()
}

watch(projectUlid, () => {
  cancelScheduledSave()
  dirtyFlag = false
  currentNote.value = null
  noteName.value = ''
  applyNoteContent('')
  void loadNotes()
})

onMounted(() => {
  void loadNotes()
})
</script>

<template>
  <div class="nw-page notes-page">
    <ProjectPageHeader :project-ulid="projectUlid" />

    <NWPageHeader
      :title_text="t('notes.title')"
      :subtitle_text="t('notes.pageSubtitle')"
      icon_code="note"
    >
      <template #actions>
        <NWButton type="primary" :loading="creatingFlag" @click="handleCreate">
          <NWIcon icon_code="plus" :size_px="15" />
          {{ t('notes.create') }}
        </NWButton>
      </template>
    </NWPageHeader>

    <div class="notes-page__main">
      <!-- 左侧笔记列表 -->
      <aside class="notes-page__list">
        <div class="notes-page__list-head">
          <el-input
            v-model="searchText"
            size="small"
            clearable
            :placeholder="t('notes.searchPlaceholder')"
          >
            <template #prefix><NWIcon icon_code="search" :size_px="13" /></template>
          </el-input>
          <button type="button" class="notes-page__new-btn" :title="t('notes.create')" @click="handleCreate">
            <NWIcon icon_code="plus" :size_px="15" />
          </button>
        </div>
        <div class="notes-page__list-meta">{{ t('notes.countLabel', { count: filteredNoteList.length }) }}</div>
        <NWLoading v-if="listLoading && noteList.length === 0" />
        <NWEmpty
          v-else-if="filteredNoteList.length === 0"
          compact_flag
          icon_code="note"
          :title_text="t('notes.emptyTitle')"
          :description_text="t('notes.emptyGuide')"
        />
        <ul v-else class="notes-page__items">
          <li
            v-for="noteVo in filteredNoteList"
            :key="noteVo.lore_ulid"
            class="notes-page__item"
            :class="{ 'is-active': currentNote?.lore_ulid === noteVo.lore_ulid }"
            @click="selectNote(noteVo)"
          >
            <span class="notes-page__item-name" :title="noteVo.lore_name">{{ noteVo.lore_name }}</span>
            <span class="notes-page__item-snippet">{{ noteSnippet(noteVo) }}</span>
            <span class="notes-page__item-foot">
              <span class="notes-page__item-time">{{ formatTime(noteVo.updated_at_ms) }}</span>
              <button type="button" class="notes-page__item-delete" @click.stop="handleDelete(noteVo)">
                <NWIcon icon_code="trash" :size_px="12" />
              </button>
            </span>
          </li>
        </ul>
      </aside>

      <!-- 右侧编辑区 -->
      <section class="notes-page__editor">
        <template v-if="currentNote">
          <div class="notes-page__editor-bar">
            <el-input
              v-model="noteName"
              class="notes-page__name-input"
              maxlength="128"
              :placeholder="t('notes.namePlaceholder')"
              @input="handleNameInput"
            />
            <el-radio-group
              :model-value="editorMode"
              size="small"
              class="notes-page__editor-mode"
              @change="handleModeChange"
            >
              <el-radio-button value="rich">{{ t('markdownNote.modeRich') }}</el-radio-button>
              <el-radio-button value="markdown">{{ t('markdownNote.modeMarkdown') }}</el-radio-button>
            </el-radio-group>
            <span class="notes-page__save-state" :class="`is-${saveStatus}`">
              <NWIcon
                :icon_code="saveStatus === 'failed' ? 'alertCircle' : saveStatus === 'saved' ? 'checkCircle' : 'clock'"
                :size_px="13"
              />
              <template v-if="saveStatus === 'saving'">{{ t('notes.saveStatus.saving') }}</template>
              <template v-else-if="saveStatus === 'saved'">{{ t('notes.saveStatus.saved') }}</template>
              <template v-else-if="saveStatus === 'failed'">
                {{ t('notes.saveStatus.saveFailed') }}
                <button type="button" class="notes-page__retry" @click="saveNote">{{ t('notes.saveStatus.retry') }}</button>
              </template>
              <template v-else>{{ t('notes.saveStatus.unsaved') }}</template>
            </span>
          </div>
          <!-- 编辑器按需加载：Suspense 兜住异步组件解析期，避免空白闪烁 -->
          <Suspense>
            <template #default>
              <MarkdownNoteEditor
                v-if="editorMode === 'markdown'"
                v-model="contentHtml"
                class="notes-page__editor-body"
                :placeholder="t('markdownNote.editorTip')"
                @change="handleContentChange"
              />
              <NoteEditor
                v-else
                v-model="contentHtml"
                class="notes-page__editor-body"
                :placeholder="t('notes.contentPlaceholder')"
                @change="handleContentChange"
              />
            </template>
            <template #fallback>
              <div class="notes-page__editor-loading">{{ t('notes.editorLoading') }}</div>
            </template>
          </Suspense>
        </template>
        <div v-else class="notes-page__editor-empty">
          <NWEmpty
            icon_code="note"
            :title_text="t('notes.selectTitle')"
            :description_text="t('notes.selectGuide')"
          >
            <template #actions>
              <NWButton size="small" type="primary" :loading="creatingFlag" @click="handleCreate">{{ t('notes.create') }}</NWButton>
            </template>
          </NWEmpty>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped src="./notes.css"></style>

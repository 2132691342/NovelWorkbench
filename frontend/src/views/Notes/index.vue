<!-- 创作笔记页：左侧列表与右侧编辑器。 -->
<script setup lang="ts">
  import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
  import { ElButton, ElEmpty, ElInput, ElMessage, ElMessageBox, ElScrollbar } from 'element-plus'
  import { deleteLore, listLoreNotes, NW_LORE_TYPE_NOTE, saveLore } from '@/api/lore'
  import type { NWLoreVo } from '@/api/lore'
  import { useProjectStore } from '@/store/modules/project'
  import { formatRelativeTime } from '@/utils/time'
  import NoteEditor from './components/NoteEditor.vue'

  const projectStore = useProjectStore()
  const projectUlid = computed(() => projectStore.currentProjectUlid)

  const noteList = ref<NWLoreVo[]>([])
  const listLoading = ref(false)
  const keywordText = ref('')
  const currentUlid = ref('')

  const editorRef = ref<InstanceType<typeof NoteEditor> | null>(null)

  const currentNote = computed<NWLoreVo | null>(
    () => noteList.value.find((note) => note.lore_ulid === currentUlid.value) ?? null
  )

  /** 关键词在本地过滤标题与正文，避免每次输入都请求后端 */
  const filteredList = computed<NWLoreVo[]>(() => {
    const keyword = keywordText.value.trim().toLowerCase()
    if (!keyword) return noteList.value
    return noteList.value.filter(
      (note) =>
        note.lore_name.toLowerCase().includes(keyword) ||
        note.content_text.toLowerCase().includes(keyword)
    )
  })

  /** 列表摘要：压平空白后截断 */
  const noteSnippet = (note: NWLoreVo): string => {
    const plainText = note.content_text.replace(/\s+/g, ' ').trim()
    return plainText.length > 40 ? `${plainText.slice(0, 40)}…` : plainText || '空笔记'
  }

  const loadNotes = async (preferredUlid = '') => {
    if (!projectUlid.value) {
      noteList.value = []
      currentUlid.value = ''
      return
    }
    listLoading.value = true
    try {
      noteList.value = await listLoreNotes(projectUlid.value)
      const candidateUlid = preferredUlid || currentUlid.value
      const stillExists = noteList.value.some((note) => note.lore_ulid === candidateUlid)
      currentUlid.value = stillExists ? candidateUlid : (noteList.value[0]?.lore_ulid ?? '')
    } catch {
      noteList.value = []
      currentUlid.value = ''
    } finally {
      listLoading.value = false
    }
  }

  /** 切换笔记前先落盘，避免丢字 */
  const selectNote = async (loreUlid: string) => {
    if (loreUlid === currentUlid.value) return
    await editorRef.value?.flushSave()
    currentUlid.value = loreUlid
  }

  const handleCreate = async () => {
    if (!projectUlid.value) {
      ElMessage.warning('请先在顶栏选择项目')
      return
    }
    await editorRef.value?.flushSave()
    const createdNote = await saveLore({
      project_ulid: projectUlid.value,
      lore_name: '未命名笔记',
      lore_type_code: NW_LORE_TYPE_NOTE,
      content_text: '',
      content_mode_code: 'markdown'
    })
    await loadNotes(createdNote.lore_ulid)
  }

  const handleDelete = async (note: NWLoreVo) => {
    try {
      await ElMessageBox.confirm(`将删除笔记「${note.lore_name}」，删除后不可恢复。`, '删除笔记', {
        type: 'warning'
      })
    } catch {
      return
    }
    await deleteLore(note.lore_ulid)
    ElMessage.success('已删除笔记')
    await loadNotes()
  }

  /** 自动保存回写列表项，保持标题与时间同步 */
  const handleSaved = (savedNote: NWLoreVo) => {
    const index = noteList.value.findIndex((note) => note.lore_ulid === savedNote.lore_ulid)
    if (index >= 0) noteList.value[index] = savedNote
  }

  const handleBeforeUnload = () => {
    void editorRef.value?.flushSave()
  }

  watch(projectUlid, async (nextUlid) => {
    if (!nextUlid) {
      noteList.value = []
      currentUlid.value = ''
      return
    }
    await loadNotes()
  })

  onMounted(async () => {
    if (!projectStore.projectOptions.length) await projectStore.loadProjectOptions()
    if (projectUlid.value) await loadNotes()
    window.addEventListener('beforeunload', handleBeforeUnload)
  })

  onBeforeUnmount(() => {
    window.removeEventListener('beforeunload', handleBeforeUnload)
    void editorRef.value?.flushSave()
  })
</script>

<template>
  <div class="nw-page">
    <ElEmpty
      v-if="!projectUlid"
      description="请先在顶栏选择项目，再记录创作笔记"
      :image-size="90"
    />

    <div v-else class="notes">
      <aside class="notes__side">
        <div class="notes__side-head">
          <ElInput
            v-model="keywordText"
            placeholder="搜索笔记"
            clearable
            size="small"
          />
          <ElButton type="primary" plain size="small" @click="handleCreate">
            <Icon icon="mdi:plus" :size="14" />
          </ElButton>
        </div>

        <ElScrollbar v-loading="listLoading" class="notes__list-scroll">
          <div class="notes__list">
            <button
              v-for="note in filteredList"
              :key="note.lore_ulid"
              type="button"
              class="note-item"
              :class="{ 'is-active': note.lore_ulid === currentUlid }"
              @click="selectNote(note.lore_ulid)"
            >
              <span class="note-item__head">
                <span class="note-item__title">{{ note.lore_name }}</span>
                <span class="note-item__time">{{ formatRelativeTime(note.updated_at) }}</span>
              </span>
              <span class="note-item__snippet">{{ noteSnippet(note) }}</span>
              <span class="note-item__delete" title="删除笔记" @click.stop="handleDelete(note)">
                <Icon icon="mdi:delete-outline" :size="12" />
              </span>
            </button>

            <ElEmpty
              v-if="!listLoading && filteredList.length === 0"
              :description="keywordText ? '没有匹配的笔记' : '还没有笔记，点右上角新建'"
              :image-size="70"
            />
          </div>
        </ElScrollbar>

        <div class="notes__side-foot">
          <span class="nw-hint">共 {{ noteList.length }} 篇 · 输入后自动保存</span>
        </div>
      </aside>

      <section class="notes__main">
        <NoteEditor
          ref="editorRef"
          :note="currentNote"
          :project-ulid="projectUlid"
          @saved="handleSaved"
        />
      </section>
    </div>
  </div>
</template>

<style lang="less" scoped>
  .notes {
    display: flex;
    height: calc(
      100vh - var(--top-tool-height) - var(--tags-view-height) - 2 * var(--app-content-padding)
    );
    overflow: hidden;
    background: var(--nw-surface);
    border: 1px solid var(--nw-border);
    border-radius: var(--nw-radius-lg);

    &__side {
      display: flex;
      flex: none;
      flex-direction: column;
      width: 268px;
      min-height: 0;
      border-right: 1px solid var(--nw-border-soft);
    }

    &__side-head {
      display: flex;
      flex: none;
      gap: 6px;
      align-items: center;
      padding: 10px;
      border-bottom: 1px solid var(--nw-border-soft);
    }

    &__list-scroll {
      flex: 1;
      min-height: 0;
    }

    &__list {
      display: flex;
      flex-direction: column;
      gap: 2px;
      padding: 8px;
    }

    &__side-foot {
      flex: none;
      padding: 8px 12px;
      border-top: 1px solid var(--nw-border-soft);
    }

    &__main {
      display: flex;
      flex: 1;
      flex-direction: column;
      min-width: 0;
      min-height: 0;
    }
  }

  .note-item {
    position: relative;
    display: flex;
    flex-direction: column;
    gap: 3px;
    padding: 8px 10px;
    text-align: left;
    cursor: pointer;
    background: transparent;
    border: none;
    border-radius: var(--nw-radius-base);
    transition: background 0.15s ease;

    &:hover {
      background: var(--nw-surface-subtle);
    }

    &.is-active {
      background: var(--nw-primary-soft);
    }

    &.is-active .note-item__title {
      color: var(--el-color-primary);
    }

    &__head {
      display: flex;
      gap: 8px;
      align-items: baseline;
      justify-content: space-between;
    }

    &__title {
      flex: 1;
      min-width: 0;
      overflow: hidden;
      font-size: 13px;
      font-weight: 600;
      color: var(--nw-ink);
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    &__time {
      flex: none;
      font-size: 10.5px;
      color: var(--nw-text-muted);
    }

    &__snippet {
      overflow: hidden;
      font-size: 11.5px;
      color: var(--nw-text-secondary);
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    &__delete {
      position: absolute;
      right: 8px;
      bottom: 6px;
      display: none;
      padding: 2px;
      color: var(--nw-text-muted);
      border-radius: var(--nw-radius-sm);
    }

    &__delete:hover {
      color: var(--nw-danger);
      background: var(--el-fill-color-light);
    }

    &:hover &__delete {
      display: block;
    }
  }

  @media (width <= 900px) {
    .notes__side {
      width: 220px;
    }
  }
</style>

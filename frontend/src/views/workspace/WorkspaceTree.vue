<!--
  写作台左栏目录树：卷分组 + 章节列表。
  卷支持新建/重命名与概要编辑/删除，章支持新建/重命名/删除；
  结构性变更后向父组件发 tree-changed 请求重载目录。
-->
<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { createChapter, deleteChapter, updateChapter } from '@/api/chapter'
import { createVolume, deleteVolume, updateVolume } from '@/api/project'
import type { NWChapterBriefVo, NWProjectTreeVo, NWVolumeVo } from '@/api/contracts'
import NWButton from '@/components/NWButton.vue'
import NWEmpty from '@/components/NWEmpty.vue'
import NWIcon from '@/components/NWIcon.vue'
import { exportProjectOutline } from './workspaceTreeExport'

const props = defineProps<{
  treeData: NWProjectTreeVo | null
  projectUlid: string
  currentChapterUlid: string
}>()

const emit = defineEmits<{
  'select-chapter': [chapterUlid: string]
  'tree-changed': []
  'chapter-created': [chapterUlid: string]
  'chapter-renamed': [chapterUlid: string, chapterName: string]
  'chapter-deleted': [chapterUlid: string]
}>()

const { t } = useI18n()

/** 卷弹窗表单状态 */
interface VolumeFormState {
  volume_name: string
  summary_text: string
}

/** 章弹窗表单状态 */
interface ChapterFormState {
  chapter_name: string
}

const actionLoading = ref(false)
/** 目录搜索关键词（书名/卷名/章名） */
const searchKeyword = ref('')
/** 卷折叠状态（按卷 ULID） */
const collapsedVolumeMap = ref<Record<string, boolean>>({})
const volumeDialogVisible = ref(false)
const volumeEditingFlag = ref(false)
const editingVolumeUlid = ref('')
const chapterDialogVisible = ref(false)
const chapterEditingFlag = ref(false)
const editingChapterUlid = ref('')
const targetVolumeUlid = ref('')
const volumeFormRef = ref<FormInstance>()
const chapterFormRef = ref<FormInstance>()

const volumeForm = reactive<VolumeFormState>({ volume_name: '', summary_text: '' })
const chapterForm = reactive<ChapterFormState>({ chapter_name: '' })

const volumeRules = computed<FormRules>(() => ({
  volume_name: [
    { required: true, message: t('workspace.tree.ruleVolumeNameRequired'), trigger: 'blur' },
  ],
}))

const chapterRules = computed<FormRules>(() => ({
  chapter_name: [
    { required: true, message: t('workspace.tree.ruleChapterNameRequired'), trigger: 'blur' },
  ],
}))

/** 关键词过滤后的目录（空关键词显示全部） */
const filteredVolumeList = computed(() => {
  const keyword = searchKeyword.value.trim().toLowerCase()
  const volumeList = props.treeData?.volume_list ?? []
  if (!keyword) {
    return volumeList
  }
  return volumeList
    .map((volumeNode) => ({
      ...volumeNode,
      chapter_list: volumeNode.chapter_list.filter((chapterItem) =>
        chapterItem.chapter_name.toLowerCase().includes(keyword),
      ),
    }))
    .filter(
      (volumeNode) =>
        volumeNode.volume_vo.volume_name.toLowerCase().includes(keyword) ||
        volumeNode.chapter_list.length > 0,
    )
})

/** 全部章节数 */
const totalChapterCount = computed(() =>
  (props.treeData?.volume_list ?? []).reduce(
    (total, volumeNode) => total + volumeNode.chapter_list.length,
    0,
  ),
)

/** 卷是否折叠（搜索时强制展开） */
function isVolumeCollapsed(volumeUlid: string): boolean {
  if (searchKeyword.value.trim()) {
    return false
  }
  return collapsedVolumeMap.value[volumeUlid] === true
}

function toggleVolume(volumeUlid: string): void {
  collapsedVolumeMap.value = {
    ...collapsedVolumeMap.value,
    [volumeUlid]: !collapsedVolumeMap.value[volumeUlid],
  }
}

/** 章状态转展示文案 */
function chapterStatusText(status: string): string {
  switch (status) {
    case 'draft':
      return t('workspace.tree.statusDraft')
    case 'revised':
      return t('workspace.tree.statusRevised')
    case 'locked':
      return t('workspace.tree.statusLocked')
    default:
      return status
  }
}

/** 打开新建卷弹窗 */
function openCreateVolume(): void {
  volumeEditingFlag.value = false
  editingVolumeUlid.value = ''
  volumeForm.volume_name = ''
  volumeForm.summary_text = ''
  volumeDialogVisible.value = true
}

/** 打开编辑卷弹窗（重命名 / 概要） */
function openEditVolume(volumeVo: NWVolumeVo): void {
  volumeEditingFlag.value = true
  editingVolumeUlid.value = volumeVo.volume_ulid
  volumeForm.volume_name = volumeVo.volume_name
  volumeForm.summary_text = volumeVo.summary_text
  volumeDialogVisible.value = true
}

/** 保存卷（新建 / 编辑共用） */
async function handleVolumeSubmit(): Promise<void> {
  if (!volumeFormRef.value) {
    return
  }
  try {
    await volumeFormRef.value.validate()
  } catch {
    return
  }
  actionLoading.value = true
  try {
    if (volumeEditingFlag.value) {
      await updateVolume({
        volume_ulid: editingVolumeUlid.value,
        volume_name: volumeForm.volume_name,
        summary_text: volumeForm.summary_text,
      })
    } else {
      await createVolume({
        project_ulid: props.projectUlid,
        volume_name: volumeForm.volume_name,
        summary_text: volumeForm.summary_text,
      })
    }
    ElMessage.success(t('workspace.tree.saveSuccess'))
    volumeDialogVisible.value = false
    emit('tree-changed')
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    actionLoading.value = false
  }
}

/** 删除卷（二次确认，级联删章节） */
async function handleDeleteVolume(volumeVo: NWVolumeVo): Promise<void> {
  try {
    await ElMessageBox.confirm(
      t('workspace.tree.deleteVolumeConfirm'),
      t('common.delete'),
      { type: 'warning' },
    )
  } catch {
    return
  }
  actionLoading.value = true
  try {
    await deleteVolume(volumeVo.volume_ulid)
    ElMessage.success(t('workspace.tree.deleteSuccess'))
    emit('tree-changed')
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    actionLoading.value = false
  }
}

/** 卷下拉命令分发 */
function handleVolumeCommand(command: string | number | object, volumeVo: NWVolumeVo): void {
  switch (String(command)) {
    case 'add-chapter':
      openCreateChapter(volumeVo.volume_ulid)
      break
    case 'edit-volume':
      openEditVolume(volumeVo)
      break
    case 'delete-volume':
      void handleDeleteVolume(volumeVo)
      break
  }
}

/** 打开新建章节弹窗 */
function openCreateChapter(volumeUlid: string): void {
  chapterEditingFlag.value = false
  editingChapterUlid.value = ''
  targetVolumeUlid.value = volumeUlid
  chapterForm.chapter_name = ''
  chapterDialogVisible.value = true
}

/** 打开重命名章节弹窗 */
function openRenameChapter(chapter: NWChapterBriefVo, volumeUlid: string): void {
  chapterEditingFlag.value = true
  editingChapterUlid.value = chapter.chapter_ulid
  targetVolumeUlid.value = volumeUlid
  chapterForm.chapter_name = chapter.chapter_name
  chapterDialogVisible.value = true
}

/** 保存章节（新建 / 重命名共用） */
async function handleChapterSubmit(): Promise<void> {
  if (!chapterFormRef.value) {
    return
  }
  try {
    await chapterFormRef.value.validate()
  } catch {
    return
  }
  actionLoading.value = true
  try {
    if (chapterEditingFlag.value) {
      await updateChapter({
        chapter_ulid: editingChapterUlid.value,
        chapter_name: chapterForm.chapter_name,
      })
      emit('chapter-renamed', editingChapterUlid.value, chapterForm.chapter_name)
    } else {
      const chapterVo = await createChapter({
        project_ulid: props.projectUlid,
        volume_ulid: targetVolumeUlid.value,
        chapter_name: chapterForm.chapter_name,
      })
      emit('chapter-created', chapterVo.chapter_ulid)
    }
    ElMessage.success(t('workspace.tree.saveSuccess'))
    chapterDialogVisible.value = false
    emit('tree-changed')
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    actionLoading.value = false
  }
}

/** 删除章节（二次确认） */
async function handleDeleteChapter(chapter: NWChapterBriefVo): Promise<void> {
  try {
    await ElMessageBox.confirm(
      t('workspace.tree.deleteChapterConfirm'),
      t('common.delete'),
      { type: 'warning' },
    )
  } catch {
    return
  }
  actionLoading.value = true
  try {
    await deleteChapter(chapter.chapter_ulid)
    ElMessage.success(t('workspace.tree.deleteSuccess'))
    emit('chapter-deleted', chapter.chapter_ulid)
    emit('tree-changed')
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    actionLoading.value = false
  }
}

/** 导出项目目录为 XLSX 大纲（卷/章/排序/字数/状态）。 */
async function handleExportOutline(): Promise<void> {
  await exportProjectOutline(props.treeData, chapterStatusText, {
    volume: t('workspace.tree.exportVolume'),
    chapter: t('workspace.tree.exportChapter'),
    order: t('workspace.tree.exportOrder'),
    words: t('workspace.tree.exportWords'),
    status: t('workspace.tree.exportStatus'),
    sheetName: t('workspace.tree.exportSheetName'),
    fileName: t('workspace.tree.exportFileName'),
    success: t('workspace.tree.exportSuccess'),
    failed: t('workspace.tree.exportFailed'),
    empty: t('workspace.tree.exportEmpty'),
  })
}

/** 章节下拉命令分发 */
function handleChapterCommand(
  command: string | number | object,
  chapter: NWChapterBriefVo,
  volumeUlid: string,
): void {
  switch (String(command)) {
    case 'rename-chapter':
      openRenameChapter(chapter, volumeUlid)
      break
    case 'delete-chapter':
      void handleDeleteChapter(chapter)
      break
  }
}
</script>

<template>
  <div class="workspace-tree">
    <div class="workspace-tree__head">
      <div class="workspace-tree__title-row">
        <span class="workspace-tree__title">{{ t('workspace.tree.title') }}</span>
        <span class="workspace-tree__count">{{ totalChapterCount }} {{ t('layout.chaptersUnit') }}</span>
      </div>
      <div class="workspace-tree__actions">
        <el-tooltip :content="t('workspace.tree.exportOutline')" placement="bottom">
          <button type="button" class="workspace-tree__icon-btn" @click="handleExportOutline">
            <NWIcon icon_code="fileText" :size_px="15" />
          </button>
        </el-tooltip>
        <NWButton size="small" type="primary" @click="openCreateVolume">
          <NWIcon icon_code="plus" :size_px="13" />
          {{ t('workspace.tree.addVolume') }}
        </NWButton>
      </div>
    </div>

    <div class="workspace-tree__search">
      <el-input
        v-model="searchKeyword"
        size="small"
        clearable
        :placeholder="t('workspace.tree.searchPlaceholder')"
      >
        <template #prefix><NWIcon icon_code="search" :size_px="13" /></template>
      </el-input>
    </div>

    <NWEmpty
      v-if="!treeData || treeData.volume_list.length === 0"
      compact_flag
      icon_code="folderOpen"
      :title_text="t('workspace.tree.emptyTitle')"
      :description_text="t('workspace.tree.empty')"
    >
      <template #actions>
        <NWButton size="small" type="primary" @click="openCreateVolume">
          {{ t('workspace.tree.addVolume') }}
        </NWButton>
      </template>
    </NWEmpty>
    <NWEmpty
      v-else-if="filteredVolumeList.length === 0"
      compact_flag
      icon_code="search"
      :title_text="t('workspace.tree.searchEmpty')"
    />

    <div v-else class="workspace-tree__scroll">
      <section
        v-for="volumeNode in filteredVolumeList"
        :key="volumeNode.volume_vo.volume_ulid"
        class="tree-volume"
      >
        <header class="tree-volume__header" @click="toggleVolume(volumeNode.volume_vo.volume_ulid)">
          <NWIcon
            :icon_code="isVolumeCollapsed(volumeNode.volume_vo.volume_ulid) ? 'chevronRight' : 'chevronDown'"
            :size_px="13"
            class="tree-volume__chevron"
          />
          <span class="tree-volume__name" :title="volumeNode.volume_vo.volume_name">
            {{ volumeNode.volume_vo.volume_name }}
          </span>
          <span class="tree-volume__count">{{ volumeNode.chapter_list.length }}</span>
          <el-dropdown trigger="click" @command="handleVolumeCommand($event, volumeNode.volume_vo)">
            <button type="button" class="tree-icon-button" :aria-label="t('common.action')" @click.stop>
              <NWIcon icon_code="more" :size_px="15" />
            </button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="add-chapter">{{ t('workspace.tree.addChapter') }}</el-dropdown-item>
                <el-dropdown-item command="edit-volume">{{ t('workspace.tree.editVolume') }}</el-dropdown-item>
                <el-dropdown-item command="delete-volume" divided>{{ t('common.delete') }}</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </header>

        <ul v-show="!isVolumeCollapsed(volumeNode.volume_vo.volume_ulid)" class="tree-volume__chapters">
          <li
            v-for="chapter in volumeNode.chapter_list"
            :key="chapter.chapter_ulid"
            class="tree-chapter"
            :class="{ 'tree-chapter--active': chapter.chapter_ulid === currentChapterUlid }"
            @click="emit('select-chapter', chapter.chapter_ulid)"
          >
            <span class="tree-chapter__name" :title="chapter.chapter_name">{{ chapter.chapter_name }}</span>
            <span class="tree-chapter__meta">
              <span class="tree-chapter__words">{{ chapter.word_count }}</span>
              <span class="tree-chapter__status" :class="`tree-chapter__status--${chapter.chapter_status}`">
                {{ chapterStatusText(chapter.chapter_status) }}
              </span>
              <el-dropdown trigger="click" @command="handleChapterCommand($event, chapter, volumeNode.volume_vo.volume_ulid)">
                <button type="button" class="tree-icon-button" :aria-label="t('common.action')" @click.stop>
                  <NWIcon icon_code="more" :size_px="14" />
                </button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item command="rename-chapter">{{ t('workspace.tree.renameChapter') }}</el-dropdown-item>
                    <el-dropdown-item command="delete-chapter" divided>{{ t('common.delete') }}</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </span>
          </li>
        </ul>

        <button
          v-if="!isVolumeCollapsed(volumeNode.volume_vo.volume_ulid) && volumeNode.chapter_list.length === 0"
          type="button"
          class="tree-volume__empty-add"
          @click="openCreateChapter(volumeNode.volume_vo.volume_ulid)"
        >
          <NWIcon icon_code="plus" :size_px="12" />
          {{ t('workspace.tree.addChapter') }}
        </button>
      </section>
    </div>

    <el-dialog
      v-model="volumeDialogVisible"
      :title="volumeEditingFlag ? t('workspace.tree.editVolumeTitle') : t('workspace.tree.createVolumeTitle')"
      width="480px"
    >
      <el-form ref="volumeFormRef" :model="volumeForm" :rules="volumeRules" label-position="top">
        <el-form-item :label="t('workspace.tree.volumeName')" prop="volume_name">
          <el-input v-model="volumeForm.volume_name" maxlength="128" />
        </el-form-item>
        <el-form-item :label="t('workspace.tree.volumeSummary')" prop="summary_text">
          <el-input v-model="volumeForm.summary_text" type="textarea" :rows="3" maxlength="2048" />
        </el-form-item>
      </el-form>
      <template #footer>
        <NWButton @click="volumeDialogVisible = false">{{ t('common.cancel') }}</NWButton>
        <NWButton type="primary" :loading="actionLoading" @click="handleVolumeSubmit">{{ t('common.save') }}</NWButton>
      </template>
    </el-dialog>

    <el-dialog
      v-model="chapterDialogVisible"
      :title="chapterEditingFlag ? t('workspace.tree.renameChapterTitle') : t('workspace.tree.createChapterTitle')"
      width="480px"
    >
      <el-form ref="chapterFormRef" :model="chapterForm" :rules="chapterRules" label-position="top">
        <el-form-item :label="t('workspace.tree.chapterName')" prop="chapter_name">
          <el-input v-model="chapterForm.chapter_name" maxlength="128" />
        </el-form-item>
      </el-form>
      <template #footer>
        <NWButton @click="chapterDialogVisible = false">{{ t('common.cancel') }}</NWButton>
        <NWButton type="primary" :loading="actionLoading" @click="handleChapterSubmit">{{ t('common.save') }}</NWButton>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped src="./WorkspaceTree.css"></style>

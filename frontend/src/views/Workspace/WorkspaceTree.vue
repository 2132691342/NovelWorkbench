<!-- 卷章目录树：增删改卷章、搜索与折叠。 -->
<script setup lang="ts">
  import { computed, reactive, ref } from 'vue'
  import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
  import { createChapter, deleteChapter, updateChapter } from '@/api/chapter'
  import { createVolume, deleteVolume, updateVolume } from '@/api/project'
  import type { NWChapterBriefVo, NWProjectTreeVo, NWVolumeVo } from '@/api/contracts'

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

  const searchKeyword = ref('')
  const collapsedVolumeMap = ref<Record<string, boolean>>({})
  const actionLoading = ref(false)

  const volumeDialogVisible = ref(false)
  const editingVolumeUlid = ref('')
  const volumeFormRef = ref<FormInstance>()
  const volumeForm = reactive({ volume_name: '', summary_text: '' })

  const chapterDialogVisible = ref(false)
  const editingChapterUlid = ref('')
  const targetVolumeUlid = ref('')
  const chapterFormRef = ref<FormInstance>()
  const chapterForm = reactive({ chapter_name: '' })

  const volumeRules: FormRules = {
    volume_name: [{ required: true, message: '请输入卷名', trigger: 'blur' }]
  }
  const chapterRules: FormRules = {
    chapter_name: [{ required: true, message: '请输入章节名', trigger: 'blur' }]
  }

  /** 关键词过滤后的目录（卷名或章名命中即保留该卷） */
  const filteredVolumeList = computed(() => {
    const keyword = searchKeyword.value.trim().toLowerCase()
    const volumeList = props.treeData?.volume_list ?? []
    if (!keyword) return volumeList
    return volumeList
      .map((volumeNode) => ({
        ...volumeNode,
        chapter_list: volumeNode.chapter_list.filter((chapter) =>
          chapter.chapter_name.toLowerCase().includes(keyword)
        )
      }))
      .filter(
        (volumeNode) =>
          volumeNode.volume_vo.volume_name.toLowerCase().includes(keyword) ||
          volumeNode.chapter_list.length > 0
      )
  })

  const totalChapterCount = computed(() =>
    (props.treeData?.volume_list ?? []).reduce(
      (total, volumeNode) => total + volumeNode.chapter_list.length,
      0
    )
  )

  const isVolumeCollapsed = (volumeUlid: string) =>
    searchKeyword.value.trim().length === 0 && collapsedVolumeMap.value[volumeUlid] === true

  const toggleVolume = (volumeUlid: string) => {
    collapsedVolumeMap.value = {
      ...collapsedVolumeMap.value,
      [volumeUlid]: !collapsedVolumeMap.value[volumeUlid]
    }
  }

  const statusLabel = (statusCode: string) =>
    ({ draft: '草稿', revised: '已修订', locked: '已锁定' })[statusCode] ?? statusCode

  const openCreateVolume = () => {
    editingVolumeUlid.value = ''
    volumeForm.volume_name = ''
    volumeForm.summary_text = ''
    volumeDialogVisible.value = true
  }

  const openEditVolume = (volume: NWVolumeVo) => {
    editingVolumeUlid.value = volume.volume_ulid
    volumeForm.volume_name = volume.volume_name
    volumeForm.summary_text = volume.summary_text
    volumeDialogVisible.value = true
  }

  const submitVolume = async () => {
    const valid = await volumeFormRef.value?.validate().catch(() => false)
    if (!valid) return
    actionLoading.value = true
    try {
      if (editingVolumeUlid.value) {
        await updateVolume({
          volume_ulid: editingVolumeUlid.value,
          volume_name: volumeForm.volume_name,
          summary_text: volumeForm.summary_text
        })
      } else {
        await createVolume({
          project_ulid: props.projectUlid,
          volume_name: volumeForm.volume_name,
          summary_text: volumeForm.summary_text
        })
      }
      ElMessage.success('已保存')
      volumeDialogVisible.value = false
      emit('tree-changed')
    } finally {
      actionLoading.value = false
    }
  }

  const removeVolume = async (volume: NWVolumeVo) => {
    try {
      await ElMessageBox.confirm(
        `将删除「${volume.volume_name}」及其下全部章节，确定吗？`,
        '删除卷',
        { type: 'warning' }
      )
    } catch {
      return
    }
    actionLoading.value = true
    try {
      await deleteVolume(volume.volume_ulid)
      ElMessage.success('已删除')
      emit('tree-changed')
    } finally {
      actionLoading.value = false
    }
  }

  const handleVolumeCommand = (command: string | number | object, volume: NWVolumeVo) => {
    if (command === 'add-chapter') openCreateChapter(volume.volume_ulid)
    if (command === 'edit-volume') openEditVolume(volume)
    if (command === 'delete-volume') void removeVolume(volume)
  }

  const openCreateChapter = (volumeUlid: string) => {
    editingChapterUlid.value = ''
    targetVolumeUlid.value = volumeUlid
    chapterForm.chapter_name = ''
    chapterDialogVisible.value = true
  }

  const openRenameChapter = (chapter: NWChapterBriefVo, volumeUlid: string) => {
    editingChapterUlid.value = chapter.chapter_ulid
    targetVolumeUlid.value = volumeUlid
    chapterForm.chapter_name = chapter.chapter_name
    chapterDialogVisible.value = true
  }

  const submitChapter = async () => {
    const valid = await chapterFormRef.value?.validate().catch(() => false)
    if (!valid) return
    actionLoading.value = true
    try {
      if (editingChapterUlid.value) {
        await updateChapter({
          chapter_ulid: editingChapterUlid.value,
          chapter_name: chapterForm.chapter_name
        })
        emit('chapter-renamed', editingChapterUlid.value, chapterForm.chapter_name)
      } else {
        const chapter = await createChapter({
          project_ulid: props.projectUlid,
          volume_ulid: targetVolumeUlid.value,
          chapter_name: chapterForm.chapter_name
        })
        emit('chapter-created', chapter.chapter_ulid)
      }
      ElMessage.success('已保存')
      chapterDialogVisible.value = false
      emit('tree-changed')
    } finally {
      actionLoading.value = false
    }
  }

  const removeChapter = async (chapter: NWChapterBriefVo) => {
    try {
      await ElMessageBox.confirm(`将删除「${chapter.chapter_name}」及其正文，确定吗？`, '删除章节', {
        type: 'warning'
      })
    } catch {
      return
    }
    actionLoading.value = true
    try {
      await deleteChapter(chapter.chapter_ulid)
      ElMessage.success('已删除')
      emit('chapter-deleted', chapter.chapter_ulid)
      emit('tree-changed')
    } finally {
      actionLoading.value = false
    }
  }
</script>

<template>
  <section class="tree">
    <header class="tree__head">
      <ElInput v-model="searchKeyword" size="small" placeholder="搜索卷或章节" clearable />
      <ElButton size="small" plain :loading="actionLoading" @click="openCreateVolume">
        <Icon icon="mdi:plus" :size="14" />
        <span class="ml-4px">新卷</span>
      </ElButton>
    </header>

    <p class="tree__meta">{{ filteredVolumeList.length }} 卷 / {{ totalChapterCount }} 章</p>

    <div class="tree__body">
      <div v-for="volumeNode in filteredVolumeList" :key="volumeNode.volume_vo.volume_ulid" class="tree__volume">
        <div class="tree__volume-head">
          <button type="button" class="tree__toggle" @click="toggleVolume(volumeNode.volume_vo.volume_ulid)">
            {{ isVolumeCollapsed(volumeNode.volume_vo.volume_ulid) ? '▸' : '▾' }}
          </button>
          <span class="tree__volume-name" :title="volumeNode.volume_vo.volume_name">
            {{ volumeNode.volume_vo.volume_name }}
          </span>
          <ElDropdown trigger="click" @command="(command: string | number | object) => handleVolumeCommand(command, volumeNode.volume_vo)">
            <button type="button" class="tree__more">
              <Icon icon="mdi:dots-horizontal" :size="15" />
            </button>
            <template #dropdown>
              <ElDropdownMenu>
                <ElDropdownItem command="add-chapter">新建章节</ElDropdownItem>
                <ElDropdownItem command="edit-volume">重命名 / 概要</ElDropdownItem>
                <ElDropdownItem divided command="delete-volume">删除卷</ElDropdownItem>
              </ElDropdownMenu>
            </template>
          </ElDropdown>
        </div>

        <div v-show="!isVolumeCollapsed(volumeNode.volume_vo.volume_ulid)" class="tree__chapters">
          <div
            v-for="chapter in volumeNode.chapter_list"
            :key="chapter.chapter_ulid"
            :class="['tree__chapter', { 'is-active': chapter.chapter_ulid === currentChapterUlid }]"
            @click="emit('select-chapter', chapter.chapter_ulid)"
          >
            <span class="tree__chapter-name" :title="chapter.chapter_name">
              {{ chapter.chapter_name }}
            </span>
            <span class="tree__chapter-meta">{{ chapter.word_count.toLocaleString() }} 字</span>
            <span class="nw-badge nw-badge--neutral">{{ statusLabel(chapter.chapter_status) }}</span>
            <ElDropdown
              trigger="click"
              @command="
                (command: string | number | object) =>
                  command === 'rename'
                    ? openRenameChapter(chapter, volumeNode.volume_vo.volume_ulid)
                    : removeChapter(chapter)
              "
            >
              <button type="button" class="tree__more" @click.stop>
                <Icon icon="mdi:dots-horizontal" :size="14" />
              </button>
              <template #dropdown>
                <ElDropdownMenu>
                  <ElDropdownItem command="rename">重命名</ElDropdownItem>
                  <ElDropdownItem divided command="delete">删除</ElDropdownItem>
                </ElDropdownMenu>
              </template>
            </ElDropdown>
          </div>

          <button
            v-if="volumeNode.chapter_list.length === 0"
            type="button"
            class="tree__add"
            @click="openCreateChapter(volumeNode.volume_vo.volume_ulid)"
          >
            + 新建章节
          </button>
        </div>
      </div>

      <ElEmpty v-if="filteredVolumeList.length === 0" description="还没有卷，先新建一卷" :image-size="60" />
    </div>

    <ElDialog v-model="volumeDialogVisible" :title="editingVolumeUlid ? '编辑卷' : '新建卷'" width="460px">
      <ElForm ref="volumeFormRef" :model="volumeForm" :rules="volumeRules" label-position="top">
        <ElFormItem label="卷名" prop="volume_name">
          <ElInput v-model="volumeForm.volume_name" maxlength="80" />
        </ElFormItem>
        <ElFormItem label="卷概要">
          <ElInput v-model="volumeForm.summary_text" type="textarea" :rows="3" maxlength="500" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="volumeDialogVisible = false">取消</ElButton>
        <ElButton type="primary" :loading="actionLoading" @click="submitVolume">保存</ElButton>
      </template>
    </ElDialog>

    <ElDialog
      v-model="chapterDialogVisible"
      :title="editingChapterUlid ? '重命名章节' : '新建章节'"
      width="420px"
    >
      <ElForm ref="chapterFormRef" :model="chapterForm" :rules="chapterRules" label-position="top">
        <ElFormItem label="章节名" prop="chapter_name">
          <ElInput v-model="chapterForm.chapter_name" maxlength="128" @keyup.enter="submitChapter" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="chapterDialogVisible = false">取消</ElButton>
        <ElButton type="primary" :loading="actionLoading" @click="submitChapter">保存</ElButton>
      </template>
    </ElDialog>
  </section>
</template>

<style lang="less" scoped>
  .tree {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    background: var(--nw-surface);
    border-right: 1px solid var(--nw-border);

    &__head {
      display: flex;
      flex: none;
      gap: 6px;
      align-items: center;
      padding: 8px 10px;
      border-bottom: 1px solid var(--nw-border-soft);
    }

    &__meta {
      flex: none;
      padding: 6px 12px;
      font-size: 11.5px;
      color: var(--el-text-color-placeholder);
    }

    &__body {
      flex: 1;
      min-height: 0;
      padding: 0 6px 16px;
      overflow-y: auto;
    }

    &__volume {
      margin-bottom: 4px;
    }

    &__volume-head {
      display: flex;
      gap: 4px;
      align-items: center;
      height: 32px;
      padding: 0 4px;
      border-radius: var(--nw-radius-base);

      &:hover {
        background: var(--el-fill-color-light);
      }
    }

    &__toggle {
      width: 16px;
      padding: 0;
      font-size: 11px;
      color: var(--el-text-color-placeholder);
      cursor: pointer;
      background: transparent;
      border: 0;
    }

    &__volume-name {
      flex: 1;
      overflow: hidden;
      font-size: 12.5px;
      font-weight: 650;
      color: var(--nw-ink);
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    &__more {
      display: grid;
      padding: 2px;
      color: var(--el-text-color-placeholder);
      cursor: pointer;
      background: transparent;
      border: 0;
      border-radius: var(--nw-radius-sm);
      opacity: 0;
      place-items: center;
    }

    &__volume-head:hover &__more,
    &__chapter:hover &__more {
      opacity: 1;
    }

    &__chapters {
      display: flex;
      flex-direction: column;
      gap: 1px;
      padding-left: 14px;
    }

    &__chapter {
      display: flex;
      gap: 6px;
      align-items: center;
      height: 30px;
      padding: 0 6px;
      cursor: pointer;
      border-radius: var(--nw-radius-base);
      transition: background-color 0.14s ease;

      &:hover {
        background: var(--el-fill-color-light);
      }

      &.is-active {
        background: var(--nw-primary-soft);

        .tree__chapter-name {
          font-weight: 600;
          color: var(--el-color-primary);
        }
      }
    }

    &__chapter-name {
      flex: 1;
      overflow: hidden;
      font-size: 12.5px;
      color: var(--el-text-color-regular);
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    &__chapter-meta {
      font-size: 10.5px;
      color: var(--el-text-color-placeholder);
    }

    &__add {
      height: 28px;
      margin-top: 2px;
      font-size: 12px;
      color: var(--el-color-primary);
      text-align: left;
      cursor: pointer;
      background: transparent;
      border: 1px dashed var(--nw-border);
      border-radius: var(--nw-radius-base);

      &:hover {
        background: var(--nw-primary-soft);
      }
    }
  }
</style>

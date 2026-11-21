<!-- 快照弹窗：查看历史快照并执行回滚。 -->
<script setup lang="ts">
  import { ref, watch } from 'vue'
  import { ElMessage, ElMessageBox } from 'element-plus'
  import {
    createChapterSnapshot,
    listChapterSnapshots,
    restoreChapterSnapshot
  } from '@/api/chapter'
  import type { NWChapterDetailVo, NWChapterSnapshotVo } from '@/api/contracts'
  import { formatTime } from '@/utils/time'

  const props = defineProps<{ visible: boolean; chapterUlid: string }>()

  const emit = defineEmits<{
    'update:visible': [visible: boolean]
    restored: [chapterDetail: NWChapterDetailVo]
  }>()

  const snapshotList = ref<NWChapterSnapshotVo[]>([])
  const listLoading = ref(false)
  const actionLoading = ref(false)
  const changeNote = ref('')

  const typeLabel = (typeCode: string) =>
    ({ manual: '手动留档', ai_before: 'AI 改写前', ai_after: 'AI 改写后' })[typeCode] ?? typeCode

  const loadSnapshotList = async () => {
    if (!props.chapterUlid) {
      snapshotList.value = []
      return
    }
    listLoading.value = true
    try {
      snapshotList.value = await listChapterSnapshots(props.chapterUlid)
    } catch {
      snapshotList.value = []
    } finally {
      listLoading.value = false
    }
  }

  const createSnapshot = async () => {
    actionLoading.value = true
    try {
      await createChapterSnapshot({
        chapter_ulid: props.chapterUlid,
        change_note: changeNote.value
      })
      ElMessage.success('已留档')
      changeNote.value = ''
      await loadSnapshotList()
    } finally {
      actionLoading.value = false
    }
  }

  const restoreSnapshot = async (snapshot: NWChapterSnapshotVo) => {
    try {
      await ElMessageBox.confirm(
        '将用该快照覆盖当前正文，服务端会先自动留档当前版本，确定回滚吗？',
        '回滚快照',
        { type: 'warning' }
      )
    } catch {
      return
    }
    actionLoading.value = true
    try {
      const chapterDetail = await restoreChapterSnapshot({ snapshot_ulid: snapshot.snapshot_ulid })
      ElMessage.success('已回滚')
      emit('restored', chapterDetail)
      emit('update:visible', false)
    } finally {
      actionLoading.value = false
    }
  }

  watch(
    () => props.visible,
    (visible) => {
      if (visible) void loadSnapshotList()
    }
  )
</script>

<template>
  <ElDialog
    :model-value="visible"
    title="章节快照"
    width="560px"
    @update:model-value="emit('update:visible', $event)"
  >
    <div class="snapshot__create">
      <ElInput v-model="changeNote" placeholder="留档说明（可空）" maxlength="120" />
      <ElButton type="primary" plain :loading="actionLoading" @click="createSnapshot">
        留档当前版本
      </ElButton>
    </div>

    <div v-loading="listLoading" class="snapshot__list">
      <div v-for="snapshot in snapshotList" :key="snapshot.snapshot_ulid" class="snapshot__item">
        <div class="snapshot__item-main">
          <span class="nw-badge">{{ typeLabel(snapshot.snapshot_type) }}</span>
          <span class="snapshot__note">{{ snapshot.change_note || '无说明' }}</span>
        </div>
        <div class="snapshot__item-meta">
          <span>{{ snapshot.word_count.toLocaleString() }} 字</span>
          <span>{{ formatTime(snapshot.created_at) }}</span>
          <ElButton size="small" text type="primary" @click="restoreSnapshot(snapshot)">回滚</ElButton>
        </div>
      </div>
      <ElEmpty v-if="!listLoading && snapshotList.length === 0" description="还没有快照" :image-size="60" />
    </div>
  </ElDialog>
</template>

<style lang="less" scoped>
  .snapshot {
    &__create {
      display: flex;
      gap: 8px;
      margin-bottom: 14px;
    }

    &__list {
      max-height: 420px;
      overflow-y: auto;
    }

    &__item {
      padding: 9px 11px;
      margin-bottom: 6px;
      background: var(--nw-surface-subtle);
      border: 1px solid var(--nw-border-soft);
      border-radius: var(--nw-radius-md);
    }

    &__item-main {
      display: flex;
      gap: 8px;
      align-items: center;
    }

    &__note {
      overflow: hidden;
      font-size: 12.5px;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    &__item-meta {
      display: flex;
      gap: 12px;
      align-items: center;
      margin-top: 4px;
      font-size: 11.5px;
      color: var(--el-text-color-placeholder);
    }
  }
</style>

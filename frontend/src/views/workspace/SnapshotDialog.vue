<!--
  章节快照弹窗：快照列表、手动留档、回滚（二次确认，服务端回滚前自动留档当前正文）。
-->
<script setup lang="ts">
import { ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import {
  createChapterSnapshot,
  listChapterSnapshots,
  restoreChapterSnapshot,
} from '@/api/chapter'
import type { NWChapterDetailVo, NWChapterSnapshotVo } from '@/api/contracts'
import NWButton from '@/components/NWButton.vue'
import NWEmpty from '@/components/NWEmpty.vue'
import NWLoading from '@/components/NWLoading.vue'
import { formatTime } from '@/utils/time'

const props = defineProps<{
  visible: boolean
  chapterUlid: string
}>()

const emit = defineEmits<{
  'update:visible': [visible: boolean]
  restored: [chapterDetail: NWChapterDetailVo]
}>()

const { t } = useI18n()

const listLoading = ref(false)
const actionLoading = ref(false)
const snapshotList = ref<NWChapterSnapshotVo[]>([])
const changeNote = ref('')

/** 快照来源转展示文案 */
function snapshotTypeText(snapshotType: string): string {
  switch (snapshotType) {
    case 'manual':
      return t('workspace.snapshot.typeManual')
    case 'ai_before':
      return t('workspace.snapshot.typeAiBefore')
    case 'ai_after':
      return t('workspace.snapshot.typeAiAfter')
    default:
      return snapshotType
  }
}

/** 加载快照列表 */
async function loadSnapshotList(): Promise<void> {
  if (!props.chapterUlid) {
    snapshotList.value = []
    return
  }
  listLoading.value = true
  try {
    const listData = await listChapterSnapshots(props.chapterUlid)
    snapshotList.value = Array.isArray(listData) ? listData : []
  } catch {
    // 失败提示由 http 层统一弹出
    snapshotList.value = []
  } finally {
    listLoading.value = false
  }
}

/** 手动留档 */
async function handleCreateSnapshot(): Promise<void> {
  actionLoading.value = true
  try {
    await createChapterSnapshot({
      chapter_ulid: props.chapterUlid,
      change_note: changeNote.value,
    })
    ElMessage.success(t('workspace.snapshot.createSuccess'))
    changeNote.value = ''
    await loadSnapshotList()
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    actionLoading.value = false
  }
}

/** 回滚到快照（二次确认） */
async function handleRestoreSnapshot(snapshot: NWChapterSnapshotVo): Promise<void> {
  try {
    await ElMessageBox.confirm(
      t('workspace.snapshot.restoreConfirm'),
      t('workspace.snapshot.restore'),
      { type: 'warning' },
    )
  } catch {
    return
  }
  actionLoading.value = true
  try {
    const chapterDetail = await restoreChapterSnapshot({ snapshot_ulid: snapshot.snapshot_ulid })
    ElMessage.success(t('workspace.snapshot.restoreSuccess'))
    emit('restored', chapterDetail)
    emit('update:visible', false)
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    actionLoading.value = false
  }
}

watch(
  () => props.visible,
  (visibleValue) => {
    if (visibleValue) {
      changeNote.value = ''
      void loadSnapshotList()
    }
  },
)
</script>

<template>
  <el-dialog
    :model-value="visible"
    :title="t('workspace.snapshot.title')"
    width="560px"
    @update:model-value="emit('update:visible', $event)"
  >
    <div class="snapshot-dialog__create">
      <el-input
        v-model="changeNote"
        maxlength="256"
        :placeholder="t('workspace.snapshot.changeNotePlaceholder')"
      />
      <NWButton type="primary" :loading="actionLoading" @click="handleCreateSnapshot">
        {{ t('workspace.snapshot.create') }}
      </NWButton>
    </div>

    <NWLoading v-if="listLoading" />
    <NWEmpty
      v-else-if="snapshotList.length === 0"
      :description_text="t('workspace.snapshot.empty')"
    />
    <ul v-else class="snapshot-dialog__list">
      <li
        v-for="snapshot in snapshotList"
        :key="snapshot.snapshot_ulid"
        class="snapshot-item"
      >
        <div class="snapshot-item__main">
          <div class="snapshot-item__line">
            <span class="snapshot-item__type">
              {{ snapshotTypeText(snapshot.snapshot_type) }}
            </span>
            <span class="snapshot-item__note" :title="snapshot.change_note">
              {{ snapshot.change_note || t('workspace.snapshot.title') }}
            </span>
          </div>
          <div class="snapshot-item__meta">
            <span>{{ t('workspace.snapshot.wordCount') }} {{ snapshot.word_count }}</span>
            <span>
              {{ t('workspace.snapshot.createdAt') }}
              {{ formatTime(snapshot.created_at) }}
            </span>
          </div>
        </div>
        <NWButton size="small" :disabled="actionLoading" @click="handleRestoreSnapshot(snapshot)">
          {{ t('workspace.snapshot.restore') }}
        </NWButton>
      </li>
    </ul>
  </el-dialog>
</template>

<style scoped>
.snapshot-dialog__create {
  display: flex;
  gap: 8px;
  margin-bottom: 16px;
}

.snapshot-dialog__create .el-input {
  flex: 1;
}

.snapshot-dialog__list {
  margin: 0;
  padding: 0;
  list-style: none;
  max-height: 360px;
  overflow-y: auto;
}

.snapshot-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border: 1px solid var(--border-light);
  border-radius: var(--radius-base);
  margin-bottom: 8px;
}

.snapshot-item__main {
  flex: 1;
  min-width: 0;
}

.snapshot-item__line {
  display: flex;
  align-items: center;
  gap: 8px;
}

.snapshot-item__type {
  padding: 1px 8px;
  border-radius: var(--radius-pill);
  background-color: var(--primary-light);
  color: var(--primary-active);
  font-size: 12px;
  white-space: nowrap;
}

.snapshot-item__note {
  font-size: 13px;
  color: var(--text-main);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.snapshot-item__meta {
  display: flex;
  gap: 16px;
  margin-top: 4px;
  font-size: 12px;
  color: var(--text-secondary);
}
</style>

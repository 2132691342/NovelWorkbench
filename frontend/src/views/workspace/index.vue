<!--
  写作工作台：顶栏（项目名/字数统计/快照入口/返回仪表盘）+
  三栏布局（左目录树 260px / 中章节编辑器 / 右 AI 面板 380px，左右可折叠）。
-->
<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { getChapterDetail } from '@/api/chapter'
import { getProjectTree } from '@/api/project'
import type {
  NWChapterBriefVo,
  NWChapterDetailVo,
  NWProjectTreeVo,
} from '@/api/contracts'
import NWButton from '@/components/NWButton.vue'
import NWLoading from '@/components/NWLoading.vue'
import AiPanel from './AiPanel.vue'
import ChapterEditor from './ChapterEditor.vue'
import SnapshotDialog from './SnapshotDialog.vue'
import WorkspaceTree from './WorkspaceTree.vue'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()

const projectUlid = computed(() => String(route.params.project_ulid ?? ''))

const editorRef = ref<InstanceType<typeof ChapterEditor> | null>(null)
const aiPanelRef = ref<InstanceType<typeof AiPanel> | null>(null)
const treeData = ref<NWProjectTreeVo | null>(null)
const currentChapterDetail = ref<NWChapterDetailVo | null>(null)
const treeLoading = ref(false)
const liveWordCount = ref(0)
const leftCollapsed = ref(false)
const rightCollapsed = ref(false)
const snapshotDialogVisible = ref(false)

/** 扁平章节列表（供 AI 面板 @引用章节页签） */
const flatChapterList = computed<NWChapterBriefVo[]>(() => {
  const chapterList: NWChapterBriefVo[] = []
  for (const volumeNode of treeData.value?.volume_list ?? []) {
    chapterList.push(...volumeNode.chapter_list)
  }
  return chapterList
})

const projectWordCount = computed(() => treeData.value?.project_vo.total_word_count ?? 0)

/** 加载目录树；当前章节被删除时清空编辑器，初次加载自动选中第一章 */
async function loadTree(initialFlag = false): Promise<void> {
  if (!projectUlid.value) {
    return
  }
  treeLoading.value = true
  try {
    treeData.value = await getProjectTree(projectUlid.value)
    const currentUlid = currentChapterDetail.value?.chapter_ulid ?? ''
    const stillExists = flatChapterList.value.some((item) => item.chapter_ulid === currentUlid)
    if (currentUlid && !stillExists) {
      currentChapterDetail.value = null
      liveWordCount.value = 0
    }
    if (initialFlag && !currentChapterDetail.value && flatChapterList.value.length > 0) {
      await selectChapter(flatChapterList.value[0].chapter_ulid)
    }
  } catch {
    // 失败提示由 http 层统一弹出；接口未就绪保持空态
    treeData.value = null
  } finally {
    treeLoading.value = false
  }
}

/** 切换章节：先落盘当前防抖内容，再拉取章节详情 */
async function selectChapter(chapterUlid: string): Promise<void> {
  if (currentChapterDetail.value?.chapter_ulid === chapterUlid) {
    return
  }
  await editorRef.value?.flushSave()
  try {
    currentChapterDetail.value = await getChapterDetail(chapterUlid)
    liveWordCount.value = 0
  } catch {
    // 失败提示由 http 层统一弹出
  }
}

/** 目录树结构变更（增删改卷章）后重载 */
function handleTreeChanged(): void {
  void loadTree()
}

/** 目录树重命名同步编辑器标题 */
function handleChapterRenamed(chapterUlid: string, chapterName: string): void {
  if (currentChapterDetail.value?.chapter_ulid === chapterUlid) {
    currentChapterDetail.value.chapter_name = chapterName
  }
}

/** 删除的是当前章节时清空编辑器 */
function handleChapterDeleted(chapterUlid: string): void {
  if (currentChapterDetail.value?.chapter_ulid === chapterUlid) {
    currentChapterDetail.value = null
    liveWordCount.value = 0
  }
}

/** 自动保存成功：同步目录树的字数/状态/名称 */
function handleSaveSuccess(payload: {
  chapter_ulid: string
  chapter_name: string
  word_count: number
  chapter_status: string
}): void {
  for (const volumeNode of treeData.value?.volume_list ?? []) {
    const chapterItem = volumeNode.chapter_list.find(
      (item) => item.chapter_ulid === payload.chapter_ulid,
    )
    if (chapterItem) {
      chapterItem.chapter_name = payload.chapter_name
      chapterItem.word_count = payload.word_count
      chapterItem.chapter_status = payload.chapter_status
      break
    }
  }
}

function handleWordCountChange(wordCount: number): void {
  liveWordCount.value = wordCount
}

/** AI 续写：预置 skill_code=continue + 引用当前章节 */
function handleAiContinue(): void {
  const refList = currentChapterDetail.value ? [currentChapterDetail.value.chapter_ulid] : []
  aiPanelRef.value?.presetSkill('continue', refList)
}

/** AI 润色：预置 skill_code=polish + 引用当前章节 */
function handleAiPolish(): void {
  const refList = currentChapterDetail.value ? [currentChapterDetail.value.chapter_ulid] : []
  aiPanelRef.value?.presetSkill('polish', refList)
}

/** AI 回复插入编辑器光标处（走编辑器同一自动保存通道） */
function handleInsertToChapter(contentText: string): void {
  if (!currentChapterDetail.value) {
    ElMessage.warning(t('workspace.insertNoChapter'))
    return
  }
  editorRef.value?.insertTextAtCursor(contentText)
}

/** 快照入口（顶栏与编辑器工具条共用）；打开前先落盘防抖内容，保证留档/回滚基于最新正文 */
async function handleOpenSnapshot(): Promise<void> {
  if (!currentChapterDetail.value) {
    ElMessage.warning(t('workspace.insertNoChapter'))
    return
  }
  await editorRef.value?.flushSave()
  snapshotDialogVisible.value = true
}

/** 回滚成功：重载章节正文与目录树字数 */
function handleSnapshotRestored(chapterDetail: NWChapterDetailVo): void {
  currentChapterDetail.value = chapterDetail
  void loadTree()
}

function handleBack(): void {
  void router.push('/')
}

watch(projectUlid, () => {
  currentChapterDetail.value = null
  liveWordCount.value = 0
  void loadTree(true)
})

onMounted(() => {
  void loadTree(true)
})
</script>

<template>
  <div class="workspace-page">
    <!-- 顶栏 -->
    <header class="workspace-page__topbar">
      <NWButton size="small" @click="handleBack">
        {{ t('workspace.backToDashboard') }}
      </NWButton>
      <h2 class="workspace-page__project-name">
        {{ treeData?.project_vo.project_name ?? '' }}
      </h2>
      <span class="workspace-page__words">
        {{ t('workspace.projectWords') }} {{ projectWordCount }}
        /
        {{ t('workspace.chapterWords') }} {{ liveWordCount }}
      </span>
      <div class="workspace-page__topbar-actions">
        <NWButton size="small" @click="leftCollapsed = !leftCollapsed">
          {{ t('workspace.toggleTree') }}
        </NWButton>
        <NWButton size="small" @click="handleOpenSnapshot">
          {{ t('workspace.snapshotEntry') }}
        </NWButton>
        <NWButton size="small" @click="rightCollapsed = !rightCollapsed">
          {{ t('workspace.toggleAi') }}
        </NWButton>
      </div>
    </header>

    <NWLoading v-if="treeLoading && !treeData" />

    <!-- 三栏主体 -->
    <div v-else class="workspace-page__main">
      <aside v-show="!leftCollapsed" class="workspace-page__left">
        <WorkspaceTree
          :tree-data="treeData"
          :project-ulid="projectUlid"
          :current-chapter-ulid="currentChapterDetail?.chapter_ulid ?? ''"
          @select-chapter="selectChapter"
          @tree-changed="handleTreeChanged"
          @chapter-renamed="handleChapterRenamed"
          @chapter-deleted="handleChapterDeleted"
        />
      </aside>

      <section class="workspace-page__center">
        <ChapterEditor
          ref="editorRef"
          :chapter="currentChapterDetail"
          @save-success="handleSaveSuccess"
          @word-count-change="handleWordCountChange"
          @ai-continue="handleAiContinue"
          @ai-polish="handleAiPolish"
          @open-snapshot="handleOpenSnapshot"
        />
      </section>

      <aside v-show="!rightCollapsed" class="workspace-page__right">
        <AiPanel
          ref="aiPanelRef"
          :project-ulid="projectUlid"
          :chapter-list="flatChapterList"
          @insert-to-chapter="handleInsertToChapter"
        />
      </aside>
    </div>

    <SnapshotDialog
      v-model:visible="snapshotDialogVisible"
      :chapter-ulid="currentChapterDetail?.chapter_ulid ?? ''"
      @restored="handleSnapshotRestored"
    />
  </div>
</template>

<style scoped>
.workspace-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}

.workspace-page__topbar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  margin-bottom: 12px;
  background-color: var(--bg-card);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-card);
}

.workspace-page__project-name {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--text-main);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.workspace-page__words {
  font-size: 13px;
  color: var(--text-secondary);
  white-space: nowrap;
}

.workspace-page__topbar-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-left: auto;
}

.workspace-page__main {
  display: flex;
  flex: 1;
  min-height: 0;
  gap: 12px;
}

.workspace-page__left {
  width: 260px;
  flex-shrink: 0;
  padding: 12px;
  background-color: var(--bg-card);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-card);
  overflow: hidden;
}

.workspace-page__center {
  flex: 1;
  min-width: 0;
  padding: 16px;
  background-color: var(--bg-card);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-card);
  overflow: hidden;
}

.workspace-page__right {
  width: 380px;
  flex-shrink: 0;
  padding: 12px;
  background-color: var(--bg-card);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-card);
  overflow: hidden;
}
</style>

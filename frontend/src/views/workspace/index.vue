<!--
  写作工作台：项目栏 + 三栏布局（目录树 / 章节编辑器 / AI 面板）。
  左右栏可独立折叠；AI 面板默认附带当前章节作为上下文。
-->
<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { getChapterDetail } from '@/api/chapter'
import { getProjectTree } from '@/api/project'
import type { NWChapterBriefVo, NWChapterDetailVo, NWProjectTreeVo } from '@/api/contracts'
import NWButton from '@/components/NWButton.vue'
import NWIcon from '@/components/NWIcon.vue'
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

const projectStatusText = computed(() => {
  switch (treeData.value?.project_vo.project_status) {
    case 'serial':
      return t('dashboard.statusSerial')
    case 'completed':
      return t('dashboard.statusCompleted')
    default:
      return t('dashboard.statusDraft')
  }
})

const currentChapterName = computed(() => currentChapterDetail.value?.chapter_name ?? '')

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

/** 新建章节后自动打开编辑器 */
function handleChapterCreated(chapterUlid: string): void {
  void selectChapter(chapterUlid)
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
      const wordDelta = payload.word_count - chapterItem.word_count
      chapterItem.chapter_name = payload.chapter_name
      chapterItem.word_count = payload.word_count
      chapterItem.chapter_status = payload.chapter_status
      if (treeData.value) {
        treeData.value.project_vo.total_word_count += wordDelta
      }
      break
    }
  }
}

function handleWordCountChange(wordCount: number): void {
  liveWordCount.value = wordCount
}

/** AI 续写：预置 skill_code=continue + 引用当前章节 */
function handleAiContinue(): void {
  if (rightCollapsed.value) {
    rightCollapsed.value = false
  }
  const refList = currentChapterDetail.value ? [currentChapterDetail.value.chapter_ulid] : []
  aiPanelRef.value?.presetSkill('continue', refList)
}

/** AI 润色：预置 skill_code=polish + 引用当前章节 */
function handleAiPolish(): void {
  if (rightCollapsed.value) {
    rightCollapsed.value = false
  }
  const refList = currentChapterDetail.value ? [currentChapterDetail.value.chapter_ulid] : []
  aiPanelRef.value?.presetSkill('polish', refList)
}

/** AI 生成梗概：预置 skill_code=summary + 引用当前章节 */
function handleAiSummary(): void {
  if (rightCollapsed.value) {
    rightCollapsed.value = false
  }
  const refList = currentChapterDetail.value ? [currentChapterDetail.value.chapter_ulid] : []
  aiPanelRef.value?.presetSkill('summary', refList)
}

/** AI 回复插入编辑器光标处（走编辑器同一自动保存通道） */
function handleInsertToChapter(contentText: string): void {
  if (!currentChapterDetail.value) {
    ElMessage.warning(t('workspace.insertNoChapter'))
    return
  }
  editorRef.value?.insertTextAtCursor(contentText)
  ElMessage.success(t('workspace.insertSuccess'))
}

/** 快照入口：打开前先落盘，保证留档/回滚基于最新正文 */
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

/** 专注模式：同时收起左右栏 */
function toggleFocusMode(): void {
  const focusFlag = !(leftCollapsed.value && rightCollapsed.value)
  leftCollapsed.value = focusFlag
  rightCollapsed.value = focusFlag
}

const focusModeFlag = computed(() => leftCollapsed.value && rightCollapsed.value)

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
    <!-- 工作台顶栏 -->
    <header class="workspace-page__topbar">
      <button type="button" class="workspace-page__back" :title="t('workspace.backToDashboard')" @click="handleBack">
        <NWIcon icon_code="arrowLeft" :size_px="16" />
      </button>

      <span class="workspace-page__project-cover">
        {{ (treeData?.project_vo.project_name || 'N').slice(0, 1) }}
      </span>
      <div class="workspace-page__project-info">
        <div class="workspace-page__project-row">
          <h2 class="workspace-page__project-name">{{ treeData?.project_vo.project_name || '' }}</h2>
          <span class="nw-badge nw-badge--primary">{{ projectStatusText }}</span>
        </div>
        <span class="workspace-page__words">
          {{ t('workspace.projectWords') }} <strong>{{ projectWordCount.toLocaleString() }}</strong>
          <span class="workspace-page__word-divider">/</span>
          {{ t('workspace.chapterWords') }} <strong>{{ liveWordCount.toLocaleString() }}</strong>
        </span>
      </div>

      <div class="workspace-page__topbar-actions">
        <el-tooltip :content="t('workspace.focusMode')" placement="bottom">
          <button
            type="button"
            class="workspace-page__icon-btn"
            :class="{ 'is-active': focusModeFlag }"
            @click="toggleFocusMode"
          >
            <NWIcon icon_code="maximize" :size_px="17" />
          </button>
        </el-tooltip>
        <el-tooltip :content="t('workspace.toggleTree')" placement="bottom">
          <button
            type="button"
            class="workspace-page__icon-btn"
            :class="{ 'is-active': !leftCollapsed }"
            @click="leftCollapsed = !leftCollapsed"
          >
            <NWIcon icon_code="panelLeft" :size_px="17" />
          </button>
        </el-tooltip>
        <el-tooltip :content="t('workspace.toggleAi')" placement="bottom">
          <button
            type="button"
            class="workspace-page__icon-btn"
            :class="{ 'is-active': !rightCollapsed }"
            @click="rightCollapsed = !rightCollapsed"
          >
            <NWIcon icon_code="panelRight" :size_px="17" />
          </button>
        </el-tooltip>
        <span class="workspace-page__action-divider" />
        <NWButton size="small" @click="handleOpenSnapshot">
          <NWIcon icon_code="history" :size_px="14" />
          {{ t('workspace.snapshotEntry') }}
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
          @chapter-created="handleChapterCreated"
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
          @ai-summary="handleAiSummary"
          @open-snapshot="handleOpenSnapshot"
        />
      </section>

      <aside v-show="!rightCollapsed" class="workspace-page__right">
        <AiPanel
          ref="aiPanelRef"
          :project-ulid="projectUlid"
          :chapter-list="flatChapterList"
          :current-chapter-ulid="currentChapterDetail?.chapter_ulid ?? ''"
          :current-chapter-name="currentChapterName"
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

<style scoped src="./workspace.css"></style>

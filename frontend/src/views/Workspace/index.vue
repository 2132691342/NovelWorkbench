<!-- 写作台：目录树、正文编辑器与 AI 面板三栏布局。 -->
<script setup lang="ts">
  import { computed, onMounted, ref, watch } from 'vue'
  import { useRouter } from 'vue-router'
  import { ElMessage, ElMessageBox } from 'element-plus'
  import { getChapterDetail } from '@/api/chapter'
  import { getProjectTree } from '@/api/project'
  import type { NWChapterBriefVo, NWChapterDetailVo, NWProjectTreeVo } from '@/api/contracts'
  import { useProjectStore } from '@/store/modules/project'
  import AiPanel from './AiPanel.vue'
  import ChapterCheckPanel from './ChapterCheckPanel.vue'
  import ChapterEditor from './ChapterEditor.vue'
  import SnapshotDialog from './SnapshotDialog.vue'
  import WorkspaceTree from './WorkspaceTree.vue'

  const projectStore = useProjectStore()
  const router = useRouter()

  const projectUlid = computed(() => projectStore.currentProjectUlid)
  const treeData = ref<NWProjectTreeVo | null>(null)
  const currentChapter = ref<NWChapterDetailVo | null>(null)
  const treeLoading = ref(false)
  const liveWordCount = ref(0)
  const leftCollapsed = ref(false)
  const rightCollapsed = ref(false)
  const snapshotVisible = ref(false)

  const editorRef = ref<InstanceType<typeof ChapterEditor> | null>(null)
  const aiPanelRef = ref<InstanceType<typeof AiPanel> | null>(null)
  const checkPanelRef = ref<InstanceType<typeof ChapterCheckPanel> | null>(null)
  const checkVisible = ref(false)

  const flatChapterList = computed<NWChapterBriefVo[]>(() =>
    (treeData.value?.volume_list ?? []).flatMap((volumeNode) => volumeNode.chapter_list)
  )

  const loadTree = async (initialFlag = false) => {
    if (!projectUlid.value) return
    treeLoading.value = true
    try {
      treeData.value = await getProjectTree(projectUlid.value)
      const currentUlid = currentChapter.value?.chapter_ulid ?? ''
      const stillExists = flatChapterList.value.some((item) => item.chapter_ulid === currentUlid)
      if (currentUlid && !stillExists) {
        currentChapter.value = null
        liveWordCount.value = 0
      }
      if (initialFlag && !currentChapter.value && flatChapterList.value.length > 0) {
        await selectChapter(flatChapterList.value[0].chapter_ulid)
      }
    } catch {
      treeData.value = null
    } finally {
      treeLoading.value = false
    }
  }

  const selectChapter = async (chapterUlid: string) => {
    if (currentChapter.value?.chapter_ulid === chapterUlid) return
    await editorRef.value?.flushSave()
    try {
      currentChapter.value = await getChapterDetail(chapterUlid)
      liveWordCount.value = 0
    } catch {
      // 失败提示由请求层统一弹出
    }
  }

  /** 目录树重命名后同步编辑器标题 */
  const handleChapterRenamed = (chapterUlid: string, chapterName: string) => {
    if (currentChapter.value?.chapter_ulid === chapterUlid) {
      currentChapter.value.chapter_name = chapterName
    }
  }

  /** 自动保存成功：同步目录树字数与项目总字数 */
  const handleSaveSuccess = (payload: {
    chapter_ulid: string
    chapter_name: string
    word_count: number
    chapter_status: string
  }) => {
    for (const volumeNode of treeData.value?.volume_list ?? []) {
      const chapter = volumeNode.chapter_list.find(
        (item) => item.chapter_ulid === payload.chapter_ulid
      )
      if (!chapter) continue
      if (treeData.value) {
        treeData.value.project_vo.total_word_count += payload.word_count - chapter.word_count
      }
      chapter.chapter_name = payload.chapter_name
      chapter.word_count = payload.word_count
      chapter.chapter_status = payload.chapter_status
      return
    }
  }

  const handleChapterDeleted = (chapterUlid: string) => {
    if (currentChapter.value?.chapter_ulid === chapterUlid) {
      currentChapter.value = null
      liveWordCount.value = 0
    }
  }

  const openAiWithPrompt = (promptCode: string) => {
    rightCollapsed.value = false
    const refList = currentChapter.value ? [currentChapter.value.chapter_ulid] : []
    aiPanelRef.value?.presetPrompt(promptCode, refList)
  }

  const insertToChapter = (contentText: string) => {
    if (!currentChapter.value) {
      ElMessage.warning('请先打开一个章节')
      return
    }
    editorRef.value?.insertTextAtCursor(contentText)
    ElMessage.success('已插入光标处')
  }

  /** 打开体检面板并对当前正文跑一次门禁 */
  const openCheck = async () => {
    if (!currentChapter.value) {
      ElMessage.warning('请先打开一个章节')
      return
    }
    await editorRef.value?.flushSave()
    checkVisible.value = true
    await checkPanelRef.value?.run()
  }

  /** 顶栏章节动作的可用条件：有当前章且有正文（体检与复审共用同一口径） */
  const chapterActionDisabled = computed(() => !currentChapter.value || liveWordCount.value === 0)

  /** 复审指令：只出问题不重写，正文由 AI 面板的章节引用带上 */
  const REVIEW_INSTRUCTION = `请以编辑身份复审本章，只输出问题不重写正文：
1) 结构与节奏
2) 人物与动机
3) 设定与前后一致
4) AI 味与套话
每类给出「有问题/无明显问题」+ 一句依据；结尾给一句总评与是否可发。`

  /** AI 复审本章：正文走章节引用，已跑过体检则把问题清单一并交给编辑 */
  const askAiReview = async () => {
    if (!currentChapter.value) {
      ElMessage.warning('请先打开一个章节')
      return
    }
    await editorRef.value?.flushSave()
    const issueText = (checkPanelRef.value?.report?.issue_list ?? [])
      .map((issue) => `- ${issue.title_text}：${issue.detail_text}`)
      .join('\n')
    const promptText = issueText
      ? `${REVIEW_INSTRUCTION}\n\n已知体检问题（供参考，不必逐条复述）：\n${issueText}`
      : REVIEW_INSTRUCTION
    askAiWithPrompt(promptText)
  }

  /** 本章开跑工作流：带章深链，工作流页据此预选手选章节 */
  const openWorkflowForChapter = () => {
    if (!projectUlid.value || !currentChapter.value) {
      ElMessage.warning('请先打开一个章节')
      return
    }
    void router.push({
      path: `/workflow/${projectUlid.value}`,
      query: { chapter: currentChapter.value.chapter_ulid }
    })
  }

  /** 体检面板把问题清单交给 AI 面板（带当前章节引用，用去 AI 味提示词改） */
  const askAiWithPrompt = (promptText: string, promptCode = 'deai') => {
    rightCollapsed.value = false
    const refList = currentChapter.value ? [currentChapter.value.chapter_ulid] : []
    aiPanelRef.value?.presetPrompt(promptCode, refList)
    aiPanelRef.value?.presetText(promptText)
  }

  const replaceChapter = async (contentText: string) => {
    if (!currentChapter.value) {
      ElMessage.warning('请先打开一个章节')
      return
    }
    try {
      await ElMessageBox.confirm('将用 AI 回复替换整章正文，确定吗？', '替换整章', {
        type: 'warning'
      })
    } catch {
      return
    }
    editorRef.value?.replaceContent(contentText)
    ElMessage.success('已替换，正在自动保存')
  }

  const openSnapshot = async () => {
    if (!currentChapter.value) {
      ElMessage.warning('请先打开一个章节')
      return
    }
    await editorRef.value?.flushSave()
    snapshotVisible.value = true
  }

  const handleSnapshotRestored = (chapterDetail: NWChapterDetailVo) => {
    currentChapter.value = chapterDetail
    void loadTree()
  }

  const focusMode = computed(() => leftCollapsed.value && rightCollapsed.value)
  const toggleFocusMode = () => {
    const nextFlag = !focusMode.value
    leftCollapsed.value = nextFlag
    rightCollapsed.value = nextFlag
  }

  /** AI 面板转来的斜杠动作：写作台级动作用当前章执行 */
  const handlePanelAction = (actionCode: string) => {
    if (actionCode === 'check') {
      void openCheck()
      return
    }
    if (actionCode === 'focus') {
      toggleFocusMode()
      return
    }
    if (actionCode === 'snapshot') {
      void openSnapshot()
    }
  }

  watch(
    projectUlid,
    () => {
      currentChapter.value = null
      liveWordCount.value = 0
      void loadTree(true)
    },
    { immediate: true }
  )

  onMounted(() => {
    if (!projectUlid.value) void projectStore.loadProjectOptions()
  })
</script>

<template>
  <div class="workspace">
    <header class="workspace__topbar">
      <div class="workspace__project">
        <span class="workspace__cover">
          {{ (treeData?.project_vo.project_name || 'N').slice(0, 1) }}
        </span>
        <div>
          <h2 class="workspace__name">{{ treeData?.project_vo.project_name || '未选择项目' }}</h2>
          <p class="workspace__meta">
            全书 {{ (treeData?.project_vo.total_word_count ?? 0).toLocaleString() }} 字
            <span class="workspace__dot">·</span>
            本章 {{ liveWordCount.toLocaleString() }} 字
          </p>
        </div>
      </div>

      <div class="workspace__actions">
        <ElButton size="small" text @click="toggleFocusMode">
          <Icon icon="mdi:book-open-variant" :size="15" />
          <span class="ml-4px">{{ focusMode ? '退出专注' : '专注' }}</span>
        </ElButton>
        <ElButton size="small" text @click="leftCollapsed = !leftCollapsed">
          <Icon icon="mdi:file-tree" :size="15" />
        </ElButton>
        <ElButton size="small" text @click="rightCollapsed = !rightCollapsed">
          <Icon icon="mdi:robot-outline" :size="15" />
        </ElButton>
        <ElButton size="small" text :disabled="chapterActionDisabled" @click="openCheck">
          <Icon icon="mdi:clipboard-check-outline" :size="15" />
          <span class="ml-4px">体检</span>
        </ElButton>
        <ElButton size="small" text :disabled="chapterActionDisabled" @click="askAiReview()">
          <Icon icon="mdi:shield-check-outline" :size="15" />
          <span class="ml-4px">AI 复审</span>
        </ElButton>
        <ElButton
          type="primary"
          plain
          size="small"
          :disabled="!currentChapter"
          @click="openWorkflowForChapter()"
        >
          <Icon icon="mdi:play-circle-outline" :size="15" />
          <span class="ml-4px">本章开跑</span>
        </ElButton>
      </div>
    </header>

    <div v-loading="treeLoading && !treeData" class="workspace__main">
      <aside v-show="!leftCollapsed" class="workspace__left">
        <WorkspaceTree
          :tree-data="treeData"
          :project-ulid="projectUlid"
          :current-chapter-ulid="currentChapter?.chapter_ulid ?? ''"
          @select-chapter="selectChapter"
          @tree-changed="loadTree()"
          @chapter-created="selectChapter"
          @chapter-renamed="handleChapterRenamed"
          @chapter-deleted="handleChapterDeleted"
        />
      </aside>

      <section class="workspace__center">
        <ChapterEditor
          ref="editorRef"
          :chapter="currentChapter"
          @save-success="handleSaveSuccess"
          @word-count-change="(wordCount) => (liveWordCount = wordCount)"
          @ai-continue="openAiWithPrompt('continue')"
          @ai-polish="openAiWithPrompt('polish')"
          @ai-summary="openAiWithPrompt('summary')"
          @open-snapshot="openSnapshot"
        />
        <ChapterCheckPanel
          v-if="checkVisible"
          ref="checkPanelRef"
          :chapter-ulid="currentChapter?.chapter_ulid ?? ''"
          :content-text="editorRef?.getContentText() ?? ''"
          @ask-ai="askAiWithPrompt"
        />
      </section>

      <aside v-show="!rightCollapsed" class="workspace__right">
        <AiPanel
          ref="aiPanelRef"
          :project-ulid="projectUlid"
          :chapter-list="flatChapterList"
          :current-chapter-ulid="currentChapter?.chapter_ulid ?? ''"
          :current-chapter-name="currentChapter?.chapter_name ?? ''"
          @insert-to-chapter="insertToChapter"
          @replace-chapter="replaceChapter"
          @action="handlePanelAction"
        />
      </aside>
    </div>

    <SnapshotDialog
      v-model:visible="snapshotVisible"
      :chapter-ulid="currentChapter?.chapter_ulid ?? ''"
      @restored="handleSnapshotRestored"
    />
  </div>
</template>

<style lang="less" scoped>
  .workspace {
    display: flex;
    flex-direction: column;
    height: calc(100vh - var(--top-tool-height) - var(--tags-view-height) - 2 * var(--app-content-padding));
    overflow: hidden;
    background: var(--nw-surface);
    border: 1px solid var(--nw-border);
    border-radius: var(--nw-radius-lg);

    &__topbar {
      display: flex;
      flex: none;
      align-items: center;
      justify-content: space-between;
      height: 52px;
      padding: 0 14px;
      border-bottom: 1px solid var(--nw-border-soft);
    }

    &__project {
      display: flex;
      gap: 10px;
      align-items: center;
      min-width: 0;
    }

    &__cover {
      display: grid;
      flex: none;
      width: 32px;
      height: 32px;
      font-size: 14px;
      font-weight: 650;
      color: #fff;
      background: linear-gradient(135deg, var(--saas_dashboard-color-1), var(--saas_dashboard-color-2));
      border-radius: 8px;
      place-items: center;
    }

    &__name {
      overflow: hidden;
      font-size: 14px;
      font-weight: 650;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    &__meta {
      font-size: 11.5px;
      color: var(--el-text-color-placeholder);
    }

    &__dot {
      margin: 0 4px;
    }

    &__actions {
      display: flex;
      gap: 2px;
      align-items: center;
    }

    &__main {
      display: flex;
      flex: 1;
      min-height: 0;
    }

    &__left {
      flex: none;
      width: 240px;
      min-height: 0;
    }

    &__center {
      display: flex;
      flex: 1;
      flex-direction: column;
      min-width: 0;
      min-height: 0;

      :deep(.editor) {
        flex: 1;
        min-height: 0;
      }
    }

    &__right {
      flex: none;
      width: 380px;
      min-height: 0;
    }
  }

  @media (width <= 1280px) {
    .workspace__right {
      width: 320px;
    }
  }

  @media (width <= 1024px) {
    .workspace__left {
      width: 200px;
    }

    .workspace__right {
      width: 300px;
    }
  }
</style>

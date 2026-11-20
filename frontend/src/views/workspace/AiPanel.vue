<!--
  AI 面板：模型选择 + 会话切换 + 上下文开关 + 快捷指令 + 消息流 + 输入区。
  模型列表来自「当前账号可用模型」（按角色授权），默认选中系统默认或第一个可用模型。
-->
<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import {
  createChatSession,
  deleteChatSession,
  getChatHistory,
  listChatSessions,
} from '@/api/chat'
import { getModelCapability, type NWModelCapabilityVo, type NWModelOptionVo } from '@/api/model'
import { useSSE } from '@/composables/useSSE'
import type { NWChapterBriefVo, NWChatSendReqVo, NWChatSessionVo } from '@/api/contracts'
import NWIcon from '@/components/NWIcon.vue'
import NWLoading from '@/components/NWLoading.vue'
import AiPanelHeader from './AiPanelHeader.vue'
import ChatComposer from './ChatComposer.vue'
import ChatMessage from './ChatMessage.vue'
import type { NWChatDisplayMessage, NWChatSendPayload } from './chatTypes'
import { NW_QUICK_PROMPT_LIST } from './quickPrompts'

const props = defineProps<{
  projectUlid: string
  chapterList: NWChapterBriefVo[]
  currentChapterUlid?: string
  currentChapterName?: string
}>()

const emit = defineEmits<{
  'insert-to-chapter': [contentText: string]
  'replace-chapter': [contentText: string]
}>()

const { t } = useI18n()
const { streamingFlag, startStream, stopStream } = useSSE()

/** 会话历史一次拉取条数 */
const NW_HISTORY_PAGE_SIZE = 50

const composerRef = ref<InstanceType<typeof ChatComposer> | null>(null)
const messagesHost = ref<HTMLDivElement | null>(null)
const sessionList = ref<NWChatSessionVo[]>([])
const currentSessionUlid = ref('')
const messageList = ref<NWChatDisplayMessage[]>([])
const historyLoading = ref(false)
const sessionCreating = ref(false)
const capabilityVo = ref<NWModelCapabilityVo | null>(null)
const selectedModelUlid = ref('')
/** 是否自动附带当前章节 */
const attachChapterFlag = ref(true)

/** 当前账号可用模型（按角色授权过滤） */
const modelOptionList = computed<NWModelOptionVo[]>(() => capabilityVo.value?.option_list ?? [])

/** 当前选中的会话 */
const currentSessionVo = computed(
  () => sessionList.value.find((sessionItem) => sessionItem.session_ulid === currentSessionUlid.value) ?? null,
)

/** 快捷指令（文案按 i18n 键渲染） */
const quickPromptList = computed(() =>
  NW_QUICK_PROMPT_LIST.map((promptItem) => ({
    prompt_code: promptItem.prompt_code,
    prompt_title: t(promptItem.title_key),
    prompt_desc: t(promptItem.desc_key),
    prompt_text: t(promptItem.text_key),
    skill_code: promptItem.skill_code,
  })),
)

/** 组装 /chat/send 请求体（当前章节自动并入引用清单） */
function buildStreamRequest(payload: NWChatSendPayload): NWChatSendReqVo {
  const request: NWChatSendReqVo = {
    session_ulid: currentSessionUlid.value || undefined,
    project_ulid: props.projectUlid,
    content_text: payload.content_text,
    history_count: payload.history_count,
  }
  if (payload.skill_code) {
    request.skill_code = payload.skill_code
  }
  const refChapterUlidList = [...payload.ref_chapter_ulid_list]
  if (
    attachChapterFlag.value &&
    props.currentChapterUlid &&
    !refChapterUlidList.includes(props.currentChapterUlid)
  ) {
    refChapterUlidList.unshift(props.currentChapterUlid)
  }
  if (payload.ref_doc_ulid_list.length > 0) {
    request.ref_doc_ulid_list = payload.ref_doc_ulid_list
  }
  if (refChapterUlidList.length > 0) {
    request.ref_chapter_ulid_list = refChapterUlidList
  }
  if (payload.ref_message_ulid_list.length > 0) {
    request.ref_message_ulid_list = payload.ref_message_ulid_list
  }
  if (payload.prompt_profile_ulid) {
    request.prompt_profile_ulid = payload.prompt_profile_ulid
  }
  if (selectedModelUlid.value) {
    request.model_ulid = selectedModelUlid.value
  }
  return request
}

/** 加载当前账号可用模型（失败降级为空，面板照常可用） */
async function loadModelOptions(): Promise<void> {
  try {
    const capability = await getModelCapability()
    capabilityVo.value = capability
    const existsFlag = capability.option_list.some((item) => item.model_ulid === selectedModelUlid.value)
    if (!existsFlag) {
      selectedModelUlid.value = capability.default_model_ulid || capability.option_list[0]?.model_ulid || ''
    }
  } catch {
    capabilityVo.value = null
  }
}

/** 发起流式生成并把事件回写到助手消息 */
async function runStream(
  request: NWChatSendReqVo,
  assistantMessage: NWChatDisplayMessage,
): Promise<void> {
  assistantMessage.content_text = ''
  assistantMessage.reasoning_text = ''
  assistantMessage.error_message = ''
  assistantMessage.reference_list = []
  assistantMessage.streaming_flag = true
  startWaitTimer(assistantMessage)
  await startStream('/chat/send', request, {
    onEvent: (event) => {
      if (event.session_ulid && !currentSessionUlid.value) {
        currentSessionUlid.value = event.session_ulid
        void loadSessions()
      }
      if (event.event_type === 'thinking') {
        assistantMessage.reasoning_text += event.content_text
      } else if (event.event_type === 'chunk') {
        assistantMessage.content_text += event.content_text
      } else if (event.event_type === 'done') {
        assistantMessage.content_text = event.content_text
        assistantMessage.reasoning_text = event.reasoning_text || assistantMessage.reasoning_text
        assistantMessage.reference_list = event.reference_list ?? []
      } else if (event.event_type === 'error') {
        assistantMessage.error_message = event.error_message || t('workspace.ai.sendFailed')
      }
    },
    onError: () => {
      assistantMessage.error_message = t('workspace.ai.sendFailed')
    },
  })
  stopWaitTimer()
  assistantMessage.streaming_flag = false
  if (assistantMessage.error_message) {
    ElMessage.error(assistantMessage.error_message)
  }
  void loadSessions()
  scrollMessagesToBottom()
}

/** 等待计时器句柄（同一时刻只有一条流在等待） */
let waitTimerId: number | null = null

/** 启动等待计时：每 200ms 刷新已等待时长，让用户看到界面仍在工作 */
function startWaitTimer(targetMessage: NWChatDisplayMessage): void {
  stopWaitTimer()
  const startedMs = Date.now()
  targetMessage.waited_ms = 0
  waitTimerId = window.setInterval(() => {
    targetMessage.waited_ms = Date.now() - startedMs
  }, 200)
}

/** 停止等待计时 */
function stopWaitTimer(): void {
  if (waitTimerId !== null) {
    window.clearInterval(waitTimerId)
    waitTimerId = null
  }
}

/** 滚动消息流到底部 */
function scrollMessagesToBottom(): void {
  void nextTick(() => {
    if (messagesHost.value) {
      messagesHost.value.scrollTop = messagesHost.value.scrollHeight
    }
  })
}

/** 发送消息：落用户气泡 + 占位助手气泡后发起流 */
function handleSend(payload: NWChatSendPayload): void {
  if (modelOptionList.value.length === 0) {
    ElMessage.warning(t('workspace.ai.modelUnavailable'))
    return
  }
  const createdMs = Date.now()
  messageList.value.push(
    {
      message_ulid: '',
      role_code: 'user',
      content_text: payload.content_text,
      reasoning_text: '',
      skill_code: payload.skill_code,
      created_at_ms: createdMs,
      reference_list: [],
      error_message: '',
      streaming_flag: false,
      waited_ms: 0,
      retry_payload: null,
    },
    {
      message_ulid: '',
      role_code: 'assistant',
      content_text: '',
      reasoning_text: '',
      skill_code: '',
      created_at_ms: createdMs,
      reference_list: [],
      error_message: '',
      streaming_flag: true,
      waited_ms: 0,
      retry_payload: payload,
    },
  )
  const assistantMessage = messageList.value[messageList.value.length - 1]
  scrollMessagesToBottom()
  void runStream(buildStreamRequest(payload), assistantMessage)
}

/** 失败重试：原样重发保存的请求载荷 */
function handleRetry(message: NWChatDisplayMessage): void {
  if (streamingFlag.value || !message.retry_payload) {
    return
  }
  void runStream(buildStreamRequest(message.retry_payload), message)
}

/** 停止生成（AbortController 中止 SSE 流） */
function handleStop(): void {
  if (!streamingFlag.value) {
    return
  }
  stopStream()
  ElMessage.success(t('workspace.ai.stopSuccess'))
}

/** 加载会话列表（无当前会话时选中第一个） */
async function loadSessions(): Promise<void> {
  try {
    const listData = await listChatSessions(props.projectUlid)
    sessionList.value = Array.isArray(listData) ? listData : []
    if (!currentSessionUlid.value && sessionList.value.length > 0) {
      await selectSession(sessionList.value[0].session_ulid)
    }
    if (sessionList.value.length === 0) {
      currentSessionUlid.value = ''
      messageList.value = []
    }
  } catch {
    sessionList.value = []
  }
}

/** 新建会话并切换 */
async function handleCreateSession(): Promise<void> {
  if (sessionCreating.value) {
    return
  }
  sessionCreating.value = true
  try {
    const sessionVo = await createChatSession({ project_ulid: props.projectUlid })
    sessionList.value.unshift(sessionVo)
    await selectSession(sessionVo.session_ulid)
    ElMessage.success(t('workspace.ai.sessionCreateSuccess'))
    composerRef.value?.focus()
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    sessionCreating.value = false
  }
}

/** 删除当前会话（二次确认，级联删消息） */
async function handleDeleteSession(): Promise<void> {
  if (!currentSessionUlid.value) {
    return
  }
  try {
    await ElMessageBox.confirm(
      t('workspace.ai.sessionDeleteConfirm'),
      t('workspace.ai.sessionDelete'),
      { type: 'warning', confirmButtonText: t('common.confirm'), cancelButtonText: t('common.cancel') },
    )
  } catch {
    return
  }
  try {
    await deleteChatSession(currentSessionUlid.value)
    messageList.value = []
    currentSessionUlid.value = ''
    await loadSessions()
    ElMessage.success(t('workspace.ai.sessionDeleteSuccess'))
  } catch {
    // 失败提示由 http 层统一弹出
  }
}

/** 切换会话并加载历史 */
async function selectSession(sessionUlid: string): Promise<void> {
  if (streamingFlag.value) {
    stopStream()
  }
  currentSessionUlid.value = sessionUlid
  historyLoading.value = true
  try {
    const pageData = await getChatHistory({
      session_ulid: sessionUlid,
      page_num: 1,
      page_size: NW_HISTORY_PAGE_SIZE,
    })
    messageList.value = (pageData.items ?? []).map((messageVo) => ({
      message_ulid: messageVo.message_ulid,
      role_code: messageVo.role_code,
      content_text: messageVo.content_text,
      reasoning_text: messageVo.reasoning_text ?? '',
      skill_code: messageVo.skill_code,
      created_at_ms: messageVo.created_at_ms,
      reference_list: [],
      error_message: '',
      streaming_flag: false,
      waited_ms: 0,
      retry_payload: null,
    }))
  } catch {
    messageList.value = []
  } finally {
    historyLoading.value = false
    scrollMessagesToBottom()
  }
}

/** 外部预置技能与章节引用（AI 续写 / AI 润色入口） */
function presetSkill(skillCode: string, refChapterUlidList: string[]): void {
  composerRef.value?.presetSkill(skillCode, refChapterUlidList)
  composerRef.value?.focus()
}

/** 套用快捷指令：填技能并写入提示词 */
function applyQuickPrompt(promptItem: { prompt_text: string; skill_code: string }): void {
  if (promptItem.skill_code && props.currentChapterUlid) {
    composerRef.value?.presetSkill(promptItem.skill_code, [props.currentChapterUlid])
  }
  composerRef.value?.setInputText(promptItem.prompt_text)
}

defineExpose({ presetSkill })

watch(
  () => props.projectUlid,
  () => {
    currentSessionUlid.value = ''
    messageList.value = []
    void loadSessions()
  },
  { immediate: true },
)

watch(
  () => messageList.value.length,
  () => {
    scrollMessagesToBottom()
  },
)

onMounted(() => {
  void loadModelOptions()
})

onBeforeUnmount(() => {
  stopWaitTimer()
})
</script>

<template>
  <div class="ai-panel">
    <AiPanelHeader
      v-model:current-session-ulid="currentSessionUlid"
      v-model:selected-model-ulid="selectedModelUlid"
      v-model:attach-chapter-flag="attachChapterFlag"
      :session-list="sessionList"
      :session-deletable-flag="Boolean(currentSessionVo)"
      :model-option-list="modelOptionList"
      :current-chapter-ulid="currentChapterUlid"
      :current-chapter-name="currentChapterName"
      :message-count="messageList.length"
      @select-session="selectSession"
      @create-session="handleCreateSession"
      @delete-session="handleDeleteSession"
    />

    <div ref="messagesHost" class="ai-panel__messages">
      <NWLoading v-if="historyLoading" />
      <template v-else>
        <div v-if="messageList.length === 0" class="ai-panel__welcome">
          <div class="ai-panel__welcome-head">
            <span class="ai-panel__welcome-icon"><NWIcon icon_code="wand" :size_px="22" /></span>
            <div>
              <h3 class="ai-panel__welcome-title">{{ t('workspace.ai.emptyTitle') }}</h3>
              <p class="ai-panel__welcome-desc">{{ t('workspace.ai.emptyDesc') }}</p>
            </div>
          </div>
          <div class="ai-panel__quick-grid">
            <button
              v-for="promptItem in quickPromptList"
              :key="promptItem.prompt_code"
              type="button"
              class="ai-panel__quick-card"
              @click="applyQuickPrompt(promptItem)"
            >
              <strong>{{ promptItem.prompt_title }}</strong>
              <small>{{ promptItem.prompt_desc }}</small>
            </button>
          </div>
        </div>
        <ChatMessage
          v-for="(messageItem, messageIndex) in messageList"
          :key="messageIndex"
          :message="messageItem"
          @retry="handleRetry(messageItem)"
          @insert-to-chapter="(contentText) => emit('insert-to-chapter', contentText)"
          @replace-chapter="(contentText) => emit('replace-chapter', contentText)"
        />
      </template>
    </div>

    <ChatComposer
      ref="composerRef"
      :project-ulid="projectUlid"
      :chapter-list="chapterList"
      :message-list="messageList"
      :disabled-flag="streamingFlag"
      @send="handleSend"
      @stop="handleStop"
    />
  </div>
</template>

<style scoped src="./AiPanel.css"></style>

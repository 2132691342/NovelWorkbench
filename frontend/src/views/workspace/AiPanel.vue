<!--
  AI 面板：会话列表（新建/切换/删除）+ 消息流（流式/失败重试）+ 输入区接线。
  发送走 SSE（composables/useSSE），done 后消息展示 AI 依据与插入到章节。
-->
<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import {
  createChatSession,
  deleteChatSession,
  getChatHistory,
  listChatSessions,
} from '@/api/chat'
import { getChatOptions, type NWModelCapabilityVo, type NWModelProviderVo } from '@/api/model'
import { useSSE } from '@/composables/useSSE'
import type {
  NWChapterBriefVo,
  NWChatSendReqVo,
  NWChatSessionVo,
} from '@/api/contracts'
import NWButton from '@/components/NWButton.vue'
import NWEmpty from '@/components/NWEmpty.vue'
import NWLoading from '@/components/NWLoading.vue'
import ChatComposer from './ChatComposer.vue'
import ChatMessage from './ChatMessage.vue'
import type { NWChatDisplayMessage, NWChatSendPayload } from './chatTypes'

const props = defineProps<{
  projectUlid: string
  chapterList: NWChapterBriefVo[]
}>()

const emit = defineEmits<{
  'insert-to-chapter': [contentText: string]
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
const providerList = ref<NWModelProviderVo[]>([])
const capabilityVo = ref<NWModelCapabilityVo | null>(null)
const selectedProviderUlid = ref('')

/** 可选对话供应商（仅启用的） */
const chatProviderOptions = computed(() =>
  providerList.value.filter((providerItem) => providerItem.enabled_flag),
)

/** 组装 /chat/send 请求体 */
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
  if (payload.ref_doc_ulid_list.length > 0) {
    request.ref_doc_ulid_list = payload.ref_doc_ulid_list
  }
  if (payload.ref_chapter_ulid_list.length > 0) {
    request.ref_chapter_ulid_list = payload.ref_chapter_ulid_list
  }
  if (selectedProviderUlid.value) {
    request.provider_ulid = selectedProviderUlid.value
  }
  return request
}

/** 加载模型供应商与能力（失败降级为空，面板照常可用） */
async function loadModelOptions(): Promise<void> {
  try {
    const options = await getChatOptions()
    providerList.value = Array.isArray(options.provider_list) ? options.provider_list : []
    capabilityVo.value = options.capability_vo
    if (!selectedProviderUlid.value && options.capability_vo.default_provider_ulid) {
      selectedProviderUlid.value = options.capability_vo.default_provider_ulid
    }
  } catch {
    providerList.value = []
    capabilityVo.value = null
  }
}

/**
 * 发起流式生成并把事件回写到助手消息：
 * chunk 增量追加；done 覆盖为完整回复并挂 RAG 依据；error 落错误气泡可重试。
 */
async function runStream(
  request: NWChatSendReqVo,
  assistantMessage: NWChatDisplayMessage,
): Promise<void> {
  assistantMessage.content_text = ''
  assistantMessage.error_message = ''
  assistantMessage.reference_list = []
  assistantMessage.streaming_flag = true
  await startStream('/chat/send', request, {
    onEvent: (event) => {
      if (event.session_ulid && !currentSessionUlid.value) {
        // 首次发送自动建会话：补上会话上下文并刷新会话列表
        currentSessionUlid.value = event.session_ulid
        void loadSessions()
      }
      if (event.event_type === 'chunk') {
        assistantMessage.content_text += event.content_text
      } else if (event.event_type === 'done') {
        assistantMessage.content_text = event.content_text
        assistantMessage.reference_list = event.reference_list ?? []
      } else if (event.event_type === 'error') {
        assistantMessage.error_message = event.error_message || t('workspace.ai.sendFailed')
      }
    },
    onError: () => {
      assistantMessage.error_message = t('workspace.ai.sendFailed')
    },
  })
  assistantMessage.streaming_flag = false
  void loadSessions()
  scrollMessagesToBottom()
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
  const createdMs = Date.now()
  messageList.value.push(
    {
      message_ulid: '',
      role_code: 'user',
      content_text: payload.content_text,
      skill_code: payload.skill_code,
      created_at_ms: createdMs,
      reference_list: [],
      error_message: '',
      streaming_flag: false,
      retry_payload: null,
    },
    {
      message_ulid: '',
      role_code: 'assistant',
      content_text: '',
      skill_code: '',
      created_at_ms: createdMs,
      reference_list: [],
      error_message: '',
      streaming_flag: true,
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
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    sessionCreating.value = false
  }
}

/** 删除会话（二次确认，级联删消息） */
async function handleDeleteSession(sessionUlid: string): Promise<void> {
  try {
    await ElMessageBox.confirm(
      t('workspace.ai.sessionDeleteConfirm'),
      t('workspace.ai.sessionDelete'),
      { type: 'warning' },
    )
  } catch {
    return
  }
  try {
    await deleteChatSession(sessionUlid)
    if (currentSessionUlid.value === sessionUlid) {
      currentSessionUlid.value = ''
      messageList.value = []
    }
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
      skill_code: messageVo.skill_code,
      created_at_ms: messageVo.created_at_ms,
      reference_list: [],
      error_message: '',
      streaming_flag: false,
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
</script>

<template>
  <div class="ai-panel">
    <header class="ai-panel__header">
      <span class="ai-panel__title">{{ t('workspace.ai.title') }}</span>
      <el-select
        v-if="chatProviderOptions.length > 0"
        v-model="selectedProviderUlid"
        size="small"
        class="ai-panel__model-select"
        :placeholder="t('workspace.ai.modelPlaceholder')"
      >
        <el-option
          v-for="providerItem in chatProviderOptions"
          :key="providerItem.provider_ulid"
          :value="providerItem.provider_ulid"
          :label="`${providerItem.provider_name} / ${providerItem.chat_model_name}`"
        />
      </el-select>
      <NWButton size="small" type="primary" :loading="sessionCreating" @click="handleCreateSession">
        {{ t('workspace.ai.sessionCreate') }}
      </NWButton>
    </header>


    <!-- 会话列表 -->
    <div class="ai-panel__sessions">
      <div v-if="sessionList.length === 0" class="ai-panel__sessions-empty">
        {{ t('workspace.ai.sessionEmpty') }}
      </div>
      <div
        v-for="sessionItem in sessionList"
        :key="sessionItem.session_ulid"
        class="session-item"
        :class="{ 'is-active': sessionItem.session_ulid === currentSessionUlid }"
        @click="selectSession(sessionItem.session_ulid)"
      >
        <span class="session-item__name" :title="sessionItem.session_name">
          {{ sessionItem.session_name || sessionItem.session_ulid }}
        </span>
        <span class="session-item__count">
          {{ t('workspace.ai.messageCount', { count: sessionItem.message_count }) }}
        </span>
        <NWButton
          type="text"
          size="small"
          class="session-item__delete"
          :aria-label="t('workspace.ai.sessionDelete')"
          @click.stop="handleDeleteSession(sessionItem.session_ulid)"
        >
          x
        </NWButton>
      </div>
    </div>

    <!-- 消息流 -->
    <div ref="messagesHost" class="ai-panel__messages">
      <NWLoading v-if="historyLoading" />
      <template v-else>
        <NWEmpty
          v-if="messageList.length === 0"
          :description_text="t('workspace.ai.historyEmpty')"
        />
        <ChatMessage
          v-for="(messageItem, messageIndex) in messageList"
          :key="messageIndex"
          :message="messageItem"
          @retry="handleRetry(messageItem)"
          @insert-to-chapter="(contentText) => emit('insert-to-chapter', contentText)"
        />
      </template>
    </div>

    <ChatComposer
      ref="composerRef"
      :project-ulid="projectUlid"
      :chapter-list="chapterList"
      :disabled-flag="streamingFlag"
      @send="handleSend"
      @stop="handleStop"
    />
  </div>
</template>
<style scoped src="./AiPanel.css"></style>


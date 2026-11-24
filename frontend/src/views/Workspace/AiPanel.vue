<!-- AI 面板：会话管理、流式对话与结果处置。 -->
<script setup lang="ts">
  import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
  import { ElMessage, ElMessageBox } from 'element-plus'
  import {
    createChatSession,
    deleteChatSession,
    getChatHistory,
    listChatSessions
  } from '@/api/chat'
  import { getModelCapability, type NWModelCapabilityVo, type NWModelOptionVo } from '@/api/model'
  import { listPromptProfiles } from '@/api/prompt'
  import { listSkills } from '@/api/skill'
  import { listTools, type NWToolVo } from '@/api/tool'
  import type {
    NWChapterBriefVo,
    NWChatSendReqVo,
    NWChatSessionVo,
    NWPromptProfileVo,
    NWSkillVo
  } from '@/api/contracts'
  import { useSSE } from '@/composables/useSSE'
  import { applyStreamEvent } from './chatStream'
  import { COMPOSER_ACTION_LIST } from './composerCommands'
  import ChatComposer from './ChatComposer.vue'
  import ChatMessage from './ChatMessage.vue'
  import type { ChatDisplayMessage, ChatSendPayload } from './types'

  const props = defineProps<{
    projectUlid: string
    chapterList: NWChapterBriefVo[]
    currentChapterUlid?: string
    currentChapterName?: string
  }>()

  const emit = defineEmits<{
    'insert-to-chapter': [contentText: string]
    'replace-chapter': [contentText: string]
    /** 写作台级斜杠动作（体检/专注/快照）交由外层执行 */
    action: [actionCode: string]
  }>()

  const { streamingFlag, startStream, stopStream } = useSSE()

  const composerRef = ref<InstanceType<typeof ChatComposer> | null>(null)
  const messageList = ref<ChatDisplayMessage[]>([])
  const sessionList = ref<NWChatSessionVo[]>([])
  const currentSessionUlid = ref('')
  const capability = ref<NWModelCapabilityVo | null>(null)
  const selectedModelUlid = ref('')
  const attachChapter = ref(true)
  const historyLoading = ref(false)
  const skillList = ref<NWSkillVo[]>([])
  const toolList = ref<NWToolVo[]>([])
  const profileList = ref<NWPromptProfileVo[]>([])
  const messagesHost = ref<HTMLDivElement | null>(null)

  const modelOptions = computed<NWModelOptionVo[]>(() => capability.value?.option_list ?? [])
  const canSend = computed(() => !streamingFlag.value && modelOptions.value.length > 0)

  const scrollToBottom = () => {
    void nextTick(() => {
      if (messagesHost.value) messagesHost.value.scrollTop = messagesHost.value.scrollHeight
    })
  }

  const loadModelOptions = async () => {
    try {
      const result = await getModelCapability()
      capability.value = result
      if (!result.option_list.some((item) => item.model_ulid === selectedModelUlid.value)) {
        selectedModelUlid.value =
          result.default_model_ulid || result.option_list[0]?.model_ulid || ''
      }
    } catch {
      capability.value = null
    }
  }

  const loadSkills = async () => {
    if (!props.projectUlid) return
    try {
      skillList.value = await listSkills({ project_ulid: props.projectUlid, enabled_only: true })
    } catch {
      skillList.value = []
    }
  }

  const loadTools = async () => {
    try {
      toolList.value = await listTools({ enabled_only: true })
    } catch {
      toolList.value = []
    }
  }

  const loadProfiles = async () => {
    try {
      profileList.value = await listPromptProfiles({
        project_ulid: props.projectUlid,
        enabled_only: true
      })
    } catch {
      profileList.value = []
    }
  }

  const loadHistory = async (sessionUlid: string) => {
    currentSessionUlid.value = sessionUlid
    historyLoading.value = true
    try {
      const page = await getChatHistory({ session_ulid: sessionUlid, page_num: 1, page_size: 50 })
      messageList.value = (page.items ?? []).map((message) => ({
        message_ulid: message.message_ulid,
        role_code: message.role_code,
        content_text: message.content_text,
        reasoning_text: message.reasoning_text ?? '',
        prompt_code: message.prompt_code,
        tool_code_list: message.tool_code_list ?? [],
        created_at: message.created_at,
        reference_list: [],
        error_message: '',
        streaming_flag: false,
        waited_ms: 0,
        retry_payload: null
      }))
    } catch {
      messageList.value = []
    } finally {
      historyLoading.value = false
      scrollToBottom()
    }
  }

  const loadSessions = async () => {
    try {
      sessionList.value = await listChatSessions(props.projectUlid)
      if (!currentSessionUlid.value && sessionList.value.length > 0) {
        await loadHistory(sessionList.value[0].session_ulid)
      }
      if (sessionList.value.length === 0) {
        currentSessionUlid.value = ''
        messageList.value = []
      }
    } catch {
      sessionList.value = []
    }
  }

  const createSession = async () => {
    const session = await createChatSession({ project_ulid: props.projectUlid })
    sessionList.value.unshift(session)
    await loadHistory(session.session_ulid)
    composerRef.value?.focus()
  }

  const deleteSession = async () => {
    if (!currentSessionUlid.value) return
    try {
      await ElMessageBox.confirm('将删除该会话及其全部消息，确定吗？', '删除会话', {
        type: 'warning'
      })
    } catch {
      return
    }
    await deleteChatSession(currentSessionUlid.value)
    messageList.value = []
    currentSessionUlid.value = ''
    await loadSessions()
  }

  /** 组装 /chat/send 请求体（当前章节按开关自动并入引用） */
  const buildRequest = (payload: ChatSendPayload): NWChatSendReqVo => {
    const request: NWChatSendReqVo = {
      session_ulid: currentSessionUlid.value || undefined,
      project_ulid: props.projectUlid,
      content_text: payload.content_text,
      history_count: payload.history_count
    }
    if (payload.prompt_code) request.prompt_code = payload.prompt_code
    if (payload.tool_code_list.length > 0) request.tool_code_list = payload.tool_code_list
    if (payload.system_prompt_code) request.system_prompt_code = payload.system_prompt_code
    if (selectedModelUlid.value) request.model_ulid = selectedModelUlid.value

    const chapterRefs = [...payload.ref_chapter_ulid_list]
    if (
      attachChapter.value &&
      props.currentChapterUlid &&
      !chapterRefs.includes(props.currentChapterUlid)
    ) {
      chapterRefs.unshift(props.currentChapterUlid)
    }
    if (payload.ref_doc_ulid_list.length > 0) request.ref_doc_ulid_list = payload.ref_doc_ulid_list
    if (chapterRefs.length > 0) request.ref_chapter_ulid_list = chapterRefs
    if (payload.ref_summary_ulid_list.length > 0) {
      request.ref_summary_ulid_list = payload.ref_summary_ulid_list
    }
    if (payload.ref_prompt_ulid_list.length > 0) {
      request.ref_prompt_ulid_list = payload.ref_prompt_ulid_list
    }
    if (payload.ref_message_ulid_list.length > 0) {
      request.ref_message_ulid_list = payload.ref_message_ulid_list
    }
    return request
  }

  let waitTimerId: number | null = null

  const stopWaitTimer = () => {
    if (waitTimerId !== null) {
      window.clearInterval(waitTimerId)
      waitTimerId = null
    }
  }

  /** 等待计时：每 200ms 刷新已等待时长，让用户看到界面仍在工作 */
  const startWaitTimer = (message: ChatDisplayMessage) => {
    stopWaitTimer()
    const startedMs = Date.now()
    message.waited_ms = 0
    waitTimerId = window.setInterval(() => {
      message.waited_ms = Date.now() - startedMs
    }, 200)
  }

  const runStream = async (request: NWChatSendReqVo, assistant: ChatDisplayMessage) => {
    assistant.content_text = ''
    assistant.reasoning_text = ''
    assistant.error_message = ''
    assistant.reference_list = []
    assistant.streaming_flag = true
    startWaitTimer(assistant)

    await startStream('/chat/send', request, {
      onEvent: (event) => {
        if (event.session_ulid && !currentSessionUlid.value) {
          currentSessionUlid.value = event.session_ulid
          void loadSessions()
        }
        applyStreamEvent(assistant, event, '生成失败，请重试')
      },
      onError: () => {
        assistant.error_message = '生成失败，请重试'
      }
    })

    stopWaitTimer()
    assistant.streaming_flag = false
    if (assistant.error_message) ElMessage.error(assistant.error_message)
    void loadSessions()
    scrollToBottom()
  }

  const send = (payload: ChatSendPayload) => {
    const createdAtText = new Date().toISOString()
    messageList.value.push(
      {
        message_ulid: '',
        role_code: 'user',
        content_text: payload.content_text,
        reasoning_text: '',
        prompt_code: payload.prompt_code,
        tool_code_list: [...payload.tool_code_list],
        created_at: createdAtText,
        reference_list: [],
        error_message: '',
        streaming_flag: false,
        waited_ms: 0,
        retry_payload: null
      },
      {
        message_ulid: '',
        role_code: 'assistant',
        content_text: '',
        reasoning_text: '',
        prompt_code: '',
        tool_code_list: [...payload.tool_code_list],
        created_at: createdAtText,
        reference_list: [],
        error_message: '',
        streaming_flag: true,
        waited_ms: 0,
        retry_payload: payload
      }
    )
    const assistant = messageList.value[messageList.value.length - 1]
    scrollToBottom()
    void runStream(buildRequest(payload), assistant)
  }

  const retry = (message: ChatDisplayMessage) => {
    if (streamingFlag.value || !message.retry_payload) return
    void runStream(buildRequest(message.retry_payload), message)
  }

  const stop = () => {
    if (!streamingFlag.value) return
    stopStream()
    ElMessage.success('已停止生成')
  }

  /** 上一条 AI 回复正文（/copy 用） */
  const lastAssistantText = (): string => {
    for (let i = messageList.value.length - 1; i >= 0; i--) {
      const message = messageList.value[i]
      if (message.role_code === 'assistant' && message.content_text) return message.content_text
    }
    return ''
  }

  /** 命令速查：把当前可用的三类触发符与提示词清单摊开给用户看 */
  const showCommandHelp = async () => {
    const actionRows = COMPOSER_ACTION_LIST.map(
      (action) => `<li><code>/${action.action_code}</code> ${action.action_name} — ${action.action_desc}</li>`
    ).join('')
    const promptRows = commandPromptList.value
      .map(
        (prompt) =>
          `<li><code>/${prompt.prompt_code}</code> ${prompt.prompt_name} — ${prompt.description_text}</li>`
      )
      .join('')
    const toolRows = toolList.value
      .map(
        (tool) => `<li><code>#${tool.tool_code}</code> ${tool.tool_name} — ${tool.description_text}</li>`
      )
      .join('')
    await ElMessageBox.alert(
      `<div class="command-help">
        <p><b>/ 本地动作</b>（立即执行，不消耗模型）</p>
        <ul>${actionRows}</ul>
        <p><b>/ 指令提示词</b>（以系统级消息注入本轮）</p>
        <ul>${promptRows || '<li>暂无启用的指令提示词</li>'}</ul>
        <p><b># 工具</b>（服务端确定性执行，结果注入上下文）</p>
        <ul>${toolRows || '<li>暂无启用的工具</li>'}</ul>
        <p><b>@ 引用</b>：文档 / 章节 / 摘要 / 提示词 / 历史消息；@ 弹层内 ← → 切页签，↑ ↓ 选择，Enter 确认。</p>
        <p>键盘：<code>Ctrl/⌘ + Enter</code> 发送，<code>Esc</code> 关闭弹层，输入框为空时 <code>Backspace</code> 逐个撤销命令徽标。</p>
      </div>`,
      '命令速查',
      { dangerouslyUseHTMLString: true, confirmButtonText: '知道了' }
    ).catch(() => undefined)
  }

  /** 斜杠动作分发：面板级就地处理，写作台级交给外层 */
  const handleAction = (actionCode: string) => {
    if (actionCode === 'new') {
      void createSession()
      return
    }
    if (actionCode === 'attach') {
      attachChapter.value = !attachChapter.value
      ElMessage.success(attachChapter.value ? '已附带当前章节' : '已取消附带当前章节')
      return
    }
    if (actionCode === 'copy') {
      const contentText = lastAssistantText()
      if (!contentText) {
        ElMessage.warning('还没有可复制的 AI 回复')
        return
      }
      void navigator.clipboard.writeText(contentText).then(() => ElMessage.success('已复制上一条回复'))
      return
    }
    if (actionCode === 'help') {
      void showCommandHelp()
      return
    }
    emit('action', actionCode)
  }

  watch(
    () => props.projectUlid,
    () => {
      currentSessionUlid.value = ''
      messageList.value = []
      void loadSessions()
      void loadPrompts()
      void loadTools()
    },
    { immediate: true }
  )

  watch(() => messageList.value.length, scrollToBottom)

  onMounted(() => void loadModelOptions())
  onBeforeUnmount(stopWaitTimer)

  defineExpose({
    presetPrompt: (promptCode: string, chapterUlidList: string[]) =>
      composerRef.value?.presetPrompt(promptCode, chapterUlidList),
    presetText: (promptText: string) => composerRef.value?.setInputText(promptText)
  })
</script>

<template>
  <section class="ai-panel">
    <header class="ai-panel__head">
      <ElSelect
        v-model="currentSessionUlid"
        class="ai-panel__session"
        placeholder="会话"
        @change="loadHistory"
      >
        <ElOption
          v-for="session in sessionList"
          :key="session.session_ulid"
          :label="session.session_name || '未命名会话'"
          :value="session.session_ulid"
        />
      </ElSelect>
      <ElButton size="small" plain @click="createSession">
        <Icon icon="mdi:plus" :size="14" />
      </ElButton>
      <ElButton size="small" plain :disabled="!currentSessionUlid" @click="deleteSession">
        <Icon icon="mdi:delete-outline" :size="14" />
      </ElButton>
    </header>

    <div class="ai-panel__models">
      <ElSelect
        v-model="selectedModelUlid"
        size="small"
        class="ai-panel__model"
        placeholder="选择模型"
      >
        <ElOption
          v-for="model in modelOptions"
          :key="model.model_ulid"
          :label="model.model_label || model.model_name"
          :value="model.model_ulid"
        />
      </ElSelect>
      <ElCheckbox v-model="attachChapter" size="small">
        附带当前章节{{ currentChapterName ? `：${currentChapterName}` : '' }}
      </ElCheckbox>
    </div>

    <div ref="messagesHost" v-loading="historyLoading" class="ai-panel__messages">
      <div v-if="messageList.length === 0 && !historyLoading" class="ai-panel__welcome">
        <p class="ai-panel__welcome-title">开始一次创作对话</p>
        <ul class="ai-panel__tips">
          <li>输入 <code>/</code> 选命令：本地动作立即执行，提示词以系统级消息注入</li>
          <li>输入 <code>#</code> 选工具，<code>@</code> 引用文档、章节、摘要、档案或历史消息</li>
          <li><code>Ctrl/⌘ + Enter</code> 发送，<code>/help</code> 看全部命令</li>
        </ul>
      </div>

      <ChatMessage
        v-for="(message, index) in messageList"
        :key="message.message_ulid || index"
        :message="message"
        @retry="retry(message)"
        @insert-to-chapter="(text) => emit('insert-to-chapter', text)"
        @replace-chapter="(text) => emit('replace-chapter', text)"
      />
    </div>

    <ChatComposer
      ref="composerRef"
      :project-ulid="projectUlid"
      :chapter-list="chapterList"
      :message-list="messageList"
      :prompt-list="commandPromptList"
      :tool-list="toolList"
      :system-prompt-list="systemPromptList"
      :streaming="streamingFlag"
      :can-send="canSend"
      @send="send"
      @stop="stop"
      @action="handleAction"
    />
  </section>
</template>

<style lang="less" scoped>
  .ai-panel {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    background: var(--nw-surface);
    border-left: 1px solid var(--nw-border);

    &__head,
    &__models {
      display: flex;
      flex: none;
      gap: 6px;
      align-items: center;
      padding: 8px 10px;
      border-bottom: 1px solid var(--nw-border-soft);
    }

    &__session,
    &__model {
      flex: 1;
      min-width: 0;
    }

    &__messages {
      flex: 1;
      min-height: 0;
      padding: 12px;
      overflow-y: auto;
    }

    &__welcome {
      padding: 18px;
      background: var(--nw-surface-subtle);
      border: 1px solid var(--nw-border-soft);
      border-radius: var(--nw-radius-lg);
    }

    &__welcome-title {
      margin-bottom: 8px;
      font-size: 13px;
      font-weight: 650;
    }

    &__tips {
      padding-left: 16px;
      font-size: 12px;
      line-height: 1.9;
      color: var(--el-text-color-secondary);
    }
  }
</style>

<style lang="less">
  /* 命令速查渲染在 body 下的消息框里，作用域样式够不到，只能全局给 */
  .command-help {
    font-size: 12.5px;
    line-height: 1.85;
    color: var(--el-text-color-regular);

    p {
      margin: 8px 0 2px;
      color: var(--nw-ink);
    }

    ul {
      margin: 0;
      padding-left: 16px;
    }

    li {
      list-style: none;
    }

    code {
      padding: 0 4px;
      font-size: 11.5px;
      color: var(--el-color-primary);
      background: var(--nw-primary-soft);
      border-radius: 4px;
    }
  }
</style>

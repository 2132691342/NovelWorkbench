<!-- 审核面板：只读展示候选正文与字数差，通过即入库、退回可选快捷意见。
     快捷标签拼进 feedback_text，作者仍可自由补一句；退回意见会注入下一轮起草。 -->
<script setup lang="ts">
  import { computed, ref } from 'vue'
  import { ElMessage } from 'element-plus'
  import { approveWorkflowRun, rejectWorkflowRun, type NWWorkflowRunDetailVo } from '@/api/workflow'

  const props = defineProps<{
    detail: NWWorkflowRunDetailVo
  }>()

  const emit = defineEmits<{
    changed: []
  }>()

  /** 快捷意见标签：覆盖最高频的四类退稿原因，点一下就拼进意见 */
  const quickFeedbackList = [
    '节奏拖沓',
    '人物失真',
    'AI 味重',
    '信息不足',
    '钩子太弱',
    '字数不符'
  ]

  const acting = ref(false)
  const feedbackText = ref('')
  const pickedTagList = ref<string[]>([])

  /** 最终提交的意见 = 快捷标签 + 自由输入，避免作者重复打字 */
  const composedFeedback = computed(() => {
    const parts: string[] = []
    if (pickedTagList.value.length > 0) parts.push(pickedTagList.value.join('、'))
    const freeText = feedbackText.value.trim()
    if (freeText) parts.push(freeText)
    return parts.join('；')
  })

  const candidateWordCount = computed(() => props.detail.candidate_word_count ?? 0)
  const dbWordCount = computed(() => props.detail.chapter_word_count ?? 0)
  const wordDelta = computed(() => candidateWordCount.value - dbWordCount.value)

  const toggleTag = (tag: string) => {
    pickedTagList.value = pickedTagList.value.includes(tag)
      ? pickedTagList.value.filter((item) => item !== tag)
      : [...pickedTagList.value, tag]
  }

  const approve = async () => {
    acting.value = true
    try {
      await approveWorkflowRun(props.detail.run_ulid)
      ElMessage.success('已通过并入库，继续下一章')
      pickedTagList.value = []
      feedbackText.value = ''
      emit('changed')
    } finally {
      acting.value = false
    }
  }

  const reject = async () => {
    const feedback = composedFeedback.value
    if (feedback.length < 2) {
      ElMessage.warning('请至少选一个意见标签，或写一句退回原因')
      return
    }
    acting.value = true
    try {
      await rejectWorkflowRun(props.detail.run_ulid, feedback)
      ElMessage.success('已退回，AI 正按意见重写本章')
      pickedTagList.value = []
      feedbackText.value = ''
      emit('changed')
    } finally {
      acting.value = false
    }
  }
</script>

<template>
  <section class="wf-review">
    <header class="wf-review__head">
      <Icon icon="mdi:account-check-outline" :size="15" />
      <strong>待你审核：{{ detail.current_chapter_name || '当前章节' }}</strong>
      <span class="wf-review__words">
        候选 {{ candidateWordCount }} 字
        <template v-if="dbWordCount > 0">
          · 库内 {{ dbWordCount }} 字
          <span :class="['wf-review__delta', wordDelta >= 0 ? 'is-up' : 'is-down']">
            （{{ wordDelta >= 0 ? '+' : '' }}{{ wordDelta }}）
          </span>
        </template>
        <template v-else>· 库内暂无正文</template>
      </span>
    </header>

    <pre class="wf-review__content">{{ detail.candidate_text || '（暂无候选正文）' }}</pre>

    <div class="wf-review__tags">
      <span class="wf-review__tags-label">快捷意见</span>
      <ElTag
        v-for="tag in quickFeedbackList"
        :key="tag"
        :type="pickedTagList.includes(tag) ? 'warning' : 'info'"
        :effect="pickedTagList.includes(tag) ? 'dark' : 'plain'"
        class="wf-review__tag"
        @click="toggleTag(tag)"
      >
        {{ tag }}
      </ElTag>
    </div>

    <ElInput
      v-model="feedbackText"
      type="textarea"
      :rows="2"
      maxlength="1024"
      show-word-limit
      placeholder="补充退回原因（可选）：例如「第三段赶路删掉，结尾把钩子提前」"
    />

    <div class="wf-review__actions">
      <ElButton type="success" :loading="acting" @click="approve()">
        <Icon icon="mdi:check" :size="15" />
        <span class="wf-review__btn-text">通过并入库</span>
      </ElButton>
      <ElButton type="warning" plain :loading="acting" @click="reject()">
        <Icon icon="mdi:backup-restore" :size="15" />
        <span class="wf-review__btn-text">退回重写</span>
      </ElButton>
      <span class="nw-hint">通过会先留快照再覆盖章节正文，可随时回滚</span>
    </div>
  </section>
</template>

<style lang="less" scoped>
  .wf-review {
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 12px 14px;
    background: var(--nw-surface);
    border: 1px solid var(--nw-warning);
    border-radius: var(--nw-radius-lg);

    &__head {
      display: flex;
      flex-wrap: wrap;
      gap: 6px;
      align-items: center;
      color: var(--nw-warning);

      strong {
        font-size: 13px;
        color: var(--nw-ink);
      }
    }

    &__words {
      margin-left: auto;
      font-size: 11.5px;
      font-weight: 400;
      color: var(--el-text-color-secondary);
    }

    &__delta {
      font-weight: 600;

      &.is-up {
        color: var(--nw-success);
      }

      &.is-down {
        color: var(--nw-danger);
      }
    }

    &__content {
      max-height: 320px;
      padding: 12px;
      margin: 0;
      overflow: auto;
      font-family: inherit;
      font-size: 13px;
      line-height: 1.8;
      white-space: pre-wrap;
      word-break: break-word;
      background: var(--nw-surface-subtle);
      border: 1px solid var(--nw-border);
      border-radius: var(--nw-radius-md);
    }

    &__tags {
      display: flex;
      flex-wrap: wrap;
      gap: 6px;
      align-items: center;
    }

    &__tags-label {
      font-size: 11.5px;
      color: var(--el-text-color-secondary);
    }

    &__tag {
      cursor: pointer;
      transition: transform 0.16s ease;

      &:hover {
        transform: translateY(-1px);
      }
    }

    &__actions {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      align-items: center;
    }

    &__btn-text {
      margin-left: 4px;
    }
  }
</style>

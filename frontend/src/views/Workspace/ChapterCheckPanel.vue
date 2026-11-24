<!-- 章节体检面板：量化门禁结果、问题清单与一键让 AI 修。 -->
<script setup lang="ts">
  import { computed, ref } from 'vue'
  import { ElMessage } from 'element-plus'
  import { checkChapter, type NWChapterCheckVo } from '@/api/chapter'

  const props = defineProps<{
    chapterUlid: string
    contentText: string
  }>()

  const emit = defineEmits<{ 'ask-ai': [promptText: string] }>()

  const loading = ref(false)
  const report = ref<NWChapterCheckVo | null>(null)

  const verdictMeta = computed(() => {
    switch (report.value?.verdict_code) {
      case 'pass':
        return { label: '可发', type: 'success' as const }
      case 'block':
        return { label: '不可发', type: 'danger' as const }
      default:
        return { label: '需修', type: 'warning' as const }
    }
  })

  const levelType = (levelCode: string) =>
    levelCode === 'block' ? 'danger' : levelCode === 'warn' ? 'warning' : 'info'

  const levelLabel = (levelCode: string) =>
    levelCode === 'block' ? '阻断' : levelCode === 'warn' ? '警示' : '建议'

  /** 只有「命中次数」类问题才显示「N 处」：字数与段落均匀度里的数字是统计量，不是命中数 */
  const COUNT_ISSUE_CODE_SET = new Set([
    'dash',
    'banned_word',
    'warn_word',
    'emotion_named',
    'bare_tag'
  ])

  const showHitCount = (issueCode: string, hitCount: number) =>
    hitCount > 0 && COUNT_ISSUE_CODE_SET.has(issueCode)

  const run = async () => {
    loading.value = true
    try {
      report.value = await checkChapter({
        chapter_ulid: props.chapterUlid || undefined,
        content_text: props.contentText || undefined
      })
    } finally {
      loading.value = false
    }
  }

  /** 把问题清单打包成一条修复指令，交给 AI 面板 */
  const askAiFix = () => {
    if (!report.value || report.value.issue_list.length === 0) {
      ElMessage.info('当前没有需要修的问题')
      return
    }
    const issueText = report.value.issue_list
      .map((issue) => `- ${issue.title_text}：${issue.detail_text}`)
      .join('\n')
    emit(
      'ask-ai',
      `请按下面体检问题逐条修订本章正文，保持人物、情节与既定事实不变，只改表达与节奏：\n${issueText}`
    )
  }

  defineExpose({ run, report })
</script>

<template>
  <div class="chapter-check">
    <div class="chapter-check__head">
      <div class="chapter-check__title">
        <Icon icon="mdi:clipboard-check-outline" :size="16" />
        <span>章节体检</span>
      </div>
      <div class="chapter-check__actions">
        <ElButton size="small" text @click="askAiFix">
          <Icon icon="mdi:robot-outline" :size="14" />
          <span class="chapter-check__btn-text">让 AI 修</span>
        </ElButton>
        <ElButton size="small" type="primary" plain :loading="loading" @click="run">
          <Icon icon="mdi:play" :size="14" />
          <span class="chapter-check__btn-text">体检</span>
        </ElButton>
      </div>
    </div>

    <div v-if="!report && !loading" class="chapter-check__hint">
      体检覆盖字数、破折号、零容忍词、情绪命名、对话标签与章末钩子；结果只读，不改动正文。
    </div>

    <template v-if="report">
      <div class="chapter-check__score">
        <div class="chapter-check__ring" :data-verdict="report.verdict_code">
          <span class="chapter-check__score-value">{{ report.score_value }}</span>
          <span class="chapter-check__score-unit">分</span>
        </div>
        <div class="chapter-check__summary">
          <ElTag :type="verdictMeta.type" effect="light" size="small">{{ verdictMeta.label }}</ElTag>
          <div class="chapter-check__metrics">
            <span>{{ report.word_count.toLocaleString() }} 字</span>
            <span>{{ report.paragraph_count }} 段</span>
            <span>对话 {{ Math.round(report.dialogue_ratio * 100) }}%</span>
            <span>破折号 {{ report.dash_count }}</span>
            <span :class="report.hook_present ? '' : 'is-warn'">
              钩子 {{ report.hook_present ? '有' : '无' }}
            </span>
          </div>
        </div>
      </div>

      <div v-if="report.issue_list.length === 0" class="chapter-check__pass">
        <Icon icon="mdi:check-circle-outline" :size="16" />
        <span>未命中门禁问题</span>
      </div>

      <ul v-else class="chapter-check__issues">
        <li v-for="issue in report.issue_list" :key="issue.issue_code" class="chapter-check__issue">
          <div class="chapter-check__issue-head">
            <ElTag :type="levelType(issue.level_code)" effect="plain" size="small">
              {{ levelLabel(issue.level_code) }}
            </ElTag>
            <span class="chapter-check__issue-title">{{ issue.title_text }}</span>
            <span v-if="showHitCount(issue.issue_code, issue.hit_count)" class="chapter-check__issue-count">
              {{ issue.hit_count }} 处
            </span>
          </div>
          <p class="chapter-check__issue-detail">{{ issue.detail_text }}</p>
          <p v-if="issue.sample_text" class="chapter-check__issue-sample">示例：{{ issue.sample_text }}</p>
        </li>
      </ul>
    </template>
  </div>
</template>

<style lang="less" scoped>
  .chapter-check {
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 12px;
    background: var(--nw-surface);
    border-top: 1px solid var(--nw-border-soft);

    &__head {
      display: flex;
      align-items: center;
      justify-content: space-between;
    }

    &__title {
      display: flex;
      gap: 6px;
      align-items: center;
      font-size: 13px;
      font-weight: 650;
      color: var(--nw-ink);
    }

    &__actions {
      display: flex;
      gap: 4px;
    }

    &__btn-text {
      margin-left: 4px;
    }

    &__hint {
      font-size: 12px;
      line-height: 1.7;
      color: var(--nw-text-muted);
    }

    &__score {
      display: flex;
      gap: 14px;
      align-items: center;
    }

    &__ring {
      display: grid;
      flex: none;
      width: 56px;
      height: 56px;
      color: var(--nw-success);
      background: var(--nw-surface-subtle);
      border: 2px solid var(--nw-success);
      border-radius: 50%;
      place-items: center;

      &[data-verdict='revise'] {
        color: var(--nw-warning);
        border-color: var(--nw-warning);
      }

      &[data-verdict='block'] {
        color: var(--nw-danger);
        border-color: var(--nw-danger);
      }
    }

    &__score-value {
      font-size: 18px;
      font-weight: 700;
      line-height: 1;
    }

    &__score-unit {
      margin-top: 2px;
      font-size: 10px;
      opacity: 0.8;
    }

    &__summary {
      display: flex;
      flex-direction: column;
      gap: 6px;
      min-width: 0;
    }

    &__metrics {
      display: flex;
      flex-wrap: wrap;
      gap: 10px;
      font-size: 11.5px;
      color: var(--nw-text-secondary);

      .is-warn {
        color: var(--nw-warning);
      }
    }

    &__pass {
      display: flex;
      gap: 6px;
      align-items: center;
      font-size: 12.5px;
      color: var(--nw-success);
    }

    &__issues {
      display: flex;
      flex-direction: column;
      gap: 8px;
      max-height: 240px;
      padding: 0;
      margin: 0;
      overflow-y: auto;
      list-style: none;
    }

    &__issue {
      padding: 8px 10px;
      background: var(--nw-surface-subtle);
      border: 1px solid var(--nw-border-soft);
      border-radius: var(--nw-radius-md);
    }

    &__issue-head {
      display: flex;
      gap: 6px;
      align-items: center;
    }

    &__issue-title {
      font-size: 12.5px;
      font-weight: 600;
      color: var(--nw-ink);
    }

    &__issue-count {
      margin-left: auto;
      font-size: 11px;
      color: var(--nw-text-muted);
    }

    &__issue-detail {
      margin-top: 4px;
      font-size: 11.5px;
      line-height: 1.7;
      color: var(--nw-text-secondary);
    }

    &__issue-sample {
      margin-top: 4px;
      font-size: 11.5px;
      color: var(--nw-text-muted);
      word-break: break-all;
    }
  }
</style>

<!-- 新建工作流：选模板 → 选章节 → 定约束，一步开跑。 -->
<script setup lang="ts">
  import { computed, ref, watch } from 'vue'
  import { ElMessage } from 'element-plus'
  import {
    createWorkflowRun,
    type NWWorkflowRunCreateReqVo,
    type NWWorkflowTemplateVo
  } from '@/api/workflow'
  import type { NWChapterBriefVo } from '@/api/contracts'

  const props = defineProps<{
    visible: boolean
    projectUlid: string
    templateList: NWWorkflowTemplateVo[]
    chapterList: NWChapterBriefVo[]
  }>()

  const emit = defineEmits<{
    'update:visible': [visible: boolean]
    created: [runUlid: string]
  }>()

  const submitting = ref(false)
  const templateCode = ref('')
  const chapterMode = ref<'blank' | 'pick' | 'new'>('blank')
  const pickedChapterUlidList = ref<string[]>([])
  const newChapterCount = ref(3)
  const targetWordCount = ref(2500)
  const gateScore = ref(80)
  const reviewGateFlag = ref(true)
  const extraRequirementText = ref('')

  const template = computed(
    () => props.templateList.find((item) => item.template_code === templateCode.value) ?? null
  )

  /** 空白章节：起草类模板的默认目标，避免用户手选 */
  const blankChapterList = computed(() =>
    props.chapterList.filter((chapter) => chapter.word_count === 0)
  )

  /** 已有正文的章节：打磨 / 去 AI 味类模板的默认目标 */
  const writtenChapterList = computed(() =>
    props.chapterList.filter((chapter) => chapter.word_count > 0)
  )

  /** 模板用途决定可选章节范围：起草类要空白章，打磨类要已有正文 */
  const needsWrittenChapter = computed(() =>
    ['chapter_polish', 'chapter_deai'].includes(templateCode.value)
  )

  const stepTypeLabel: Record<string, string> = {
    draft: 'AI 出稿',
    check: '体检',
    revise: 'AI 修订',
    review: 'AI 复审',
    human_review: '人工审核',
    summary: '摘要',
    apply: '入库'
  }

  const resetForm = () => {
    templateCode.value = props.templateList[0]?.template_code ?? ''
    chapterMode.value = blankChapterList.value.length > 0 ? 'blank' : 'new'
    applyTemplateDefaultMode()
    pickedChapterUlidList.value = []
    newChapterCount.value = 3
    targetWordCount.value = 2500
    gateScore.value = 80
    reviewGateFlag.value = true
    extraRequirementText.value = ''
  }

  /** 切模板时把章节范围重置到该模板的合理默认值 */
  const applyTemplateDefaultMode = () => {
    if (needsWrittenChapter.value) {
      chapterMode.value = 'pick'
      pickedChapterUlidList.value = writtenChapterList.value.map((chapter) => chapter.chapter_ulid)
      return
    }
    chapterMode.value = blankChapterList.value.length > 0 ? 'blank' : 'new'
  }

  watch(templateCode, () => applyTemplateDefaultMode())

  watch(
    () => props.visible,
    (visible) => {
      if (visible) resetForm()
    }
  )

  const close = () => emit('update:visible', false)

  const submit = async () => {
    if (!templateCode.value) {
      ElMessage.warning('请选择一个工作流模板')
      return
    }
    if (chapterMode.value === 'pick' && pickedChapterUlidList.value.length === 0) {
      ElMessage.warning(
        needsWrittenChapter.value ? '请选择要处理的已有章节' : '请至少选择一个章节'
      )
      return
    }
    const payload: NWWorkflowRunCreateReqVo = {
      project_ulid: props.projectUlid,
      template_code: templateCode.value,
      target_word_count: targetWordCount.value,
      gate_score: gateScore.value,
      review_gate_flag: reviewGateFlag.value,
      extra_requirement_text: extraRequirementText.value || undefined,
      auto_start_flag: true
    }
    if (chapterMode.value === 'pick') payload.chapter_ulid_list = pickedChapterUlidList.value
    if (chapterMode.value === 'new') payload.new_chapter_count = newChapterCount.value
    submitting.value = true
    try {
      const run = await createWorkflowRun(payload)
      ElMessage.success('工作流已启动，可在下方监控进度')
      emit('created', run.run_ulid)
      close()
    } finally {
      submitting.value = false
    }
  }
</script>

<template>
  <ElDialog
    :model-value="visible"
    title="新建自动化任务"
    width="680px"
    destroy-on-close
    @update:model-value="emit('update:visible', $event)"
  >
    <div class="wf-create">
      <section class="wf-create__section">
        <h4 class="wf-create__title">1. 选一个流程</h4>
        <div class="wf-create__templates">
          <button
            v-for="item in templateList"
            :key="item.template_code"
            type="button"
            :class="['wf-template', { 'is-active': templateCode === item.template_code }]"
            @click="templateCode = item.template_code"
          >
            <strong>{{ item.template_name }}</strong>
            <small>{{ item.description_text }}</small>
          </button>
        </div>
        <div v-if="template" class="wf-create__steps">
          <template v-for="(step, index) in template.step_list" :key="index">
            <span class="wf-create__step">{{ stepTypeLabel[step.step_type_code] ?? step.step_type_code }}</span>
            <span v-if="index < template.step_list.length - 1" class="wf-create__arrow">→</span>
          </template>
        </div>
      </section>

      <section class="wf-create__section">
        <h4 class="wf-create__title">2. 处理哪些章节</h4>
        <ElRadioGroup v-if="!needsWrittenChapter" v-model="chapterMode" class="wf-create__modes">
          <ElRadioButton value="blank">空白章节（{{ blankChapterList.length }}）</ElRadioButton>
          <ElRadioButton value="pick">手动选择</ElRadioButton>
          <ElRadioButton value="new">新建章节</ElRadioButton>
        </ElRadioGroup>
        <ElRadioGroup v-else v-model="chapterMode" class="wf-create__modes">
          <ElRadioButton value="pick">选择已有章节（{{ writtenChapterList.length }}）</ElRadioButton>
        </ElRadioGroup>

        <p v-if="chapterMode === 'blank'" class="wf-create__hint">
          {{
            blankChapterList.length > 0
              ? `将处理 ${blankChapterList.length} 个空白章节，按目录顺序逐章执行。`
              : '当前没有空白章节，请改为手动选择或新建章节。'
          }}
        </p>
        <p v-else-if="needsWrittenChapter && writtenChapterList.length === 0" class="wf-create__hint">
          项目内还没有写好的章节。本模板处理已有正文，请先写章节，或改用「逐章成稿」。
        </p>

        <ElSelect
          v-else-if="chapterMode === 'pick'"
          v-model="pickedChapterUlidList"
          multiple
          filterable
          collapse-tags
          collapse-tags-tooltip
          placeholder="选择要处理的章节"
          class="wf-create__select"
        >
          <ElOption
            v-for="chapter in chapterList"
            :key="chapter.chapter_ulid"
            :label="`第 ${chapter.chapter_order} 章 ${chapter.chapter_name}（${chapter.word_count} 字）`"
            :value="chapter.chapter_ulid"
          />
        </ElSelect>

        <div v-else class="wf-create__row">
          <span class="wf-create__label">新建</span>
          <ElInputNumber v-model="newChapterCount" :min="1" :max="50" size="small" />
          <span class="wf-create__hint">个空白章节（挂到最后一卷）</span>
        </div>
      </section>

      <section class="wf-create__section">
        <h4 class="wf-create__title">3. 约束与审核</h4>
        <div class="wf-create__grid">
          <label class="wf-create__field">
            <span>每章目标字数</span>
            <ElInputNumber v-model="targetWordCount" :min="300" :max="20000" :step="500" />
          </label>
          <label class="wf-create__field">
            <span>体检通过分</span>
            <ElInputNumber v-model="gateScore" :min="60" :max="100" :step="5" />
          </label>
          <label class="wf-create__field wf-create__field--switch">
            <span>人工审核门禁</span>
            <ElSwitch v-model="reviewGateFlag" />
          </label>
        </div>
        <ElInput
          v-model="extraRequirementText"
          type="textarea"
          :rows="3"
          maxlength="1000"
          show-word-limit
          placeholder="额外写作要求（可选）：例如「每章至少一次反转，主角不能开挂」"
        />
        <p class="wf-create__hint">
          开启审核门禁后，每章产出会停在待审核状态，你确认或退回后才继续下一章。
        </p>
      </section>
    </div>

    <template #footer>
      <ElButton @click="close">取消</ElButton>
      <ElButton type="primary" :loading="submitting" @click="submit()">
        <Icon icon="mdi:play" :size="15" />
        <span class="wf-create__btn-text">启动任务</span>
      </ElButton>
    </template>
  </ElDialog>
</template>

<style lang="less" scoped>
  .wf-create {
    display: flex;
    flex-direction: column;
    gap: 18px;

    &__section {
      display: flex;
      flex-direction: column;
      gap: 10px;
    }

    &__title {
      margin: 0;
      font-size: 13px;
      font-weight: 700;
      color: var(--el-text-color-primary);
    }

    &__templates {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(190px, 1fr));
      gap: 8px;
    }

    &__steps {
      display: flex;
      flex-wrap: wrap;
      gap: 6px;
      align-items: center;
      padding: 8px 10px;
      font-size: 12px;
      background: var(--nw-surface-subtle);
      border: 1px solid var(--nw-border-soft);
      border-radius: var(--nw-radius-md);
    }

    &__step {
      padding: 1px 8px;
      color: var(--el-color-primary);
      background: var(--nw-primary-soft);
      border-radius: 10px;
    }

    &__arrow {
      color: var(--el-text-color-placeholder);
    }

    &__modes {
      display: flex;
      gap: 4px;
    }

    &__hint {
      margin: 0;
      font-size: 12px;
      color: var(--el-text-color-secondary);
    }

    &__select {
      width: 100%;
    }

    &__row {
      display: flex;
      gap: 8px;
      align-items: center;
    }

    &__label {
      font-size: 12.5px;
      color: var(--el-text-color-regular);
    }

    &__grid {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
      gap: 12px;
    }

    &__field {
      display: flex;
      flex-direction: column;
      gap: 6px;
      font-size: 12.5px;
      color: var(--el-text-color-regular);

      &--switch {
        flex-direction: row;
        align-items: center;
        justify-content: space-between;
      }
    }

    &__btn-text {
      margin-left: 4px;
    }
  }

  .wf-template {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 10px 12px;
    text-align: left;
    cursor: pointer;
    background: var(--nw-surface);
    border: 1px solid var(--nw-border);
    border-radius: var(--nw-radius-md);
    transition:
      transform 0.18s cubic-bezier(0.34, 1.4, 0.64, 1),
      border-color 0.18s ease,
      box-shadow 0.18s ease;

    strong {
      font-size: 13px;
      color: var(--nw-ink);
    }

    small {
      font-size: 11.5px;
      line-height: 1.5;
      color: var(--el-text-color-secondary);
    }

    &:hover {
      transform: translateY(-1px) scale(1.01);
      border-color: var(--el-color-primary);
      box-shadow: 0 8px 20px rgb(99 102 241 / 14%);
    }

    &:active {
      transform: scale(0.99);
    }

    &.is-active {
      background: var(--nw-primary-soft);
      border-color: var(--el-color-primary);
    }
  }
</style>

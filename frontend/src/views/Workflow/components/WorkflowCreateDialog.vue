<!-- 新建自动化任务：三步向导（选流程 → 查素材 → 定约束），每步都能看清「会跑什么、缺什么」。
     向导不引入新依赖：步骤头自己画，章节手选与约束表单拆到 WorkflowConstraintStep。 -->
<script setup lang="ts">
  import { computed, ref, watch } from 'vue'
  import { ElMessage } from 'element-plus'
  import {
    createWorkflowRun,
    type NWWorkflowReadinessVo,
    type NWWorkflowRunCreateReqVo,
    type NWWorkflowTemplateVo
  } from '@/api/workflow'
  import type { NWChapterBriefVo } from '@/api/contracts'
  import WorkflowConstraintStep from './WorkflowConstraintStep.vue'
  import WorkflowReadinessPanel from './WorkflowReadinessPanel.vue'

  const props = defineProps<{
    visible: boolean
    projectUlid: string
    templateList: NWWorkflowTemplateVo[]
    chapterList: NWChapterBriefVo[]
    readiness: NWWorkflowReadinessVo | null
    readinessLoading: boolean
    /** 深链预选章节（写作台「本章开跑」带入），空串表示按模板默认范围 */
    presetChapterUlid?: string
  }>()

  const emit = defineEmits<{
    'update:visible': [visible: boolean]
    created: [runUlid: string]
    'fix-route': [route: string]
    'refresh-readiness': []
  }>()

  const stepIndex = ref(0)
  const submitting = ref(false)
  const templateCode = ref('')

  /** 第 3 步的约束表单（子组件直接改字段，这里持有同一对象） */
  const constraint = ref({
    targetWordCount: 2500,
    gateScore: 80,
    reviewGateFlag: true,
    extraRequirementText: '',
    chapterMode: 'blank' as 'blank' | 'pick' | 'new',
    newChapterCount: 3,
    pickedChapterUlidList: [] as string[]
  })

  const stepTitleList = ['选流程', '查素材', '定约束']

  /** 模板用途说明：写清「什么时候用」，比步骤名更能帮用户选对 */
  const templateUsageMap: Record<string, string> = {
    chapter_draft: '适合：空白章从零写起，一次把一批章写完',
    chapter_polish: '适合：已有草稿要提升质量，逐章润色',
    chapter_deai: '适合：入库前批量清洗 AI 腔，只改文风不动情节'
  }

  /** 步骤类型说明：标出哪几步消耗模型，让用户对时长与费用有预期 */
  const stepTypeMeta: Record<string, { label: string; icon: string; usesModel: boolean }> = {
    draft: { label: 'AI 出稿', icon: 'mdi:pen', usesModel: true },
    check: { label: '量化体检', icon: 'mdi:clipboard-check-outline', usesModel: false },
    revise: { label: '按问题修订', icon: 'mdi:auto-fix', usesModel: true },
    review: { label: 'AI 复审', icon: 'mdi:magnify-scan', usesModel: true },
    human_review: { label: '人工审核', icon: 'mdi:account-check-outline', usesModel: false },
    summary: { label: '生成摘要', icon: 'mdi:text-box-outline', usesModel: true },
    apply: { label: '定稿入库', icon: 'mdi:content-save-check-outline', usesModel: false }
  }

  const template = computed(
    () => props.templateList.find((item) => item.template_code === templateCode.value) ?? null
  )

  const blankChapterList = computed(() => props.chapterList.filter((chapter) => chapter.word_count === 0))
  const writtenChapterList = computed(() => props.chapterList.filter((chapter) => chapter.word_count > 0))

  /** 模板用途决定可选章节范围：起草类要空白章，打磨类要已有正文 */
  const needsWrittenChapter = computed(() =>
    ['chapter_polish', 'chapter_deai'].includes(templateCode.value)
  )

  const pickableChapterList = computed(() =>
    needsWrittenChapter.value ? writtenChapterList.value : props.chapterList
  )

  const blockingItemList = computed(
    () => props.readiness?.item_list.filter((item) => !item.ready_flag && item.blocking_flag) ?? []
  )

  const modelStepCount = computed(
    () => template.value?.step_list.filter((step) => stepTypeMeta[step.step_type_code]?.usesModel).length ?? 0
  )

  const canGoNext = computed(() => {
    if (stepIndex.value === 0) return Boolean(templateCode.value)
    return true
  })

  /** 深链预选章：只有它确实在当前模板的可选范围内才生效 */
  const applyPresetChapter = () => {
    const presetUlid = (props.presetChapterUlid ?? '').trim()
    if (!presetUlid) return false
    if (!pickableChapterList.value.some((chapter) => chapter.chapter_ulid === presetUlid)) return false
    constraint.value.chapterMode = 'pick'
    constraint.value.pickedChapterUlidList = [presetUlid]
    return true
  }

  /** 预选章已有正文时默认走「存量打磨」：用成稿模板会把作者写好的正文覆盖掉 */
  const templateCodeOfPreset = () => {
    const presetUlid = (props.presetChapterUlid ?? '').trim()
    if (!presetUlid) return ''
    const presetChapter = props.chapterList.find((chapter) => chapter.chapter_ulid === presetUlid)
    if (!presetChapter || presetChapter.word_count === 0) return ''
    const existsFlag = props.templateList.some((item) => item.template_code === 'chapter_polish')
    return existsFlag ? 'chapter_polish' : ''
  }

  /** 切模板时把章节范围重置到该模板的合理默认值（预选章只在打开向导时生效一次） */
  const applyTemplateDefaultMode = () => {
    constraint.value.pickedChapterUlidList = []
    if (needsWrittenChapter.value) {
      constraint.value.chapterMode = 'pick'
      constraint.value.pickedChapterUlidList = writtenChapterList.value.map((chapter) => chapter.chapter_ulid)
      return
    }
    constraint.value.chapterMode = blankChapterList.value.length > 0 ? 'blank' : 'new'
  }

  const resetForm = () => {
    stepIndex.value = 0
    templateCode.value = templateCodeOfPreset() || props.templateList[0]?.template_code || ''
    constraint.value = {
      targetWordCount: 2500,
      gateScore: 80,
      reviewGateFlag: true,
      extraRequirementText: '',
      chapterMode: 'blank',
      newChapterCount: 3,
      pickedChapterUlidList: []
    }
    applyTemplateDefaultMode()
    applyPresetChapter()
  }

  /** 用户换模板：把章节范围重置到该模板的合理默认值（预选章只对打开时的模板生效） */
  const selectTemplate = (nextTemplateCode: string) => {
    templateCode.value = nextTemplateCode
    applyTemplateDefaultMode()
  }

  watch(
    () => props.visible,
    (visible) => {
      if (visible) {
        resetForm()
        emit('refresh-readiness')
      }
    }
  )

  const close = () => emit('update:visible', false)

  const goPrev = () => {
    if (stepIndex.value > 0) stepIndex.value -= 1
  }

  const goNext = () => {
    if (!canGoNext.value) {
      ElMessage.warning('请先选择一个流程')
      return
    }
    if (stepIndex.value < stepTitleList.length - 1) stepIndex.value += 1
  }

  const submit = async () => {
    if (!templateCode.value) {
      ElMessage.warning('请选择一个工作流模板')
      stepIndex.value = 0
      return
    }
    const state = constraint.value
    if (state.chapterMode === 'pick' && state.pickedChapterUlidList.length === 0) {
      ElMessage.warning(needsWrittenChapter.value ? '请选择要处理的已有章节' : '请至少选择一个章节')
      return
    }
    const payload: NWWorkflowRunCreateReqVo = {
      project_ulid: props.projectUlid,
      template_code: templateCode.value,
      target_word_count: state.targetWordCount,
      gate_score: state.gateScore,
      review_gate_flag: state.reviewGateFlag,
      extra_requirement_text: state.extraRequirementText || undefined,
      auto_start_flag: true
    }
    if (state.chapterMode === 'pick') payload.chapter_ulid_list = state.pickedChapterUlidList
    if (state.chapterMode === 'new') payload.new_chapter_count = state.newChapterCount
    submitting.value = true
    try {
      const run = await createWorkflowRun(payload)
      ElMessage.success('任务已启动，正在逐章执行')
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
    width="760px"
    destroy-on-close
    class="wf-wizard-dialog"
    @update:model-value="emit('update:visible', $event)"
  >
    <div class="wf-wizard">
      <ol class="wf-wizard__steps">
        <li
          v-for="(title, index) in stepTitleList"
          :key="title"
          :class="['wf-wizard__step', { 'is-active': stepIndex === index, 'is-done': stepIndex > index }]"
        >
          <span class="wf-wizard__step-index">
            <Icon v-if="stepIndex > index" icon="mdi:check" :size="12" />
            <template v-else>{{ index + 1 }}</template>
          </span>
          <span>{{ title }}</span>
        </li>
      </ol>

      <!-- 第 1 步：选流程 -->
      <section v-show="stepIndex === 0" class="wf-wizard__pane">
        <div class="wf-wizard__templates">
          <button
            v-for="item in templateList"
            :key="item.template_code"
            type="button"
            :class="['wf-template', { 'is-active': templateCode === item.template_code }]"
            @click="selectTemplate(item.template_code)"
          >
            <strong>{{ item.template_name }}</strong>
            <small class="wf-template__usage">{{ templateUsageMap[item.template_code] ?? item.description_text }}</small>
            <small class="wf-template__desc">{{ item.description_text }}</small>
          </button>
        </div>

        <div v-if="template" class="wf-wizard__chain">
          <div class="wf-wizard__chain-head">
            <strong>会跑 {{ template.step_list.length }} 步</strong>
            <span class="nw-hint">其中 {{ modelStepCount }} 步消耗模型</span>
          </div>
          <ul class="wf-chain">
            <li v-for="(step, index) in template.step_list" :key="index" class="wf-chain__item">
              <span :class="['wf-chain__icon', { 'is-model': stepTypeMeta[step.step_type_code]?.usesModel }]">
                <Icon :icon="stepTypeMeta[step.step_type_code]?.icon ?? 'mdi:circle-small'" :size="13" />
              </span>
              <span class="wf-chain__name">{{ step.step_name }}</span>
              <ElTag v-if="stepTypeMeta[step.step_type_code]?.usesModel" size="small" effect="plain" type="primary">
                模型
              </ElTag>
              <ElTag v-else-if="step.step_type_code === 'human_review'" size="small" effect="plain" type="warning">
                需要你点头
              </ElTag>
              <ElTag v-else size="small" effect="plain" type="info">自动</ElTag>
            </li>
          </ul>
        </div>
      </section>

      <!-- 第 2 步：查素材 -->
      <section v-show="stepIndex === 1" class="wf-wizard__pane">
        <WorkflowReadinessPanel
          :readiness="readiness"
          :loading="readinessLoading"
          compact
          @fix="(route) => emit('fix-route', route)"
        />
        <ElAlert
          v-if="blockingItemList.length > 0"
          type="warning"
          :closable="false"
          show-icon
          :title="`还有 ${blockingItemList.length} 项必填素材未就绪`"
        >
          仍然可以继续，但产出很可能偏离你的设定：AI 缺少这些信息时只能自己编。建议先去补齐再开跑。
        </ElAlert>
        <p v-else-if="readiness" class="wf-wizard__ready-hint">
          <Icon icon="mdi:check-decagram" :size="14" />
          <span>素材齐备，可以直接进入下一步。</span>
        </p>
      </section>

      <!-- 第 3 步：定约束 -->
      <section v-show="stepIndex === 2">
        <WorkflowConstraintStep
          :state="constraint"
          :pickable-chapter-list="pickableChapterList"
          :blank-chapter-count="blankChapterList.length"
          :needs-written-chapter="needsWrittenChapter"
        />
      </section>
    </div>

    <template #footer>
      <div class="wf-wizard__footer">
        <ElButton v-if="stepIndex > 0" text @click="goPrev()">
          <Icon icon="mdi:chevron-left" :size="15" />
          <span class="wf-wizard__btn-text">上一步</span>
        </ElButton>
        <div class="wf-wizard__spacer" />
        <ElButton @click="close">取消</ElButton>
        <ElButton v-if="stepIndex < stepTitleList.length - 1" type="primary" plain @click="goNext()">
          <span>下一步</span>
          <Icon icon="mdi:chevron-right" :size="15" />
        </ElButton>
        <ElButton v-else type="primary" :loading="submitting" @click="submit()">
          <Icon icon="mdi:play" :size="15" />
          <span class="wf-wizard__btn-text">开始执行</span>
        </ElButton>
      </div>
    </template>
  </ElDialog>
</template>

<style lang="less" scoped>
  @import './workflowWizard.less';
</style>

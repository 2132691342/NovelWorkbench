<!-- 项目编辑弹窗：基础信息 + 自动化默认参数。
     自动化参数是「一次配置、长期复用」的那一档：字数、门禁分、审核开关、整体要求定在这里，
     之后在自动化页点「一键开跑」就不用再填。 -->
<script setup lang="ts">
  import { computed, reactive, ref } from 'vue'
  import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
  import { useI18n } from 'vue-i18n'
  import { createProject, updateProject } from '@/api/project'
  import type { NWProjectVo } from '@/api/contracts'

  const emit = defineEmits<{ saved: [] }>()

  /** 弹窗可见性：由组件内部管理，父组件通过 ref.open() 打开 */
  const visible = ref(false)

  const { t } = useI18n()

  const formRef = ref<FormInstance>()
  const editing = ref<NWProjectVo | null>(null)
  const saving = ref(false)
  const activeTab = ref<'basic' | 'automation'>('basic')

  const form = reactive({
    project_name: '',
    genre_code: '',
    style_text: '',
    synopsis_text: '',
    project_status: 'draft',
    auto_target_word_count: 2500,
    auto_gate_score: 80,
    auto_review_gate_flag: true,
    auto_requirement_text: ''
  })

  const title = computed(() => (editing.value ? '编辑项目' : '新建项目'))

  const genreOptions = [
    { value: '', label: '未分类' },
    { value: 'fantasy', label: '玄幻' },
    { value: 'scifi', label: '科幻' },
    { value: 'romance', label: '言情' },
    { value: 'suspense', label: '悬疑' },
    { value: 'wuxia', label: '武侠' },
    { value: 'urban', label: '都市' },
    { value: 'history', label: '历史' }
  ]

  const targetWordPresetList = [1500, 2500, 3500, 5000]

  const rules = computed<FormRules>(() => ({
    project_name: [{ required: true, message: t('common.required'), trigger: 'blur' }]
  }))

  /** 打开弹窗：传项目为编辑，不传为新建 */
  const open = (project?: NWProjectVo) => {
    editing.value = project ?? null
    activeTab.value = 'basic'
    form.project_name = project?.project_name ?? ''
    form.genre_code = project?.genre_code ?? ''
    form.style_text = project?.style_text ?? ''
    form.synopsis_text = project?.synopsis_text ?? ''
    form.project_status = project?.project_status ?? 'draft'
    form.auto_target_word_count = project?.auto_target_word_count || 2500
    form.auto_gate_score = project?.auto_gate_score || 80
    form.auto_review_gate_flag = project?.auto_review_gate_flag ?? true
    form.auto_requirement_text = project?.auto_requirement_text ?? ''
    visible.value = true
  }

  const automationPayload = () => ({
    auto_target_word_count: form.auto_target_word_count,
    auto_gate_score: form.auto_gate_score,
    auto_review_gate_flag: form.auto_review_gate_flag,
    auto_requirement_text: form.auto_requirement_text
  })

  const submit = async () => {
    const valid = await formRef.value?.validate().catch(() => false)
    if (!valid) return

    saving.value = true
    try {
      if (editing.value) {
        // 后端字段是三态：缺省不改、空串清空。这里始终带上表单当前值，
        // 用户确实清空了某个字段时就必须把空串传下去，不能靠「值非空才传」偷懒。
        await updateProject({
          project_ulid: editing.value.project_ulid,
          project_name: form.project_name,
          genre_code: form.genre_code,
          style_text: form.style_text,
          synopsis_text: form.synopsis_text,
          project_status: form.project_status,
          ...automationPayload()
        })
      } else {
        // 新建只提交基础信息：自动化参数走服务端默认值，建好后可在编辑里调整
        await createProject({
          project_name: form.project_name,
          genre_code: form.genre_code,
          style_text: form.style_text,
          synopsis_text: form.synopsis_text
        })
      }
      ElMessage.success(t('common.saveSuccess'))
      visible.value = false
      emit('saved')
    } finally {
      saving.value = false
    }
  }

  defineExpose({ open })
</script>

<template>
  <ElDialog v-model="visible" :title="title" width="620px" append-to-body destroy-on-close>
    <ElForm ref="formRef" :model="form" :rules="rules" label-position="top">
      <ElTabs v-model="activeTab" class="project-form__tabs">
        <ElTabPane label="基础信息" name="basic">
          <ElFormItem label="项目名称" prop="project_name">
            <ElInput v-model="form.project_name" maxlength="80" show-word-limit placeholder="例如：长夜行" />
          </ElFormItem>
          <div class="project-form__row">
            <ElFormItem label="题材">
              <ElSelect v-model="form.genre_code" class="project-form__select">
                <ElOption v-for="item in genreOptions" :key="item.value" :label="item.label" :value="item.value" />
              </ElSelect>
            </ElFormItem>
            <ElFormItem v-if="editing" label="状态">
              <ElSelect v-model="form.project_status" class="project-form__select">
                <ElOption label="草稿" value="draft" />
                <ElOption label="连载中" value="serial" />
                <ElOption label="已完结" value="completed" />
              </ElSelect>
            </ElFormItem>
          </div>
          <ElFormItem label="文风要求">
            <ElInput
              v-model="form.style_text"
              type="textarea"
              :rows="2"
              maxlength="300"
              placeholder="例如：冷峻克制，短句为主，少形容词"
            />
          </ElFormItem>
          <ElFormItem label="简介">
            <ElInput v-model="form.synopsis_text" type="textarea" :rows="3" maxlength="500" />
          </ElFormItem>
        </ElTabPane>

        <ElTabPane label="自动化默认值" name="automation" :disabled="!editing">
          <p class="nw-hint project-form__hint">
            这里的值会作为「一键开跑」的默认参数，开跑时仍可临时覆盖。定好一次，之后每章都按同一把尺子写。
          </p>
          <ElFormItem label="每章目标字数">
            <div class="project-form__slider">
              <ElSlider v-model="form.auto_target_word_count" :min="300" :max="20000" :step="100" />
              <span class="project-form__value">{{ form.auto_target_word_count }} 字</span>
            </div>
            <div class="project-form__presets">
              <ElButton
                v-for="preset in targetWordPresetList"
                :key="preset"
                size="small"
                plain
                :type="form.auto_target_word_count === preset ? 'primary' : 'default'"
                @click="form.auto_target_word_count = preset"
              >
                {{ preset }}
              </ElButton>
            </div>
          </ElFormItem>
          <ElFormItem label="体检通过分">
            <div class="project-form__slider">
              <ElSlider v-model="form.auto_gate_score" :min="60" :max="100" :step="1" />
              <span class="project-form__value">{{ form.auto_gate_score }} 分</span>
            </div>
            <p class="nw-hint">低于这个分数会自动进入修订步骤，仍然不达标就停下来等你看。</p>
          </ElFormItem>
          <ElFormItem label="人工审核门禁">
            <ElSwitch v-model="form.auto_review_gate_flag" />
            <p class="nw-hint">
              {{
                form.auto_review_gate_flag
                  ? '每章写完停下来等你点头，通过后才入库并继续下一章'
                  : 'AI 直通入库，不等审核（适合批量清洗已有稿）'
              }}
            </p>
          </ElFormItem>
          <ElFormItem label="整体要求（每章都注入）">
            <ElInput
              v-model="form.auto_requirement_text"
              type="textarea"
              :rows="3"
              maxlength="1000"
              show-word-limit
              placeholder="例如：主角不能开挂；每章至少一次反转；禁止写回忆倒叙"
            />
          </ElFormItem>
        </ElTabPane>
      </ElTabs>
    </ElForm>

    <template #footer>
      <ElButton @click="visible = false">{{ t('common.cancel') }}</ElButton>
      <ElButton type="primary" :loading="saving" @click="submit">{{ t('common.save') }}</ElButton>
    </template>
  </ElDialog>
</template>

<style lang="less" scoped>
  .project-form {
    &__tabs {
      :deep(.el-tabs__header) {
        margin-bottom: 12px;
      }
    }

    &__row {
      display: grid;
      gap: 12px;
      grid-template-columns: 1fr 1fr;
    }

    &__select {
      width: 100%;
    }

    &__hint {
      margin: 0 0 12px;
    }

    &__slider {
      display: flex;
      gap: 12px;
      align-items: center;
      width: 100%;

      :deep(.el-slider) {
        flex: 1;
      }
    }

    &__value {
      min-width: 62px;
      font-size: 12px;
      font-variant-numeric: tabular-nums;
      color: var(--nw-ink);
      text-align: right;
    }

    &__presets {
      display: flex;
      gap: 6px;
      margin-top: 6px;
    }
  }
</style>

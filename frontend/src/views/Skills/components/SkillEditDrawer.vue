<!-- 技能编辑抽屉：维护模板、变量与启停状态。 -->
<script setup lang="ts">
  import { computed, reactive, ref } from 'vue'
  import {
    ElButton,
    ElDrawer,
    ElForm,
    ElFormItem,
    ElInput,
    ElInputNumber,
    ElMessage,
    ElOption,
    ElSelect,
    ElSwitch,
    ElTag,
    type FormInstance,
    type FormRules
  } from 'element-plus'
  import { saveSkill, type NWSkillSaveReqVo } from '@/api/skill'
  import type { NWProjectVo, NWSkillVariableVo, NWSkillVo } from '@/api/contracts'
  import SkillReferencePanel from './SkillReferencePanel.vue'

  const props = defineProps<{ projects: NWProjectVo[] }>()
  const emit = defineEmits<{ saved: [] }>()

  const visible = ref(false)
  const saving = ref(false)
  const formRef = ref<FormInstance>()
  const editing = ref<NWSkillVo | null>(null)

  const categoryOptions = [
    { value: 'write', label: '写作' },
    { value: 'revise', label: '修订' },
    { value: 'outline', label: '规划' },
    { value: 'setting', label: '设定' },
    { value: 'review', label: '审稿' },
    { value: 'other', label: '其他' }
  ]

  const form = reactive({
    skill_code: '',
    skill_name: '',
    scope_code: 'global',
    project_ulid: '',
    category_code: 'other',
    description_text: '',
    prompt_template: '',
    sort_order: 0,
    enabled_flag: true
  })

  const variableList = ref<NWSkillVariableVo[]>([])

  /** 模板输入提示（模板字符串在模板属性里易被误解析，统一放在脚本中） */
  const templatePlaceholder =
    '用 {{variable_name}} 引用变量，内置占位符：{{user_input}}、{{chapter_content}}、{{context}}'

  const title = computed(() => (editing.value ? '编辑技能' : '新建技能'))
  const isBuiltin = computed(() => editing.value?.builtin_flag === true)
  const canManageReferences = computed(() => Boolean(editing.value?.skill_ulid))

  const rules: FormRules = {
    skill_code: [
      { required: true, message: '请填写技能编码', trigger: 'blur' },
      { pattern: /^[a-z0-9_-]{2,32}$/, message: '2-32 位小写字母、数字、下划线或连字符', trigger: 'blur' }
    ],
    skill_name: [{ required: true, message: '请填写技能名称', trigger: 'blur' }],
    prompt_template: [{ required: true, message: '请填写提示词模板', trigger: 'blur' }]
  }

  /** 打开抽屉：传技能为编辑，不传为新建 */
  const open = (skill?: NWSkillVo, defaultProjectUlid = '') => {
    editing.value = skill ?? null
    form.skill_code = skill?.skill_code ?? ''
    form.skill_name = skill?.skill_name ?? ''
    form.scope_code = skill?.scope_code ?? 'global'
    form.project_ulid = skill?.project_ulid || defaultProjectUlid
    form.category_code = skill?.category_code || 'other'
    form.description_text = skill?.description_text ?? ''
    form.prompt_template = skill?.prompt_template ?? ''
    form.sort_order = skill?.sort_order ?? 0
    form.enabled_flag = skill?.enabled_flag ?? true
    variableList.value = (skill?.variable_list ?? []).map((item) => ({ ...item }))
    visible.value = true
  }

  const addVariable = () => {
    variableList.value.push({
      variable_name: '',
      variable_label: '',
      default_value: '',
      required_flag: false,
      description_text: ''
    })
  }

  const removeVariable = (index: number) => {
    variableList.value.splice(index, 1)
  }

  const submit = async () => {
    const valid = await formRef.value?.validate().catch(() => false)
    if (!valid) return
    if (form.scope_code === 'project' && !form.project_ulid) {
      ElMessage.warning('项目技能必须选择所属项目')
      return
    }
    const invalidVariable = variableList.value.find((item) => !item.variable_name.trim())
    if (invalidVariable) {
      ElMessage.warning('变量名不能为空')
      return
    }

    const req: NWSkillSaveReqVo = {
      skill_ulid: editing.value?.skill_ulid,
      skill_code: form.skill_code,
      skill_name: form.skill_name,
      scope_code: form.scope_code,
      project_ulid: form.scope_code === 'project' ? form.project_ulid : undefined,
      category_code: form.category_code,
      description_text: form.description_text,
      prompt_template: form.prompt_template,
      variable_list: variableList.value.map((item) => ({
        variable_name: item.variable_name.trim(),
        variable_label: item.variable_label,
        default_value: item.default_value,
        required_flag: item.required_flag,
        description_text: item.description_text
      })),
      enabled_flag: form.enabled_flag,
      sort_order: form.sort_order
    }

    saving.value = true
    try {
      const saved = await saveSkill(req)
      editing.value = saved
      ElMessage.success('技能已保存')
      emit('saved')
    } finally {
      saving.value = false
    }
  }

  defineExpose({ open })
</script>

<template>
  <ElDrawer v-model="visible" :title="title" size="680px" append-to-body destroy-on-close>
    <ElForm ref="formRef" :model="form" :rules="rules" label-position="top">
      <div class="skill-form__row">
        <ElFormItem label="技能编码" prop="skill_code">
          <ElInput
            v-model="form.skill_code"
            :disabled="isBuiltin"
            placeholder="例如：polish"
            maxlength="32"
          />
        </ElFormItem>
        <ElFormItem label="技能名称" prop="skill_name">
          <ElInput v-model="form.skill_name" maxlength="64" placeholder="例如：润色改写" />
        </ElFormItem>
      </div>

      <div class="skill-form__row">
        <ElFormItem label="作用域">
          <ElSelect v-model="form.scope_code" :disabled="isBuiltin" class="skill-form__select">
            <ElOption label="全局（所有项目可用）" value="global" />
            <ElOption label="项目（仅所属项目可用）" value="project" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem v-if="form.scope_code === 'project'" label="所属项目">
          <ElSelect
            v-model="form.project_ulid"
            class="skill-form__select"
            filterable
            placeholder="选择项目"
          >
            <ElOption
              v-for="item in props.projects"
              :key="item.project_ulid"
              :label="item.project_name"
              :value="item.project_ulid"
            />
          </ElSelect>
        </ElFormItem>
      </div>

      <div class="skill-form__row">
        <ElFormItem label="分类">
          <ElSelect v-model="form.category_code" class="skill-form__select">
            <ElOption
              v-for="item in categoryOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="排序号">
          <ElInputNumber v-model="form.sort_order" :min="0" :max="9999" controls-position="right" />
        </ElFormItem>
      </div>

      <ElFormItem label="说明">
        <ElInput v-model="form.description_text" maxlength="256" show-word-limit />
      </ElFormItem>

      <ElFormItem label="提示词模板" prop="prompt_template">
        <ElInput
          v-model="form.prompt_template"
          type="textarea"
          :rows="10"
          maxlength="8000"
          show-word-limit
          :placeholder="templatePlaceholder"
        />
      </ElFormItem>

      <div class="skill-form__vars">
        <div class="skill-form__vars-head">
          <span class="skill-form__vars-title">变量声明</span>
          <span class="nw-hint">模板中的占位符必须在此登记，否则保存会被拒绝</span>
          <div class="skill-form__spacer" />
          <ElButton size="small" plain @click="addVariable">
            <Icon icon="mdi:plus" :size="13" />
            <span class="ml-4px">添加变量</span>
          </ElButton>
        </div>

        <p v-if="variableList.length === 0" class="nw-hint">还没有变量，模板可直接使用内置占位符。</p>

        <div v-for="(item, index) in variableList" :key="index" class="skill-form__var-item">
          <div class="skill-form__var">
            <ElInput v-model="item.variable_name" placeholder="变量名" maxlength="32" />
            <ElInput v-model="item.variable_label" placeholder="展示名" maxlength="32" />
            <ElInput v-model="item.default_value" placeholder="缺省值" maxlength="200" />
            <div class="skill-form__var-flags">
              <ElSwitch v-model="item.required_flag" size="small" />
              <span class="nw-hint">必填</span>
            </div>
            <ElButton size="small" text type="danger" @click="removeVariable(index)">
              <Icon icon="mdi:delete-outline" :size="14" />
            </ElButton>
          </div>
          <ElInput
            v-model="item.description_text"
            placeholder="填写提示（告诉使用者这个变量该填什么）"
            maxlength="200"
            size="small"
          />
        </div>
      </div>

      <ElFormItem label="启用">
        <div class="skill-form__enabled">
          <ElSwitch v-model="form.enabled_flag" />
          <ElTag v-if="isBuiltin" size="small" type="info" effect="plain">内置技能</ElTag>
          <ElTag v-if="editing?.customized_flag" size="small" type="warning" effect="plain">
            已自定义
          </ElTag>
        </div>
      </ElFormItem>

      <SkillReferencePanel v-if="canManageReferences" :skill-ulid="editing?.skill_ulid ?? ''" />
      <p v-else class="nw-hint">保存技能后即可管理参考资料。</p>
    </ElForm>

    <template #footer>
      <ElButton @click="visible = false">关闭</ElButton>
      <ElButton type="primary" :loading="saving" @click="submit">保存</ElButton>
    </template>
  </ElDrawer>
</template>

<style lang="less" scoped>
  .skill-form {
    &__row {
      display: grid;
      gap: 12px;
      grid-template-columns: 1fr 1fr;
    }

    &__select {
      width: 100%;
    }

    &__spacer {
      flex: 1;
    }

    &__vars {
      display: flex;
      flex-direction: column;
      gap: 8px;
      padding: 12px;
      margin-bottom: 18px;
      background: var(--nw-surface-subtle);
      border-radius: var(--nw-radius-md);
    }

    &__vars-head {
      display: flex;
      flex-wrap: wrap;
      gap: 10px;
      align-items: center;
    }

    &__vars-title {
      font-size: 13px;
      font-weight: 650;
      color: var(--nw-ink);
    }

    &__var-item {
      display: flex;
      flex-direction: column;
      gap: 6px;
    }

    &__var {
      display: grid;
      gap: 8px;
      align-items: center;
      grid-template-columns: 1.2fr 1fr 1.2fr auto auto;
    }

    &__var-flags {
      display: flex;
      gap: 6px;
      align-items: center;
    }

    &__enabled {
      display: flex;
      gap: 10px;
      align-items: center;
    }
  }
</style>

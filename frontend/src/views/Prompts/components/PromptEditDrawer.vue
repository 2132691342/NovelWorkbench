<!-- 档案编辑抽屉：维护系统提示词正文与作用域。 -->
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
  import { savePromptProfile, type NWPromptProfileSaveReqVo } from '@/api/prompt'
  import type { NWProjectVo, NWPromptProfileVo } from '@/api/contracts'

  const props = defineProps<{ projects: NWProjectVo[] }>()
  const emit = defineEmits<{ saved: [] }>()

  const visible = ref(false)
  const saving = ref(false)
  const formRef = ref<FormInstance>()
  const editing = ref<NWPromptProfileVo | null>(null)

  const form = reactive({
    prompt_profile_code: '',
    prompt_profile_name: '',
    scope_code: 'global',
    project_ulid: '',
    description_text: '',
    system_prompt_text: '',
    sort_order: 0,
    enabled_flag: true
  })

  const title = computed(() => (editing.value ? '编辑提示词档案' : '新建提示词档案'))
  const isBuiltin = computed(() => editing.value?.builtin_flag === true)

  const rules: FormRules = {
    prompt_profile_code: [
      { required: true, message: '请填写档案编码', trigger: 'blur' },
      { pattern: /^[a-z0-9_-]{2,32}$/, message: '2-32 位小写字母、数字、下划线或连字符', trigger: 'blur' }
    ],
    prompt_profile_name: [{ required: true, message: '请填写档案名', trigger: 'blur' }],
    system_prompt_text: [
      { required: true, message: '请填写系统提示词', trigger: 'blur' },
      { min: 10, message: '系统提示词至少 10 个字符', trigger: 'blur' }
    ]
  }

  /** 打开抽屉：传档案为编辑，不传为新建 */
  const open = (profile?: NWPromptProfileVo, defaultProjectUlid = '') => {
    editing.value = profile ?? null
    form.prompt_profile_code = profile?.prompt_profile_code ?? ''
    form.prompt_profile_name = profile?.prompt_profile_name ?? ''
    form.scope_code = profile?.scope_code ?? 'global'
    form.project_ulid = profile?.project_ulid || defaultProjectUlid
    form.description_text = profile?.description_text ?? ''
    form.system_prompt_text = profile?.system_prompt_text ?? ''
    form.sort_order = profile?.sort_order ?? 0
    form.enabled_flag = profile?.enabled_flag ?? true
    visible.value = true
  }

  const submit = async () => {
    const valid = await formRef.value?.validate().catch(() => false)
    if (!valid) return
    if (form.scope_code === 'project' && !form.project_ulid) {
      ElMessage.warning('项目档案必须选择所属项目')
      return
    }

    const req: NWPromptProfileSaveReqVo = {
      prompt_profile_ulid: editing.value?.prompt_profile_ulid,
      prompt_profile_code: form.prompt_profile_code,
      prompt_profile_name: form.prompt_profile_name,
      scope_code: form.scope_code,
      project_ulid: form.scope_code === 'project' ? form.project_ulid : undefined,
      description_text: form.description_text,
      system_prompt_text: form.system_prompt_text,
      enabled_flag: form.enabled_flag,
      sort_order: form.sort_order
    }

    saving.value = true
    try {
      await savePromptProfile(req)
      ElMessage.success('提示词档案已保存')
      visible.value = false
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
      <div class="prompt-form__row">
        <ElFormItem label="档案编码" prop="prompt_profile_code">
          <ElInput
            v-model="form.prompt_profile_code"
            :disabled="isBuiltin"
            maxlength="32"
            placeholder="例如：default"
          />
        </ElFormItem>
        <ElFormItem label="档案名" prop="prompt_profile_name">
          <ElInput v-model="form.prompt_profile_name" maxlength="64" placeholder="例如：通用写作助手" />
        </ElFormItem>
      </div>

      <div class="prompt-form__row">
        <ElFormItem label="作用域">
          <ElSelect v-model="form.scope_code" :disabled="Boolean(editing)" class="prompt-form__select">
            <ElOption label="全局（所有项目可用）" value="global" />
            <ElOption label="项目（仅所属项目可用）" value="project" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem v-if="form.scope_code === 'project'" label="所属项目">
          <ElSelect
            v-model="form.project_ulid"
            class="prompt-form__select"
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
        <ElFormItem label="排序号">
          <ElInputNumber v-model="form.sort_order" :min="0" :max="9999" controls-position="right" />
        </ElFormItem>
      </div>

      <ElFormItem label="说明">
        <ElInput v-model="form.description_text" maxlength="256" show-word-limit />
      </ElFormItem>

      <ElFormItem label="系统提示词" prop="system_prompt_text">
        <ElInput
          v-model="form.system_prompt_text"
          type="textarea"
          :rows="16"
          maxlength="8000"
          show-word-limit
          placeholder="定义 AI 的身份、语气、写作底线与输出约束"
        />
      </ElFormItem>

      <ElFormItem label="启用">
        <div class="prompt-form__enabled">
          <ElSwitch v-model="form.enabled_flag" />
          <ElTag v-if="isBuiltin" size="small" type="info" effect="plain">内置档案</ElTag>
          <ElTag v-if="editing?.customized_flag" size="small" type="warning" effect="plain">
            已自定义
          </ElTag>
        </div>
      </ElFormItem>
    </ElForm>

    <template #footer>
      <ElButton @click="visible = false">取消</ElButton>
      <ElButton type="primary" :loading="saving" @click="submit">保存</ElButton>
    </template>
  </ElDrawer>
</template>

<style lang="less" scoped>
  .prompt-form {
    &__row {
      display: grid;
      gap: 12px;
      grid-template-columns: 1fr 1fr;
    }

    &__select {
      width: 100%;
    }

    &__enabled {
      display: flex;
      gap: 10px;
      align-items: center;
    }
  }
</style>

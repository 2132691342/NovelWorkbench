<!-- 提示词编辑抽屉：维护提示词种类、作用域、正文与启停。 -->
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
  import { savePrompt, type NWPromptSaveReqVo } from '@/api/prompt'
  import type { NWProjectVo, NWPromptVo } from '@/api/contracts'

  const props = defineProps<{ projects: NWProjectVo[] }>()
  const emit = defineEmits<{ saved: [] }>()

  const visible = ref(false)
  const saving = ref(false)
  const formRef = ref<FormInstance>()
  const editing = ref<NWPromptVo | null>(null)

  const form = reactive({
    prompt_code: '',
    prompt_name: '',
    prompt_kind_code: 'command',
    scope_code: 'global',
    project_ulid: '',
    category_code: 'other',
    description_text: '',
    prompt_text: '',
    sort_order: 0,
    enabled_flag: true
  })

  const title = computed(() => (editing.value ? '编辑提示词' : '新建提示词'))
  const isBuiltin = computed(() => editing.value?.builtin_flag === true)

  const rules: FormRules = {
    prompt_code: [
      { required: true, message: '请填写提示词编码', trigger: 'blur' },
      { pattern: /^[a-z0-9_-]{2,32}$/, message: '2-32 位小写字母、数字、下划线或连字符', trigger: 'blur' }
    ],
    prompt_name: [{ required: true, message: '请填写提示词名称', trigger: 'blur' }],
    prompt_text: [
      { required: true, message: '请填写提示词正文', trigger: 'blur' },
      { min: 10, message: '提示词正文至少 10 个字符', trigger: 'blur' }
    ]
  }

  /** 打开抽屉：传提示词为编辑，不传为新建 */
  const open = (prompt?: NWPromptVo, defaultProjectUlid = '') => {
    editing.value = prompt ?? null
    form.prompt_code = prompt?.prompt_code ?? ''
    form.prompt_name = prompt?.prompt_name ?? ''
    form.prompt_kind_code = prompt?.prompt_kind_code ?? 'command'
    form.scope_code = prompt?.scope_code ?? 'global'
    form.project_ulid = prompt?.project_ulid || defaultProjectUlid
    form.category_code = prompt?.category_code || 'other'
    form.description_text = prompt?.description_text ?? ''
    form.prompt_text = prompt?.prompt_text ?? ''
    form.sort_order = prompt?.sort_order ?? 0
    form.enabled_flag = prompt?.enabled_flag ?? true
    visible.value = true
  }

  const submit = async () => {
    const valid = await formRef.value?.validate().catch(() => false)
    if (!valid) return
    if (form.scope_code === 'project' && !form.project_ulid) {
      ElMessage.warning('项目提示词必须选择所属项目')
      return
    }

    const req: NWPromptSaveReqVo = {
      prompt_ulid: editing.value?.prompt_ulid,
      prompt_code: form.prompt_code,
      prompt_name: form.prompt_name,
      prompt_kind_code: form.prompt_kind_code,
      scope_code: form.scope_code,
      project_ulid: form.scope_code === 'project' ? form.project_ulid : undefined,
      category_code: form.category_code,
      description_text: form.description_text,
      prompt_text: form.prompt_text,
      enabled_flag: form.enabled_flag,
      sort_order: form.sort_order
    }

    saving.value = true
    try {
      await savePrompt(req)
      ElMessage.success('提示词已保存')
      visible.value = false
      emit('saved')
    } finally {
      saving.value = false
    }
  }

  defineExpose({ open })
</script>

<template>
  <ElDrawer v-model="visible" :title="title" size="720px" append-to-body destroy-on-close>
    <ElForm ref="formRef" :model="form" :rules="rules" label-position="top">
      <div class="prompt-form__row">
        <ElFormItem label="提示词编码" prop="prompt_code">
          <ElInput
            v-model="form.prompt_code"
            :disabled="isBuiltin"
            maxlength="32"
            placeholder="例如：continue"
          />
        </ElFormItem>
        <ElFormItem label="提示词名称" prop="prompt_name">
          <ElInput v-model="form.prompt_name" maxlength="64" placeholder="例如：续写正文" />
        </ElFormItem>
      </div>

      <div class="prompt-form__row">
        <ElFormItem label="种类">
          <ElSelect v-model="form.prompt_kind_code" :disabled="isBuiltin" class="prompt-form__select">
            <ElOption label="身份提示词（会话常驻，系统级）" value="system" />
            <ElOption label="指令提示词（/ 触发，系统级）" value="command" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="分类">
          <ElSelect v-model="form.category_code" class="prompt-form__select">
            <ElOption label="写作" value="write" />
            <ElOption label="修订" value="revise" />
            <ElOption label="规划" value="outline" />
            <ElOption label="设定" value="setting" />
            <ElOption label="审稿" value="review" />
            <ElOption label="其他" value="other" />
          </ElSelect>
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

      <ElFormItem label="提示词正文" prop="prompt_text">
        <ElInput
          v-model="form.prompt_text"
          type="textarea"
          :rows="18"
          maxlength="12000"
          show-word-limit
          placeholder="写清身份与底线，或写清这一单活怎么做、按什么格式交付。可用 {{context}}、{{chapter_content}}、{{user_input}} 占位符。"
        />
      </ElFormItem>

      <ElFormItem label="启用">
        <div class="prompt-form__enabled">
          <ElSwitch v-model="form.enabled_flag" />
          <ElTag v-if="isBuiltin" size="small" type="info" effect="plain">内置提示词</ElTag>
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

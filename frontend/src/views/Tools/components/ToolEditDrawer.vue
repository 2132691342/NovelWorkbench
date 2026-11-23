<!-- 工具编辑抽屉：名称、说明、触发关键词与参数声明；内置工具不可改编码。 -->
<script setup lang="ts">
  import { computed, reactive, ref } from 'vue'
  import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
  import { saveTool, type NWToolParamVo, type NWToolVo } from '@/api/tool'

  const emit = defineEmits<{ saved: [] }>()

  const visible = ref(false)
  const saving = ref(false)
  const formRef = ref<FormInstance>()
  const editing = ref<NWToolVo | null>(null)

  const form = reactive({
    tool_code: '',
    tool_name: '',
    description_text: '',
    trigger_keywords: '',
    param_list: [] as NWToolParamVo[],
    enabled_flag: true,
    sort_order: 100
  })

  const title = computed(() => (editing.value ? `编辑工具 · ${editing.value.tool_name}` : '新建工具'))
  const builtinFlag = computed(() => editing.value?.builtin_flag === true)

  const rules: FormRules = {
    tool_code: [
      { required: true, message: '请输入工具编码', trigger: 'blur' },
      { max: 32, message: '最长 32 个字符', trigger: 'blur' }
    ],
    tool_name: [{ required: true, message: '请输入工具名称', trigger: 'blur' }]
  }

  const open = (row?: NWToolVo) => {
    editing.value = row ?? null
    form.tool_code = row?.tool_code ?? ''
    form.tool_name = row?.tool_name ?? ''
    form.description_text = row?.description_text ?? ''
    form.trigger_keywords = row?.trigger_keywords ?? ''
    form.param_list = (row?.param_list ?? []).map((item) => ({ ...item }))
    form.enabled_flag = row?.enabled_flag ?? true
    form.sort_order = row?.sort_order ?? 100
    visible.value = true
    formRef.value?.clearValidate()
  }

  const addParam = () => {
    form.param_list.push({
      param_name: '',
      param_label: '',
      required_flag: false,
      default_value: '',
      description_text: ''
    })
  }

  const removeParam = (index: number) => {
    form.param_list.splice(index, 1)
  }

  const submit = async () => {
    const valid = await formRef.value?.validate().catch(() => false)
    if (!valid) return
    saving.value = true
    try {
      await saveTool({
        tool_ulid: editing.value?.tool_ulid,
        tool_code: form.tool_code,
        tool_name: form.tool_name,
        description_text: form.description_text,
        trigger_keywords: form.trigger_keywords,
        param_list: form.param_list.filter((item) => item.param_name.trim() !== ''),
        enabled_flag: form.enabled_flag,
        sort_order: form.sort_order
      })
      ElMessage.success(editing.value ? '工具已更新' : '工具已创建')
      visible.value = false
      emit('saved')
    } finally {
      saving.value = false
    }
  }

  defineExpose({ open })
</script>

<template>
  <ElDrawer v-model="visible" :title="title" size="600px" destroy-on-close>
    <ElAlert v-if="builtinFlag" type="warning" :closable="false" show-icon class="tool-edit__alert">
      内置工具由服务端实现，只能修改名称、说明、触发关键词与启用状态；编码不可改、不可删除。
    </ElAlert>
    <ElForm ref="formRef" :model="form" :rules="rules" label-position="top">
      <div class="tool-edit__row">
        <ElFormItem label="工具编码" prop="tool_code" class="tool-edit__half">
          <ElInput v-model="form.tool_code" :disabled="builtinFlag" maxlength="32" placeholder="如 chapter_check" />
        </ElFormItem>
        <ElFormItem label="排序号" class="tool-edit__half">
          <ElInputNumber v-model="form.sort_order" :min="0" :max="9999" controls-position="right" />
        </ElFormItem>
      </div>
      <ElFormItem label="工具名称" prop="tool_name">
        <ElInput v-model="form.tool_name" maxlength="64" placeholder="如 章节体检" />
      </ElFormItem>
      <ElFormItem label="一句话说明">
        <ElInput
          v-model="form.description_text"
          type="textarea"
          :rows="3"
          maxlength="256"
          show-word-limit
          placeholder="模型与用户都会看到这段说明，写清它解决什么问题"
        />
      </ElFormItem>
      <ElFormItem label="触发关键词">
        <ElInput
          v-model="form.trigger_keywords"
          maxlength="256"
          placeholder="逗号分隔，正文命中即自动运行，如 体检,能不能发,过稿"
        />
      </ElFormItem>
      <ElFormItem label="启用">
        <ElSwitch v-model="form.enabled_flag" />
      </ElFormItem>

      <div class="tool-edit__params-head">
        <span class="tool-edit__params-title">参数声明</span>
        <ElButton size="small" text type="primary" @click="addParam">
          <Icon icon="mdi:plus" :size="14" />
          <span class="tool-edit__btn-text">添加参数</span>
        </ElButton>
      </div>
      <ElEmpty v-if="form.param_list.length === 0" description="没有参数（用正文与项目上下文）" :image-size="60" />
      <div v-for="(param, index) in form.param_list" :key="index" class="tool-edit__param">
        <div class="tool-edit__row">
          <ElInput v-model="param.param_name" placeholder="参数名" class="tool-edit__half" />
          <ElInput v-model="param.param_label" placeholder="展示名" class="tool-edit__half" />
        </div>
        <div class="tool-edit__row">
          <ElInput v-model="param.default_value" placeholder="缺省值" class="tool-edit__half" />
          <ElCheckbox v-model="param.required_flag" class="tool-edit__half">必填</ElCheckbox>
          <ElButton size="small" text type="danger" @click="removeParam(index)">移除</ElButton>
        </div>
        <ElInput v-model="param.description_text" placeholder="填写提示" />
      </div>
    </ElForm>
    <template #footer>
      <ElButton @click="visible = false">取消</ElButton>
      <ElButton type="primary" :loading="saving" @click="submit">保存</ElButton>
    </template>
  </ElDrawer>
</template>

<style lang="less" scoped>
  .tool-edit {
    &__alert {
      margin-bottom: 14px;
    }

    &__row {
      display: flex;
      gap: 12px;
      align-items: center;
    }

    &__half {
      flex: 1;
      min-width: 0;
    }

    &__params-head {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 10px 0 6px;
      border-top: 1px dashed var(--nw-border);
    }

    &__params-title {
      font-size: 13px;
      font-weight: 700;
      color: var(--nw-ink);
    }

    &__param {
      display: flex;
      flex-direction: column;
      gap: 8px;
      padding: 10px;
      margin-bottom: 8px;
      background: var(--nw-surface-subtle);
      border: 1px solid var(--nw-border-soft);
      border-radius: var(--nw-radius-md);
    }

    &__btn-text {
      margin-left: 4px;
    }
  }
</style>

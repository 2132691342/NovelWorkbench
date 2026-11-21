<!-- 模型编辑抽屉：维护模型名称与参数。 -->
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
    type FormInstance,
    type FormRules
  } from 'element-plus'
  import {
    saveModelCatalog,
    type NWModelCatalogVo,
    type NWModelProviderVo
  } from '@/api/model'

  const props = defineProps<{ providers: NWModelProviderVo[] }>()
  const emit = defineEmits<{ saved: [] }>()

  const visible = ref(false)
  const saving = ref(false)
  const formRef = ref<FormInstance>()
  const editing = ref<NWModelCatalogVo | null>(null)

  const form = reactive({
    provider_ulid: '',
    model_name: '',
    model_label: '',
    temperature: 0.7,
    max_tokens: 0,
    context_window_count: 0,
    enabled_flag: true,
    default_flag: false,
    sort_order: 0,
    remark_text: ''
  })

  const title = computed(() => (editing.value ? '编辑模型' : '新建模型'))

  const rules: FormRules = {
    provider_ulid: [{ required: true, message: '请选择供应商', trigger: 'change' }],
    model_name: [{ required: true, message: '请填写模型名', trigger: 'blur' }]
  }

  /** 打开抽屉：传模型为编辑，providerUlid 为新建时的默认供应商 */
  const open = (model?: NWModelCatalogVo, providerUlid = '') => {
    editing.value = model ?? null
    form.provider_ulid = model?.provider_ulid ?? providerUlid ?? ''
    form.model_name = model?.model_name ?? ''
    form.model_label = model?.model_label ?? ''
    form.temperature = model?.temperature ?? 0.7
    form.max_tokens = model?.max_tokens ?? 0
    form.context_window_count = model?.context_window_count ?? 0
    form.enabled_flag = model?.enabled_flag ?? true
    form.default_flag = model?.default_flag ?? false
    form.sort_order = model?.sort_order ?? 0
    form.remark_text = model?.remark_text ?? ''
    visible.value = true
  }

  const submit = async () => {
    const valid = await formRef.value?.validate().catch(() => false)
    if (!valid) return

    saving.value = true
    try {
      await saveModelCatalog({
        model_ulid: editing.value?.model_ulid,
        provider_ulid: form.provider_ulid,
        model_name: form.model_name,
        model_label: form.model_label || undefined,
        temperature: form.temperature,
        max_tokens: form.max_tokens,
        context_window_count: form.context_window_count,
        enabled_flag: form.enabled_flag,
        default_flag: form.default_flag,
        sort_order: form.sort_order,
        remark_text: form.remark_text,
        role_ulid_list: editing.value?.role_ulid_list
      })
      ElMessage.success('模型已保存')
      visible.value = false
      emit('saved')
    } finally {
      saving.value = false
    }
  }

  defineExpose({ open })
</script>

<template>
  <ElDrawer v-model="visible" :title="title" size="520px" append-to-body destroy-on-close>
    <ElForm ref="formRef" :model="form" :rules="rules" label-position="top">
      <ElFormItem label="所属供应商" prop="provider_ulid">
        <ElSelect v-model="form.provider_ulid" class="model-form__select" placeholder="选择供应商">
          <ElOption
            v-for="item in props.providers"
            :key="item.provider_ulid"
            :label="`${item.provider_name}（${item.protocol_code}）`"
            :value="item.provider_ulid"
          />
        </ElSelect>
      </ElFormItem>
      <div class="model-form__row">
        <ElFormItem label="模型名" prop="model_name">
          <ElInput v-model="form.model_name" placeholder="gpt-4o-mini" />
        </ElFormItem>
        <ElFormItem label="展示名">
          <ElInput v-model="form.model_label" placeholder="留空则用模型名" />
        </ElFormItem>
      </div>
      <div class="model-form__row">
        <ElFormItem label="采样温度">
          <ElInputNumber
            v-model="form.temperature"
            :min="0"
            :max="2"
            :step="0.1"
            controls-position="right"
          />
        </ElFormItem>
        <ElFormItem label="输出上限（token）">
          <ElInputNumber
            v-model="form.max_tokens"
            :min="0"
            :max="128000"
            controls-position="right"
          />
        </ElFormItem>
      </div>
      <div class="model-form__row">
        <ElFormItem label="上下文窗口（token）">
          <ElInputNumber
            v-model="form.context_window_count"
            :min="0"
            :max="10000000"
            controls-position="right"
          />
        </ElFormItem>
        <ElFormItem label="排序号">
          <ElInputNumber v-model="form.sort_order" :min="0" :max="9999" controls-position="right" />
        </ElFormItem>
      </div>
      <div class="model-form__row">
        <ElFormItem label="启用">
          <ElSwitch v-model="form.enabled_flag" />
        </ElFormItem>
        <ElFormItem label="系统默认">
          <ElSwitch v-model="form.default_flag" />
        </ElFormItem>
      </div>
      <ElFormItem label="备注">
        <ElInput v-model="form.remark_text" type="textarea" :rows="2" maxlength="256" />
      </ElFormItem>
    </ElForm>
    <template #footer>
      <ElButton @click="visible = false">取消</ElButton>
      <ElButton type="primary" :loading="saving" @click="submit">保存</ElButton>
    </template>
  </ElDrawer>
</template>

<style lang="less" scoped>
  .model-form {
    &__select {
      width: 100%;
    }

    &__row {
      display: grid;
      gap: 12px;
      grid-template-columns: 1fr 1fr;
    }
  }
</style>

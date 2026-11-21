<!-- 供应商编辑抽屉：维护接入地址、协议与密钥。 -->
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
  import { saveModelProvider, type NWModelProviderVo, type NWProtocolCode } from '@/api/model'

  const emit = defineEmits<{ saved: [] }>()

  /** 抽屉可见性：父组件通过 ref.open() 控制 */
  const visible = ref(false)
  const saving = ref(false)
  const formRef = ref<FormInstance>()
  const editing = ref<NWModelProviderVo | null>(null)

  const form = reactive({
    provider_name: '',
    protocol_code: 'openai' as NWProtocolCode,
    base_url: '',
    api_key_text: '',
    enabled_flag: true,
    sort_order: 0,
    remark_text: ''
  })

  const title = computed(() => (editing.value ? '编辑供应商' : '新建供应商'))

  const protocolOptions: { value: NWProtocolCode; label: string; hint: string }[] = [
    { value: 'openai', label: 'OpenAI 兼容', hint: '网关地址需包含 /v1' },
    { value: 'anthropic', label: 'Anthropic 原生', hint: '网关地址通常不含 /v1' }
  ]

  /** 网关地址占位文案（模板内直接写 URL 字面量会被模板编译器误判为表达式） */
  const baseUrlPlaceholder = 'https://api.example.com/v1'

  const protocolHint = computed(
    () => protocolOptions.find((item) => item.value === form.protocol_code)?.hint ?? ''
  )

  /** 密钥输入提示：编辑已有密钥时说明留空即保持原值 */
  const apiKeyPlaceholder = computed(() => {
    const current = editing.value
    if (current?.api_key_configured) {
      return `留空保持原密钥（${current.api_key_masked}）`
    }
    return '粘贴供应商密钥'
  })

  const rules: FormRules = {
    provider_name: [{ required: true, message: '请填写供应商名称', trigger: 'blur' }],
    base_url: [
      { required: true, message: '请填写网关地址', trigger: 'blur' },
      { type: 'url', message: '请填写合法的 URL', trigger: 'blur' }
    ]
  }

  /** 打开抽屉：传供应商为编辑，不传为新建 */
  const open = (provider?: NWModelProviderVo) => {
    editing.value = provider ?? null
    form.provider_name = provider?.provider_name ?? ''
    form.protocol_code = provider?.protocol_code ?? 'openai'
    form.base_url = provider?.base_url ?? ''
    form.api_key_text = ''
    form.enabled_flag = provider?.enabled_flag ?? true
    form.sort_order = provider?.sort_order ?? 0
    form.remark_text = provider?.remark_text ?? ''
    visible.value = true
  }

  const submit = async () => {
    const valid = await formRef.value?.validate().catch(() => false)
    if (!valid) return
    if (!editing.value && !form.api_key_text.trim()) {
      ElMessage.warning('首次创建供应商必须填写 API Key')
      return
    }

    saving.value = true
    try {
      await saveModelProvider({
        provider_ulid: editing.value?.provider_ulid,
        provider_name: form.provider_name,
        protocol_code: form.protocol_code,
        base_url: form.base_url,
        api_key_text: form.api_key_text.trim() || undefined,
        enabled_flag: form.enabled_flag,
        sort_order: form.sort_order,
        remark_text: form.remark_text
      })
      ElMessage.success('供应商已保存')
      visible.value = false
      emit('saved')
    } finally {
      saving.value = false
    }
  }

  defineExpose({ open })
</script>

<template>
  <ElDrawer v-model="visible" :title="title" size="480px" append-to-body destroy-on-close>
    <ElForm ref="formRef" :model="form" :rules="rules" label-position="top">
      <ElFormItem label="供应商名称" prop="provider_name">
        <ElInput v-model="form.provider_name" maxlength="64" placeholder="例如：官方直连 / 中转网关" />
      </ElFormItem>
      <ElFormItem label="协议">
        <ElSelect v-model="form.protocol_code" class="provider-form__select">
          <ElOption
            v-for="item in protocolOptions"
            :key="item.value"
            :label="item.label"
            :value="item.value"
          />
        </ElSelect>
        <span class="nw-hint">{{ protocolHint }}</span>
      </ElFormItem>
      <ElFormItem label="网关地址" prop="base_url">
        <ElInput v-model="form.base_url" :placeholder="baseUrlPlaceholder" />
      </ElFormItem>
      <ElFormItem label="API Key">
        <ElInput
          v-model="form.api_key_text"
          type="password"
          show-password
          :placeholder="apiKeyPlaceholder"
        />
        <span class="nw-hint">密钥由后端加密存储，列表仅回显掩码。</span>
      </ElFormItem>
      <div class="provider-form__row">
        <ElFormItem label="排序号">
          <ElInputNumber v-model="form.sort_order" :min="0" :max="9999" controls-position="right" />
        </ElFormItem>
        <ElFormItem label="启用">
          <ElSwitch v-model="form.enabled_flag" />
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
  .provider-form {
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

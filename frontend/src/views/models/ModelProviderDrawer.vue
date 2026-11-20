<!--
  供应商编辑抽屉：全宽（76%，最大 880px）两列表单，网关地址与密钥分列，保存后回传 ULID 供列表定位。
-->
<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { useI18n } from 'vue-i18n'
import {
  saveModelProvider,
  type NWModelProviderSaveReqVo,
  type NWModelProviderVo,
  type NWProtocolCode,
} from '@/api/model'
import NWButton from '@/components/NWButton.vue'

const emit = defineEmits<{ saved: [providerUlid: string] }>()

const { t } = useI18n()

/** 协议选项 */
const NW_PROTOCOL_OPTION_LIST: Array<{ value: NWProtocolCode; label_key: string }> = [
  { value: 'openai', label_key: 'models.protocolOpenAI' },
  { value: 'anthropic', label_key: 'models.protocolAnthropic' },
]

const drawerVisible = ref(false)
const savingFlag = ref(false)
const editingUlid = ref('')
const formRef = ref<FormInstance>()

const formState = reactive({
  provider_name: '',
  protocol_code: 'openai' as NWProtocolCode,
  base_url: '',
  api_key_text: '',
  enabled_flag: true,
  sort_order: 0,
  remark_text: '',
})

const formRules = computed<FormRules>(() => ({
  provider_name: [
    { required: true, message: t('models.ruleNameRequired'), trigger: 'blur' },
    { max: 64, message: t('models.ruleNameMax'), trigger: 'blur' },
  ],
  base_url: [
    { required: true, message: t('models.ruleBaseURLRequired'), trigger: 'blur' },
    { pattern: /^https?:\/\/.+/, message: t('models.ruleBaseURLInvalid'), trigger: 'blur' },
  ],
}))

/** 打开新建 */
function openCreate(): void {
  editingUlid.value = ''
  formState.provider_name = ''
  formState.protocol_code = 'openai'
  formState.base_url = ''
  formState.api_key_text = ''
  formState.enabled_flag = true
  formState.sort_order = 0
  formState.remark_text = ''
  drawerVisible.value = true
  void formRef.value?.clearValidate()
}

/** 打开编辑（密钥不回显，留空表示不修改） */
function openEdit(providerVo: NWModelProviderVo): void {
  editingUlid.value = providerVo.provider_ulid
  formState.provider_name = providerVo.provider_name
  formState.protocol_code = providerVo.protocol_code
  formState.base_url = providerVo.base_url
  formState.api_key_text = ''
  formState.enabled_flag = providerVo.enabled_flag
  formState.sort_order = providerVo.sort_order
  formState.remark_text = providerVo.remark_text
  drawerVisible.value = true
  void formRef.value?.clearValidate()
}

/** 保存供应商 */
async function handleSubmit(): Promise<void> {
  if (!formRef.value) {
    return
  }
  try {
    await formRef.value.validate()
  } catch {
    return
  }
  if (!editingUlid.value && !formState.api_key_text.trim()) {
    ElMessage.warning(t('models.ruleAPIKeyRequired'))
    return
  }
  savingFlag.value = true
  try {
    const saveReq: NWModelProviderSaveReqVo = {
      provider_name: formState.provider_name.trim(),
      protocol_code: formState.protocol_code,
      base_url: formState.base_url.trim(),
      enabled_flag: formState.enabled_flag,
      sort_order: formState.sort_order,
      remark_text: formState.remark_text.trim(),
    }
    if (editingUlid.value) {
      saveReq.provider_ulid = editingUlid.value
    }
    if (formState.api_key_text.trim()) {
      saveReq.api_key_text = formState.api_key_text.trim()
    }
    const savedVo = await saveModelProvider(saveReq)
    ElMessage.success(t('models.saveSuccess'))
    drawerVisible.value = false
    emit('saved', savedVo.provider_ulid)
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    savingFlag.value = false
  }
}

defineExpose({ openCreate, openEdit })
</script>

<template>
  <el-drawer
    v-model="drawerVisible"
    :title="editingUlid ? t('models.editProviderTitle') : t('models.createProviderTitle')"
    size="76%"
    class="model-form-drawer"
  >
    <el-form
      ref="formRef"
      :model="formState"
      :rules="formRules"
      label-position="top"
      class="model-form"
    >
      <div class="model-form__grid">
        <el-form-item :label="t('models.providerName')" prop="provider_name">
          <el-input
            v-model="formState.provider_name"
            maxlength="64"
            show-word-limit
            :placeholder="t('models.providerNamePlaceholder')"
          />
        </el-form-item>
        <el-form-item :label="t('models.protocol')">
          <el-radio-group v-model="formState.protocol_code">
            <el-radio-button
              v-for="protocolOption in NW_PROTOCOL_OPTION_LIST"
              :key="protocolOption.value"
              :value="protocolOption.value"
            >
              {{ t(protocolOption.label_key) }}
            </el-radio-button>
          </el-radio-group>
          <p class="model-form__hint">{{ t('models.protocolHint') }}</p>
        </el-form-item>
        <el-form-item :label="t('models.baseURL')" prop="base_url" class="model-form__span">
          <el-input v-model="formState.base_url" :placeholder="t('models.baseURLPlaceholder')" />
          <p class="model-form__hint">{{ t('models.baseURLHint') }}</p>
        </el-form-item>
        <el-form-item :label="t('models.apiKey')" class="model-form__span">
          <el-input
            v-model="formState.api_key_text"
            type="password"
            show-password
            :placeholder="editingUlid ? t('models.apiKeyKeepPlaceholder') : t('models.apiKeyPlaceholder')"
          />
          <p class="model-form__hint">{{ t('models.apiKeyHint') }}</p>
        </el-form-item>
        <el-form-item :label="t('models.sortOrder')">
          <el-input-number v-model="formState.sort_order" :min="0" :max="9999" />
        </el-form-item>
        <el-form-item :label="t('models.enabled')">
          <el-switch v-model="formState.enabled_flag" />
          <span class="model-form__inline-hint">{{ t('models.enabledHint') }}</span>
        </el-form-item>
        <el-form-item :label="t('models.remark')" class="model-form__span">
          <el-input v-model="formState.remark_text" maxlength="256" :rows="3" type="textarea" />
        </el-form-item>
      </div>
    </el-form>
    <template #footer>
      <NWButton @click="drawerVisible = false">{{ t('common.cancel') }}</NWButton>
      <NWButton type="primary" :loading="savingFlag" @click="handleSubmit">{{ t('common.save') }}</NWButton>
    </template>
  </el-drawer>
</template>

<style scoped src="./modelForm.css"></style>

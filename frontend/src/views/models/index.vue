<!--
  模型供应商页：供应商卡片列表 + 新建编辑抽屉 + 连通性测试。系统仅用对话协议，不接入向量模型。
-->
<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { useI18n } from 'vue-i18n'
import {
  deleteModelProvider,
  getModelCapability,
  listModelProviders,
  saveModelProvider,
  testModelProvider,
  type NWModelCapabilityVo,
  type NWModelProviderVo,
  type NWProtocolCode,
  type NWModelProviderSaveReqVo,
} from '@/api/model'
import NWButton from '@/components/NWButton.vue'
import NWCard from '@/components/NWCard.vue'
import NWEmpty from '@/components/NWEmpty.vue'
import NWLoading from '@/components/NWLoading.vue'

const { t } = useI18n()

/** 协议选项 */
const NW_PROTOCOL_OPTION_LIST: Array<{ value: NWProtocolCode; label_key: string }> = [
  { value: 'openai', label_key: 'models.protocolOpenAI' },
  { value: 'anthropic', label_key: 'models.protocolAnthropic' },
]

const providerList = ref<NWModelProviderVo[]>([])
const capabilityVo = ref<NWModelCapabilityVo | null>(null)
const listLoading = ref(false)
const drawerVisible = ref(false)
const savingFlag = ref(false)
const testingUlid = ref('')
const editingUlid = ref('')
const formRef = ref<FormInstance>()

interface ProviderFormState {
  provider_name: string
  protocol_code: NWProtocolCode
  base_url: string
  api_key_text: string
  chat_model_name: string
  temperature: number
  max_tokens: number
  enabled_flag: boolean
  default_flag: boolean
  sort_order: number
  remark_text: string
}

const formState = reactive<ProviderFormState>({
  provider_name: '',
  protocol_code: 'openai',
  base_url: '',
  api_key_text: '',
  chat_model_name: '',
  temperature: 0.7,
  max_tokens: 2048,
  enabled_flag: true,
  default_flag: false,
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
    {
      pattern: /^https?:\/\/.+/,
      message: t('models.ruleBaseURLInvalid'),
      trigger: 'blur',
    },
  ],
  chat_model_name: [
    { required: true, message: t('models.ruleChatModelRequired'), trigger: 'blur' },
  ],
}))


/** 协议展示文案 */
function protocolText(protocolCode: string): string {
  return protocolCode === 'anthropic'
    ? t('models.protocolAnthropic')
    : t('models.protocolOpenAI')
}

/** 测试状态展示文案 */
function testStatusText(statusCode: string): string {
  switch (statusCode) {
    case 'success':
      return t('models.testSuccess')
    case 'failed':
      return t('models.testFailed')
    default:
      return t('models.testUntested')
  }
}

/** 加载供应商列表与能力探测 */
async function loadData(): Promise<void> {
  listLoading.value = true
  try {
    const [providers, capability] = await Promise.all([
      listModelProviders(),
      getModelCapability(),
    ])
    providerList.value = Array.isArray(providers) ? providers : []
    capabilityVo.value = capability
  } catch {
    providerList.value = []
  } finally {
    listLoading.value = false
  }
}

/** 打开新建抽屉 */
function handleCreate(): void {
  editingUlid.value = ''
  formState.provider_name = ''
  formState.protocol_code = 'openai'
  formState.base_url = ''
  formState.api_key_text = ''
  formState.chat_model_name = ''
  formState.temperature = 0.7
  formState.max_tokens = 2048
  formState.enabled_flag = true
  formState.default_flag = providerList.value.length === 0
  formState.sort_order = providerList.value.length
  formState.remark_text = ''
  drawerVisible.value = true
}

/** 打开编辑抽屉（密钥不回显，留空表示不修改） */
function handleEdit(providerVo: NWModelProviderVo): void {
  editingUlid.value = providerVo.provider_ulid
  formState.provider_name = providerVo.provider_name
  formState.protocol_code = providerVo.protocol_code
  formState.base_url = providerVo.base_url
  formState.api_key_text = ''
  formState.chat_model_name = providerVo.chat_model_name
  formState.temperature = providerVo.temperature || 0.7
  formState.max_tokens = providerVo.max_tokens || 2048
  formState.enabled_flag = providerVo.enabled_flag
  formState.default_flag = providerVo.default_flag
  formState.sort_order = providerVo.sort_order
  formState.remark_text = providerVo.remark_text
  drawerVisible.value = true
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
      provider_name: formState.provider_name,
      protocol_code: formState.protocol_code,
      base_url: formState.base_url.trim(),
      chat_model_name: formState.chat_model_name.trim(),
      enabled_flag: formState.enabled_flag,
      default_flag: formState.default_flag,
      sort_order: formState.sort_order,
      remark_text: formState.remark_text,
    }
    if (editingUlid.value) {
      saveReq.provider_ulid = editingUlid.value
    }
    if (formState.api_key_text.trim()) {
      saveReq.api_key_text = formState.api_key_text.trim()
    }
    saveReq.temperature = formState.temperature
    saveReq.max_tokens = formState.max_tokens
    await saveModelProvider(saveReq)
    ElMessage.success(t('models.saveSuccess'))
    drawerVisible.value = false
    await loadData()
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    savingFlag.value = false
  }
}

/** 连通性测试（结果回写后端，前端刷新列表） */
async function handleTest(providerVo: NWModelProviderVo): Promise<void> {
  testingUlid.value = providerVo.provider_ulid
  try {
    const result = await testModelProvider(providerVo.provider_ulid)
    if (result.status_code === 'success') {
      ElMessage.success(`${t('models.testSuccess')} · ${result.latency_ms}ms`)
    } else {
      ElMessage.error(result.message_text || t('models.testFailed'))
    }
    await loadData()
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    testingUlid.value = ''
  }
}

/** 设为默认（切换全局默认对话供应商） */
async function handleSetDefault(providerVo: NWModelProviderVo): Promise<void> {
  try {
    await saveModelProvider({
      provider_ulid: providerVo.provider_ulid,
      provider_name: providerVo.provider_name,
      protocol_code: providerVo.protocol_code,
      base_url: providerVo.base_url,
      chat_model_name: providerVo.chat_model_name,
      temperature: providerVo.temperature,
      max_tokens: providerVo.max_tokens,
      default_flag: true,
      sort_order: providerVo.sort_order,
      remark_text: providerVo.remark_text,
    })
    ElMessage.success(t('models.defaultSuccess'))
    await loadData()
  } catch {
    // 失败提示由 http 层统一弹出
  }
}

/** 删除供应商 */
async function handleDelete(providerVo: NWModelProviderVo): Promise<void> {
  try {
    await ElMessageBox.confirm(t('models.deleteConfirm'), t('common.delete'), {
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel'),
      type: 'warning',
    })
  } catch {
    return
  }
  try {
    await deleteModelProvider(providerVo.provider_ulid)
    ElMessage.success(t('models.deleteSuccess'))
    await loadData()
  } catch {
    // 失败提示由 http 层统一弹出
  }
}

onMounted(() => {
  void loadData()
})
</script>

<template>
  <div class="models-page">
    <header class="models-page__header">
      <div>
        <h2 class="models-page__title">{{ t('models.title') }}</h2>
        <p class="models-page__subtitle">{{ t('models.subtitle') }}</p>
      </div>
      <NWButton type="primary" @click="handleCreate">{{ t('models.create') }}</NWButton>
    </header>

    <!-- 能力提示条 -->
    <div
      v-if="capabilityVo"
      class="models-page__capability"
      :class="{
        'is-ready': capabilityVo.chat_ready,
        'is-down': !capabilityVo.chat_ready,
      }"
    >
      <span class="models-page__capability-dot" />
      <span>{{ capabilityVo.hint_text }}</span>
      <span v-if="capabilityVo.default_provider_name" class="models-page__capability-meta">
        {{ t('models.currentDefault') }}：{{ capabilityVo.default_provider_name }} /
        {{ capabilityVo.default_chat_model_name }}
      </span>
    </div>

    <NWLoading v-if="listLoading" />
    <NWEmpty v-else-if="providerList.length === 0" :description_text="t('models.emptyTip')" />
    <div v-else class="models-grid">
      <NWCard v-for="providerVo in providerList" :key="providerVo.provider_ulid" :hoverable="true">
        <article class="provider-card">
          <header class="provider-card__header">
            <div class="provider-card__title-wrap">
              <h3 class="provider-card__name">{{ providerVo.provider_name }}</h3>
              <span class="provider-card__protocol">{{ protocolText(providerVo.protocol_code) }}</span>
              <span v-if="providerVo.default_flag" class="provider-card__badge">
                {{ t('models.defaultTag') }}
              </span>
              <span v-if="!providerVo.enabled_flag" class="provider-card__badge provider-card__badge--muted">
                {{ t('models.disabledTag') }}
              </span>
            </div>
            <span
              class="provider-card__status"
              :class="`is-${providerVo.last_test_status_code || 'untested'}`"
            >
              {{ testStatusText(providerVo.last_test_status_code) }}
            </span>
          </header>

          <dl class="provider-card__meta">
            <div>
              <dt>{{ t('models.baseURL') }}</dt>
              <dd :title="providerVo.base_url">{{ providerVo.base_url }}</dd>
            </div>
            <div>
              <dt>{{ t('models.chatModel') }}</dt>
              <dd>{{ providerVo.chat_model_name }}</dd>
            </div>
            <div>
              <dt>{{ t('models.apiKey') }}</dt>
              <dd>{{ providerVo.api_key_configured ? providerVo.api_key_masked : t('models.apiKeyMissing') }}</dd>
            </div>
          </dl>

          <p v-if="providerVo.last_test_message" class="provider-card__message">
            {{ providerVo.last_test_message }}
          </p>

          <footer class="provider-card__footer">
            <NWButton
              size="small"
              :loading="testingUlid === providerVo.provider_ulid"
              @click="handleTest(providerVo)"
            >
              {{ t('models.test') }}
            </NWButton>
            <NWButton
              v-if="!providerVo.default_flag"
              size="small"
              @click="handleSetDefault(providerVo)"
            >
              {{ t('models.setDefault') }}
            </NWButton>
            <NWButton size="small" @click="handleEdit(providerVo)">{{ t('common.edit') }}</NWButton>
            <NWButton
              v-if="!providerVo.default_flag"
              type="danger-text"
              size="small"
              @click="handleDelete(providerVo)"
            >
              {{ t('common.delete') }}
            </NWButton>
          </footer>
        </article>
      </NWCard>
    </div>

    <!-- 供应商编辑抽屉 -->
    <el-drawer
      v-model="drawerVisible"
      :title="editingUlid ? t('models.editTitle') : t('models.createTitle')"
      size="560px"
    >
      <el-form ref="formRef" :model="formState" :rules="formRules" label-position="top">
        <el-form-item :label="t('models.providerName')" prop="provider_name">
          <el-input v-model="formState.provider_name" maxlength="64" :placeholder="t('models.providerNamePlaceholder')" />
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
          <p class="models-form__hint">{{ t('models.protocolHint') }}</p>
        </el-form-item>
        <el-form-item :label="t('models.baseURL')" prop="base_url">
          <el-input v-model="formState.base_url" :placeholder="t('models.baseURLPlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('models.apiKey')">
          <el-input
            v-model="formState.api_key_text"
            type="password"
            show-password
            :placeholder="editingUlid ? t('models.apiKeyKeepPlaceholder') : t('models.apiKeyPlaceholder')"
          />
        </el-form-item>
        <el-form-item :label="t('models.chatModel')" prop="chat_model_name">
          <el-input v-model="formState.chat_model_name" :placeholder="t('models.chatModelPlaceholder')" />
        </el-form-item>
        <div class="models-form__row">
          <el-form-item :label="t('models.temperature')">
            <el-input-number v-model="formState.temperature" :min="0" :max="2" :step="0.1" />
          </el-form-item>
          <el-form-item :label="t('models.maxTokens')">
            <el-input-number v-model="formState.max_tokens" :min="0" :max="128000" :step="256" />
          </el-form-item>
          <el-form-item :label="t('models.sortOrder')">
            <el-input-number v-model="formState.sort_order" :min="0" :max="9999" />
          </el-form-item>
        </div>
        <el-form-item :label="t('models.remark')">
          <el-input v-model="formState.remark_text" maxlength="256" />
        </el-form-item>
        <el-form-item>
          <el-checkbox v-model="formState.enabled_flag">{{ t('models.enabled') }}</el-checkbox>
          <el-checkbox v-model="formState.default_flag">{{ t('models.defaultTag') }}</el-checkbox>
        </el-form-item>
      </el-form>
      <template #footer>
        <NWButton @click="drawerVisible = false">{{ t('common.cancel') }}</NWButton>
        <NWButton type="primary" :loading="savingFlag" @click="handleSubmit">
          {{ t('common.save') }}
        </NWButton>
      </template>
    </el-drawer>
  </div>
</template>

<style scoped src="./models.css"></style>

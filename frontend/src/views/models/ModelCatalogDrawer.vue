<!--
  模型编辑抽屉：模型名/展示名/参数/上下文窗口 + 角色授权勾选，保存即整体覆盖授权。
-->
<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { useI18n } from 'vue-i18n'
import type { NWRoleVo } from '@/api/admin'
import {
  saveModelCatalog,
  type NWModelCatalogSaveReqVo,
  type NWModelCatalogVo,
  type NWModelProviderVo,
} from '@/api/model'
import NWButton from '@/components/NWButton.vue'

const props = defineProps<{ roleList: NWRoleVo[] }>()
const emit = defineEmits<{ saved: [] }>()

const { t } = useI18n()

const drawerVisible = ref(false)
const savingFlag = ref(false)
const editingUlid = ref('')
const providerName = ref('')
const formRef = ref<FormInstance>()

const formState = reactive({
  provider_ulid: '',
  model_name: '',
  model_label: '',
  temperature: 0.7,
  max_tokens: 2048,
  context_window_count: 0,
  enabled_flag: true,
  default_flag: false,
  sort_order: 0,
  remark_text: '',
  role_ulid_list: [] as string[],
})

const formRules = computed<FormRules>(() => ({
  model_name: [
    { required: true, message: t('models.ruleModelNameRequired'), trigger: 'blur' },
    { max: 128, message: t('models.ruleModelNameMax'), trigger: 'blur' },
  ],
}))

/** 打开新建（默认归属当前供应商） */
function openCreate(providerVo: NWModelProviderVo): void {
  editingUlid.value = ''
  providerName.value = providerVo.provider_name
  formState.provider_ulid = providerVo.provider_ulid
  formState.model_name = ''
  formState.model_label = ''
  formState.temperature = 0.7
  formState.max_tokens = 2048
  formState.context_window_count = 0
  formState.enabled_flag = true
  formState.default_flag = false
  formState.sort_order = 0
  formState.remark_text = ''
  formState.role_ulid_list = []
  drawerVisible.value = true
  void formRef.value?.clearValidate()
}

/** 打开编辑 */
function openEdit(modelVo: NWModelCatalogVo): void {
  editingUlid.value = modelVo.model_ulid
  providerName.value = modelVo.provider_name
  formState.provider_ulid = modelVo.provider_ulid
  formState.model_name = modelVo.model_name
  formState.model_label = modelVo.model_label
  formState.temperature = modelVo.temperature || 0.7
  formState.max_tokens = modelVo.max_tokens || 2048
  formState.context_window_count = modelVo.context_window_count
  formState.enabled_flag = modelVo.enabled_flag
  formState.default_flag = modelVo.default_flag
  formState.sort_order = modelVo.sort_order
  formState.remark_text = modelVo.remark_text
  formState.role_ulid_list = [...modelVo.role_ulid_list]
  drawerVisible.value = true
  void formRef.value?.clearValidate()
}

/** 保存模型（含角色授权整体覆盖） */
async function handleSubmit(): Promise<void> {
  if (!formRef.value) {
    return
  }
  try {
    await formRef.value.validate()
  } catch {
    return
  }
  savingFlag.value = true
  try {
    const saveReq: NWModelCatalogSaveReqVo = {
      provider_ulid: formState.provider_ulid,
      model_name: formState.model_name.trim(),
      model_label: formState.model_label.trim(),
      temperature: formState.temperature,
      max_tokens: formState.max_tokens,
      context_window_count: formState.context_window_count,
      enabled_flag: formState.enabled_flag,
      default_flag: formState.default_flag,
      sort_order: formState.sort_order,
      remark_text: formState.remark_text.trim(),
      role_ulid_list: formState.role_ulid_list,
    }
    if (editingUlid.value) {
      saveReq.model_ulid = editingUlid.value
    }
    await saveModelCatalog(saveReq)
    ElMessage.success(t('models.saveSuccess'))
    drawerVisible.value = false
    emit('saved')
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
    :title="editingUlid ? t('models.editModelTitle') : t('models.createModelTitle')"
    size="76%"
    class="model-form-drawer"
  >
    <p class="model-form__provider">
      {{ t('models.belongsProvider') }}：<strong>{{ providerName }}</strong>
    </p>
    <el-form ref="formRef" :model="formState" :rules="formRules" label-position="top" class="model-form">
      <div class="model-form__grid">
        <el-form-item :label="t('models.modelName')" prop="model_name">
          <el-input v-model="formState.model_name" maxlength="128" :placeholder="t('models.chatModelPlaceholder')" />
          <p class="model-form__hint">{{ t('models.modelNameHint') }}</p>
        </el-form-item>
        <el-form-item :label="t('models.modelLabel')">
          <el-input v-model="formState.model_label" maxlength="64" :placeholder="t('models.modelLabelPlaceholder')" />
          <p class="model-form__hint">{{ t('models.modelLabelHint') }}</p>
        </el-form-item>
        <el-form-item :label="t('models.temperature')">
          <el-slider v-model="formState.temperature" :min="0" :max="2" :step="0.1" show-input />
        </el-form-item>
        <el-form-item :label="t('models.maxTokens')">
          <el-input-number v-model="formState.max_tokens" :min="0" :max="128000" :step="256" />
        </el-form-item>
        <el-form-item :label="t('models.contextWindow')">
          <el-input-number v-model="formState.context_window_count" :min="0" :max="10000000" :step="1024" />
          <p class="model-form__hint">{{ t('models.contextWindowHint') }}</p>
        </el-form-item>
        <el-form-item :label="t('models.sortOrder')">
          <el-input-number v-model="formState.sort_order" :min="0" :max="9999" />
        </el-form-item>
        <el-form-item :label="t('models.enabled')">
          <el-switch v-model="formState.enabled_flag" />
        </el-form-item>
        <el-form-item :label="t('models.defaultTag')">
          <el-switch v-model="formState.default_flag" />
          <span class="model-form__inline-hint">{{ t('models.defaultHint') }}</span>
        </el-form-item>
        <el-form-item :label="t('models.roleGrant')" class="model-form__span">
          <el-select
            v-model="formState.role_ulid_list"
            multiple
            filterable
            class="model-form__roles"
            :placeholder="t('models.grantPlaceholder')"
          >
            <el-option
              v-for="roleVo in props.roleList"
              :key="roleVo.role_ulid"
              :value="roleVo.role_ulid"
              :label="`${roleVo.role_name}（${roleVo.role_code}）`"
            />
          </el-select>
          <p class="model-form__hint">{{ t('models.grantHint') }}</p>
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

<!--
  模型供应商页：左侧供应商列表（含模型数/连通状态），右侧模型目录表格。
  供应商只承载「网关 + 凭据」，模型是使用与授权的最小单位：每个模型可设默认、测试连通、按角色分配。
-->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { listAdminRoles, type NWRoleVo } from '@/api/admin'
import {
  deleteModelCatalog,
  deleteModelProvider,
  getModelCapability,
  grantModelCatalog,
  listModelCatalogs,
  listModelProviders,
  saveModelCatalog,
  setDefaultModelCatalog,
  testModelCatalog,
  testModelProvider,
  type NWModelCapabilityVo,
  type NWModelCatalogSaveReqVo,
  type NWModelCatalogVo,
  type NWModelProviderVo,
} from '@/api/model'
import NWButton from '@/components/NWButton.vue'
import NWEmpty from '@/components/NWEmpty.vue'
import NWIcon from '@/components/NWIcon.vue'
import NWLoading from '@/components/NWLoading.vue'
import ModelProviderDrawer from './ModelProviderDrawer.vue'
import ModelCatalogDrawer from './ModelCatalogDrawer.vue'
import ModelGrantDialog from './ModelGrantDialog.vue'

const { t } = useI18n()

const providerList = ref<NWModelProviderVo[]>([])
const modelList = ref<NWModelCatalogVo[]>([])
const roleList = ref<NWRoleVo[]>([])
const capabilityVo = ref<NWModelCapabilityVo | null>(null)
const activeProviderUlid = ref('')
const listLoading = ref(false)
const modelLoading = ref(false)
const testingKey = ref('')
const providerDrawerRef = ref<InstanceType<typeof ModelProviderDrawer> | null>(null)
const modelDrawerRef = ref<InstanceType<typeof ModelCatalogDrawer> | null>(null)
const grantDialogRef = ref<InstanceType<typeof ModelGrantDialog> | null>(null)

/** 当前选中的供应商（左侧高亮项） */
const activeProviderVo = computed(
  () => providerList.value.find((item) => item.provider_ulid === activeProviderUlid.value) ?? null,
)

/** 当前供应商下的模型（后端已按供应商过滤，这里再兜底一次） */
const activeModelList = computed(() =>
  modelList.value.filter((item) => item.provider_ulid === activeProviderUlid.value),
)

/** 协议展示文案 */
function protocolText(protocolCode: string): string {
  return protocolCode === 'anthropic' ? t('models.protocolAnthropic') : t('models.protocolOpenAI')
}

/** 测试状态展示文案 */
function testStatusText(statusCode: string): string {
  if (statusCode === 'success') {
    return t('models.testSuccess')
  }
  if (statusCode === 'failed') {
    return t('models.testFailed')
  }
  return t('models.testUntested')
}

/** 测试状态样式类 */
function testStatusClass(statusCode: string): string {
  return `is-${statusCode || 'untested'}`
}

/** 加载供应商、模型与角色（一次并行拉取） */
async function loadAll(): Promise<void> {
  listLoading.value = true
  try {
    const [providers, models, capability, roles] = await Promise.all([
      listModelProviders(),
      listModelCatalogs(),
      getModelCapability(),
      listAdminRoles({ page_num: 1, page_size: 100 }),
    ])
    providerList.value = Array.isArray(providers) ? providers : []
    modelList.value = Array.isArray(models) ? models : []
    capabilityVo.value = capability
    roleList.value = roles.items ?? []
    if (!providerList.value.some((item) => item.provider_ulid === activeProviderUlid.value)) {
      activeProviderUlid.value = providerList.value[0]?.provider_ulid ?? ''
    }
  } catch {
    providerList.value = []
    modelList.value = []
  } finally {
    listLoading.value = false
  }
}

/** 只刷新模型与能力（模型增删改后） */
async function refreshModels(): Promise<void> {
  modelLoading.value = true
  try {
    const [models, capability] = await Promise.all([listModelCatalogs(), getModelCapability()])
    modelList.value = Array.isArray(models) ? models : []
    capabilityVo.value = capability
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    modelLoading.value = false
  }
}

/** 切换左侧供应商 */
function handleSelectProvider(providerUlid: string): void {
  activeProviderUlid.value = providerUlid
}

/** 新建供应商 */
function handleCreateProvider(): void {
  providerDrawerRef.value?.openCreate()
}

/** 编辑供应商 */
function handleEditProvider(providerVo: NWModelProviderVo): void {
  providerDrawerRef.value?.openEdit(providerVo)
}

/** 供应商保存成功 */
async function handleProviderSaved(savedUlid: string): Promise<void> {
  await loadAll()
  if (savedUlid) {
    activeProviderUlid.value = savedUlid
  }
}

/** 删除供应商 */
async function handleDeleteProvider(providerVo: NWModelProviderVo): Promise<void> {
  try {
    await ElMessageBox.confirm(
      t('models.deleteProviderConfirm', { name: providerVo.provider_name }),
      t('common.delete'),
      { confirmButtonText: t('common.confirm'), cancelButtonText: t('common.cancel'), type: 'warning' },
    )
  } catch {
    return
  }
  try {
    await deleteModelProvider(providerVo.provider_ulid)
    ElMessage.success(t('models.deleteSuccess'))
    await loadAll()
  } catch {
    // 失败提示由 http 层统一弹出
  }
}

/** 供应商连通性测试（用其下首个启用模型探测） */
async function handleTestProvider(providerVo: NWModelProviderVo): Promise<void> {
  testingKey.value = `provider:${providerVo.provider_ulid}`
  try {
    const result = await testModelProvider(providerVo.provider_ulid)
    if (result.status_code === 'success') {
      ElMessage.success(`${t('models.testSuccess')} · ${result.latency_ms}ms`)
    } else {
      ElMessage.error(result.message_text || t('models.testFailed'))
    }
    await loadAll()
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    testingKey.value = ''
  }
}

/** 新建模型（默认落在当前供应商） */
function handleCreateModel(): void {
  if (!activeProviderVo.value) {
    ElMessage.warning(t('models.pickProviderFirst'))
    return
  }
  modelDrawerRef.value?.openCreate(activeProviderVo.value)
}

/** 编辑模型 */
function handleEditModel(modelVo: NWModelCatalogVo): void {
  modelDrawerRef.value?.openEdit(modelVo)
}

/** 模型保存成功 */
async function handleModelSaved(): Promise<void> {
  await refreshModels()
}

/** 模型连通性测试 */
async function handleTestModel(modelVo: NWModelCatalogVo): Promise<void> {
  testingKey.value = `model:${modelVo.model_ulid}`
  try {
    const result = await testModelCatalog(modelVo.model_ulid)
    if (result.status_code === 'success') {
      ElMessage.success(`${t('models.testSuccess')} · ${result.latency_ms}ms`)
    } else {
      ElMessage.error(result.message_text || t('models.testFailed'))
    }
    await refreshModels()
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    testingKey.value = ''
  }
}

/** 设为系统默认模型 */
async function handleSetDefaultModel(modelVo: NWModelCatalogVo): Promise<void> {
  try {
    await setDefaultModelCatalog(modelVo.model_ulid)
    ElMessage.success(t('models.defaultSuccess'))
    await refreshModels()
  } catch {
    // 失败提示由 http 层统一弹出
  }
}

/** 切换模型启用状态 */
async function handleToggleModel(modelVo: NWModelCatalogVo): Promise<void> {
  const saveReq: NWModelCatalogSaveReqVo = {
    model_ulid: modelVo.model_ulid,
    provider_ulid: modelVo.provider_ulid,
    model_name: modelVo.model_name,
    model_label: modelVo.model_label,
    temperature: modelVo.temperature,
    max_tokens: modelVo.max_tokens,
    context_window_count: modelVo.context_window_count,
    enabled_flag: !modelVo.enabled_flag,
    sort_order: modelVo.sort_order,
    remark_text: modelVo.remark_text,
  }
  try {
    await saveModelCatalog(saveReq)
    ElMessage.success(modelVo.enabled_flag ? t('models.disableSuccess') : t('models.enableSuccess'))
    await refreshModels()
  } catch {
    // 失败提示由 http 层统一弹出
  }
}

/** 删除模型 */
async function handleDeleteModel(modelVo: NWModelCatalogVo): Promise<void> {
  try {
    await ElMessageBox.confirm(
      t('models.deleteModelConfirm', { name: modelVo.model_label || modelVo.model_name }),
      t('common.delete'),
      { confirmButtonText: t('common.confirm'), cancelButtonText: t('common.cancel'), type: 'warning' },
    )
  } catch {
    return
  }
  try {
    await deleteModelCatalog(modelVo.model_ulid)
    ElMessage.success(t('models.deleteSuccess'))
    await refreshModels()
  } catch {
    // 失败提示由 http 层统一弹出
  }
}

/** 打开角色分配对话框 */
function handleGrantModel(modelVo: NWModelCatalogVo): void {
  grantDialogRef.value?.open(modelVo, roleList.value)
}

/** 角色分配保存成功 */
async function handleGrantSaved(modelUlid: string, roleUlidList: string[]): Promise<void> {
  try {
    await grantModelCatalog(modelUlid, roleUlidList)
    ElMessage.success(t('models.grantSuccess'))
    await refreshModels()
  } catch {
    // 失败提示由 http 层统一弹出
  }
}

onMounted(() => {
  void loadAll()
})
</script>

<template>
  <div class="nw-page models-page">
    <header class="models-page__header">
      <div>
        <h2 class="models-page__title">{{ t('models.title') }}</h2>
        <p class="models-page__subtitle">{{ t('models.subtitle') }}</p>
      </div>
      <div class="models-page__header-actions">
        <NWButton @click="handleCreateProvider">
          <NWIcon icon_code="plus" :size_px="14" />
          {{ t('models.create') }}
        </NWButton>
        <NWButton type="primary" :disabled="!activeProviderVo" @click="handleCreateModel">
          <NWIcon icon_code="plus" :size_px="14" />
          {{ t('models.createModel') }}
        </NWButton>
      </div>
    </header>

    <!-- 能力提示条 -->
    <div
      v-if="capabilityVo"
      class="models-page__capability"
      :class="capabilityVo.chat_ready ? 'is-ready' : 'is-down'"
    >
      <span class="models-page__capability-dot" />
      <span>{{ capabilityVo.hint_text }}</span>
      <span v-if="capabilityVo.default_model_label" class="models-page__capability-meta">
        {{ t('models.currentDefault') }}：{{ capabilityVo.default_provider_name }} /
        {{ capabilityVo.default_model_label }}
      </span>
    </div>

    <NWLoading v-if="listLoading" />
    <div v-else class="models-layout">
      <!-- 左：供应商列表 -->
      <aside class="models-side">
        <div class="models-side__head">
          <span class="models-side__title">{{ t('models.providerCount', { count: providerList.length }) }}</span>
          <button type="button" class="models-side__add" :title="t('models.create')" @click="handleCreateProvider">
            <NWIcon icon_code="plus" :size_px="14" />
          </button>
        </div>
        <NWEmpty v-if="providerList.length === 0" :description_text="t('models.emptyTip')" />
        <ul v-else class="models-side__list">
          <li
            v-for="providerVo in providerList"
            :key="providerVo.provider_ulid"
            class="provider-row"
            :class="{ 'is-active': providerVo.provider_ulid === activeProviderUlid }"
            @click="handleSelectProvider(providerVo.provider_ulid)"
          >
            <div class="provider-row__top">
              <span class="provider-row__name" :title="providerVo.provider_name">{{ providerVo.provider_name }}</span>
              <span class="provider-row__status" :class="testStatusClass(providerVo.last_test_status_code)">
                {{ testStatusText(providerVo.last_test_status_code) }}
              </span>
            </div>
            <div class="provider-row__meta">
              <span class="provider-row__protocol">{{ protocolText(providerVo.protocol_code) }}</span>
              <span class="provider-row__count">
                {{ t('models.modelCountLabel', { count: providerVo.model_count }) }}
              </span>
              <span v-if="!providerVo.enabled_flag" class="provider-row__off">{{ t('models.disabledTag') }}</span>
            </div>
            <div class="provider-row__url" :title="providerVo.base_url">{{ providerVo.base_url }}</div>
          </li>
        </ul>
      </aside>

      <!-- 右：模型表格 -->
      <section class="models-main">
        <template v-if="activeProviderVo">
          <header class="models-main__head">
            <div class="models-main__title-wrap">
              <h3 class="models-main__title">{{ activeProviderVo.provider_name }}</h3>
              <span class="models-main__badge">{{ protocolText(activeProviderVo.protocol_code) }}</span>
              <span v-if="!activeProviderVo.enabled_flag" class="models-main__badge is-muted">
                {{ t('models.disabledTag') }}
              </span>
            </div>
            <div class="models-main__actions">
              <NWButton
                size="small"
                :loading="testingKey === `provider:${activeProviderVo.provider_ulid}`"
                @click="handleTestProvider(activeProviderVo)"
              >
                {{ t('models.test') }}
              </NWButton>
              <NWButton size="small" @click="handleEditProvider(activeProviderVo)">{{ t('common.edit') }}</NWButton>
              <NWButton type="danger-text" size="small" @click="handleDeleteProvider(activeProviderVo)">
                {{ t('common.delete') }}
              </NWButton>
            </div>
          </header>

          <dl class="models-main__meta">
            <div>
              <dt>{{ t('models.baseURL') }}</dt>
              <dd :title="activeProviderVo.base_url">{{ activeProviderVo.base_url }}</dd>
            </div>
            <div>
              <dt>{{ t('models.apiKey') }}</dt>
              <dd>{{ activeProviderVo.api_key_configured ? activeProviderVo.api_key_masked : t('models.apiKeyMissing') }}</dd>
            </div>
            <div>
              <dt>{{ t('models.lastTest') }}</dt>
              <dd>{{ activeProviderVo.last_test_message || t('models.testUntested') }}</dd>
            </div>
          </dl>

          <div class="models-main__table-head">
            <h4 class="models-main__table-title">
              {{ t('models.modelCatalog') }}
              <span class="models-main__table-count">{{ activeModelList.length }}</span>
            </h4>
            <NWButton size="small" type="primary" @click="handleCreateModel">
              <NWIcon icon_code="plus" :size_px="13" />
              {{ t('models.createModel') }}
            </NWButton>
          </div>

          <NWLoading v-if="modelLoading" />
          <NWEmpty v-else-if="activeModelList.length === 0" :description_text="t('models.modelEmptyTip')" />
          <el-table v-else :data="activeModelList" class="models-table" row-key="model_ulid">
            <el-table-column :label="t('models.modelName')" min-width="220">
              <template #default="{ row }">
                <div class="models-table__model">
                  <span class="models-table__model-label">{{ row.model_label || row.model_name }}</span>
                  <span v-if="row.default_flag" class="models-table__tag is-default">{{ t('models.defaultTag') }}</span>
                  <span v-if="!row.enabled_flag" class="models-table__tag is-muted">{{ t('models.disabledTag') }}</span>
                </div>
                <div class="models-table__model-name">
                  {{ row.model_name }}
                  <span v-if="row.context_window_count > 0" class="models-table__context">
                    · {{ t('models.contextWindow') }} {{ row.context_window_count.toLocaleString() }}
                  </span>
                </div>
              </template>
            </el-table-column>
            <el-table-column :label="t('models.roleGrant')" min-width="150">
              <template #default="{ row }">
                <div v-if="row.role_name_list.length > 0" class="models-table__roles">
                  <span v-for="roleName in row.role_name_list" :key="roleName" class="models-table__role">
                    {{ roleName }}
                  </span>
                </div>
                <span v-else class="models-table__roles-empty">{{ t('models.grantNone') }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="t('models.testStatus')" width="96">
              <template #default="{ row }">
                <span class="models-table__status" :class="testStatusClass(row.last_test_status_code)">
                  {{ testStatusText(row.last_test_status_code) }}
                </span>
              </template>
            </el-table-column>
            <el-table-column :label="t('common.actions')" width="268">
              <template #default="{ row }">
                <div class="models-table__actions">
                  <NWButton
                    size="small"
                    :loading="testingKey === `model:${row.model_ulid}`"
                    @click="handleTestModel(row)"
                  >
                    {{ t('models.test') }}
                  </NWButton>
                  <NWButton size="small" @click="handleGrantModel(row)">{{ t('models.grant') }}</NWButton>
                  <NWButton
                    v-if="!row.default_flag"
                    size="small"
                    @click="handleSetDefaultModel(row)"
                  >
                    {{ t('models.setDefault') }}
                  </NWButton>
                  <el-dropdown trigger="click">
                    <NWButton size="small" type="text">{{ t('common.more') }}</NWButton>
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item @click="handleEditModel(row)">{{ t('common.edit') }}</el-dropdown-item>
                        <el-dropdown-item @click="handleToggleModel(row)">
                          {{ row.enabled_flag ? t('models.disable') : t('models.enable') }}
                        </el-dropdown-item>
                        <el-dropdown-item divided @click="handleDeleteModel(row)">
                          {{ t('common.delete') }}
                        </el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                </div>
              </template>
            </el-table-column>
          </el-table>
        </template>
        <NWEmpty v-else :description_text="t('models.pickProviderTip')" />
      </section>
    </div>

    <ModelProviderDrawer ref="providerDrawerRef" @saved="handleProviderSaved" />
    <ModelCatalogDrawer ref="modelDrawerRef" :role-list="roleList" @saved="handleModelSaved" />
    <ModelGrantDialog ref="grantDialogRef" @saved="handleGrantSaved" />
  </div>
</template>

<style scoped src="./models.css"></style>

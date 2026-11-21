<!-- 模型生态页：供应商、模型目录与用量三个页签。 -->
<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import { ElCard, ElMessage, ElMessageBox, ElTag } from 'element-plus'
  import {
    deleteModelCatalog,
    deleteModelProvider,
    getModelCapability,
    listModelCatalogs,
    listModelProviders,
    saveModelProvider,
    setDefaultModelCatalog,
    testModelCatalog,
    testModelProvider,
    type NWModelCapabilityVo,
    type NWModelCatalogVo,
    type NWModelProviderVo
  } from '@/api/model'
  import ProviderFormDrawer from './components/ProviderFormDrawer.vue'
  import ModelFormDrawer from './components/ModelFormDrawer.vue'
  import GrantRoleDialog from './components/GrantRoleDialog.vue'
  import UsagePanel from './components/UsagePanel.vue'
  import ProviderTable from './components/ProviderTable.vue'
  import CatalogTable from './components/CatalogTable.vue'
  import type { ModelTestResult } from './components/types'

  const capability = ref<NWModelCapabilityVo | null>(null)
  const providerList = ref<NWModelProviderVo[]>([])
  const modelList = ref<NWModelCatalogVo[]>([])
  const providerLoading = ref(false)
  const modelLoading = ref(false)
  const activeProviderUlid = ref('')
  const providerTestingUlid = ref('')
  const modelTestingUlid = ref('')
  const providerTestMap = ref<Record<string, ModelTestResult>>({})
  const modelTestMap = ref<Record<string, ModelTestResult>>({})

  const providerDrawer = ref<InstanceType<typeof ProviderFormDrawer>>()
  const modelDrawer = ref<InstanceType<typeof ModelFormDrawer>>()
  const grantDialog = ref<InstanceType<typeof GrantRoleDialog>>()

  const activeProvider = computed(
    () => providerList.value.find((item) => item.provider_ulid === activeProviderUlid.value) ?? null
  )

  /** 能力横幅文案：就绪时展示默认模型，否则展示后端给出的提示 */
  const capabilityHint = computed(() => {
    if (!capability.value) return '正在探测 AI 能力…'
    if (!capability.value.chat_ready) {
      return capability.value.hint_text || '尚未配置可用的默认模型，请先创建供应商与模型并授权角色。'
    }
    const modelText =
      capability.value.default_model_label || capability.value.default_model_name || '未命名模型'
    return `默认模型：${modelText}（${capability.value.default_provider_name}）`
  })

  const loadCapability = async () => {
    try {
      capability.value = await getModelCapability()
    } catch {
      capability.value = null
    }
  }

  const loadProviders = async () => {
    providerLoading.value = true
    try {
      providerList.value = await listModelProviders()
      if (
        activeProviderUlid.value &&
        !providerList.value.some((item) => item.provider_ulid === activeProviderUlid.value)
      ) {
        activeProviderUlid.value = ''
      }
    } catch {
      providerList.value = []
    } finally {
      providerLoading.value = false
    }
  }

  const loadModels = async () => {
    modelLoading.value = true
    try {
      modelList.value = await listModelCatalogs(activeProviderUlid.value)
    } catch {
      modelList.value = []
    } finally {
      modelLoading.value = false
    }
  }

  const refreshAll = async () => {
    await Promise.all([loadProviders(), loadModels(), loadCapability()])
  }

  /** 切换供应商筛选：再次点击已选供应商则清空筛选 */
  const selectProvider = async (providerUlid: string) => {
    activeProviderUlid.value = activeProviderUlid.value === providerUlid ? '' : providerUlid
    await loadModels()
  }

  const clearFilter = async () => {
    activeProviderUlid.value = ''
    await loadModels()
  }

  const toggleProviderEnabled = async (row: NWModelProviderVo, enabled: boolean) => {
    try {
      await saveModelProvider({
        provider_ulid: row.provider_ulid,
        provider_name: row.provider_name,
        protocol_code: row.protocol_code,
        base_url: row.base_url,
        enabled_flag: enabled,
        sort_order: row.sort_order,
        remark_text: row.remark_text
      })
      ElMessage.success(enabled ? '供应商已启用' : '供应商已停用')
      await refreshAll()
    } catch {
      row.enabled_flag = !enabled
    }
  }

  const runProviderTest = async (row: NWModelProviderVo) => {
    providerTestingUlid.value = row.provider_ulid
    try {
      const result = await testModelProvider(row.provider_ulid)
      providerTestMap.value = {
        ...providerTestMap.value,
        [row.provider_ulid]: {
          status_code: result.status_code,
          latency_ms: result.latency_ms,
          message_text: result.message_text
        }
      }
      ElMessage.success(
        result.status_code === 'success'
          ? `连通正常，延迟 ${result.latency_ms} ms`
          : `测试未通过：${result.message_text}`
      )
      await loadProviders()
    } finally {
      providerTestingUlid.value = ''
    }
  }

  const runModelTest = async (row: NWModelCatalogVo) => {
    modelTestingUlid.value = row.model_ulid
    try {
      const result = await testModelCatalog(row.model_ulid)
      modelTestMap.value = {
        ...modelTestMap.value,
        [row.model_ulid]: {
          status_code: result.status_code,
          latency_ms: result.latency_ms,
          message_text: result.message_text
        }
      }
      ElMessage.success(
        result.status_code === 'success'
          ? `连通正常，延迟 ${result.latency_ms} ms`
          : `测试未通过：${result.message_text}`
      )
      await loadModels()
    } finally {
      modelTestingUlid.value = ''
    }
  }

  const removeProvider = async (row: NWModelProviderVo) => {
    try {
      await ElMessageBox.confirm(
        `将删除供应商「${row.provider_name}」，其下还有 ${row.model_count} 个模型时会被拒绝，确定吗？`,
        '删除供应商',
        { type: 'warning' }
      )
    } catch {
      return
    }
    await deleteModelProvider(row.provider_ulid)
    ElMessage.success('供应商已删除')
    await refreshAll()
  }

  const removeModel = async (row: NWModelCatalogVo) => {
    try {
      await ElMessageBox.confirm(
        `将删除模型「${row.model_label || row.model_name}」，确定吗？`,
        '删除模型',
        { type: 'warning' }
      )
    } catch {
      return
    }
    await deleteModelCatalog(row.model_ulid)
    ElMessage.success('模型已删除')
    await refreshAll()
  }

  const markDefault = async (row: NWModelCatalogVo) => {
    await setDefaultModelCatalog(row.model_ulid)
    ElMessage.success(`已将「${row.model_label || row.model_name}」设为系统默认`)
    await refreshAll()
  }

  onMounted(() => {
    void refreshAll()
  })
</script>

<template>
  <div class="nw-page">
    <ElCard shadow="never" class="capability" :class="{ 'is-ready': capability?.chat_ready }">
      <div class="capability__row">
        <Icon
          :icon="capability?.chat_ready ? 'mdi:check' : 'mdi:alert-circle-outline'"
          :size="20"
        />
        <div class="capability__text">
          <strong class="capability__title">
            {{ capability?.chat_ready ? 'AI 对话已就绪' : 'AI 对话不可用' }}
          </strong>
          <span class="nw-hint">{{ capabilityHint }}</span>
        </div>
        <span
          class="nw-badge"
          :class="capability?.chat_ready ? 'nw-badge--success' : 'nw-badge--warning'"
        >
          {{ capability?.chat_ready ? '可用' : '不可用' }}
        </span>
      </div>
      <div v-if="(capability?.option_list?.length ?? 0) > 0" class="capability__options">
        <ElTag
          v-for="item in capability?.option_list"
          :key="item.model_ulid"
          size="small"
          effect="plain"
          :type="item.default_flag ? 'primary' : 'info'"
        >
          {{ item.model_label || item.model_name }}
        </ElTag>
      </div>
    </ElCard>

    <ProviderTable
      :providers="providerList"
      :loading="providerLoading"
      :testing-ulid="providerTestingUlid"
      :test-map="providerTestMap"
      :active-provider-ulid="activeProviderUlid"
      @create="providerDrawer?.open()"
      @edit="(row) => providerDrawer?.open(row)"
      @remove="removeProvider"
      @test="runProviderTest"
      @select="selectProvider"
      @toggle="toggleProviderEnabled"
    />

    <CatalogTable
      :models="modelList"
      :providers="providerList"
      :loading="modelLoading"
      :testing-ulid="modelTestingUlid"
      :test-map="modelTestMap"
      :active-provider="activeProvider"
      @create="modelDrawer?.open(undefined, activeProviderUlid)"
      @edit="(row) => modelDrawer?.open(row)"
      @remove="removeModel"
      @test="runModelTest"
      @set-default="markDefault"
      @grant="(row) => grantDialog?.open(row)"
      @clear-filter="clearFilter"
    />

    <UsagePanel />

    <ProviderFormDrawer ref="providerDrawer" @saved="refreshAll" />
    <ModelFormDrawer ref="modelDrawer" :providers="providerList" @saved="refreshAll" />
    <GrantRoleDialog ref="grantDialog" @saved="refreshAll" />
  </div>
</template>

<style lang="less" scoped>
  .capability {
    border-radius: var(--nw-radius-lg);

    &__row {
      display: flex;
      gap: 12px;
      align-items: center;
      color: var(--nw-text-secondary);
    }

    &__text {
      display: flex;
      flex: 1;
      flex-direction: column;
      gap: 4px;
    }

    &__title {
      font-size: 14px;
      font-weight: 650;
      color: var(--nw-ink);
    }

    &__options {
      display: flex;
      flex-wrap: wrap;
      gap: 6px;
      padding-top: 10px;
      margin-top: 10px;
      border-top: 1px solid var(--nw-border-soft);
    }

    &.is-ready {
      border-left: 3px solid var(--nw-success);
    }
  }
</style>

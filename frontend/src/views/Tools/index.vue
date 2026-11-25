<!-- 工具管理页：内置工具与自建工具的启用/停用、描述修改与试运行。 -->
<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import { ElMessage, ElMessageBox } from 'element-plus'
  import {
    deleteTool,
    invokeTool,
    listTools,
    saveTool,
    type NWToolInvokeVo,
    type NWToolVo
  } from '@/api/tool'
  import { useProjectStore } from '@/store/modules/project'
  import { useRowAction } from '@/composables/useRowAction'
  import ToolEditDrawer from './components/ToolEditDrawer.vue'

  const projectStore = useProjectStore()
  const loading = ref(false)
  const rows = ref<NWToolVo[]>([])
  const keywordText = ref('')
  const togglingUlid = ref('')
  const drawer = ref<InstanceType<typeof ToolEditDrawer>>()

  const invokeVisible = ref(false)
  const invokeLoading = ref(false)
  const invokeToolCode = ref('')
  const invokeContentText = ref('')
  const invokeResult = ref<NWToolInvokeVo | null>(null)
  const rowAction = useRowAction()

  const builtinCount = computed(() => rows.value.filter((row) => row.builtin_flag).length)
  const enabledCount = computed(() => rows.value.filter((row) => row.enabled_flag).length)

  const load = async () => {
    loading.value = true
    try {
      rows.value = await listTools({ keyword_text: keywordText.value || undefined })
    } catch {
      rows.value = []
    } finally {
      loading.value = false
    }
  }

  const toggleEnabled = async (row: NWToolVo, enabled: boolean) => {
    togglingUlid.value = row.tool_ulid
    try {
      await saveTool({
        tool_ulid: row.tool_ulid,
        tool_code: row.tool_code,
        tool_name: row.tool_name,
        description_text: row.description_text,
        param_list: row.param_list,
        enabled_flag: enabled,
        sort_order: row.sort_order
      })
      ElMessage.success(enabled ? '已启用' : '已停用')
      await load()
    } finally {
      togglingUlid.value = ''
    }
  }

  const remove = async (row: NWToolVo) => {
    try {
      await ElMessageBox.confirm(`将删除工具「${row.tool_name}」，确定吗？`, '删除工具', {
        type: 'warning'
      })
    } catch {
      return
    }
    await rowAction.run(row.tool_ulid, async () => {
      await deleteTool(row.tool_ulid)
      ElMessage.success('工具已删除')
      await load()
    })
  }

  /** 打开试运行：默认用当前项目的第一章正文 */
  const openInvoke = (row: NWToolVo) => {
    invokeToolCode.value = row.tool_code
    invokeContentText.value = ''
    invokeResult.value = null
    invokeVisible.value = true
  }

  const runInvoke = async () => {
    invokeLoading.value = true
    try {
      invokeResult.value = await invokeTool({
        project_ulid: projectStore.currentProjectUlid || undefined,
        tool_code: invokeToolCode.value,
        content_text: invokeContentText.value || undefined,
        param_value: { query_text: invokeContentText.value }
      })
    } finally {
      invokeLoading.value = false
    }
  }

  onMounted(() => {
    void load()
    if (projectStore.projectOptions.length === 0) void projectStore.loadProjectOptions()
  })
</script>

<template>
  <div class="nw-page">
    <div class="nw-toolbar">
      <ElInput
        v-model="keywordText"
        placeholder="搜索工具名称或编码"
        clearable
        class="tool-page__search"
        @keyup.enter="load()"
        @clear="load()"
      />
      <ElButton type="primary" @click="load()">
        <Icon icon="mdi:magnify" :size="15" />
        <span class="tool-page__btn-text">查询</span>
      </ElButton>
      <div class="tool-page__spacer" />
      <span class="nw-hint">
        共 {{ rows.length }} 个（内置 {{ builtinCount }} · 启用 {{ enabledCount }}）
      </span>
    </div>

    <ElTable v-loading="loading" :data="rows" class="tool-page__table" row-key="tool_ulid">
      <ElTableColumn label="工具" min-width="240">
        <template #default="{ row }">
          <div class="tool-page__name">
            <span>{{ row.tool_name }}</span>
            <span v-if="row.builtin_flag" class="nw-badge nw-badge--neutral">内置</span>
            <span v-if="row.customized_flag" class="nw-badge nw-badge--warning">已改</span>
          </div>
          <div class="nw-hint">#{{ row.tool_code }}</div>
        </template>
      </ElTableColumn>
      <ElTableColumn label="说明" min-width="300" show-overflow-tooltip>
        <template #default="{ row }">{{ row.description_text }}</template>
      </ElTableColumn>
      <ElTableColumn label="参数" width="90" align="center">
        <template #default="{ row }">{{ row.param_list?.length ?? 0 }}</template>
      </ElTableColumn>
      <ElTableColumn label="启用" width="90" align="center">
        <template #default="{ row }">
          <ElSwitch
            :model-value="row.enabled_flag"
            :loading="togglingUlid === row.tool_ulid"
            @update:model-value="(value: string | number | boolean) => toggleEnabled(row, Boolean(value))"
          />
        </template>
      </ElTableColumn>
      <ElTableColumn label="操作" width="190" fixed="right">
        <template #default="{ row }">
          <ElButton size="small" text type="primary" @click="openInvoke(row)">试运行</ElButton>
          <ElButton size="small" text @click="drawer?.open(row)">编辑</ElButton>
          <ElButton
            v-if="!row.builtin_flag"
            size="small"
            text
            type="danger"
            :loading="rowAction.isPending(row.tool_ulid)"
            :disabled="rowAction.busyFlag.value"
            @click="remove(row)"
          >
            删除
          </ElButton>
        </template>
      </ElTableColumn>
      <template #empty>
        <ElEmpty description="暂无工具" />
      </template>
    </ElTable>

    <ToolEditDrawer ref="drawer" @saved="load" />

    <ElDrawer v-model="invokeVisible" :title="`试运行 · ${invokeToolCode}`" size="520px" destroy-on-close>
      <div class="tool-invoke">
        <ElAlert type="info" :closable="false" show-icon>
          工具结果以「工具结果」块注入对话上下文；试运行只读，不会写入项目数据。
        </ElAlert>
        <ElInput
          v-model="invokeContentText"
          type="textarea"
          :rows="8"
          placeholder="粘贴待检正文；留空则取当前项目章节（需工具支持）"
        />
        <ElButton type="primary" :loading="invokeLoading" @click="runInvoke">
          <Icon icon="mdi:play" :size="15" />
          <span class="tool-page__btn-text">执行</span>
        </ElButton>
        <template v-if="invokeResult">
          <div class="tool-invoke__metrics">
            <span class="nw-badge nw-badge--neutral">耗时 {{ invokeResult.latency_ms }} ms</span>
            <span
              v-for="(value, key) in invokeResult.metric_map"
              :key="key"
              class="nw-badge nw-badge--neutral"
            >
              {{ key }}：{{ value }}
            </span>
          </div>
          <pre class="tool-invoke__result">{{ invokeResult.result_text }}</pre>
        </template>
      </div>
    </ElDrawer>
  </div>
</template>

<style lang="less" scoped>
  .tool-page {
    &__search {
      width: 260px;
    }

    &__btn-text {
      margin-left: 4px;
    }

    &__spacer {
      flex: 1;
    }

    &__table {
      margin-top: 12px;
    }

    &__name {
      display: flex;
      gap: 6px;
      align-items: center;
      font-weight: 600;
      color: var(--nw-ink);
    }
  }

  .tool-invoke {
    display: flex;
    flex-direction: column;
    gap: 12px;

    &__metrics {
      display: flex;
      flex-wrap: wrap;
      gap: 6px;
    }

    &__result {
      max-height: 420px;
      padding: 12px;
      overflow: auto;
      font-family: ui-monospace, 'SFMono-Regular', Consolas, monospace;
      font-size: 12px;
      line-height: 1.7;
      white-space: pre-wrap;
      word-break: break-word;
      background: var(--nw-surface-subtle);
      border: 1px solid var(--nw-border);
      border-radius: var(--nw-radius-md);
    }
  }
</style>

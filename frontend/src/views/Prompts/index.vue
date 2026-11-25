<!-- 提示词库页：按流程域分组展示身份提示词与指令提示词，支持编辑、启停、预览与命令解析。 -->
<script setup lang="ts">
  import { computed, onMounted, ref, watch } from 'vue'
  import {
    ElButton,
    ElCard,
    ElEmpty,
    ElMessage,
    ElMessageBox,
    ElOption,
    ElSelect,
    ElSwitch,
    ElTable,
    ElTableColumn,
    ElTag
  } from 'element-plus'
  import { deletePrompt, listPrompts, savePrompt } from '@/api/prompt'
  import type { NWPromptVo } from '@/api/contracts'
  import { useProjectStore } from '@/store/modules/project'
  import { useRowAction } from '@/composables/useRowAction'
  import { formatRelativeTime } from '@/utils/time'
  import PromptEditDrawer from './components/PromptEditDrawer.vue'
  import PromptPreviewDialog from './components/PromptPreviewDialog.vue'

  /** 分组结构：按流程域归组，无流程域的自建提示词归入「自建提示词」 */
  interface PromptGroup {
    groupKey: string
    groupName: string
    rows: NWPromptVo[]
  }

  const projectStore = useProjectStore()

  const loading = ref(false)
  const rows = ref<NWPromptVo[]>([])
  const projectUlid = ref(projectStore.currentProjectUlid)
  const enabledOnly = ref(false)
  const kindCode = ref('')
  const keywordText = ref('')
  const togglingUlid = ref('')
  const rowAction = useRowAction()

  const drawer = ref<InstanceType<typeof PromptEditDrawer>>()
  const previewVisible = ref(false)
  const previewPrompt = ref<NWPromptVo | null>(null)

  const projectNameOf = (ulid: string) =>
    projectStore.projectOptions.find((item) => item.project_ulid === ulid)?.project_name ?? ''

  const kindLabel = (code: string) => (code === 'system' ? '身份' : '指令')

  const load = async () => {
    loading.value = true
    try {
      rows.value = await listPrompts({
        project_ulid: projectUlid.value || undefined,
        prompt_kind_code: kindCode.value || undefined,
        keyword_text: keywordText.value || undefined,
        enabled_only: enabledOnly.value || undefined
      })
    } catch {
      rows.value = []
    } finally {
      loading.value = false
    }
  }

  const groups = computed<PromptGroup[]>(() => {
    const groupMap = new Map<string, PromptGroup>()
    for (const row of rows.value) {
      const groupKey = row.domain_code || 'custom'
      const groupName = row.domain_name || '自建提示词'
      const existing = groupMap.get(groupKey)
      if (existing) {
        existing.rows.push(row)
      } else {
        groupMap.set(groupKey, { groupKey, groupName, rows: [row] })
      }
    }
    return [...groupMap.values()].map((group) => ({
      ...group,
      rows: [...group.rows].sort((left, right) => {
        if (left.prompt_kind_code !== right.prompt_kind_code) {
          return left.prompt_kind_code === 'system' ? -1 : 1
        }
        return left.sort_order - right.sort_order
      })
    }))
  })

  const builtinCount = computed(() => rows.value.filter((row) => row.builtin_flag).length)
  const enabledCount = computed(() => rows.value.filter((row) => row.enabled_flag).length)

  /** 开关切换需回传完整提示词体（保存为整体覆盖） */
  const toggleEnabled = async (row: NWPromptVo, enabled: boolean) => {
    togglingUlid.value = row.prompt_ulid
    try {
      await savePrompt({
        prompt_ulid: row.prompt_ulid,
        prompt_code: row.prompt_code,
        prompt_name: row.prompt_name,
        prompt_kind_code: row.prompt_kind_code,
        scope_code: row.scope_code,
        project_ulid: row.project_ulid || undefined,
        category_code: row.category_code,
        description_text: row.description_text,
        prompt_text: row.prompt_text,
        enabled_flag: enabled,
        sort_order: row.sort_order
      })
      ElMessage.success(enabled ? '提示词已启用' : '提示词已停用')
      await load()
    } catch {
      row.enabled_flag = !enabled
    } finally {
      togglingUlid.value = ''
    }
  }

  const remove = async (row: NWPromptVo) => {
    try {
      await ElMessageBox.confirm(`将删除提示词「${row.prompt_name}」，确定吗？`, '删除提示词', {
        type: 'warning'
      })
    } catch {
      return
    }
    await rowAction.run(row.prompt_ulid, async () => {
      await deletePrompt(row.prompt_ulid)
      ElMessage.success('提示词已删除')
      await load()
    })
  }

  const openPreview = (row: NWPromptVo) => {
    previewPrompt.value = row
    previewVisible.value = true
  }

  watch([projectUlid, enabledOnly, kindCode], () => void load())

  onMounted(async () => {
    const initialUlid = projectUlid.value
    if (projectStore.projectOptions.length === 0) {
      await projectStore.loadProjectOptions()
      projectUlid.value = projectStore.currentProjectUlid
    }
    // 项目切换会触发 watch 里的加载，这里只处理未切换的情况
    if (projectUlid.value === initialUlid) await load()
  })
</script>

<template>
  <div class="nw-page">
    <div class="nw-toolbar">
      <ElSelect
        v-model="projectUlid"
        placeholder="仅全局提示词"
        clearable
        filterable
        class="prompts__project"
      >
        <ElOption
          v-for="item in projectStore.projectOptions"
          :key="item.project_ulid"
          :label="item.project_name"
          :value="item.project_ulid"
        />
      </ElSelect>
      <ElSelect v-model="kindCode" placeholder="全部种类" clearable class="prompts__kind">
        <ElOption label="身份提示词（会话常驻）" value="system" />
        <ElOption label="指令提示词（/ 触发）" value="command" />
      </ElSelect>
      <ElButton type="primary" plain @click="drawer?.open(undefined, projectUlid)">
        <Icon icon="mdi:plus" :size="15" />
        <span class="ml-4px">新建提示词</span>
      </ElButton>
      <div class="prompts__spacer" />
      <div class="prompts__filter">
        <ElSwitch v-model="enabledOnly" size="small" />
        <span class="nw-hint">仅看启用</span>
      </div>
      <span class="nw-hint">
        共 {{ rows.length }} 条 · 启用 {{ enabledCount }} · 内置 {{ builtinCount }}
      </span>
    </div>

    <div v-loading="loading" class="prompts__groups">
      <ElCard v-for="group in groups" :key="group.groupKey" shadow="never" class="prompts__group">
        <template #header>
          <div class="prompts__group-head">
            <span class="prompts__group-title">{{ group.groupName }}</span>
            <span class="nw-hint">{{ group.rows.length }} 条</span>
          </div>
        </template>

        <ElTable :data="group.rows" size="small">
          <ElTableColumn label="提示词" min-width="240">
            <template #default="{ row }">
              <div class="cell-main">
                <span class="cell-main__title">
                  {{ row.prompt_name }}
                  <span class="cell-main__code">
                    {{ row.prompt_kind_code === 'command' ? '/' : '' }}{{ row.prompt_code }}
                  </span>
                </span>
                <span class="cell-main__sub">
                  {{ row.description_text || row.prompt_text.slice(0, 60) }}
                </span>
              </div>
            </template>
          </ElTableColumn>
          <ElTableColumn label="种类" width="90">
            <template #default="{ row }">
              <ElTag size="small" :type="row.prompt_kind_code === 'system' ? 'success' : 'info'" effect="plain">
                {{ kindLabel(row.prompt_kind_code) }}
              </ElTag>
            </template>
          </ElTableColumn>
          <ElTableColumn label="标记" width="170">
            <template #default="{ row }">
              <div class="cell-tags">
                <ElTag v-if="row.builtin_flag" size="small" type="info" effect="plain">内置</ElTag>
                <ElTag v-if="row.customized_flag" size="small" type="warning" effect="plain">
                  已自定义
                </ElTag>
                <ElTag v-if="row.overridden_flag" size="small" type="danger" effect="plain">
                  被项目覆盖
                </ElTag>
                <ElTag v-if="!row.builtin_flag" size="small" type="success" effect="plain">自建</ElTag>
              </div>
            </template>
          </ElTableColumn>
          <ElTableColumn label="作用域" width="150">
            <template #default="{ row }">
              <span v-if="row.scope_code === 'project'" class="cell-scope">
                项目 · {{ projectNameOf(row.project_ulid) || '未知项目' }}
              </span>
              <span v-else class="cell-scope">全局</span>
            </template>
          </ElTableColumn>
          <ElTableColumn label="字数" width="80">
            <template #default="{ row }">{{ row.prompt_text.length }}</template>
          </ElTableColumn>
          <ElTableColumn label="更新" width="110">
            <template #default="{ row }">
              <span class="cell-scope">{{ formatRelativeTime(row.updated_at) }}</span>
            </template>
          </ElTableColumn>
          <ElTableColumn label="启用" width="70">
            <template #default="{ row }">
              <ElSwitch
                :model-value="row.enabled_flag"
                size="small"
                :loading="togglingUlid === row.prompt_ulid"
                @update:model-value="toggleEnabled(row as NWPromptVo, Boolean($event))"
              />
            </template>
          </ElTableColumn>
          <ElTableColumn label="操作" width="180" fixed="right">
            <template #default="{ row }">
              <ElButton size="small" text @click="openPreview(row as NWPromptVo)">预览</ElButton>
              <ElButton size="small" text @click="drawer?.open(row as NWPromptVo, projectUlid)">
                编辑
              </ElButton>
              <ElButton
                size="small"
                text
                type="danger"
                :loading="rowAction.isPending(row.prompt_ulid)"
                :disabled="row.builtin_flag || rowAction.busyFlag.value"
                @click="remove(row as NWPromptVo)"
              >
                删除
              </ElButton>
            </template>
          </ElTableColumn>
        </ElTable>
      </ElCard>

      <ElEmpty v-if="!loading && groups.length === 0" description="还没有提示词，先建一条吧" />
    </div>

    <PromptEditDrawer ref="drawer" :projects="projectStore.projectOptions" @saved="load" />
    <PromptPreviewDialog v-model:visible="previewVisible" :prompt="previewPrompt" />
  </div>
</template>

<style lang="less" scoped>
  .prompts {
    &__project {
      width: 180px;
    }

    &__kind {
      width: 200px;
    }

    &__spacer {
      flex: 1;
    }

    &__filter {
      display: flex;
      gap: 6px;
      align-items: center;
    }

    &__groups {
      display: flex;
      flex-direction: column;
      gap: 14px;
    }

    &__group {
      border-radius: var(--nw-radius-lg);
    }

    &__group-head {
      display: flex;
      gap: 10px;
      align-items: baseline;
    }

    &__group-title {
      font-size: 14px;
      font-weight: 650;
      color: var(--nw-ink);
    }
  }

  .cell-main {
    display: flex;
    flex-direction: column;
    gap: 2px;

    &__title {
      display: flex;
      gap: 8px;
      align-items: baseline;
      font-weight: 600;
      color: var(--nw-ink);
    }

    &__code {
      font-family: var(--el-font-family-mono, monospace);
      font-size: 11px;
      font-weight: 400;
      color: var(--nw-primary);
    }

    &__sub {
      overflow: hidden;
      font-size: 11px;
      color: var(--nw-text-muted);
      text-overflow: ellipsis;
      white-space: nowrap;
    }
  }

  .cell-tags {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
  }

  .cell-scope {
    font-size: 12px;
    color: var(--nw-text-secondary);
  }
</style>

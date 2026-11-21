<!-- 提示词档案页：按流程域分组展示与编辑档案。 -->
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
  import { deletePromptProfile, listPromptProfiles, savePromptProfile } from '@/api/prompt'
  import type { NWPromptProfileVo } from '@/api/contracts'
  import { useProjectStore } from '@/store/modules/project'
  import { formatRelativeTime } from '@/utils/time'
  import PromptEditDrawer from './components/PromptEditDrawer.vue'
  import PromptPreviewDialog from './components/PromptPreviewDialog.vue'

  /** 分组结构：按流程域归组，无流程域的自建档案归入「自建档案」 */
  interface ProfileGroup {
    groupKey: string
    groupName: string
    rows: NWPromptProfileVo[]
  }

  const projectStore = useProjectStore()

  const loading = ref(false)
  const rows = ref<NWPromptProfileVo[]>([])
  const projectUlid = ref(projectStore.currentProjectUlid)
  const enabledOnly = ref(false)
  const togglingUlid = ref('')

  const drawer = ref<InstanceType<typeof PromptEditDrawer>>()
  const previewVisible = ref(false)
  const previewProfile = ref<NWPromptProfileVo | null>(null)

  const projectNameOf = (ulid: string) =>
    projectStore.projectOptions.find((item) => item.project_ulid === ulid)?.project_name ?? ''

  const load = async () => {
    loading.value = true
    try {
      rows.value = await listPromptProfiles({
        project_ulid: projectUlid.value || undefined,
        enabled_only: enabledOnly.value || undefined
      })
    } catch {
      rows.value = []
    } finally {
      loading.value = false
    }
  }

  const groups = computed<ProfileGroup[]>(() => {
    const groupMap = new Map<string, ProfileGroup>()
    for (const row of rows.value) {
      const groupKey = row.domain_code || 'custom'
      const groupName = row.domain_name || '自建档案'
      const existing = groupMap.get(groupKey)
      if (existing) {
        existing.rows.push(row)
      } else {
        groupMap.set(groupKey, { groupKey, groupName, rows: [row] })
      }
    }
    return [...groupMap.values()].map((group) => ({
      ...group,
      rows: [...group.rows].sort((left, right) => left.sort_order - right.sort_order)
    }))
  })

  const builtinCount = computed(() => rows.value.filter((row) => row.builtin_flag).length)
  const enabledCount = computed(() => rows.value.filter((row) => row.enabled_flag).length)

  /** 开关切换需回传完整档案体（保存为整体覆盖） */
  const toggleEnabled = async (row: NWPromptProfileVo, enabled: boolean) => {
    togglingUlid.value = row.prompt_profile_ulid
    try {
      await savePromptProfile({
        prompt_profile_ulid: row.prompt_profile_ulid,
        prompt_profile_code: row.prompt_profile_code,
        prompt_profile_name: row.prompt_profile_name,
        scope_code: row.scope_code,
        project_ulid: row.project_ulid || undefined,
        description_text: row.description_text,
        system_prompt_text: row.system_prompt_text,
        enabled_flag: enabled,
        sort_order: row.sort_order
      })
      ElMessage.success(enabled ? '档案已启用' : '档案已停用')
      await load()
    } catch {
      row.enabled_flag = !enabled
    } finally {
      togglingUlid.value = ''
    }
  }

  const remove = async (row: NWPromptProfileVo) => {
    try {
      await ElMessageBox.confirm(
        `将删除提示词档案「${row.prompt_profile_name}」，确定吗？`,
        '删除档案',
        { type: 'warning' }
      )
    } catch {
      return
    }
    await deletePromptProfile(row.prompt_profile_ulid)
    ElMessage.success('档案已删除')
    await load()
  }

  const openPreview = (row: NWPromptProfileVo) => {
    previewProfile.value = row
    previewVisible.value = true
  }

  watch(projectUlid, () => {
    void load()
  })

  watch(enabledOnly, () => {
    void load()
  })

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
        placeholder="仅全局档案"
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
      <ElButton type="primary" plain @click="drawer?.open(undefined, projectUlid)">
        <Icon icon="mdi:plus" :size="15" />
        <span class="ml-4px">新建档案</span>
      </ElButton>
      <div class="prompts__spacer" />
      <div class="prompts__filter">
        <ElSwitch v-model="enabledOnly" size="small" />
        <span class="nw-hint">仅看启用</span>
      </div>
      <span class="nw-hint">
        共 {{ rows.length }} 个档案 · 启用 {{ enabledCount }} · 内置 {{ builtinCount }}
      </span>
    </div>

    <div v-loading="loading" class="prompts__groups">
      <ElCard v-for="group in groups" :key="group.groupKey" shadow="never" class="prompts__group">
        <template #header>
          <div class="prompts__group-head">
            <span class="prompts__group-title">{{ group.groupName }}</span>
            <span class="nw-hint">{{ group.rows.length }} 个</span>
          </div>
        </template>

        <ElTable :data="group.rows" size="small">
          <ElTableColumn label="档案" min-width="200">
            <template #default="{ row }">
              <div class="cell-main">
                <span class="cell-main__title">
                  {{ row.prompt_profile_name }}
                  <span class="cell-main__code">{{ row.prompt_profile_code }}</span>
                </span>
                <span class="cell-main__sub">
                  {{ row.description_text || row.system_prompt_text.slice(0, 60) }}
                </span>
              </div>
            </template>
          </ElTableColumn>
          <ElTableColumn label="标记" width="180">
            <template #default="{ row }">
              <div class="cell-tags">
                <ElTag v-if="row.builtin_flag" size="small" type="info" effect="plain">内置</ElTag>
                <ElTag v-if="row.customized_flag" size="small" type="warning" effect="plain">
                  已自定义
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
            <template #default="{ row }">{{ row.system_prompt_text.length }}</template>
          </ElTableColumn>
          <ElTableColumn label="排序" width="70">
            <template #default="{ row }">{{ row.sort_order }}</template>
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
                :loading="togglingUlid === row.prompt_profile_ulid"
                @update:model-value="toggleEnabled(row as NWPromptProfileVo, Boolean($event))"
              />
            </template>
          </ElTableColumn>
          <ElTableColumn label="操作" width="180" fixed="right">
            <template #default="{ row }">
              <ElButton size="small" text @click="openPreview(row as NWPromptProfileVo)">预览</ElButton>
              <ElButton size="small" text @click="drawer?.open(row as NWPromptProfileVo, projectUlid)">
                编辑
              </ElButton>
              <ElButton
                size="small"
                text
                type="danger"
                :disabled="row.builtin_flag"
                @click="remove(row as NWPromptProfileVo)"
              >
                删除
              </ElButton>
            </template>
          </ElTableColumn>
        </ElTable>
      </ElCard>

      <ElEmpty
        v-if="!loading && groups.length === 0"
        description="还没有提示词档案，先建一个吧"
      />
    </div>

    <PromptEditDrawer ref="drawer" :projects="projectStore.projectOptions" @saved="load" />
    <PromptPreviewDialog v-model:visible="previewVisible" :profile="previewProfile" />
  </div>
</template>

<style lang="less" scoped>
  .prompts {
    &__project {
      width: 180px;
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

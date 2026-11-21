<!-- 技能管理页：按流程域分组展示技能并支持编辑。 -->
<script setup lang="ts">
  import { computed, onMounted, ref, watch } from 'vue'
  import {
    ElButton,
    ElCard,
    ElEmpty,
    ElInput,
    ElMessage,
    ElMessageBox,
    ElOption,
    ElSelect,
    ElSwitch,
    ElTable,
    ElTableColumn,
    ElTag
  } from 'element-plus'
  import { deleteSkill, listSkills, saveSkill } from '@/api/skill'
  import type { NWSkillVo } from '@/api/contracts'
  import { useProjectStore } from '@/store/modules/project'
  import SkillEditDrawer from './components/SkillEditDrawer.vue'
  import SkillTrialDialog from './components/SkillTrialDialog.vue'

  /** 分组结构：流程域优先，没有流程域时按分类归组 */
  interface SkillGroup {
    groupKey: string
    groupName: string
    rows: NWSkillVo[]
  }

  const projectStore = useProjectStore()

  const loading = ref(false)
  const rows = ref<NWSkillVo[]>([])
  const keywordText = ref('')
  const scopeCode = ref('')
  const categoryCode = ref('')
  const projectUlid = ref(projectStore.currentProjectUlid)
  const togglingUlid = ref('')

  const drawer = ref<InstanceType<typeof SkillEditDrawer>>()
  const trialDialog = ref<InstanceType<typeof SkillTrialDialog>>()

  const categoryOptions = [
    { value: 'write', label: '写作' },
    { value: 'revise', label: '修订' },
    { value: 'outline', label: '规划' },
    { value: 'setting', label: '设定' },
    { value: 'review', label: '审稿' },
    { value: 'other', label: '其他' }
  ]

  const categoryLabel = (code: string) =>
    categoryOptions.find((item) => item.value === code)?.label ?? (code || '未分类')

  const projectNameOf = (ulid: string) =>
    projectStore.projectOptions.find((item) => item.project_ulid === ulid)?.project_name ?? ''

  const load = async () => {
    loading.value = true
    try {
      const result = await listSkills({
        keyword_text: keywordText.value || undefined,
        scope_code: scopeCode.value || undefined,
        category_code: categoryCode.value || undefined,
        project_ulid: projectUlid.value || undefined
      })
      // 指定项目时后端会合并全局技能，作用域筛选在这里二次收敛
      rows.value = scopeCode.value
        ? result.filter((row) => row.scope_code === scopeCode.value)
        : result
    } catch {
      rows.value = []
    } finally {
      loading.value = false
    }
  }

  const groups = computed<SkillGroup[]>(() => {
    const groupMap = new Map<string, SkillGroup>()
    for (const row of rows.value) {
      const groupName = row.domain_name || categoryLabel(row.category_code)
      const groupKey = row.domain_name || `category:${row.category_code}`
      const existing = groupMap.get(groupKey)
      if (existing) {
        existing.rows.push(row)
      } else {
        groupMap.set(groupKey, { groupKey, groupName, rows: [row] })
      }
    }
    return [...groupMap.values()].sort((left, right) =>
      left.groupName.localeCompare(right.groupName, 'zh-CN')
    )
  })

  const builtinCount = computed(() => rows.value.filter((row) => row.builtin_flag).length)
  const enabledCount = computed(() => rows.value.filter((row) => row.enabled_flag).length)

  /** 开关切换需要回传完整技能体（后端保存为整体覆盖） */
  const toggleEnabled = async (row: NWSkillVo, enabled: boolean) => {
    togglingUlid.value = row.skill_ulid
    try {
      await saveSkill({
        skill_ulid: row.skill_ulid,
        skill_code: row.skill_code,
        skill_name: row.skill_name,
        scope_code: row.scope_code,
        project_ulid: row.project_ulid || undefined,
        category_code: row.category_code,
        description_text: row.description_text,
        prompt_template: row.prompt_template,
        variable_list: row.variable_list,
        enabled_flag: enabled,
        sort_order: row.sort_order
      })
      ElMessage.success(enabled ? '技能已启用' : '技能已停用')
      await load()
    } catch {
      row.enabled_flag = !enabled
    } finally {
      togglingUlid.value = ''
    }
  }

  const remove = async (row: NWSkillVo) => {
    try {
      await ElMessageBox.confirm(
        `将删除技能「${row.skill_name}」及其参考资料，确定吗？`,
        '删除技能',
        { type: 'warning' }
      )
    } catch {
      return
    }
    await deleteSkill(row.skill_ulid)
    ElMessage.success('技能已删除')
    await load()
  }

  const openCreate = () => {
    drawer.value?.open(undefined, projectUlid.value)
  }

  watch(projectUlid, () => {
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
      <ElInput
        v-model="keywordText"
        placeholder="搜索名称或编码"
        clearable
        class="skills__search"
        @keyup.enter="load()"
        @clear="load()"
      />
      <ElSelect v-model="scopeCode" placeholder="全部作用域" clearable class="skills__select" @change="load()">
        <ElOption label="全局" value="global" />
        <ElOption label="项目" value="project" />
      </ElSelect>
      <ElSelect v-model="categoryCode" placeholder="全部分类" clearable class="skills__select" @change="load()">
        <ElOption
          v-for="item in categoryOptions"
          :key="item.value"
          :label="item.label"
          :value="item.value"
        />
      </ElSelect>
      <ElSelect
        v-model="projectUlid"
        placeholder="仅全局技能"
        clearable
        filterable
        class="skills__project"
      >
        <ElOption
          v-for="item in projectStore.projectOptions"
          :key="item.project_ulid"
          :label="item.project_name"
          :value="item.project_ulid"
        />
      </ElSelect>
      <ElButton type="primary" plain @click="openCreate">
        <Icon icon="mdi:plus" :size="15" />
        <span class="ml-4px">新建技能</span>
      </ElButton>
      <div class="skills__spacer" />
      <span class="nw-hint">
        共 {{ rows.length }} 个技能 · 启用 {{ enabledCount }} · 内置 {{ builtinCount }}
      </span>
    </div>

    <div v-loading="loading" class="skills__groups">
      <ElCard v-for="group in groups" :key="group.groupKey" shadow="never" class="skills__group">
        <template #header>
          <div class="skills__group-head">
            <span class="skills__group-title">{{ group.groupName }}</span>
            <span class="nw-hint">{{ group.rows.length }} 个</span>
          </div>
        </template>

        <ElTable :data="group.rows" size="small">
          <ElTableColumn label="技能" min-width="200">
            <template #default="{ row }">
              <div class="cell-main">
                <span class="cell-main__title">
                  {{ row.skill_name }}
                  <span class="cell-main__code">/{{ row.skill_code }}</span>
                </span>
                <span class="cell-main__sub">{{ row.description_text || '（无说明）' }}</span>
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
                <ElTag v-if="row.overridden_flag" size="small" type="danger" effect="plain">
                  被项目覆盖
                </ElTag>
                <ElTag v-if="!row.builtin_flag" size="small" type="success" effect="plain">自建</ElTag>
              </div>
            </template>
          </ElTableColumn>
          <ElTableColumn label="作用域" width="130">
            <template #default="{ row }">
              <span v-if="row.scope_code === 'project'" class="cell-scope">
                项目 · {{ projectNameOf(row.project_ulid) || '未知项目' }}
              </span>
              <span v-else class="cell-scope">全局</span>
            </template>
          </ElTableColumn>
          <ElTableColumn label="分类" width="80">
            <template #default="{ row }">{{ categoryLabel(row.category_code) }}</template>
          </ElTableColumn>
          <ElTableColumn label="变量/资料" width="100">
            <template #default="{ row }">
              {{ row.variable_list.length }} / {{ row.ref_count }}
            </template>
          </ElTableColumn>
          <ElTableColumn label="启用" width="70">
            <template #default="{ row }">
              <ElSwitch
                :model-value="row.enabled_flag"
                size="small"
                :loading="togglingUlid === row.skill_ulid"
                @update:model-value="toggleEnabled(row as NWSkillVo, Boolean($event))"
              />
            </template>
          </ElTableColumn>
          <ElTableColumn label="操作" width="200" fixed="right">
            <template #default="{ row }">
              <ElButton size="small" text @click="drawer?.open(row as NWSkillVo, projectUlid)">编辑</ElButton>
              <ElButton size="small" text @click="trialDialog?.open(row as NWSkillVo)">试用</ElButton>
              <ElButton
                size="small"
                text
                type="danger"
                :disabled="row.builtin_flag"
                @click="remove(row as NWSkillVo)"
              >
                删除
              </ElButton>
            </template>
          </ElTableColumn>
        </ElTable>
      </ElCard>

      <ElEmpty
        v-if="!loading && groups.length === 0"
        description="没有匹配的技能，换个筛选条件或新建一个"
      />
    </div>

    <SkillEditDrawer
      ref="drawer"
      :projects="projectStore.projectOptions"
      @saved="load"
    />
    <SkillTrialDialog ref="trialDialog" :project-ulid="projectUlid || projectStore.currentProjectUlid" />
  </div>
</template>

<style lang="less" scoped>
  .skills {
    &__search {
      width: 220px;
    }

    &__select {
      width: 130px;
    }

    &__project {
      width: 170px;
    }

    &__spacer {
      flex: 1;
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

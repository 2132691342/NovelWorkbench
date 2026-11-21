<!-- 角色管理页：角色卡片列表与关系图谱两个页签。 -->
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
    ElTable,
    ElTableColumn,
    ElTabPane,
    ElTabs,
    ElTooltip
  } from 'element-plus'
  import {
    deleteCharacter,
    getCharacterGraph,
    listCharacters,
    type NWCharacterGraphVo,
    type NWCharacterVo
  } from '@/api/character'
  import { useProjectStore } from '@/store/modules/project'
  import CharacterFormDrawer from './components/CharacterFormDrawer.vue'
  import CharacterGraphPanel from './components/CharacterGraphPanel.vue'
  import RelationFormDialog from './components/RelationFormDialog.vue'
  import { asRow } from '@/utils/table'

  const projectStore = useProjectStore()
  const projectUlid = computed(() => projectStore.currentProjectUlid)

  /** 角色类型选项（与后端枚举一致） */
  const roleTypeOptions = [
    { value: '', label: '全部类型' },
    { value: 'protagonist', label: '主角' },
    { value: 'antagonist', label: '反派' },
    { value: 'supporting', label: '配角' },
    { value: 'minor', label: '龙套' }
  ]

  /** 关系类型展示文案（图谱边标签复用） */
  const relationTypeLabelMap: Record<string, string> = {
    friend: '朋友',
    enemy: '敌对',
    lover: '恋人',
    kin: '亲属',
    teacher: '师徒',
    rival: '对手',
    subordinate: '上下级'
  }

  const activeTab = ref<string | number>('list')
  const roleTypeCode = ref('')
  const keywordText = ref('')
  const characterList = ref<NWCharacterVo[]>([])
  const listLoading = ref(false)

  const graphData = ref<NWCharacterGraphVo | null>(null)
  const graphLoading = ref(false)

  const formDrawer = ref<InstanceType<typeof CharacterFormDrawer> | null>(null)
  const relationDialog = ref<InstanceType<typeof RelationFormDialog> | null>(null)

  const roleTypeLabel = (code: string) =>
    roleTypeOptions.find((item) => item.value === code)?.label ?? (code || '未分类')

  /** 角色类型徽标配色 */
  const roleTypeBadge = (code: string) => {
    if (code === 'protagonist') return 'success'
    if (code === 'antagonist') return 'danger'
    if (code === 'supporting') return 'warning'
    return 'neutral'
  }

  /** 角色描述摘要：优先性格，其次背景 */
  const describeCharacter = (row: NWCharacterVo) =>
    row.personality_text || row.background_text || row.goal_text || '暂无描述'

  const graphNodeList = computed(() => graphData.value?.node_list ?? [])

  const loadCharacters = async () => {
    if (!projectUlid.value) {
      characterList.value = []
      return
    }
    listLoading.value = true
    try {
      characterList.value = await listCharacters({
        project_ulid: projectUlid.value,
        role_type_code: roleTypeCode.value || undefined,
        keyword_text: keywordText.value.trim() || undefined
      })
    } catch {
      characterList.value = []
    } finally {
      listLoading.value = false
    }
  }

  const loadGraph = async () => {
    if (!projectUlid.value) {
      graphData.value = null
      return
    }
    graphLoading.value = true
    try {
      graphData.value = await getCharacterGraph(projectUlid.value)
    } catch {
      graphData.value = null
    } finally {
      graphLoading.value = false
    }
  }

  const refreshAll = () => Promise.all([loadCharacters(), loadGraph()])

  const resetFilter = () => {
    roleTypeCode.value = ''
    keywordText.value = ''
    void loadCharacters()
  }

  const removeCharacter = async (row: NWCharacterVo) => {
    try {
      await ElMessageBox.confirm(
        `删除「${row.character_name}」会同时删除其全部关系连线，确定继续？`,
        '删除角色',
        { type: 'warning' }
      )
    } catch {
      return
    }
    await deleteCharacter(row.character_ulid)
    ElMessage.success('角色已删除')
    await refreshAll()
  }

  const openRelationDialog = () => {
    if (graphNodeList.value.length < 2) {
      ElMessage.warning('至少需要两个角色才能创建关系')
      return
    }
    relationDialog.value?.open()
  }

  watch(
    projectUlid,
    async (nextUlid) => {
      if (!nextUlid) {
        characterList.value = []
        graphData.value = null
        return
      }
      await refreshAll()
    },
    { immediate: true }
  )

  watch(activeTab, (nextTab) => {
    if (nextTab === 'graph') void loadGraph()
  })

  onMounted(async () => {
    if (!projectStore.projectOptions.length) {
      await projectStore.loadProjectOptions()
    }
  })
</script>

<template>
  <div class="nw-page">
    <ElCard shadow="never" class="character__card">
      <ElTabs v-model="activeTab" class="character__tabs">
        <ElTabPane name="list">
          <template #label>
            <span class="character__tab-label">
              <Icon icon="mdi:account-group" :size="14" />
              <span>角色列表</span>
            </span>
          </template>

          <div class="nw-toolbar character__toolbar">
            <ElInput
              v-model="keywordText"
              placeholder="搜索姓名或别名"
              clearable
              class="character__search"
              @keyup.enter="loadCharacters"
              @clear="loadCharacters"
            />
            <ElSelect
              v-model="roleTypeCode"
              class="character__role-filter"
              @change="loadCharacters"
            >
              <ElOption
                v-for="item in roleTypeOptions"
                :key="item.value"
                :label="item.label"
                :value="item.value"
              />
            </ElSelect>
            <ElButton size="small" plain :loading="listLoading" @click="loadCharacters">
              <Icon icon="mdi:reload" :size="14" />
              <span class="ml-4px">刷新</span>
            </ElButton>
            <ElButton size="small" text @click="resetFilter">重置</ElButton>
            <div class="character__spacer" />
            <span class="nw-hint">共 {{ characterList.length }} 个角色</span>
            <ElButton
              type="primary"
              plain
              size="small"
              :disabled="!projectUlid"
              @click="formDrawer?.open()"
            >
              <Icon icon="mdi:plus" :size="14" />
              <span class="ml-4px">新建角色</span>
            </ElButton>
          </div>

          <ElTable
            v-loading="listLoading"
            :data="characterList"
            size="small"
            row-key="character_ulid"
            class="character__table"
          >
            <ElTableColumn label="姓名" min-width="140">
              <template #default="{ row }">
                <div class="character__name-cell">
                  <strong class="character__name">{{ row.character_name }}</strong>
                  <span v-if="row.alias_text" class="character__alias">{{ row.alias_text }}</span>
                </div>
              </template>
            </ElTableColumn>
            <ElTableColumn label="类型" width="92">
              <template #default="{ row }">
                <span
                  class="nw-badge"
                  :class="`nw-badge--${roleTypeBadge(row.role_type_code)}`"
                >
                  {{ roleTypeLabel(row.role_type_code) }}
                </span>
              </template>
            </ElTableColumn>
            <ElTableColumn label="描述" min-width="260">
              <template #default="{ row }">
                <ElTooltip
                  :content="describeCharacter(asRow<NWCharacterVo>(row))"
                  placement="top"
                  :show-after="300"
                  popper-class="character__tip"
                >
                  <span class="character__desc">{{ describeCharacter(asRow<NWCharacterVo>(row)) }}</span>
                </ElTooltip>
              </template>
            </ElTableColumn>
            <ElTableColumn label="目标动机" min-width="160">
              <template #default="{ row }">
                <span class="character__goal">{{ row.goal_text || '—' }}</span>
              </template>
            </ElTableColumn>
            <ElTableColumn label="操作" width="140" fixed="right">
              <template #default="{ row }">
                <ElButton size="small" text type="primary" @click="formDrawer?.open(asRow<NWCharacterVo>(row))">
                  编辑
                </ElButton>
                <ElButton size="small" text type="danger" @click="removeCharacter(asRow<NWCharacterVo>(row))">
                  删除
                </ElButton>
              </template>
            </ElTableColumn>
            <template #empty>
              <ElEmpty
                :description="projectUlid ? '还没有角色，先创建主角与反派' : '请先在顶栏选择项目'"
                :image-size="80"
              />
            </template>
          </ElTable>
        </ElTabPane>

        <ElTabPane name="graph">
          <template #label>
            <span class="character__tab-label">
              <Icon icon="mdi:file-tree" :size="14" />
              <span>关系图谱</span>
            </span>
          </template>

          <div class="nw-toolbar character__toolbar">
            <span class="nw-hint">
              节点按角色类型着色，拖拽调整位置，双击连线删除关系。
            </span>
            <div class="character__spacer" />
            <ElButton size="small" plain :loading="graphLoading" @click="loadGraph">
              <Icon icon="mdi:reload" :size="14" />
              <span class="ml-4px">刷新</span>
            </ElButton>
            <ElButton
              type="primary"
              plain
              size="small"
              :disabled="!projectUlid"
              @click="openRelationDialog"
            >
              <Icon icon="mdi:plus" :size="14" />
              <span class="ml-4px">新建关系</span>
            </ElButton>
          </div>

          <div v-loading="graphLoading" class="character__graph">
            <CharacterGraphPanel
              v-if="graphData"
              :graph-data="graphData"
              :relation-type-label-map="relationTypeLabelMap"
              @changed="refreshAll"
            />
            <ElEmpty
              v-else-if="!graphLoading"
              description="还没有角色关系数据，先创建角色与关系"
              :image-size="90"
            />
          </div>
        </ElTabPane>
      </ElTabs>
    </ElCard>

    <CharacterFormDrawer
      ref="formDrawer"
      :project-ulid="projectUlid"
      @saved="refreshAll"
    />
    <RelationFormDialog
      ref="relationDialog"
      :project-ulid="projectUlid"
      :node-list="graphNodeList"
      @saved="refreshAll"
    />
  </div>
</template>

<style lang="less" scoped>
  .character {
    &__card {
      :deep(.el-card__body) {
        padding: 4px 16px 16px;
      }
    }

    &__tabs {
      :deep(.el-tabs__header) {
        margin-bottom: 14px;
      }
    }

    &__tab-label {
      display: inline-flex;
      gap: 6px;
      align-items: center;
    }

    &__toolbar {
      margin-bottom: 12px;
    }

    &__search {
      width: 220px;
    }

    &__role-filter {
      width: 130px;
    }

    &__spacer {
      flex: 1;
    }

    &__table {
      width: 100%;
    }

    &__name-cell {
      display: flex;
      flex-direction: column;
      gap: 2px;
    }

    &__name {
      font-size: 13px;
      font-weight: 650;
      color: var(--nw-ink);
    }

    &__alias {
      font-size: 11.5px;
      color: var(--nw-text-muted);
    }

    &__desc,
    &__goal {
      display: block;
      overflow: hidden;
      font-size: 12.5px;
      color: var(--el-text-color-regular);
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    &__goal {
      color: var(--nw-text-secondary);
    }

    &__graph {
      min-height: 320px;
    }
  }

  :deep(.character__tip) {
    max-width: 360px;
    line-height: 1.6;
  }
</style>

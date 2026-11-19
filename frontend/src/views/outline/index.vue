<!-- 创作规划页：左侧大纲节点树，右侧所选节点的详细场景卡片列表。 -->
<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import {
  deleteDetailedScene,
  deleteOutlineNode,
  listDetailedScenes,
  listOutlineTree,
  saveDetailedScene,
  saveOutlineNode,
  type NWDetailedSceneVo,
  type NWOutlineNodeTypeCode,
  type NWOutlineNodeStatusCode,
  type NWOutlineNodeVo,
  type NWOutlinePaceCode,
} from '@/api/outline'
import { listCharacters, type NWCharacterVo } from '@/api/character'
import NWButton from '@/components/NWButton.vue'
import NWIcon from '@/components/NWIcon.vue'
import NWPageHeader from '@/components/NWPageHeader.vue'
import NWCard from '@/components/NWCard.vue'
import NWEmpty from '@/components/NWEmpty.vue'
import NWLoading from '@/components/NWLoading.vue'
import ProjectPageHeader from '@/components/ProjectPageHeader.vue'
import { nodeTypeLabel, nodeStatusLabel, paceLabel, nodeTypeOptionList, nodeStatusOptionList, paceOptionList } from './outlineOptions'
import { useProjectStore } from '@/stores/project'
import './outline.css'

const { t } = useI18n()
const route = useRoute()
const projectStore = useProjectStore()

/** 项目上下文：优先路由参数，缺省回退到 Pinia 项目 store。 */
const projectUlid = computed(
  () => String(route.params.project_ulid ?? '') || projectStore.current_project_ulid || '',
)

const outlineTree = ref<NWOutlineNodeVo[]>([])
const treeLoading = ref(false)
const selectedNodeUlid = ref('')
const sceneList = ref<NWDetailedSceneVo[]>([])
const sceneLoading = ref(false)
const characterList = ref<NWCharacterVo[]>([])

const nodeDialogVisible = ref(false)
const nodeSaving = ref(false)
const nodeFormRef = ref<FormInstance>()
const editingNodeUlid = ref('')
const nodeForm = reactive({
  parent_outline_ulid: '',
  node_type_code: 'chapter' as NWOutlineNodeTypeCode,
  title_text: '',
  summary_text: '',
  node_status_code: 'draft' as NWOutlineNodeStatusCode,
  sort_order: 0,
})

const nodeFormRules: FormRules = {
  title_text: [
    { required: true, message: t('outline.titleLabel'), trigger: 'blur' },
    { max: 128, message: t('outline.titleLabel') + ' ≤ 128', trigger: 'blur' },
  ],
}

const sceneDialogVisible = ref(false)
const sceneSaving = ref(false)
const sceneFormRef = ref<FormInstance>()
const editingSceneUlid = ref('')
const sceneForm = reactive({
  character_ulid_list: [] as string[],
  location_text: '',
  conflict_text: '',
  pace_code: '' as NWOutlinePaceCode | '',
  estimated_words: 0,
  entry_state_text: '',
  goal_text: '',
  key_choice_text: '',
  outcome_text: '',
  exit_state_text: '',
  sort_order: 0,
})

const sceneFormRules: FormRules = {
  location_text: [{ max: 256, message: t('outline.location') + ' ≤ 256', trigger: 'blur' }],
}

/** 角色 ULID 列表转姓名列表 */
function characterNames(characterUlidList: string[]): string {
  if (!characterUlidList || characterUlidList.length === 0) return '—'
  return characterUlidList
    .map((characterUlid) => characterList.value.find((item) => item.character_ulid === characterUlid)?.character_name ?? characterUlid)
    .join('、')
}

/** 当前选中的节点 */
const selectedNode = computed(() => findNode(outlineTree.value, selectedNodeUlid.value))

function findNode(nodes: NWOutlineNodeVo[], outlineUlid: string): NWOutlineNodeVo | null {
  for (const node of nodes) {
    if (node.outline_ulid === outlineUlid) return node
    const matched = findNode(node.children ?? [], outlineUlid)
    if (matched) return matched
  }
  return null
}

/** 加载大纲树并确保选中项仍然有效 */
async function loadTree(): Promise<void> {
  if (!projectUlid.value) return
  treeLoading.value = true
  try {
    outlineTree.value = await listOutlineTree(projectUlid.value)
    if (!findNode(outlineTree.value, selectedNodeUlid.value)) {
      selectedNodeUlid.value = outlineTree.value[0]?.outline_ulid ?? ''
    }
  } catch {
    outlineTree.value = []
  } finally {
    treeLoading.value = false
  }
}

/** 加载所选节点的场景列表 */
async function loadScenes(): Promise<void> {
  if (!projectUlid.value || !selectedNodeUlid.value) {
    sceneList.value = []
    return
  }
  sceneLoading.value = true
  try {
    sceneList.value = await listDetailedScenes(projectUlid.value, selectedNodeUlid.value)
  } catch {
    sceneList.value = []
  } finally {
    sceneLoading.value = false
  }
}

/** 打开节点新增/编辑弹窗 */
function openNodeDialog(node?: NWOutlineNodeVo, parentUlid = ''): void {
  editingNodeUlid.value = node?.outline_ulid ?? ''
  nodeForm.parent_outline_ulid = node?.parent_outline_ulid ?? parentUlid
  nodeForm.node_type_code = node?.node_type_code ?? 'chapter'
  nodeForm.title_text = node?.title_text ?? ''
  nodeForm.summary_text = node?.summary_text ?? ''
  nodeForm.node_status_code = node?.node_status_code ?? 'draft'
  nodeForm.sort_order = node?.sort_order ?? 0
  nodeDialogVisible.value = true
}

/** 提交节点保存 */
async function submitNode(): Promise<void> {
  if (!projectUlid.value || !nodeFormRef.value) return
  try {
    await nodeFormRef.value.validate()
  } catch {
    return
  }
  nodeSaving.value = true
  try {
    const saved = await saveOutlineNode({
      outline_ulid: editingNodeUlid.value || undefined,
      project_ulid: projectUlid.value,
      parent_outline_ulid: nodeForm.parent_outline_ulid,
      node_type_code: nodeForm.node_type_code,
      title_text: nodeForm.title_text.trim(),
      summary_text: nodeForm.summary_text,
      node_status_code: nodeForm.node_status_code,
      sort_order: nodeForm.sort_order,
    })
    nodeDialogVisible.value = false
    selectedNodeUlid.value = saved.outline_ulid
    await loadTree()
    ElMessage.success(t('outline.nodeSaved'))
  } catch {
    /* 错误提示由 http 层统一处理 */
  } finally {
    nodeSaving.value = false
  }
}

/** 删除节点（存在子节点时后端拒绝） */
async function removeNode(node: NWOutlineNodeVo): Promise<void> {
  if (!projectUlid.value) return
  try {
    await ElMessageBox.confirm(t('outline.deleteNodeConfirm', { name: node.title_text }), t('common.confirm'), {
      type: 'warning',
    })
  } catch {
    return
  }
  try {
    await deleteOutlineNode(projectUlid.value, node.outline_ulid)
    if (selectedNodeUlid.value === node.outline_ulid) selectedNodeUlid.value = ''
    await loadTree()
    ElMessage.success(t('outline.nodeDeleted'))
  } catch {
    /* 错误提示由 http 层统一处理 */
  }
}

/** 打开场景新增/编辑弹窗 */
function openSceneDialog(scene?: NWDetailedSceneVo): void {
  editingSceneUlid.value = scene?.scene_ulid ?? ''
  sceneForm.character_ulid_list = scene?.character_ulid_list ?? []
  sceneForm.location_text = scene?.location_text ?? ''
  sceneForm.conflict_text = scene?.conflict_text ?? ''
  sceneForm.pace_code = scene?.pace_code ?? ''
  sceneForm.estimated_words = scene?.estimated_words ?? 0
  sceneForm.entry_state_text = scene?.entry_state_text ?? ''
  sceneForm.goal_text = scene?.goal_text ?? ''
  sceneForm.key_choice_text = scene?.key_choice_text ?? ''
  sceneForm.outcome_text = scene?.outcome_text ?? ''
  sceneForm.exit_state_text = scene?.exit_state_text ?? ''
  sceneForm.sort_order = scene?.sort_order ?? 0
  sceneDialogVisible.value = true
}

/** 提交场景保存 */
async function submitScene(): Promise<void> {
  if (!projectUlid.value || !selectedNodeUlid.value || !sceneFormRef.value) return
  try {
    await sceneFormRef.value.validate()
  } catch {
    return
  }
  sceneSaving.value = true
  try {
    await saveDetailedScene({
      scene_ulid: editingSceneUlid.value || undefined,
      project_ulid: projectUlid.value,
      outline_ulid: selectedNodeUlid.value,
      character_ulid_list: sceneForm.character_ulid_list,
      location_text: sceneForm.location_text,
      conflict_text: sceneForm.conflict_text,
      pace_code: sceneForm.pace_code,
      estimated_words: sceneForm.estimated_words,
      entry_state_text: sceneForm.entry_state_text,
      goal_text: sceneForm.goal_text,
      key_choice_text: sceneForm.key_choice_text,
      outcome_text: sceneForm.outcome_text,
      exit_state_text: sceneForm.exit_state_text,
      sort_order: sceneForm.sort_order,
    })
    sceneDialogVisible.value = false
    await loadScenes()
    ElMessage.success(t('outline.sceneSaved'))
  } catch {
    /* 错误提示由 http 层统一处理 */
  } finally {
    sceneSaving.value = false
  }
}

/** 删除场景卡片 */
async function removeScene(scene: NWDetailedSceneVo): Promise<void> {
  if (!projectUlid.value) return
  try {
    await ElMessageBox.confirm(t('outline.deleteSceneConfirm'), t('common.confirm'), { type: 'warning' })
  } catch {
    return
  }
  try {
    await deleteDetailedScene(projectUlid.value, scene.scene_ulid)
    await loadScenes()
    ElMessage.success(t('outline.sceneDeleted'))
  } catch {
    /* 错误提示由 http 层统一处理 */
  }
}

/** 选择节点后刷新场景列表 */
function handleNodeSelect(data: NWOutlineNodeVo): void {
  selectedNodeUlid.value = data.outline_ulid
}

watch(projectUlid, () => {
  selectedNodeUlid.value = ''
  void loadTree()
})

watch(selectedNodeUlid, () => {
  void loadScenes()
})

onMounted(() => {
  void loadTree()
  if (projectUlid.value) {
    void listCharacters({ project_ulid: projectUlid.value }).then((listData) => {
      characterList.value = Array.isArray(listData) ? listData : []
    }).catch(() => {
      characterList.value = []
    })
  }
})
</script>

<template>
  <div class="nw-page outline-page">
    <ProjectPageHeader :project-ulid="projectUlid" />

    <NWPageHeader
       :title_text="t('outline.title')"
       :subtitle_text="t('outline.subtitle')"
      icon_code="tree"
    >
      <template #actions>
        <NWButton type="primary" @click="openNodeDialog(undefined, '')">
          <NWIcon icon_code="plus" :size_px="15" />
          {{ t('outline.addRoot') }}
        </NWButton>
      </template>
    </NWPageHeader>

    <div class="outline-page__body">
      <aside class="outline-page__side">
        <NWCard :title="t('outline.structureTitle')" :hoverable="false" :loading="treeLoading">
          <el-tree
            v-if="outlineTree.length"
            :data="outlineTree"
            node-key="outline_ulid"
            :props="{ children: 'children', label: 'title_text' }"
            :current-node-key="selectedNodeUlid"
            :expand-on-click-node="false"
            highlight-current
            @node-click="handleNodeSelect"
          >
            <template #default="{ data }">
              <div class="outline-tree-node">
                <span class="outline-tree-node__title">{{ data.title_text }}</span>
                <span class="outline-tree-node__type">{{ nodeTypeLabel(data.node_type_code, t) }}</span>
                <span class="outline-tree-node__status">{{ nodeStatusLabel(data.node_status_code, t) }}</span>
                <span class="outline-tree-node__actions">
                  <NWButton size="small" @click.stop="openNodeDialog(data)">编辑</NWButton>
                  <NWButton size="small" @click.stop="openNodeDialog(undefined, data.outline_ulid)">
                    子节点
                  </NWButton>
                  <NWButton type="danger-text" size="small" @click.stop="removeNode(data)">删除</NWButton>
                </span>
              </div>
            </template>
          </el-tree>
          <NWEmpty
            v-else
            compact_flag
            icon_code="tree"
             :title_text="t('outline.emptyNodeTitle')"
             :description_text="t('outline.emptyNodeDesc')"
          >
            <template #actions>
              <NWButton size="small" type="primary" @click="openNodeDialog(undefined, '')">{{ t('outline.addRoot') }}</NWButton>
            </template>
          </NWEmpty>
        </NWCard>
      </aside>

      <section class="outline-page__main">
        <NWCard :title="selectedNode ? `${selectedNode.title_text} · ${t('outline.sceneTitle')}` : t('outline.detailScene')" :hoverable="false">
          <div class="outline-scene-toolbar">
            <span v-if="selectedNode" class="outline-scene-toolbar__hint">
              {{ nodeTypeLabel(selectedNode.node_type_code, t) }} · {{ selectedNode.summary_text || t('outline.noSummary') }}
            </span>
            <NWButton v-if="selectedNode" type="primary" size="small" @click="openSceneDialog()">
              + {{ t('outline.addScene') }}
            </NWButton>
          </div>
          <NWLoading v-if="sceneLoading" />
          <template v-else-if="sceneList.length">
            <div v-for="scene in sceneList" :key="scene.scene_ulid" class="outline-scene-card">
              <div class="outline-scene-card__head">
                <strong>{{ scene.location_text || t('outline.noLocation') }}</strong>
                <span class="outline-scene-card__pace">{{ t('outline.pace') }}：{{ scene.pace_code ? paceLabel(scene.pace_code, t) : '—' }}</span>
                <span class="outline-scene-card__words">预计 {{ scene.estimated_words }} 字</span>
                <span class="outline-scene-card__actions">
                  <NWButton size="small" @click="openSceneDialog(scene)">编辑</NWButton>
                  <NWButton type="danger-text" size="small" @click="removeScene(scene)">删除</NWButton>
                </span>
              </div>
              <div class="outline-scene-card__grid">
                <div><span class="outline-scene-card__label">出场角色</span>{{ characterNames(scene.character_ulid_list) }}</div>
                <div><span class="outline-scene-card__label">核心冲突</span>{{ scene.conflict_text || '—' }}</div>
                <div><span class="outline-scene-card__label">入场状态</span>{{ scene.entry_state_text || '—' }}</div>
                <div><span class="outline-scene-card__label">角色目标</span>{{ scene.goal_text || '—' }}</div>
                <div><span class="outline-scene-card__label">关键选择</span>{{ scene.key_choice_text || '—' }}</div>
                <div><span class="outline-scene-card__label">场景结果</span>{{ scene.outcome_text || '—' }}</div>
                <div><span class="outline-scene-card__label">出场状态</span>{{ scene.exit_state_text || '—' }}</div>
              </div>
            </div>
          </template>
          <NWEmpty
            v-else
            compact_flag
            icon_code="layers"
            :title_text="selectedNode ? t('outline.noSceneTitle') : t('outline.noNodeTitle')"
            :description_text="selectedNode ? t('outline.noSceneDesc') : t('outline.noNodeDesc')"
          >
            <template #actions>
              <NWButton v-if="selectedNode" size="small" type="primary" @click="openSceneDialog()">{{ t('outline.addScene') }}</NWButton>
            </template>
          </NWEmpty>
        </NWCard>
      </section>
    </div>

    <el-dialog
      v-model="nodeDialogVisible"
      :title="editingNodeUlid ? t('outline.editNode') : t('outline.createNode')"
      width="520px"
    >
      <el-form ref="nodeFormRef" :model="nodeForm" :rules="nodeFormRules" label-position="top">
        <el-form-item :label="t('outline.nodeType')">
          <el-select v-model="nodeForm.node_type_code">
            <el-option v-for="option in nodeTypeOptionList" :key="option.value" :label="t(option.label_key)" :value="option.value" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('outline.titleLabel')" prop="title_text">
          <el-input v-model="nodeForm.title_text" maxlength="128" show-word-limit />
        </el-form-item>
        <el-form-item :label="t('outline.summaryLabel')">
          <el-input v-model="nodeForm.summary_text" type="textarea" :rows="3" maxlength="4096" />
        </el-form-item>
        <el-form-item :label="t('outline.statusLabel')">
          <el-select v-model="nodeForm.node_status_code">
            <el-option v-for="option in nodeStatusOptionList" :key="option.value" :label="t(option.label_key)" :value="option.value" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('outline.sortOrder')">
          <el-input-number v-model="nodeForm.sort_order" :min="0" :max="9999" />
        </el-form-item>
      </el-form>
      <template #footer>
        <NWButton @click="nodeDialogVisible = false">{{ t('common.cancel') }}</NWButton>
        <NWButton type="primary" :loading="nodeSaving" @click="submitNode">{{ t('common.save') }}</NWButton>
      </template>
    </el-dialog>

    <el-dialog
      v-model="sceneDialogVisible"
      :title="editingSceneUlid ? t('outline.editScene') : t('outline.createScene')"
      width="640px"
    >
      <el-form ref="sceneFormRef" :model="sceneForm" :rules="sceneFormRules" label-position="top">
        <el-form-item :label="t('outline.location')">
          <el-input v-model="sceneForm.location_text" maxlength="256" />
        </el-form-item>
        <el-form-item :label="t('outline.characters')">
          <el-select v-model="sceneForm.character_ulid_list" multiple filterable class="outline-form-control" :placeholder="t('outline.selectCharacters')">
            <el-option
              v-for="characterVo in characterList"
              :key="characterVo.character_ulid"
              :label="characterVo.character_name"
              :value="characterVo.character_ulid"
            />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('outline.conflict')">
          <el-input v-model="sceneForm.conflict_text" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item :label="t('outline.pace')">
          <el-select v-model="sceneForm.pace_code" clearable :placeholder="t('outline.selectPace')">
            <el-option v-for="option in paceOptionList" :key="option.value" :label="t(option.label_key)" :value="option.value" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('outline.estimatedWords')">
          <el-input-number v-model="sceneForm.estimated_words" :min="0" :max="1000000" />
        </el-form-item>
        <el-form-item :label="t('outline.entryState')">
          <el-input v-model="sceneForm.entry_state_text" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item :label="t('outline.goal')">
          <el-input v-model="sceneForm.goal_text" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item :label="t('outline.keyChoice')">
          <el-input v-model="sceneForm.key_choice_text" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item :label="t('outline.outcome')">
          <el-input v-model="sceneForm.outcome_text" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item :label="t('outline.exitState')">
          <el-input v-model="sceneForm.exit_state_text" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item :label="t('outline.sortOrder')">
          <el-input-number v-model="sceneForm.sort_order" :min="0" :max="9999" />
        </el-form-item>
      </el-form>
      <template #footer>
        <NWButton @click="sceneDialogVisible = false">{{ t('common.cancel') }}</NWButton>
        <NWButton type="primary" :loading="sceneSaving" @click="submitScene">{{ t('common.save') }}</NWButton>
      </template>
    </el-dialog>
  </div>
</template>

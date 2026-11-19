<!--
  创作规划页：左侧大纲节点树（卷/篇章/情节块/章节），右侧所选节点的详细场景卡片列表。
  节点按父子关系递归展示；删除节点时若仍有子节点，由后端拒绝并弹出提示。
-->
<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
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
import NWButton from '@/components/NWButton.vue'
import NWCard from '@/components/NWCard.vue'
import NWLoading from '@/components/NWLoading.vue'
import ProjectPageHeader from '@/components/ProjectPageHeader.vue'
import { useProjectStore } from '@/stores/project'
import './outline.css'

const route = useRoute()
const projectStore = useProjectStore()

/** 项目上下文：优先路由参数，缺省回退到 Pinia 项目 store。 */
const projectUlid = computed(
  () => String(route.params.project_ulid ?? '') || projectStore.current_project_ulid || '',
)

/** 大纲节点类型选项 */
const nodeTypeOptions: Array<{ value: NWOutlineNodeTypeCode; label: string }> = [
  { value: 'volume', label: '卷' },
  { value: 'arc', label: '篇章' },
  { value: 'storyBlock', label: '情节块' },
  { value: 'chapter', label: '章节' },
]

const nodeStatusOptions: Array<{ value: NWOutlineNodeStatusCode; label: string }> = [
  { value: 'draft', label: '草稿' },
  { value: 'planning', label: '规划中' },
  { value: 'ready', label: '待写' },
  { value: 'done', label: '完成' },
]

/** 节奏选项 */
const paceOptions: Array<{ value: NWOutlinePaceCode; label: string }> = [
  { value: 'slow', label: '舒缓' },
  { value: 'medium', label: '中等' },
  { value: 'fast', label: '紧凑' },
  { value: 'climax', label: '高潮' },
]

const outlineTree = ref<NWOutlineNodeVo[]>([])
const treeLoading = ref(false)
const selectedNodeUlid = ref('')
const sceneList = ref<NWDetailedSceneVo[]>([])
const sceneLoading = ref(false)

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

const sceneDialogVisible = ref(false)
const sceneSaving = ref(false)
const sceneFormRef = ref<FormInstance>()
const editingSceneUlid = ref('')
const sceneForm = reactive({
  character_text: '',
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

const nodeFormRules: FormRules = {
  title_text: [
    { required: true, message: '请输入节点标题', trigger: 'blur' },
    { max: 128, message: '标题最多 128 个字符', trigger: 'blur' },
  ],
}

const sceneFormRules: FormRules = {
  location_text: [{ max: 256, message: '地点最多 256 个字符', trigger: 'blur' }],
}

/** 展示节点类型文案 */
function nodeTypeLabel(nodeTypeCode: string): string {
  return nodeTypeOptions.find((option) => option.value === nodeTypeCode)?.label ?? nodeTypeCode
}

/** 展示节点状态文案 */
function nodeStatusLabel(statusCode: string): string {
  return nodeStatusOptions.find((option) => option.value === statusCode)?.label ?? statusCode
}

/** 展示节奏文案 */
function paceLabel(paceCode: string): string {
  return paceOptions.find((option) => option.value === paceCode)?.label ?? paceCode
}

/** 当前选中的节点 */
const selectedNode = computed(() => findNode(outlineTree.value, selectedNodeUlid.value))

/** 递归查找节点 */
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
    ElMessage.success('大纲节点已保存')
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
    await ElMessageBox.confirm(`确定删除节点“${node.title_text}”吗？`, '删除确认', {
      type: 'warning',
    })
  } catch {
    return
  }
  try {
    await deleteOutlineNode(projectUlid.value, node.outline_ulid)
    if (selectedNodeUlid.value === node.outline_ulid) selectedNodeUlid.value = ''
    await loadTree()
    ElMessage.success('大纲节点已删除')
  } catch {
    /* 错误提示由 http 层统一处理 */
  }
}

/** 打开场景新增/编辑弹窗 */
function openSceneDialog(scene?: NWDetailedSceneVo): void {
  editingSceneUlid.value = scene?.scene_ulid ?? ''
  sceneForm.character_text = (scene?.character_ulid_list ?? []).join('\n')
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
      character_ulid_list: sceneForm.character_text
        .split(/[\n,，\s]+/)
        .map((item) => item.trim())
        .filter(Boolean),
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
    ElMessage.success('场景卡片已保存')
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
    await ElMessageBox.confirm('确定删除该场景卡片吗？', '删除确认', { type: 'warning' })
  } catch {
    return
  }
  try {
    await deleteDetailedScene(projectUlid.value, scene.scene_ulid)
    await loadScenes()
    ElMessage.success('场景卡片已删除')
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
})
</script>

<template>
  <div class="outline-page">
    <ProjectPageHeader :project-ulid="projectUlid" />

    <div class="outline-page__body">
      <aside class="outline-page__side">
        <NWCard title="创作大纲" :hoverable="false" :loading="treeLoading">
          <div class="outline-tree-toolbar">
            <NWButton type="primary" size="small" @click="openNodeDialog(undefined, '')">
              + 新增根节点
            </NWButton>
          </div>
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
                <span class="outline-tree-node__type">{{ nodeTypeLabel(data.node_type_code) }}</span>
                <span class="outline-tree-node__status">{{ nodeStatusLabel(data.node_status_code) }}</span>
                <span class="outline-tree-node__actions">
                  <NWButton size="small" @click.stop="openNodeDialog(data)">编辑</NWButton>
                  <NWButton size="small" @click.stop="openNodeDialog(undefined, data.outline_ulid)">
                    子节点
                  </NWButton>
                  <NWButton type="danger" size="small" @click.stop="removeNode(data)">删除</NWButton>
                </span>
              </div>
            </template>
          </el-tree>
          <NWEmpty v-else description-text="暂无大纲节点，先新增一个根节点吧" />
        </NWCard>
      </aside>

      <section class="outline-page__main">
        <NWCard :title="selectedNode ? `${selectedNode.title_text} · 场景` : '详细场景'" :hoverable="false">
          <div class="outline-scene-toolbar">
            <span v-if="selectedNode" class="outline-scene-toolbar__hint">
              {{ nodeTypeLabel(selectedNode.node_type_code) }} · {{ selectedNode.summary_text || '暂无概要' }}
            </span>
            <NWButton
              type="primary"
              size="small"
              :disabled="!selectedNode"
              @click="openSceneDialog()"
            >
              + 新增场景
            </NWButton>
          </div>
          <NWLoading v-if="sceneLoading" />
          <template v-else-if="sceneList.length">
            <div v-for="scene in sceneList" :key="scene.scene_ulid" class="outline-scene-card">
              <div class="outline-scene-card__head">
                <strong>{{ scene.location_text || '未填写地点' }}</strong>
                <span class="outline-scene-card__pace">节奏：{{ paceLabel(scene.pace_code) }}</span>
                <span class="outline-scene-card__words">预计 {{ scene.estimated_words }} 字</span>
                <span class="outline-scene-card__actions">
                  <NWButton size="small" @click="openSceneDialog(scene)">编辑</NWButton>
                  <NWButton type="danger" size="small" @click="removeScene(scene)">删除</NWButton>
                </span>
              </div>
              <div class="outline-scene-card__grid">
                <div><span class="outline-scene-card__label">出场角色</span>{{ scene.character_ulid_list.join('、') || '—' }}</div>
                <div><span class="outline-scene-card__label">核心冲突</span>{{ scene.conflict_text || '—' }}</div>
                <div><span class="outline-scene-card__label">入场状态</span>{{ scene.entry_state_text || '—' }}</div>
                <div><span class="outline-scene-card__label">角色目标</span>{{ scene.goal_text || '—' }}</div>
                <div><span class="outline-scene-card__label">关键选择</span>{{ scene.key_choice_text || '—' }}</div>
                <div><span class="outline-scene-card__label">场景结果</span>{{ scene.outcome_text || '—' }}</div>
                <div><span class="outline-scene-card__label">出场状态</span>{{ scene.exit_state_text || '—' }}</div>
              </div>
            </div>
          </template>
          <NWEmpty v-else :description-text="selectedNode ? '该节点还没有场景卡片' : '请先在左侧选择一个大纲节点'" />
        </NWCard>
      </section>
    </div>

    <el-dialog
      v-model="nodeDialogVisible"
      :title="editingNodeUlid ? '编辑大纲节点' : '新增大纲节点'"
      width="520px"
    >
      <el-form ref="nodeFormRef" :model="nodeForm" :rules="nodeFormRules" label-position="top">
        <el-form-item label="节点类型">
          <el-select v-model="nodeForm.node_type_code">
            <el-option v-for="option in nodeTypeOptions" :key="option.value" :label="option.label" :value="option.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="标题" prop="title_text">
          <el-input v-model="nodeForm.title_text" maxlength="128" show-word-limit />
        </el-form-item>
        <el-form-item label="概要">
          <el-input v-model="nodeForm.summary_text" type="textarea" :rows="3" maxlength="4096" />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="nodeForm.node_status_code">
            <el-option v-for="option in nodeStatusOptions" :key="option.value" :label="option.label" :value="option.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="排序号">
          <el-input-number v-model="nodeForm.sort_order" :min="0" :max="9999" />
        </el-form-item>
      </el-form>
      <template #footer>
        <NWButton @click="nodeDialogVisible = false">取消</NWButton>
        <NWButton type="primary" :loading="nodeSaving" @click="submitNode">保存</NWButton>
      </template>
    </el-dialog>

    <el-dialog
      v-model="sceneDialogVisible"
      :title="editingSceneUlid ? '编辑场景卡片' : '新增场景卡片'"
      width="640px"
    >
      <el-form ref="sceneFormRef" :model="sceneForm" :rules="sceneFormRules" label-position="top">
        <el-form-item label="地点">
          <el-input v-model="sceneForm.location_text" maxlength="256" />
        </el-form-item>
        <el-form-item label="出场角色 ULID（每行一个或逗号分隔）">
          <el-input v-model="sceneForm.character_text" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item label="核心冲突">
          <el-input v-model="sceneForm.conflict_text" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item label="节奏">
          <el-select v-model="sceneForm.pace_code" clearable placeholder="请选择">
            <el-option v-for="option in paceOptions" :key="option.value" :label="option.label" :value="option.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="预计字数">
          <el-input-number v-model="sceneForm.estimated_words" :min="0" :max="1000000" />
        </el-form-item>
        <el-form-item label="入场状态">
          <el-input v-model="sceneForm.entry_state_text" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item label="角色目标">
          <el-input v-model="sceneForm.goal_text" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item label="关键选择">
          <el-input v-model="sceneForm.key_choice_text" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item label="场景结果">
          <el-input v-model="sceneForm.outcome_text" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item label="出场状态">
          <el-input v-model="sceneForm.exit_state_text" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item label="排序号">
          <el-input-number v-model="sceneForm.sort_order" :min="0" :max="9999" />
        </el-form-item>
      </el-form>
      <template #footer>
        <NWButton @click="sceneDialogVisible = false">取消</NWButton>
        <NWButton type="primary" :loading="sceneSaving" @click="submitScene">保存</NWButton>
      </template>
    </el-dialog>
  </div>
</template>

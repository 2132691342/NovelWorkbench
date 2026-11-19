<!--
  角色关系图：AntV X6 画布（节点=角色按角色类型配色，边=关系带类型标签）。
  节点可拖拽；点边高亮并支持删除；「新建关系」对话框；工具条提供缩放/自适应/刷新。
  X6 通过动态 import 按需懒加载（vite 已配 x6 分包），卸载时 dispose 防内存泄漏。
-->
<script setup lang="ts">
import { onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { useI18n } from 'vue-i18n'
import type { Graph } from '@antv/x6'
import {
  deleteCharacterRelation,
  getCharacterGraph,
  saveCharacterRelation,
  type NWCharacterEdgeVo,
  type NWCharacterNodeVo,
} from '@/api/character'
import NWButton from '@/components/NWButton.vue'
import NWEmpty from '@/components/NWEmpty.vue'
import NWLoading from '@/components/NWLoading.vue'

const props = defineProps<{
  projectUlid: string
}>()

const { t } = useI18n()

/** 关系类型选项 */
const relationTypeOptions = ['friend', 'enemy', 'lover', 'kin', 'teacher', 'rival', 'other']

/** 角色类型 → 节点配色 */
const roleColorMap: Record<string, string> = {
  protagonist: '#6366f1',
  antagonist: '#ef4444',
  supporting: '#10b981',
  minor: '#64748b',
}

const graphHost = ref<HTMLDivElement | null>(null)
const loading = ref(false)
const nodeList = ref<NWCharacterNodeVo[]>([])
const edgeList = ref<NWCharacterEdgeVo[]>([])
const selectedEdge = ref<NWCharacterEdgeVo | null>(null)
const relationDialogVisible = ref(false)
const relationSaving = ref(false)

const relationFormRef = ref<FormInstance | null>(null)
const relationForm = reactive({
  from_character_ulid: '',
  to_character_ulid: '',
  relation_type_code: 'friend',
  relation_note: '',
})

const relationRules: FormRules = {
  from_character_ulid: [
    { required: true, message: () => t('character.relation.ruleFromRequired'), trigger: 'change' },
  ],
  to_character_ulid: [
    { required: true, message: () => t('character.relation.ruleToRequired'), trigger: 'change' },
  ],
  relation_type_code: [
    { required: true, message: () => t('character.relation.ruleTypeRequired'), trigger: 'change' },
  ],
}

let graphInstance: Graph | null = null
let disposedFlag = false
let resizeObserver: ResizeObserver | null = null // 画布尺寸就绪监听（隐藏挂载场景补适配）
let needsFit = false // 是否有待补的自适应

/** 关系类型展示文案 */
function relationTypeText(relationTypeCode: string): string {
  switch (relationTypeCode) {
    case 'friend':
      return t('character.relation.typeFriend')
    case 'enemy':
      return t('character.relation.typeEnemy')
    case 'lover':
      return t('character.relation.typeLover')
    case 'kin':
      return t('character.relation.typeKin')
    case 'teacher':
      return t('character.relation.typeTeacher')
    case 'rival':
      return t('character.relation.typeRival')
    default:
      return t('character.relation.typeOther')
  }
}

/** 取消边高亮 */
function unhighlightEdge(edgeVo: NWCharacterEdgeVo | null): void {
  if (!edgeVo || !graphInstance) {
    return
  }
  const cell = graphInstance.getCellById(edgeVo.relation_ulid)
  if (cell && cell.isEdge()) {
    cell.attr('line/stroke', '#94a3b8')
    cell.attr('line/strokeWidth', 1.5)
  }
}

/** 边高亮 */
function highlightEdge(edgeVo: NWCharacterEdgeVo): void {
  if (!graphInstance) {
    return
  }
  const cell = graphInstance.getCellById(edgeVo.relation_ulid)
  if (cell && cell.isEdge()) {
    cell.attr('line/stroke', '#6366f1')
    cell.attr('line/strokeWidth', 2.5)
  }
}

/** 选中关系边：高亮并展示信息条 */
function selectEdge(edgeVo: NWCharacterEdgeVo): void {
  unhighlightEdge(selectedEdge.value)
  selectedEdge.value = edgeVo
  highlightEdge(edgeVo)
}

/** 清除选中 */
function clearSelection(): void {
  unhighlightEdge(selectedEdge.value)
  selectedEdge.value = null
}

/** 圆形布局坐标（避免节点叠放） */
function nodePosition(index: number, total: number): { x: number; y: number } {
  const radius = Math.max(160, total * 36)
  const angle = (2 * Math.PI * index) / Math.max(total, 1) - Math.PI / 2
  return { x: 480 + radius * Math.cos(angle), y: 320 + radius * Math.sin(angle) }
}

/** 用最新图数据重建画布节点与边 */
function renderGraph(): void {
  if (!graphInstance) {
    return
  }
  graphInstance.clearCells()
  const nodes = nodeList.value
  nodes.forEach((nodeVo, index) => {
    const position = nodePosition(index, nodes.length)
    graphInstance?.addNode({
      id: nodeVo.character_ulid,
      x: position.x,
      y: position.y,
      width: 132,
      height: 42,
      shape: 'rect',
      attrs: {
        body: {
          fill: roleColorMap[nodeVo.role_type_code] ?? '#64748b',
          stroke: 'transparent',
          rx: 10,
          ry: 10,
        },
        label: {
          text: nodeVo.character_name,
          fill: '#ffffff',
          fontSize: 13,
          textWrap: { width: 120, height: 34, ellipsis: true },
        },
      },
    })
  })
  for (const edgeVo of edgeList.value) {
    const hasFrom = nodes.some((item) => item.character_ulid === edgeVo.from_character_ulid)
    const hasTo = nodes.some((item) => item.character_ulid === edgeVo.to_character_ulid)
    if (!hasFrom || !hasTo) {
      continue
    }
    graphInstance?.addEdge({
      id: edgeVo.relation_ulid,
      source: edgeVo.from_character_ulid,
      target: edgeVo.to_character_ulid,
      router: { name: 'manhattan', args: { padding: 18 } },
      connector: { name: 'rounded', args: { radius: 8 } },
      attrs: {
        line: {
          stroke: '#94a3b8',
          strokeWidth: 1.5,
          targetMarker: { name: 'block', width: 10, height: 6 },
        },
      },
      labels: [
        {
          position: 0.5,
          attrs: {
            label: { text: relationTypeText(edgeVo.relation_type_code), fill: '#475569', fontSize: 11 },
          },
        },
      ],
    })
  }
}

/** 拉取关系图数据并重绘 */
async function loadGraph(): Promise<void> {
  if (!props.projectUlid) {
    return
  }
  loading.value = true
  try {
    const graphVo = await getCharacterGraph(props.projectUlid)
    nodeList.value = graphVo.node_list ?? []
    edgeList.value = graphVo.edge_list ?? []
    clearSelection()
    renderGraph()
    handleFit()
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    loading.value = false
  }
}

/** 工具条：放大 */
function handleZoomIn(): void {
  graphInstance?.zoom(0.2)
}

/** 工具条：缩小 */
function handleZoomOut(): void {
  graphInstance?.zoom(-0.2)
}

/** 工具条：自适应布局（缩放到可视范围）；延迟到画布实际渲染后再适配，避免挂载初期容器尺寸为 0 */
function handleFit(): void {
  if (!graphInstance || nodeList.value.length === 0) {
    return
  }
  requestAnimationFrame(() => {
    if (!graphInstance || (graphHost.value?.clientWidth ?? 0) === 0) {
      needsFit = true // 容器尚不可见：挂起，由 ResizeObserver 就绪后补适配
      return
    }
    graphInstance.resize()
    graphInstance.zoomToFit({ padding: 32, maxScale: 1.2 })
    needsFit = false
  })
}

/** 删除选中关系边（二次确认） */
async function handleDeleteEdge(): Promise<void> {
  const edgeVo = selectedEdge.value
  if (!edgeVo) {
    return
  }
  try {
    await ElMessageBox.confirm(t('character.relation.deleteConfirm'), t('common.confirm'), {
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel'),
      type: 'warning',
    })
  } catch {
    return
  }
  try {
    await deleteCharacterRelation(edgeVo.relation_ulid)
    ElMessage.success(t('character.relation.deleteSuccess'))
    await loadGraph()
  } catch {
    // 失败提示由 http 层统一弹出
  }
}

/** 打开「新建关系」对话框 */
function handleOpenRelationDialog(): void {
  relationForm.from_character_ulid = ''
  relationForm.to_character_ulid = ''
  relationForm.relation_type_code = 'friend'
  relationForm.relation_note = ''
  relationDialogVisible.value = true
  relationFormRef.value?.clearValidate()
}

/** 提交新建关系 */
async function handleSaveRelation(): Promise<void> {
  if (!relationFormRef.value) {
    return
  }
  try {
    await relationFormRef.value.validate()
  } catch {
    return
  }
  if (relationForm.from_character_ulid === relationForm.to_character_ulid) {
    ElMessage.warning(t('character.relation.ruleDistinct'))
    return
  }
  relationSaving.value = true
  try {
    await saveCharacterRelation({
      project_ulid: props.projectUlid,
      from_character_ulid: relationForm.from_character_ulid,
      to_character_ulid: relationForm.to_character_ulid,
      relation_type_code: relationForm.relation_type_code,
      relation_note: relationForm.relation_note,
    })
    ElMessage.success(t('character.relation.saveSuccess'))
    relationDialogVisible.value = false
    await loadGraph()
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    relationSaving.value = false
  }
}

/** 初始化画布（X6 懒加载；卸载竞争时放弃创建） */
async function initGraph(): Promise<void> {
  const { Graph: X6Graph } = await import('@antv/x6')
  if (disposedFlag || !graphHost.value) {
    return
  }
  graphInstance = new X6Graph({
    container: graphHost.value,
    autoResize: true,
    grid: { size: 10, visible: true },
    panning: true,
    mousewheel: { enabled: true, zoomAtMousePosition: true, modifiers: ['ctrl', 'meta'] },
  })
  graphInstance.on('edge:click', ({ edge }) => {
    const edgeVo = edgeList.value.find((item) => item.relation_ulid === edge.id)
    if (edgeVo) {
      selectEdge(edgeVo)
    }
  })
  graphInstance.on('node:click', () => {
    clearSelection()
  })
  graphInstance.on('blank:click', () => {
    clearSelection()
  })
  resizeObserver = new ResizeObserver(() => {
    if (needsFit) {
      handleFit()
    }
  })
  resizeObserver.observe(graphHost.value)
  renderGraph()
  handleFit()
}

watch(
  () => props.projectUlid,
  () => {
    void loadGraph()
  },
)

onMounted(() => {
  void initGraph()
  void loadGraph()
})

onBeforeUnmount(() => {
  disposedFlag = true
  resizeObserver?.disconnect()
  resizeObserver = null
  graphInstance?.dispose()
  graphInstance = null
})

defineExpose({ reload: loadGraph })
</script>

<template>
  <div class="character-graph">
    <!-- 工具条 -->
    <div class="character-graph__toolbar">
      <NWButton size="small" type="primary" @click="handleOpenRelationDialog">
        {{ t('character.relation.create') }}
      </NWButton>
      <NWButton size="small" @click="handleZoomOut">{{ t('character.graph.zoomOut') }}</NWButton>
      <NWButton size="small" @click="handleZoomIn">{{ t('character.graph.zoomIn') }}</NWButton>
      <NWButton size="small" @click="handleFit">{{ t('character.graph.fit') }}</NWButton>
      <NWButton size="small" @click="loadGraph">{{ t('character.graph.refresh') }}</NWButton>
    </div>

    <NWLoading v-if="loading && !graphInstance" />

    <!-- 画布 -->
    <div class="character-graph__canvas-wrap">
      <div ref="graphHost" class="character-graph__canvas" />
      <NWEmpty
        v-if="!loading && nodeList.length === 0"
        class="character-graph__empty"
        :description_text="t('character.graph.empty')"
      />

      <!-- 选中关系信息条 -->
      <div v-if="selectedEdge" class="character-graph__edge-bar">
        <span class="character-graph__edge-type">
          {{ relationTypeText(selectedEdge.relation_type_code) }}
        </span>
        <span v-if="selectedEdge.relation_note" class="character-graph__edge-note">
          {{ selectedEdge.relation_note }}
        </span>
        <span class="character-graph__edge-actions">
          <NWButton size="small" type="danger-text" @click="handleDeleteEdge">
            {{ t('character.relation.delete') }}
          </NWButton>
          <NWButton size="small" @click="clearSelection">{{ t('character.graph.deselect') }}</NWButton>
        </span>
      </div>
    </div>

    <!-- 新建关系对话框 -->
    <el-dialog
      v-model="relationDialogVisible"
      :title="t('character.relation.createTitle')"
      width="480px"
    >
      <el-form
        ref="relationFormRef"
        :model="relationForm"
        :rules="relationRules"
        label-position="top"
      >
        <el-form-item :label="t('character.relation.from')" prop="from_character_ulid">
          <el-select v-model="relationForm.from_character_ulid" :placeholder="t('character.relation.fromPlaceholder')">
            <el-option
              v-for="nodeVo in nodeList"
              :key="nodeVo.character_ulid"
              :label="nodeVo.character_name"
              :value="nodeVo.character_ulid"
            />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('character.relation.to')" prop="to_character_ulid">
          <el-select v-model="relationForm.to_character_ulid" :placeholder="t('character.relation.toPlaceholder')">
            <el-option
              v-for="nodeVo in nodeList"
              :key="nodeVo.character_ulid"
              :label="nodeVo.character_name"
              :value="nodeVo.character_ulid"
            />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('character.relation.type')" prop="relation_type_code">
          <el-select v-model="relationForm.relation_type_code">
            <el-option
              v-for="relationType in relationTypeOptions"
              :key="relationType"
              :label="relationTypeText(relationType)"
              :value="relationType"
            />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('character.relation.note')">
          <el-input
            v-model="relationForm.relation_note"
            type="textarea"
            :rows="3"
            maxlength="256"
            :placeholder="t('character.relation.notePlaceholder')"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <NWButton @click="relationDialogVisible = false">{{ t('common.cancel') }}</NWButton>
        <NWButton type="primary" :loading="relationSaving" @click="handleSaveRelation">
          {{ t('common.confirm') }}
        </NWButton>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped src="./character-graph.css"></style>

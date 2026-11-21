<!-- 创作规划页：大纲树与场景卡联动。 -->
<script setup lang="ts">
  import { computed, onMounted, ref, watch } from 'vue'
  import { ElButton, ElEmpty, ElMessage, ElMessageBox, ElScrollbar } from 'element-plus'
  import {
    deleteDetailedScene,
    deleteOutlineNode,
    listDetailedScenes,
    listOutlineTree
  } from '@/api/outline'
  import type { NWDetailedSceneVo, NWOutlineNodeVo } from '@/api/outline'
  import { useProjectStore } from '@/store/modules/project'
  import NodeDetail from './components/NodeDetail.vue'
  import OutlineNodeDialog from './components/OutlineNodeDialog.vue'
  import OutlineTree from './components/OutlineTree.vue'
  import SceneDialog from './components/SceneDialog.vue'

  const projectStore = useProjectStore()
  const projectUlid = computed(() => projectStore.currentProjectUlid)

  const treeList = ref<NWOutlineNodeVo[]>([])
  const treeLoading = ref(false)
  const selectedUlid = ref('')
  const sceneList = ref<NWDetailedSceneVo[]>([])
  const sceneLoading = ref(false)

  const nodeDialog = ref<InstanceType<typeof OutlineNodeDialog> | null>(null)
  const sceneDialog = ref<InstanceType<typeof SceneDialog> | null>(null)

  /** 把树拍平，便于按 ULID 定位选中节点 */
  const flatNodeList = computed<NWOutlineNodeVo[]>(() => {
    const result: NWOutlineNodeVo[] = []
    const walk = (nodes: NWOutlineNodeVo[]) => {
      for (const node of nodes) {
        result.push(node)
        if (node.children?.length) walk(node.children)
      }
    }
    walk(treeList.value)
    return result
  })

  const selectedNode = computed<NWOutlineNodeVo | null>(
    () => flatNodeList.value.find((node) => node.outline_ulid === selectedUlid.value) ?? null
  )

  const totalNodeCount = computed(() => flatNodeList.value.length)

  /** 加载大纲树，保留仍存在的选中节点 */
  const loadTree = async (preferredUlid = '') => {
    if (!projectUlid.value) {
      treeList.value = []
      selectedUlid.value = ''
      return
    }
    treeLoading.value = true
    try {
      treeList.value = await listOutlineTree(projectUlid.value)
      const candidateUlid = preferredUlid || selectedUlid.value
      const stillExists = flatNodeList.value.some((node) => node.outline_ulid === candidateUlid)
      selectedUlid.value = stillExists
        ? candidateUlid
        : (flatNodeList.value[0]?.outline_ulid ?? '')
    } catch {
      treeList.value = []
      selectedUlid.value = ''
    } finally {
      treeLoading.value = false
    }
  }

  /** 加载当前节点的场景卡 */
  const loadScenes = async () => {
    if (!projectUlid.value || !selectedUlid.value) {
      sceneList.value = []
      return
    }
    sceneLoading.value = true
    try {
      sceneList.value = await listDetailedScenes(projectUlid.value, selectedUlid.value)
    } catch {
      sceneList.value = []
    } finally {
      sceneLoading.value = false
    }
  }

  const handleSelect = (node: NWOutlineNodeVo) => {
    if (node.outline_ulid === selectedUlid.value) return
    selectedUlid.value = node.outline_ulid
  }

  const handleCreateRoot = () => nodeDialog.value?.open()

  const handleCreateChild = (node: NWOutlineNodeVo) => nodeDialog.value?.open(undefined, node)

  const handleEditNode = (node: NWOutlineNodeVo) => nodeDialog.value?.open(node)

  const handleDeleteNode = async (node: NWOutlineNodeVo) => {
    try {
      await ElMessageBox.confirm(
        `将删除大纲节点「${node.title_text}」，有子节点时需先删除子节点。`,
        '删除节点',
        { type: 'warning' }
      )
    } catch {
      return
    }
    await deleteOutlineNode(projectUlid.value, node.outline_ulid)
    ElMessage.success('已删除节点')
    await loadTree()
  }

  const handleNodeSaved = async (node: NWOutlineNodeVo) => {
    ElMessage.success('节点已保存')
    await loadTree(node.outline_ulid)
  }

  const handleCreateScene = () => {
    if (!selectedNode.value) {
      ElMessage.warning('先选择一个大纲节点')
      return
    }
    sceneDialog.value?.open()
  }

  const handleEditScene = (scene: NWDetailedSceneVo) => sceneDialog.value?.open(scene)

  const handleDeleteScene = async (scene: NWDetailedSceneVo) => {
    try {
      await ElMessageBox.confirm('删除该场景卡后不可恢复，确定删除吗？', '删除场景卡', {
        type: 'warning'
      })
    } catch {
      return
    }
    await deleteDetailedScene(projectUlid.value, scene.scene_ulid)
    ElMessage.success('已删除场景卡')
    await loadScenes()
  }

  const handleSceneSaved = async () => {
    ElMessage.success('场景卡已保存')
    await loadScenes()
  }

  watch(selectedUlid, () => void loadScenes())

  watch(projectUlid, async (nextUlid) => {
    if (!nextUlid) {
      treeList.value = []
      sceneList.value = []
      selectedUlid.value = ''
      return
    }
    await loadTree()
  })

  onMounted(async () => {
    if (!projectStore.projectOptions.length) await projectStore.loadProjectOptions()
    if (projectUlid.value) await loadTree()
  })
</script>

<template>
  <div class="nw-page">
    <div class="nw-toolbar">
      <ElButton type="primary" plain size="small" @click="handleCreateRoot">
        <Icon icon="mdi:plus" :size="14" />
        <span class="ml-4px">新建节点</span>
      </ElButton>
      <ElButton size="small" plain :loading="treeLoading" @click="loadTree()">
        <Icon icon="mdi:reload" :size="14" />
        <span class="ml-4px">刷新</span>
      </ElButton>
      <div class="outline__spacer" />
      <span class="nw-hint">共 {{ totalNodeCount }} 个节点</span>
    </div>

    <ElEmpty
      v-if="!projectUlid"
      description="请先在顶栏选择项目，再规划大纲"
      :image-size="90"
    />

    <div v-else class="outline__layout">
      <aside class="outline__side">
        <div class="outline__side-head">
          <span class="outline__side-title">大纲结构</span>
          <ElButton size="small" text @click="handleCreateRoot">
            <Icon icon="mdi:plus" :size="13" />
          </ElButton>
        </div>
        <ElScrollbar class="outline__tree-scroll">
          <div v-loading="treeLoading" class="outline__tree-host">
            <OutlineTree
              :node-list="treeList"
              :selected-ulid="selectedUlid"
              @select="handleSelect"
              @create-child="handleCreateChild"
              @edit-node="handleEditNode"
              @delete-node="handleDeleteNode"
            />
          </div>
        </ElScrollbar>
      </aside>

      <section class="outline__main">
        <ElEmpty
          v-if="!selectedNode"
          description="从左侧选择节点，查看概要并管理场景卡"
          :image-size="90"
        />

        <NodeDetail
          v-else
          :node="selectedNode"
          :scene-list="sceneList"
          :scene-loading="sceneLoading"
          @create-child="handleCreateChild(selectedNode)"
          @edit-node="handleEditNode(selectedNode)"
          @delete-node="handleDeleteNode(selectedNode)"
          @create-scene="handleCreateScene"
          @edit-scene="handleEditScene"
          @delete-scene="handleDeleteScene"
        />
      </section>
    </div>

    <OutlineNodeDialog ref="nodeDialog" :project-ulid="projectUlid" @saved="handleNodeSaved" />
    <SceneDialog
      ref="sceneDialog"
      :project-ulid="projectUlid"
      :outline-ulid="selectedUlid"
      @saved="handleSceneSaved"
    />
  </div>
</template>

<style lang="less" scoped>
  .outline {
    &__spacer {
      flex: 1;
    }

    &__layout {
      display: flex;
      gap: 16px;
      align-items: stretch;
      height: calc(
        100vh - var(--top-tool-height) - var(--tags-view-height) - 2 * var(--app-content-padding) -
          96px
      );
      min-height: 360px;
    }

    &__side {
      display: flex;
      flex: none;
      flex-direction: column;
      width: 320px;
      min-height: 0;
      overflow: hidden;
      background: var(--nw-surface);
      border: 1px solid var(--nw-border);
      border-radius: var(--nw-radius-lg);
    }

    &__side-head {
      display: flex;
      flex: none;
      align-items: center;
      justify-content: space-between;
      height: 44px;
      padding: 0 12px;
      border-bottom: 1px solid var(--nw-border-soft);
    }

    &__side-title {
      font-size: 13px;
      font-weight: 650;
      color: var(--nw-ink);
    }

    &__tree-scroll {
      flex: 1;
      min-height: 0;
    }

    &__tree-host {
      min-height: 220px;
      padding: 8px;
    }

    &__main {
      display: flex;
      flex: 1;
      flex-direction: column;
      gap: 16px;
      min-width: 0;
      min-height: 0;
      padding-right: 4px;
      overflow-y: auto;
    }
  }

  @media (width <= 1024px) {
    .outline__layout {
      flex-direction: column;
      height: auto;
    }

    .outline__side {
      width: 100%;
    }

    .outline__tree-scroll {
      max-height: 260px;
    }
  }
</style>

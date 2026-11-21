<!-- 角色关系图：基于 X6 渲染并维护角色之间的关系。 -->
<script setup lang="ts">
  import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
  import '@antv/x6/es/index.css'
  import { ElButton, ElEmpty, ElMessage, ElMessageBox } from 'element-plus'
  import type { Edge, Graph } from '@antv/x6'
  import type { NWCharacterGraphVo } from '@/api/character'
  import { deleteCharacterRelation } from '@/api/character'

  const props = defineProps<{
    graphData: NWCharacterGraphVo | null
    relationTypeLabelMap: Record<string, string>
  }>()
  const emit = defineEmits<{ changed: [] }>()

  /** 节点尺寸（像素） */
  const NODE_WIDTH = 132
  const NODE_HEIGHT = 44
  /** 环形布局半径 */
  const LAYOUT_RADIUS = 210

  const hostRef = ref<HTMLDivElement | null>(null)
  const emptyFlag = ref(true)
  const booting = ref(true)

  let graph: Graph | null = null
  let disposed = false

  /** 读取主题令牌，保证图谱跟随品牌配色 */
  const readToken = (name: string) => {
    const value = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
    return value || '#6366f1'
  }

  /** 十六进制/rgb 颜色叠加透明度（X6 属性只认具体色值） */
  const withAlpha = (color: string, alpha: number) => {
    const hexMatch = color.match(/^#([0-9a-f]{3}|[0-9a-f]{6})$/i)
    if (hexMatch) {
      const raw = hexMatch[1]
      const full =
        raw.length === 3
          ? raw
              .split('')
              .map((char) => char + char)
              .join('')
          : raw
      const red = parseInt(full.slice(0, 2), 16)
      const green = parseInt(full.slice(2, 4), 16)
      const blue = parseInt(full.slice(4, 6), 16)
      return `rgba(${red}, ${green}, ${blue}, ${alpha})`
    }
    const rgbMatch = color.match(/^rgba?\(([^)]+)\)$/i)
    if (rgbMatch) {
      const [red, green, blue] = rgbMatch[1].split(',').map((item) => item.trim())
      return `rgba(${red}, ${green}, ${blue}, ${alpha})`
    }
    return color
  }

  /** 角色类型对应的节点配色 */
  const nodePalette = (roleTypeCode: string) => {
    const primary = readToken('--nw-primary')
    const danger = readToken('--nw-danger')
    const warning = readToken('--nw-warning')
    const ink = readToken('--nw-ink')
    if (roleTypeCode === 'protagonist') {
      return { fill: withAlpha(primary, 0.14), stroke: primary, text: ink }
    }
    if (roleTypeCode === 'antagonist') {
      return { fill: withAlpha(danger, 0.12), stroke: danger, text: ink }
    }
    if (roleTypeCode === 'supporting') {
      return { fill: withAlpha(warning, 0.14), stroke: warning, text: ink }
    }
    return { fill: readToken('--nw-surface-subtle'), stroke: readToken('--nw-border'), text: ink }
  }

  /** 环形坐标：节点均匀分布在圆心周围 */
  const circlePosition = (index: number, totalCount: number) => {
    if (totalCount <= 1) return { x: 0, y: 0 }
    const angle = (Math.PI * 2 * index) / totalCount - Math.PI / 2
    return {
      x: Math.cos(angle) * LAYOUT_RADIUS,
      y: Math.sin(angle) * LAYOUT_RADIUS
    }
  }

  const renderGraph = () => {
    if (!graph) return
    graph.clearCells()
    const data = props.graphData
    const nodeList = data?.node_list ?? []
    const edgeList = data?.edge_list ?? []
    emptyFlag.value = nodeList.length === 0
    if (nodeList.length === 0) return

    const lineColor = readToken('--nw-text-muted')
    const labelColor = readToken('--nw-text-secondary')

    nodeList.forEach((node, index) => {
      const palette = nodePalette(node.role_type_code)
      const position = circlePosition(index, nodeList.length)
      graph?.addNode({
        id: node.character_ulid,
        x: position.x - NODE_WIDTH / 2,
        y: position.y - NODE_HEIGHT / 2,
        width: NODE_WIDTH,
        height: NODE_HEIGHT,
        attrs: {
          body: {
            rx: 8,
            ry: 8,
            fill: palette.fill,
            stroke: palette.stroke,
            strokeWidth: 1.4
          },
          label: {
            text: node.character_name,
            fill: palette.text,
            fontSize: 12,
            fontWeight: 600,
            textWrap: { width: NODE_WIDTH - 16, height: NODE_HEIGHT - 10, ellipsis: true }
          }
        }
      })
    })

    edgeList.forEach((edge) => {
      const relationLabel =
        props.relationTypeLabelMap[edge.relation_type_code] ?? edge.relation_type_code
      graph?.addEdge({
        id: edge.relation_ulid,
        source: edge.from_character_ulid,
        target: edge.to_character_ulid,
        data: { relationLabel },
        connector: { name: 'rounded', args: { radius: 10 } },
        attrs: {
          line: {
            stroke: lineColor,
            strokeWidth: 1.2,
            targetMarker: { name: 'classic', size: 7 }
          }
        },
        labels: [
          {
            attrs: {
              label: {
                text: relationLabel,
                fill: labelColor,
                fontSize: 10
              },
              body: {
                fill: readToken('--nw-surface'),
                stroke: readToken('--nw-border-soft'),
                rx: 4,
                ry: 4
              }
            }
          }
        ]
      })
    })

    graph.centerContent()
  }

  /** 删除关系边（双击边触发，二次确认） */
  const handleEdgeDblClick = async (args: { edge: Edge }) => {
    const relationUlid = args.edge.id
    const relationLabel = String(
      (args.edge.getData() as { relationLabel?: string } | undefined)?.relationLabel ?? relationUlid
    )
    try {
      await ElMessageBox.confirm(`删除关系「${relationLabel || relationUlid}」？`, '删除关系', {
        type: 'warning'
      })
    } catch {
      return
    }
    await deleteCharacterRelation(relationUlid)
    ElMessage.success('关系已删除')
    emit('changed')
  }

  const zoomIn = () => graph?.zoom(0.15)
  const zoomOut = () => graph?.zoom(-0.15)
  const fitContent = () => graph?.centerContent()

  const initGraph = async () => {
    const host = hostRef.value
    if (!host) {
      booting.value = false
      return
    }
    try {
      const { Graph: X6Graph } = await import('@antv/x6')
      if (disposed || !hostRef.value) return
      graph = new X6Graph({
        container: host,
        autoResize: true,
        background: { color: 'transparent' },
        grid: { visible: true, size: 12 },
        panning: true,
        mousewheel: { enabled: true, modifiers: ['ctrl', 'meta'], minScale: 0.4, maxScale: 2 },
        interacting: { nodeMovable: true, edgeLabelMovable: false },
        connecting: { snap: true, allowBlank: false, allowLoop: false }
      })
      graph.on('edge:dblclick', handleEdgeDblClick)
      renderGraph()
    } catch {
      ElMessage.error('关系图渲染失败，请刷新页面重试')
    } finally {
      booting.value = false
    }
  }

  watch(
    () => props.graphData,
    () => renderGraph(),
    { deep: true }
  )

  onMounted(() => {
    void initGraph()
  })

  onBeforeUnmount(() => {
    disposed = true
    graph?.dispose()
    graph = null
  })
</script>

<template>
  <div class="graph-panel">
    <div class="graph-panel__toolbar">
      <span class="nw-hint">拖拽节点调整布局，双击连线删除关系</span>
      <div class="graph-panel__spacer" />
      <ElButton size="small" text @click="zoomOut">
        <Icon icon="mdi:minus" :size="14" />
      </ElButton>
      <ElButton size="small" text @click="zoomIn">
        <Icon icon="mdi:plus" :size="14" />
      </ElButton>
      <ElButton size="small" text @click="fitContent">
        <Icon icon="mdi:refresh" :size="14" />
      </ElButton>
    </div>

    <div ref="hostRef" v-loading="booting" class="graph-panel__canvas"></div>

    <ElEmpty
      v-if="emptyFlag"
      class="graph-panel__empty"
      description="还没有角色，先创建角色再看关系图"
      :image-size="80"
    />
  </div>
</template>

<style lang="less" scoped>
  .graph-panel {
    position: relative;
    display: flex;
    flex-direction: column;
    gap: 8px;

    &__toolbar {
      display: flex;
      flex-wrap: wrap;
      gap: 6px;
      align-items: center;
    }

    &__spacer {
      flex: 1;
    }

    &__canvas {
      height: 460px;
      overflow: hidden;
      border: 1px solid var(--nw-border-soft);
      border-radius: var(--nw-radius-lg);
      background: var(--nw-surface);
    }

    &__empty {
      position: absolute;
      inset: 60px 0 0;
      pointer-events: none;
    }
  }
</style>

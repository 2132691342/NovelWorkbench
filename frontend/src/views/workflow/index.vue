<!--
  创作流程设计器：基于 BPMN-JS 绘制作品创作流程，草稿保存在当前浏览器。
  保存/载入/导入/导出都走本地草稿，避免把流程资产混入业务数据库。
-->
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import Modeler from 'bpmn-js/lib/Modeler'
import 'bpmn-js/dist/assets/diagram-js.css'
import 'bpmn-js/dist/assets/bpmn-font/css/bpmn.css'
import NWButton from '@/components/NWButton.vue'
import NWCard from '@/components/NWCard.vue'
import NWEmpty from '@/components/NWEmpty.vue'

interface NWWorkflowDraft {
  draft_ulid: string
  draft_name: string
  bpmn_xml: string
  updated_at_ms: number
}

const { t } = useI18n()
const storageKey = 'nw_workflow_draft_list_v1'
const canvasRef = ref<HTMLDivElement | null>(null)
const fileInputRef = ref<HTMLInputElement | null>(null)
const draftList = ref<NWWorkflowDraft[]>([])
const currentDraft = ref<NWWorkflowDraft | null>(null)
const draftName = ref('')
const modelerReady = ref(false)

let modelerInstance: Modeler | null = null

const sortedDraftList = computed(() =>
  [...draftList.value].sort((left, right) => right.updated_at_ms - left.updated_at_ms),
)

const defaultBpmnXml = `<?xml version="1.0" encoding="UTF-8"?>
<bpmn:definitions xmlns:bpmn="http://www.omg.org/spec/BPMN/20100524/MODEL" xmlns:bpmndi="http://www.omg.org/spec/BPMN/20100524/DI" xmlns:dc="http://www.omg.org/spec/DD/20100524/DC" xmlns:di="http://www.omg.org/spec/DD/20100524/DI" id="Definitions_NovelWorkbench" targetNamespace="https://novelworkbench.local">
  <bpmn:process id="Process_WritingWorkflow" name="创作流程" isExecutable="false">
    <bpmn:startEvent id="StartEvent_Begin" name="选题" />
    <bpmn:task id="Task_Outline" name="大纲" />
    <bpmn:task id="Task_Draft" name="起草" />
    <bpmn:task id="Task_Review" name="复核" />
    <bpmn:endEvent id="EndEvent_Done" name="定稿" />
    <bpmn:sequenceFlow id="Flow_1" sourceRef="StartEvent_Begin" targetRef="Task_Outline" />
    <bpmn:sequenceFlow id="Flow_2" sourceRef="Task_Outline" targetRef="Task_Draft" />
    <bpmn:sequenceFlow id="Flow_3" sourceRef="Task_Draft" targetRef="Task_Review" />
    <bpmn:sequenceFlow id="Flow_4" sourceRef="Task_Review" targetRef="EndEvent_Done" />
  </bpmn:process>
  <bpmndi:BPMNDiagram id="BPMNDiagram_1">
    <bpmndi:BPMNPlane id="BPMNPlane_1" bpmnElement="Process_WritingWorkflow">
      <bpmndi:BPMNShape id="StartEvent_Begin_di" bpmnElement="StartEvent_Begin"><dc:Bounds x="120" y="180" width="36" height="36" /></bpmndi:BPMNShape>
      <bpmndi:BPMNShape id="Task_Outline_di" bpmnElement="Task_Outline"><dc:Bounds x="220" y="158" width="100" height="80" /></bpmndi:BPMNShape>
      <bpmndi:BPMNShape id="Task_Draft_di" bpmnElement="Task_Draft"><dc:Bounds x="380" y="158" width="100" height="80" /></bpmndi:BPMNShape>
      <bpmndi:BPMNShape id="Task_Review_di" bpmnElement="Task_Review"><dc:Bounds x="540" y="158" width="100" height="80" /></bpmndi:BPMNShape>
      <bpmndi:BPMNShape id="EndEvent_Done_di" bpmnElement="EndEvent_Done"><dc:Bounds x="700" y="180" width="36" height="36" /></bpmndi:BPMNShape>
      <bpmndi:BPMNEdge id="Flow_1_di" bpmnElement="Flow_1"><di:waypoint x="156" y="198" /><di:waypoint x="220" y="198" /></bpmndi:BPMNEdge>
      <bpmndi:BPMNEdge id="Flow_2_di" bpmnElement="Flow_2"><di:waypoint x="320" y="198" /><di:waypoint x="380" y="198" /></bpmndi:BPMNEdge>
      <bpmndi:BPMNEdge id="Flow_3_di" bpmnElement="Flow_3"><di:waypoint x="480" y="198" /><di:waypoint x="540" y="198" /></bpmndi:BPMNEdge>
      <bpmndi:BPMNEdge id="Flow_4_di" bpmnElement="Flow_4"><di:waypoint x="640" y="198" /><di:waypoint x="700" y="198" /></bpmndi:BPMNEdge>
    </bpmndi:BPMNPlane>
  </bpmndi:BPMNDiagram>
</bpmn:definitions>`

function readDraftList(): NWWorkflowDraft[] {
  try {
    const raw = window.localStorage.getItem(storageKey)
    const parsed = raw ? JSON.parse(raw) : []
    return Array.isArray(parsed) ? (parsed as NWWorkflowDraft[]) : []
  } catch {
    return []
  }
}

function persistDraftList(): void {
  window.localStorage.setItem(storageKey, JSON.stringify(draftList.value))
}

function makeDraftUlid(): string {
  return `WFL_${Date.now().toString(36).toUpperCase()}_${Math.random().toString(36).slice(2, 8).toUpperCase()}`
}

async function loadXml(xmlText: string): Promise<void> {
  if (!modelerInstance) {
    return
  }
  await modelerInstance.importXML(xmlText)
  const canvas = modelerInstance.get<{ zoom: (mode: string) => void }>('canvas')
  canvas.zoom('fit-viewport')
  modelerReady.value = true
}

function handleNew(): void {
  currentDraft.value = null
  draftName.value = t('workflow.newDiagram')
  void loadXml(defaultBpmnXml)
}

function handleLoad(draft: NWWorkflowDraft): void {
  currentDraft.value = draft
  draftName.value = draft.draft_name
  void loadXml(draft.bpmn_xml)
}

async function readCurrentXml(): Promise<string> {
  if (!modelerInstance) {
    return ''
  }
  const result = await modelerInstance.saveXML({ format: true })
  return result.xml ?? ''
}

async function handleSave(): Promise<void> {
  const name = draftName.value.trim()
  if (!name) {
    ElMessage.warning(t('workflow.nameRequired'))
    return
  }
  const bpmnXml = await readCurrentXml()
  const existing = currentDraft.value
  const draft: NWWorkflowDraft = {
    draft_ulid: existing?.draft_ulid ?? makeDraftUlid(),
    draft_name: name,
    bpmn_xml: bpmnXml,
    updated_at_ms: Date.now(),
  }
  if (existing) {
    draftList.value = draftList.value.map((item) =>
      item.draft_ulid === existing.draft_ulid ? draft : item,
    )
  } else {
    draftList.value = [draft, ...draftList.value]
  }
  currentDraft.value = draft
  persistDraftList()
  ElMessage.success(t('workflow.saveSuccess'))
}

async function handleExport(): Promise<void> {
  const xmlText = await readCurrentXml()
  const blob = new Blob([xmlText], { type: 'application/xml;charset=utf-8' })
  const objectUrl = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = objectUrl
  anchor.download = `${draftName.value.trim() || 'workflow'}.bpmn`
  anchor.click()
  URL.revokeObjectURL(objectUrl)
}

function handleImportClick(): void {
  fileInputRef.value?.click()
}

async function handleImportFile(event: Event): Promise<void> {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) {
    return
  }
  const xmlText = await file.text()
  await loadXml(xmlText)
  currentDraft.value = null
  draftName.value = file.name.replace(/\.bpmn$/i, '')
  ElMessage.success(t('workflow.importSuccess'))
  input.value = ''
}

async function handleDelete(draft: NWWorkflowDraft): Promise<void> {
  try {
    await ElMessageBox.confirm(t('workflow.deleteConfirm'), t('common.confirm'), {
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel'),
      type: 'warning',
    })
  } catch {
    return
  }
  draftList.value = draftList.value.filter((item) => item.draft_ulid !== draft.draft_ulid)
  persistDraftList()
  if (currentDraft.value?.draft_ulid === draft.draft_ulid) {
    currentDraft.value = null
    handleNew()
  }
  ElMessage.success(t('workflow.deleteSuccess'))
}

onMounted(() => {
  draftList.value = readDraftList()
  if (canvasRef.value) {
    modelerInstance = new Modeler({ container: canvasRef.value })
    void loadXml(defaultBpmnXml)
  }
})

onBeforeUnmount(() => {
  modelerInstance?.destroy()
  modelerInstance = null
})
</script>

<template>
  <div class="workflow-page">
    <NWCard :title="t('workflow.title')" :hoverable="false">
      <p class="workflow-page__subtitle">{{ t('workflow.subtitle') }}</p>
      <div class="workflow-page__main">
        <aside class="workflow-page__list">
          <NWButton type="primary" size="small" @click="handleNew">{{ t('workflow.newDiagram') }}</NWButton>
          <NWEmpty v-if="sortedDraftList.length === 0" :description_text="t('workflow.emptyTip')" />
          <ul v-else class="workflow-page__drafts">
            <li v-for="draft in sortedDraftList" :key="draft.draft_ulid" class="workflow-page__draft">
              <button type="button" class="workflow-page__draft-name" @click="handleLoad(draft)">
                {{ draft.draft_name }}
              </button>
              <el-button link type="danger" size="small" @click="handleDelete(draft)">
                {{ t('workflow.deleteDiagram') }}
              </el-button>
            </li>
          </ul>
        </aside>
        <section class="workflow-page__editor">
          <div class="workflow-page__toolbar">
            <el-input v-model="draftName" class="workflow-page__name" :placeholder="t('workflow.diagramName')" />
            <NWButton type="primary" size="small" :disabled="!modelerReady" @click="handleSave">
              {{ t('workflow.saveDiagram') }}
            </NWButton>
            <NWButton size="small" :disabled="!modelerReady" @click="handleImportClick">
              {{ t('workflow.importDiagram') }}
            </NWButton>
            <NWButton size="small" :disabled="!modelerReady" @click="handleExport">
              {{ t('workflow.exportDiagram') }}
            </NWButton>
            <input
              ref="fileInputRef"
              class="workflow-page__file-input"
              type="file"
              accept=".bpmn,.xml,application/xml,text/xml"
              @change="handleImportFile"
            />
          </div>
          <p class="workflow-page__tip">{{ t('workflow.editorTip') }}</p>
          <div ref="canvasRef" class="workflow-page__canvas" />
        </section>
      </div>
    </NWCard>
  </div>
</template>

<style scoped src="./workflow.css"></style>

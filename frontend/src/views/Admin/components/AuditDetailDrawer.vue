<!-- 审计详情抽屉：概览（谁/从哪/什么动作/结果/耗时）+ 请求体 + 响应体。 -->
<script setup lang="ts">
  import { computed, ref, watch } from 'vue'
  import {
    ElButton,
    ElDescriptions,
    ElDescriptionsItem,
    ElDrawer,
    ElEmpty,
    ElMessage,
    ElTag
  } from 'element-plus'
  import { getAdminAuditDetail, type NWAuditDetailVo, type NWAuditVo } from '@/api/admin'
  import { formatTime } from '@/utils/time'
  import AuditBodyViewer from './AuditBodyViewer.vue'

  const props = defineProps<{
    visible: boolean
    /** 列表行（先用列表数据占位，详情到达后替换） */
    row: NWAuditVo | null
  }>()

  const emit = defineEmits<{
    'update:visible': [visible: boolean]
    'open-trace': [traceId: string]
  }>()

  const loading = ref(false)
  const failed = ref(false)
  const detail = ref<NWAuditDetailVo | null>(null)

  /** 详情未到时先用列表行渲染概览，避免抽屉空白 */
  const shown = computed(() => detail.value ?? props.row)

  const levelTagType = (levelCode: string) =>
    levelCode === 'error' ? 'danger' : levelCode === 'warn' ? 'warning' : 'success'

  const levelLabel = (levelCode: string) =>
    levelCode === 'error' ? '错误' : levelCode === 'warn' ? '警示' : '正常'

  const spanLabel = (spanTypeCode: string) =>
    ({
      http: '入口请求',
      workflow_step: '工作流步骤',
      ai_call: 'AI 调用',
      job: '异步任务',
      system: '系统'
    })[spanTypeCode] ?? (spanTypeCode || '入口请求')

  /** 复制为 cURL：仅 HTTP span 有意义，后台 span 用伪 URL 说明 */
  const curlText = computed(() => {
    const item = shown.value
    if (!item) return ''
    const basePath = item.request_path || ''
    const body = detail.value?.request_body || ''
    const method = item.request_method || 'POST'
    const parts = [`curl -X ${method} '${basePath}'`, `-H 'Content-Type: application/json'`]
    if (item.trace_id) parts.push(`-H 'X-Request-ID: ${item.trace_id}'`)
    if (body) parts.push(`-d '${body.replace(/'/g, `'\\''`)}'`)
    return parts.join(' \\\n  ')
  })

  const load = async () => {
    const auditUlid = props.row?.audit_ulid
    if (!auditUlid) return
    loading.value = true
    failed.value = false
    detail.value = null
    try {
      detail.value = await getAdminAuditDetail(auditUlid)
    } catch {
      failed.value = true
    } finally {
      loading.value = false
    }
  }

  /** 复制链路 ID */
  const copyTrace = async () => {
    const traceId = shown.value?.trace_id
    if (!traceId) return
    try {
      await navigator.clipboard.writeText(traceId)
      ElMessage.success('链路 ID 已复制')
    } catch {
      ElMessage.warning('浏览器不允许写剪贴板，请手动选择复制')
    }
  }

  watch(
    () => [props.visible, props.row?.audit_ulid],
    ([visible]) => {
      if (visible) void load()
    }
  )
</script>

<template>
  <ElDrawer
    :model-value="visible"
    title="审计详情"
    size="640px"
    destroy-on-close
    @update:model-value="emit('update:visible', $event)"
  >
    <div v-loading="loading" class="audit-detail">
      <div v-if="failed" class="audit-detail__failed">
        <ElEmpty description="详情读取失败，可能是记录已被清理" :image-size="64" />
        <ElButton size="small" type="primary" plain @click="load()">重试</ElButton>
      </div>

      <template v-else-if="shown">
        <ElDescriptions :column="1" border size="small">
          <ElDescriptionsItem label="动作">
            <span class="audit-detail__action">{{ shown.action_name || shown.action_code }}</span>
            <span class="nw-hint audit-detail__mono">{{ shown.action_code }}</span>
          </ElDescriptionsItem>
          <ElDescriptionsItem label="span 类型">
            <span class="nw-badge">{{ spanLabel(shown.span_type_code) }}</span>
            <span v-if="shown.parent_span_code" class="nw-hint audit-detail__mono">
              父 span：{{ shown.parent_span_code }}
            </span>
          </ElDescriptionsItem>
          <ElDescriptionsItem label="时间">{{ formatTime(shown.created_at) }}</ElDescriptionsItem>
          <ElDescriptionsItem label="操作人">
            {{ shown.user_name || '—' }}
            <span class="nw-hint audit-detail__mono">{{ shown.user_ulid }}</span>
          </ElDescriptionsItem>
          <ElDescriptionsItem label="链路 ID">
            <span class="audit-detail__mono">{{ shown.trace_id || '—' }}</span>
            <ElButton v-if="shown.trace_id" size="small" text type="primary" @click="copyTrace()">
              复制
            </ElButton>
            <ElButton
              v-if="shown.trace_id"
              size="small"
              text
              type="primary"
              @click="emit('open-trace', shown.trace_id)"
            >
              查看时间线
            </ElButton>
          </ElDescriptionsItem>
          <ElDescriptionsItem label="来源">
            {{ shown.request_method }} {{ shown.request_path }}
          </ElDescriptionsItem>
          <ElDescriptionsItem label="结果">
            <ElTag :type="levelTagType(shown.log_level_code)" size="small" effect="light">
              {{ levelLabel(shown.log_level_code) }}
            </ElTag>
            <span class="audit-detail__mono">{{ shown.resp_code || '—' }}</span>
            <span class="nw-hint">HTTP {{ shown.http_status }}</span>
            <div class="nw-hint audit-detail__message">{{ shown.resp_message }}</div>
          </ElDescriptionsItem>
          <ElDescriptionsItem label="耗时">
            <span :class="shown.latency_ms > 1000 ? 'audit-detail__slow' : ''">
              {{ shown.latency_ms }} ms
            </span>
            <span v-if="shown.model_name" class="nw-hint">
              模型 {{ shown.model_name }} · {{ shown.token_count }} token
            </span>
          </ElDescriptionsItem>
          <ElDescriptionsItem label="目标">
            {{ shown.target_type || '—' }}
            <span class="nw-hint audit-detail__mono">{{ shown.target_ulid }}</span>
          </ElDescriptionsItem>
          <ElDescriptionsItem label="客户端">
            {{ shown.ip_addr || '—' }}
            <div class="nw-hint audit-detail__ua">{{ shown.user_agent_text }}</div>
          </ElDescriptionsItem>
          <ElDescriptionsItem label="审计 ID">
            <span class="audit-detail__mono">{{ shown.audit_ulid }}</span>
          </ElDescriptionsItem>
        </ElDescriptions>

        <AuditBodyViewer
          title-text="请求体"
          :raw-text="detail?.request_body || ''"
          empty-text="未采样：文件上传、流式请求或超过 32KB"
          :curl-text="curlText"
        />
        <AuditBodyViewer
          title-text="响应体"
          :raw-text="detail?.resp_body || ''"
          empty-text="未采样：流式响应、文件下载或超过 32KB"
        />
      </template>

      <ElEmpty v-else-if="!loading" description="未读取到审计详情" :image-size="64" />
    </div>
  </ElDrawer>
</template>

<style lang="less" scoped>
  .audit-detail {
    display: flex;
    flex-direction: column;
    gap: 16px;

    &__failed {
      display: flex;
      flex-direction: column;
      gap: 8px;
      align-items: center;
    }

    &__action {
      margin-right: 6px;
      font-weight: 600;
      color: var(--nw-ink);
    }

    &__mono {
      margin-left: 6px;
      font-family: var(--nw-font-mono);
      font-size: 12px;
    }

    &__message,
    &__ua {
      margin-top: 4px;
      word-break: break-all;
    }

    &__slow {
      font-weight: 600;
      color: var(--nw-warning);
    }
  }
</style>

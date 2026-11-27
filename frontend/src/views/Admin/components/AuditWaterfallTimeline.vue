<!-- 链路瀑布时间线：把同一 trace 的全部 span 按时间轴排成瀑布，
     横轴起点 = 开始时刻、色块长度 = 耗时，标出最慢一步与失败步骤。 -->
<script setup lang="ts">
  import { ElButton, ElTag } from 'element-plus'
  import type { NWAuditDetailVo } from '@/api/admin'

  /** 链路内的全部 span（按开始时刻排序） */
  defineProps<{
    itemList: NWAuditDetailVo[]
    totalLatency: number
    slowestUlid: string
    expanded: Set<string>
  }>()

  /** 展开/收起某个 span；打开完整详情 */
  const emit = defineEmits<{
    (event: 'toggle', auditUlid: string): void
    (event: 'open-detail', row: NWAuditDetailVo): void
  }>()

  /** 时间线首条的时间戳，用于算相对偏移 */
  const startAt = (itemList: NWAuditDetailVo[]) => {
    const first = itemList[0]
    return first ? new Date(first.created_at).getTime() : 0
  }

  /** span 类型对应图标 */
  const spanIcon = (spanTypeCode: string) =>
    ({
      http: 'mdi:web',
      workflow_step: 'mdi:sitemap-outline',
      ai_call: 'mdi:robot-outline',
      job: 'mdi:cog-outline',
      system: 'mdi:wrench-outline'
    })[spanTypeCode] ?? 'mdi:circle-small'

  /** span 类型对应中文标签 */
  const spanLabel = (spanTypeCode: string) =>
    ({
      http: '入口请求',
      workflow_step: '工作流步骤',
      ai_call: 'AI 调用',
      job: '异步任务',
      system: '系统'
    })[spanTypeCode] ?? (spanTypeCode || '入口请求')

  /** 级别对应标签色 */
  const levelTagType = (levelCode: string) =>
    levelCode === 'danger' ? 'danger' : levelCode === 'warn' ? 'warning' : 'success'

  /** 相对首条的时间偏移（毫秒） */
  const offsetOf = (item: NWAuditDetailVo, itemList: NWAuditDetailVo[]) =>
    Math.max(0, new Date(item.created_at).getTime() - startAt(itemList))

  /** 偏移文案（+0ms / +1.2s） */
  const offsetText = (item: NWAuditDetailVo, itemList: NWAuditDetailVo[]) => {
    const offsetMs = offsetOf(item, itemList)
    return offsetMs < 1000 ? `+${offsetMs}ms` : `+${(offsetMs / 1000).toFixed(1)}s`
  }

  /** 瀑布条左端百分比（相对整条链路的时间轴） */
  const barLeftOf = (item: NWAuditDetailVo, itemList: NWAuditDetailVo[], totalLatency: number) => {
    const percent = (offsetOf(item, itemList) / totalLatency) * 100
    return `${Math.min(99, Math.max(0, percent))}%`
  }

  /** 瀑布条宽度百分比（最少 1.5% 保证 0ms 步骤仍可见） */
  const barWidthOf = (item: NWAuditDetailVo, totalLatency: number) => {
    const percent = (item.latency_ms / totalLatency) * 100
    return `${Math.max(1.5, Math.min(100, percent))}%`
  }

  /** 缩进层级：父 span 动作码在本次链路内则缩进一层 */
  const isChildSpan = (item: NWAuditDetailVo, itemList: NWAuditDetailVo[]) => {
    if (!item.parent_span_code) return false
    return itemList.some((row) => row.action_code === item.parent_span_code)
  }

  /** 报文摘要：压缩空白并截断，便于在瀑布里扫读 */
  const summaryOf = (raw: string) => {
    if (!raw) return ''
    const compact = raw.replace(/\s+/g, ' ').trim()
    return compact.length > 240 ? `${compact.slice(0, 240)}…` : compact
  }
</script>

<template>
  <ol class="audit-trace__list">
    <li
      v-for="item in itemList"
      :key="item.audit_ulid"
      class="trace-row"
      :class="{
        'is-child': isChildSpan(item, itemList),
        'is-slowest': item.audit_ulid === slowestUlid,
        'is-error': item.log_level_code === 'error'
      }"
    >
      <div class="trace-row__head" @click="emit('toggle', item.audit_ulid)">
        <Icon :icon="spanIcon(item.span_type_code)" :size="15" class="trace-row__icon" />
        <div class="trace-row__main">
          <div class="trace-row__line">
            <strong class="trace-row__name">{{ item.action_name || item.action_code }}</strong>
            <span class="nw-badge">{{ spanLabel(item.span_type_code) }}</span>
            <ElTag :type="levelTagType(item.log_level_code)" size="small" effect="light">
              {{ item.http_status || item.resp_code || '—' }}
            </ElTag>
            <span v-if="item.model_name" class="nw-hint">{{ item.model_name }}</span>
            <span class="trace-row__latency">{{ item.latency_ms }} ms</span>
          </div>

          <!-- 瀑布条：横向位置表示开始时刻，长度表示耗时 -->
          <div class="trace-row__track">
            <span
              class="trace-row__track-fill"
              :class="{
                'is-error': item.log_level_code === 'error',
                'is-warn': item.log_level_code === 'warn'
              }"
              :style="{ left: barLeftOf(item, itemList, totalLatency), width: barWidthOf(item, totalLatency) }"
            />
          </div>

          <div class="trace-row__meta">
            <span class="audit-trace__mono">{{ offsetText(item, itemList) }}</span>
            <span class="audit-trace__mono">{{ item.action_code }}</span>
            <span v-if="item.token_count > 0" class="nw-hint">{{ item.token_count }} token</span>
          </div>
        </div>
      </div>

      <div v-if="expanded.has(item.audit_ulid)" class="trace-row__detail">
        <p class="trace-row__message">{{ item.resp_message || '—' }}</p>
        <p v-if="item.request_body" class="trace-row__payload">
          <span class="trace-row__payload-label">请求</span>
          {{ summaryOf(item.request_body) }}
        </p>
        <p v-if="item.resp_body" class="trace-row__payload">
          <span class="trace-row__payload-label">响应</span>
          {{ summaryOf(item.resp_body) }}
        </p>
        <p v-if="!item.request_body && !item.resp_body" class="nw-hint">该 span 未采样请求/响应报文</p>
        <ElButton size="small" text type="primary" @click="emit('open-detail', item)">打开完整详情</ElButton>
      </div>
    </li>
  </ol>
</template>

<style lang="less" scoped>
  @import './auditWaterfall.less';
</style>

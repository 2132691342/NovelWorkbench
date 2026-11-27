<!-- 审计链���表格：一行一次入口请求，展示链路级别、根动作、模型调用、最慢一步与总耗时。 -->
<script setup lang="ts">
  import { ElEmpty, ElPagination, ElSkeleton, ElTable, ElTableColumn } from 'element-plus'
  import type { NWAuditTraceSummaryVo } from '@/api/admin'
  import { formatTime } from '@/utils/time'
  import { asRow } from '@/utils/table'

  /** 表格数据（父级已按「只看慢链路」过滤） */
  defineProps<{
    items: NWAuditTraceSummaryVo[]
    total: number
    pageNum: number
    pageSize: number
    loading: boolean
  }>()

  /** 行点击打开链路瀑布 */
  const emit = defineEmits<{
    (event: 'open-trace', traceId: string): void
    (event: 'change-page', pageNum: number): void
  }>()

  /** 级别色点样式 */
  const levelDotClass = (levelCode: string) => `audit-page__dot is-${levelCode || 'info'}`

  /** 耗时分级：5 秒以上标慢，1 秒以上标警示 */
  const latencyTone = (latencyMs: number) =>
    latencyMs >= 5000 ? 'is-slow' : latencyMs >= 1000 ? 'is-warn' : ''
</script>

<template>
  <ElSkeleton v-if="loading && items.length === 0" :rows="8" animated />
  <ElTable
    v-else
    :data="items"
    class="audit-page__table"
    row-key="trace_id"
    :row-class-name="({ row }) => (row.log_level_code === 'error' ? 'is-error-row' : '')"
    @row-click="(row: unknown) => emit('open-trace', asRow<NWAuditTraceSummaryVo>(row).trace_id)"
  >
    <ElTableColumn label="时间" width="168">
      <template #default="{ row }">
        <span class="audit-page__time">{{ formatTime(row.started_at) }}</span>
      </template>
    </ElTableColumn>
    <ElTableColumn label="级别" width="66" align="center">
      <template #default="{ row }">
        <span :class="levelDotClass(row.log_level_code)" :title="row.log_level_code" />
      </template>
    </ElTableColumn>
    <ElTableColumn label="动作" min-width="220">
      <template #default="{ row }">
        <div class="audit-page__action-cell">
          <span class="audit-page__action-name">{{ row.root_action_name || row.root_action_code }}</span>
          <span class="nw-hint audit-page__action-code">{{ row.root_request_path }}</span>
        </div>
      </template>
    </ElTableColumn>
    <ElTableColumn label="操作人" width="112" show-overflow-tooltip>
      <template #default="{ row }">{{ row.user_name || '—' }}</template>
    </ElTableColumn>
    <ElTableColumn label="调用" width="118">
      <template #default="{ row }">
        <span class="audit-page__span-count">{{ row.span_count }} 步</span>
        <span v-if="row.ai_call_count > 0" class="nw-hint audit-page__ai-count">
          <Icon icon="mdi:robot-outline" :size="12" />
          {{ row.ai_call_count }} 次 · {{ row.token_count }} token
        </span>
        <span v-else class="nw-hint audit-page__ai-count">无模型调用</span>
      </template>
    </ElTableColumn>
    <ElTableColumn label="最慢一步" min-width="190">
      <template #default="{ row }">
        <span v-if="row.slowest_action_name" class="audit-page__slowest">
          {{ row.slowest_action_name }}
          <span class="nw-hint">{{ row.slowest_span_ms }} ms</span>
        </span>
        <span v-else class="nw-hint">—</span>
      </template>
    </ElTableColumn>
    <ElTableColumn label="耗时" width="112" align="right">
      <template #default="{ row }">
        <span :class="['audit-page__latency-value', latencyTone(row.total_latency_ms)]">
          {{ row.total_latency_ms }} ms
        </span>
      </template>
    </ElTableColumn>
    <ElTableColumn label="结果" width="96" align="center">
      <template #default="{ row }">
        <span :class="row.http_status >= 400 ? 'audit-page__fail' : 'audit-page__ok'">
          {{ row.http_status || '—' }}
        </span>
      </template>
    </ElTableColumn>
    <template #empty>
      <ElEmpty description="没有符合条件的链路，试试放宽时间范围或清除筛选" :image-size="72" />
    </template>
  </ElTable>

  <div class="audit-page__pager">
    <ElPagination
      :current-page="pageNum"
      :page-size="pageSize"
      :total="total"
      layout="prev, pager, next, total"
      background
      @current-change="(next: number) => emit('change-page', next)"
    />
  </div>
</template>

<style lang="less" scoped>
  @import './auditPage.less';
</style>

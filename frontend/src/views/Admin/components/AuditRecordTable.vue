<!-- 审计记录表格：一行一个 span，用于精确排查单条记录（链路视图之外的补充视角）。 -->
<script setup lang="ts">
  import { ElButton, ElEmpty, ElPagination, ElTable, ElTableColumn, ElTooltip } from 'element-plus'
  import type { NWAuditVo } from '@/api/admin'
  import { formatTime } from '@/utils/time'
  import { asRow } from '@/utils/table'

  /** 表格数据 */
  defineProps<{
    items: NWAuditVo[]
    total: number
    pageNum: number
    pageSize: number
    loading: boolean
  }>()

  /** 行点击打开详情；链路按钮打开整条链路 */
  const emit = defineEmits<{
    (event: 'open-detail', row: NWAuditVo): void
    (event: 'open-trace', traceId: string): void
    (event: 'change-page', pageNum: number): void
  }>()

  /** 级别色点样式 */
  const levelDotClass = (levelCode: string) => `audit-page__dot is-${levelCode || 'info'}`

  /** 耗时分级 */
  const latencyTone = (latencyMs: number) =>
    latencyMs >= 5000 ? 'is-slow' : latencyMs >= 1000 ? 'is-warn' : ''
</script>

<template>
  <ElTable
    :data="items"
    v-loading="loading"
    class="audit-page__table"
    row-key="audit_ulid"
    :row-class-name="({ row }) => (row.log_level_code === 'error' ? 'is-error-row' : '')"
    @row-click="(row: unknown) => emit('open-detail', asRow<NWAuditVo>(row))"
  >
    <ElTableColumn label="时间" width="168">
      <template #default="{ row }">
        <span class="audit-page__time">{{ formatTime(row.created_at) }}</span>
      </template>
    </ElTableColumn>
    <ElTableColumn label="级别" width="66" align="center">
      <template #default="{ row }">
        <span :class="levelDotClass(row.log_level_code)" :title="row.log_level_code" />
      </template>
    </ElTableColumn>
    <ElTableColumn label="动作" min-width="200">
      <template #default="{ row }">
        <div class="audit-page__action-cell">
          <span class="audit-page__action-name">{{ row.action_name || row.action_code }}</span>
          <span class="nw-hint audit-page__action-code">{{ row.action_code }}</span>
        </div>
      </template>
    </ElTableColumn>
    <ElTableColumn label="类型" width="104">
      <template #default="{ row }">
        <span class="nw-badge">{{ row.span_type_code || 'http' }}</span>
      </template>
    </ElTableColumn>
    <ElTableColumn label="操作人" width="112" show-overflow-tooltip>
      <template #default="{ row }">{{ row.user_name || '—' }}</template>
    </ElTableColumn>
    <ElTableColumn label="结果" min-width="180">
      <template #default="{ row }">
        <span :class="row.resp_code === 'SUCCESS' ? 'audit-page__ok' : 'audit-page__fail'">
          {{ row.resp_code || '—' }}
        </span>
        <span class="nw-hint audit-page__resp">{{ row.resp_message }}</span>
      </template>
    </ElTableColumn>
    <ElTableColumn label="耗时" width="96" align="right">
      <template #default="{ row }">
        <span :class="['audit-page__latency-value', latencyTone(row.latency_ms)]">{{ row.latency_ms }} ms</span>
      </template>
    </ElTableColumn>
    <ElTableColumn label="链路" width="66" align="center">
      <template #default="{ row }">
        <ElTooltip content="查看整条链路" placement="top">
          <ElButton v-if="row.trace_id" size="small" text type="primary" @click.stop="emit('open-trace', row.trace_id)">
            <Icon icon="mdi:transit-connection-variant" :size="15" />
          </ElButton>
          <span v-else class="nw-hint">—</span>
        </ElTooltip>
      </template>
    </ElTableColumn>
    <template #empty>
      <ElEmpty description="没有符合条件的记录" :image-size="72" />
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

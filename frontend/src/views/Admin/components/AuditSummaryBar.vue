<!-- 审计摘要条：留存天数、总条数、下次/上次清理时间与「立即清理」。 -->
<script setup lang="ts">
  import { ElButton, ElMessage, ElMessageBox, ElSkeleton } from 'element-plus'
  import { ref } from 'vue'
  import { purgeAdminAudit, type NWAuditRetentionVo } from '@/api/admin'
  import { formatTime } from '@/utils/time'

  const props = defineProps<{
    /** 留存策略现状（null 表示尚未取到） */
    retention: NWAuditRetentionVo | null
    /** 摘要条加载态 */
    loading: boolean
  }>()

  const emit = defineEmits<{
    /** 清理完成或需要刷新时通知父级 */
    purged: [retention: NWAuditRetentionVo]
    retry: []
  }>()

  const purging = ref(false)

  /** 立即清理：二次确认后执行，期间按钮 loading 且禁用重复点击 */
  const purgeNow = async () => {
    if (purging.value) return
    const retentionDays = props.retention?.retention_days ?? 30
    try {
      await ElMessageBox.confirm(
        `将删除超过留存期（${retentionDays} 天）的审计记录与登录日志，删除后不可恢复。`,
        '确认立即清理',
        { type: 'warning', confirmButtonText: '确认清理', cancelButtonText: '取消' }
      )
    } catch {
      return
    }
    purging.value = true
    try {
      const result = await purgeAdminAudit()
      const auditCount = result.purged_count ?? 0
      const loginCount = result.login_log_purged_count ?? 0
      ElMessage.success(
        auditCount + loginCount > 0
          ? `已清理审计 ${auditCount} 条、登录日志 ${loginCount} 条`
          : '没有过期记录'
      )
      emit('purged', result)
    } finally {
      purging.value = false
    }
  }
</script>

<template>
  <section class="audit-summary">
    <ElSkeleton v-if="loading && !retention" :rows="1" animated class="audit-summary__skeleton" />

    <template v-else-if="retention">
      <div class="audit-summary__item">
        <span class="audit-summary__label">留存</span>
        <strong class="audit-summary__value">{{ retention.retention_days }} 天</strong>
      </div>
      <div class="audit-summary__item">
        <span class="audit-summary__label">审计 / 登录日志</span>
        <strong class="audit-summary__value">
          {{ retention.total_count.toLocaleString() }} / {{ (retention.login_log_total_count ?? 0).toLocaleString() }}
        </strong>
      </div>
      <div class="audit-summary__item">
        <span class="audit-summary__label">下次清理</span>
        <strong class="audit-summary__value is-time">{{ formatTime(retention.next_purge_at) }}</strong>
        <span class="nw-hint">每日 {{ String(retention.purge_hour).padStart(2, '0') }}:00</span>
      </div>
      <div class="audit-summary__item">
        <span class="audit-summary__label">上次清理</span>
        <strong class="audit-summary__value is-time">
          {{ retention.last_purged_at ? formatTime(retention.last_purged_at) : '本次启动后尚未执行' }}
        </strong>
      </div>
      <div class="audit-summary__spacer" />
      <ElButton type="primary" plain size="small" :loading="purging" @click="purgeNow()">
        <Icon icon="mdi:delete-sweep-outline" :size="15" />
        <span class="audit-summary__btn-text">立即清理</span>
      </ElButton>
    </template>

    <div v-else class="audit-summary__failed">
      <span class="nw-hint">留存策略读取失败（可能是 Manticore 不可用）</span>
      <ElButton size="small" text type="primary" @click="emit('retry')">重试</ElButton>
    </div>
  </section>
</template>

<style lang="less" scoped>
  .audit-summary {
    display: flex;
    flex-wrap: wrap;
    gap: 18px;
    align-items: center;
    padding: 10px 14px;
    background: var(--nw-surface-subtle);
    border: 1px solid var(--nw-border-soft);
    border-radius: var(--nw-radius-md);

    &__skeleton {
      width: 100%;
    }

    &__item {
      display: flex;
      gap: 6px;
      align-items: baseline;
    }

    &__label {
      font-size: 12px;
      color: var(--el-text-color-secondary);
    }

    &__value {
      font-size: 13px;
      font-weight: 700;
      font-variant-numeric: tabular-nums;
      color: var(--nw-ink);

      &.is-time {
        font-weight: 600;
      }
    }

    &__spacer {
      flex: 1;
    }

    &__btn-text {
      margin-left: 4px;
    }

    &__failed {
      display: flex;
      gap: 8px;
      align-items: center;
    }
  }
</style>

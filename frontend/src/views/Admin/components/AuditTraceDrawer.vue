<!-- 链路瀑布抽屉：展示同一 trace_id 的全部 span 的时间瀑布（左=开始偏移，右=持续时长），
     顶部汇总总耗时、步骤数、模型调用与异常数，并标出最慢一步与失败步骤。 -->
<script setup lang="ts">
  import { computed, ref, watch } from 'vue'
  import { ElButton, ElDrawer, ElEmpty } from 'element-plus'
  import { getAdminAuditTrace, type NWAuditDetailVo, type NWAuditTraceVo } from '@/api/admin'
  import { formatTime } from '@/utils/time'
  import AuditWaterfallTimeline from './AuditWaterfallTimeline.vue'

  const props = defineProps<{
    visible: boolean
    traceId: string
  }>()

  const emit = defineEmits<{
    'update:visible': [visible: boolean]
    'open-detail': [row: NWAuditDetailVo]
  }>()

  const loading = ref(false)
  const failed = ref(false)
  const trace = ref<NWAuditTraceVo | null>(null)
  /** 展开的 span（按 audit_ulid 记录，支持多开） */
  const expanded = ref<Set<string>>(new Set())

  /** span 的结束时刻（开始时刻 + 耗时） */
  const spanEndMs = (item: NWAuditDetailVo) => new Date(item.created_at).getTime() + item.latency_ms

  /** 瀑布总宽口径：最后一条结束时刻减首条开始时刻（至少 1ms 防除零） */
  const totalLatency = computed(() => {
    const list = trace.value?.item_list ?? []
    if (list.length === 0) return 0
    const startAt = new Date(list[0].created_at).getTime()
    const endAt = Math.max(...list.map(spanEndMs))
    return Math.max(1, endAt - startAt)
  })

  /** 最慢 span 的 ULID（顶部「慢在哪一步」高亮） */
  const slowestUlid = computed(() => {
    const list = trace.value?.item_list ?? []
    if (list.length < 2) return ''
    return list.reduce((slow, item) => (item.latency_ms > slow.latency_ms ? item : slow), list[0]).audit_ulid
  })

  /** 最慢一步的文案 */
  const slowestText = computed(() => {
    const list = trace.value?.item_list ?? []
    if (list.length < 2) return ''
    const slowest = list.find((item) => item.audit_ulid === slowestUlid.value)
    if (!slowest) return ''
    return `${slowest.action_name || slowest.action_code} · ${slowest.latency_ms} ms`
  })

  /** 汇总指标：模型调用次数与 token、失败步骤数、警示步骤数 */
  const summary = computed(() => {
    const list = trace.value?.item_list ?? []
    return {
      aiCallCount: list.filter((item) => item.span_type_code === 'ai_call').length,
      tokenCount: list.reduce((sum, item) => sum + (item.token_count || 0), 0),
      errorCount: list.filter((item) => item.log_level_code === 'error').length,
      warnCount: list.filter((item) => item.log_level_code === 'warn').length
    }
  })

  /** 展开/收起某个 span */
  const toggle = (auditUlid: string) => {
    const next = new Set(expanded.value)
    if (next.has(auditUlid)) next.delete(auditUlid)
    else next.add(auditUlid)
    expanded.value = next
  }

  const load = async () => {
    if (!props.traceId) return
    loading.value = true
    failed.value = false
    expanded.value = new Set()
    try {
      trace.value = await getAdminAuditTrace(props.traceId)
    } catch {
      trace.value = null
      failed.value = true
    } finally {
      loading.value = false
    }
  }

  watch(
    () => [props.visible, props.traceId],
    ([visible]) => {
      if (visible) void load()
    }
  )
</script>

<template>
  <ElDrawer
    :model-value="visible"
    title="链路瀑布"
    size="820px"
    destroy-on-close
    @update:model-value="emit('update:visible', $event)"
  >
    <div v-loading="loading" class="audit-trace">
      <div v-if="failed" class="audit-trace__failed">
        <ElEmpty description="链路读取失败，可能是记录已过期清理" :image-size="64" />
        <ElButton size="small" type="primary" plain @click="load()">重试</ElButton>
      </div>

      <template v-else-if="trace && trace.total > 0">
        <header class="audit-trace__head">
          <div class="audit-trace__stat">
            <span class="audit-trace__stat-label">总耗时</span>
            <strong class="audit-trace__stat-value">{{ totalLatency }} ms</strong>
          </div>
          <div class="audit-trace__stat">
            <span class="audit-trace__stat-label">步骤</span>
            <strong class="audit-trace__stat-value">{{ trace.total }}</strong>
          </div>
          <div class="audit-trace__stat">
            <span class="audit-trace__stat-label">模型调用</span>
            <strong class="audit-trace__stat-value">
              {{ summary.aiCallCount }}
              <small v-if="summary.tokenCount > 0">{{ summary.tokenCount }} token</small>
            </strong>
          </div>
          <div class="audit-trace__stat">
            <span class="audit-trace__stat-label">异常</span>
            <strong
              class="audit-trace__stat-value"
              :class="{ 'is-error': summary.errorCount > 0, 'is-slow': summary.errorCount === 0 && summary.warnCount > 0 }"
            >
              {{ summary.errorCount }} 错 / {{ summary.warnCount }} 警
            </strong>
          </div>
          <div v-if="slowestText" class="audit-trace__stat is-wide">
            <span class="audit-trace__stat-label">慢在哪一步</span>
            <strong class="audit-trace__stat-value is-slow">{{ slowestText }}</strong>
          </div>
        </header>

        <p class="nw-hint audit-trace__trace-id">
          链路 ID：{{ trace.trace_id }}
          <span v-if="trace.item_list[0]"> · 开始于 {{ formatTime(trace.item_list[0].created_at) }}</span>
        </p>

        <!-- 时间刻度：左端为链路起点，右端为结束 -->
        <div class="audit-trace__axis">
          <span>0</span>
          <span>{{ Math.round(totalLatency / 2) }} ms</span>
          <span>{{ totalLatency }} ms</span>
        </div>

        <AuditWaterfallTimeline
          :item-list="trace.item_list"
          :total-latency="totalLatency"
          :slowest-ulid="slowestUlid"
          :expanded="expanded"
          @toggle="toggle"
          @open-detail="(row) => emit('open-detail', row)"
        />

        <p class="nw-hint audit-trace__footer">
          横轴为整条链路的时间轴：色块起点 = 开始时刻，长度 = 耗时。红块是失败步骤，橙框是最慢一步。
        </p>
      </template>

      <ElEmpty v-else-if="!loading" description="这条链路没有审计记录" :image-size="64" />
    </div>
  </ElDrawer>
</template>

<style lang="less" scoped>
  @import './auditWaterfall.less';
</style>

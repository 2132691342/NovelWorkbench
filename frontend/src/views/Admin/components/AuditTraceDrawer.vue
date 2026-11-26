<!-- 链路时间线抽屉：把同一 trace_id 的全部审计行按时间升序画成一条调用链。 -->
<script setup lang="ts">
  import { ref, watch } from 'vue'
  import { getAdminAuditTrace, type NWAuditTraceVo, type NWAuditVo } from '@/api/admin'
  import { formatTime } from '@/utils/time'

  const props = defineProps<{
    visible: boolean
    traceId: string
  }>()

  const emit = defineEmits<{
    'update:visible': [visible: boolean]
    'open-detail': [row: NWAuditVo]
  }>()

  const loading = ref(false)
  const trace = ref<NWAuditTraceVo | null>(null)

  const levelTagType = (levelCode: string) =>
    levelCode === 'error' ? 'danger' : levelCode === 'warn' ? 'warning' : 'success'

  const load = async () => {
    if (!props.traceId) return
    loading.value = true
    try {
      trace.value = await getAdminAuditTrace(props.traceId)
    } catch {
      trace.value = null
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
    title="链路时间线"
    size="640px"
    destroy-on-close
    @update:model-value="emit('update:visible', $event)"
  >
    <div v-loading="loading" class="audit-trace">
      <template v-if="trace">
        <p class="audit-trace__head">
          链路 <span class="audit-trace__mono">{{ trace.trace_id }}</span>
          共 {{ trace.total }} 条记录
        </p>
        <ul class="audit-trace__list">
          <li
            v-for="item in trace.item_list"
            :key="item.audit_ulid"
            class="audit-trace__item"
            @click="emit('open-detail', item)"
          >
            <span :class="['audit-trace__dot', `is-${item.log_level_code}`]" />
            <div class="audit-trace__body">
              <div class="audit-trace__line">
                <strong>{{ item.action_code }}</strong>
                <ElTag :type="levelTagType(item.log_level_code)" size="small" effect="plain">
                  {{ item.resp_code || '—' }}
                </ElTag>
                <span class="audit-trace__latency">{{ item.latency_ms }} ms</span>
              </div>
              <p class="audit-trace__meta">
                {{ formatTime(item.created_at) }} · {{ item.user_name || '系统' }} ·
                {{ item.request_method }} {{ item.request_path }}
              </p>
              <p v-if="item.resp_message" class="audit-trace__message">{{ item.resp_message }}</p>
            </div>
          </li>
        </ul>
      </template>
      <ElEmpty v-else-if="!loading" description="该链路没有记录" />
    </div>
  </ElDrawer>
</template>

<style lang="less" scoped>
  .audit-trace {
    display: flex;
    flex-direction: column;
    gap: 12px;

    &__head {
      margin: 0;
      font-size: 12px;
      color: var(--el-text-color-secondary);
    }

    &__mono {
      font-family: var(--nw-font-mono);
      font-size: 12px;
    }

    &__list {
      display: flex;
      flex-direction: column;
      gap: 10px;
      padding: 0;
      margin: 0;
      list-style: none;
    }

    &__item {
      display: flex;
      gap: 10px;
      cursor: pointer;
    }

    &__dot {
      flex: none;
      width: 8px;
      height: 8px;
      margin-top: 6px;
      background: var(--nw-info);
      border-radius: 50%;

      &.is-warn {
        background: var(--nw-warning);
      }

      &.is-error {
        background: var(--nw-danger);
      }
    }

    &__body {
      flex: 1;
      min-width: 0;
      padding: 8px 10px;
      background: var(--nw-surface);
      border: 1px solid var(--nw-border-soft);
      border-radius: var(--nw-radius-md);
      transition: border-color 0.15s ease;

      &:hover {
        border-color: var(--el-color-primary);
      }
    }

    &__line {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      align-items: center;

      strong {
        font-family: var(--nw-font-mono);
        font-size: 12px;
        color: var(--nw-ink);
      }
    }

    &__latency {
      margin-left: auto;
      font-size: 11px;
      color: var(--el-text-color-placeholder);
    }

    &__meta {
      margin: 4px 0 0;
      font-size: 11.5px;
      color: var(--el-text-color-secondary);
    }

    &__message {
      margin: 4px 0 0;
      font-size: 12px;
      color: var(--el-text-color-regular);
    }
  }
</style>

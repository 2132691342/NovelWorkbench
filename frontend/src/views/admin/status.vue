<!--
  运行状态页：进程信息（版本/启动时间/运行时长）+ 依赖健康（PostgreSQL/Redis/RabbitMQ/MinIO/Manticore/模型）
  + AI 能力就绪结论。手动刷新，避免频繁探测打爆依赖。
-->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { getSystemStatus, type NWSystemStatusVo } from '@/api/model'
import NWButton from '@/components/NWButton.vue'
import NWCard from '@/components/NWCard.vue'
import NWEmpty from '@/components/NWEmpty.vue'
import NWLoading from '@/components/NWLoading.vue'
import { formatTime } from '@/utils/time'

const { t } = useI18n()

const loading = ref(false)
const statusVo = ref<NWSystemStatusVo | null>(null)
const loadError = ref(false)
/** 加载运行状态 */
async function loadStatus(): Promise<void> {
  loading.value = true
  loadError.value = false
  try {
    statusVo.value = await getSystemStatus()
  } catch {
    loadError.value = true
    statusVo.value = null
  } finally {
    loading.value = false
  }
}

/** 运行时长文案（天/小时/分钟） */
const uptimeText = computed(() => {
  const uptimeSec = statusVo.value?.server_uptime_sec ?? 0
  const days = Math.floor(uptimeSec / 86400)
  const hours = Math.floor((uptimeSec % 86400) / 3600)
  const minutes = Math.floor((uptimeSec % 3600) / 60)
  if (days > 0) {
    return `${days}d ${hours}h ${minutes}m`
  }
  if (hours > 0) {
    return `${hours}h ${minutes}m`
  }
  return `${minutes}m`
})

/** 状态展示文案 */
function statusText(statusCode: string): string {
  switch (statusCode) {
    case 'up':
      return t('status.up')
    case 'degraded':
      return t('status.degraded')
    default:
      return t('status.down')
  }
}

onMounted(() => {
  void loadStatus()
})
</script>

<template>
  <div class="nw-page status-page">
    <header class="status-page__header">
      <div>
        <h2 class="status-page__title">{{ t('status.title') }}</h2>
        <p class="status-page__subtitle">{{ t('status.subtitle') }}</p>
      </div>
      <NWButton :loading="loading" @click="loadStatus">{{ t('status.refresh') }}</NWButton>
    </header>

    <NWLoading v-if="loading && !statusVo" />
    <div v-else-if="loadError" class="status-page__error">
      <NWEmpty :description_text="t('status.loadFailed')" />
      <NWButton type="primary" @click="loadStatus">{{ t('status.retry') }}</NWButton>
    </div>
    <template v-else-if="statusVo">
      <div class="status-page__summary">
        <NWCard :title="t('status.processTitle')">
          <dl class="status-dl">
            <div>
              <dt>{{ t('status.buildVersion') }}</dt>
              <dd>{{ statusVo.build_version_text }}</dd>
            </div>
            <div>
              <dt>{{ t('status.goVersion') }}</dt>
              <dd>{{ statusVo.go_version_text }}</dd>
            </div>
            <div>
              <dt>{{ t('status.startedAt') }}</dt>
              <dd>{{ new Date(formatTime(statusVo.server_started_at)).toLocaleString() }}</dd>
            </div>
            <div>
              <dt>{{ t('status.uptime') }}</dt>
              <dd>{{ uptimeText }}</dd>
            </div>
          </dl>
        </NWCard>

        <NWCard :title="t('status.aiCapabilityTitle')">
          <div class="status-ai">
            <span class="status-ai__item" :class="statusVo.chat_ready ? 'is-up' : 'is-down'">
              {{ t('status.chatReady') }}：{{ statusVo.chat_ready ? t('status.up') : t('status.down') }}
            </span>
            <span class="status-ai__item" :class="statusVo.chat_ready ? 'is-up' : 'is-degraded'">
              {{ t('status.searchReady') }}：{{
                statusVo.chat_ready ? t('status.up') : t('status.degraded')
              }}
            </span>
            <p class="status-ai__hint">{{ statusVo.hint_text }}</p>
          </div>
        </NWCard>
      </div>

      <NWCard :title="t('status.dependencyTitle')">
        <el-table :data="statusVo.dependency_list" size="small">
          <el-table-column prop="dependency_name" :label="t('status.dependencyName')" min-width="240" />
          <el-table-column :label="t('status.dependencyStatus')" width="140">
            <template #default="{ row }">
              <span class="status-badge" :class="`is-${row.status_code}`">
                {{ statusText(row.status_code) }}
              </span>
            </template>
          </el-table-column>
          <el-table-column :label="t('status.latency')" width="120">
            <template #default="{ row }">
              {{ row.latency_ms > 0 ? `${row.latency_ms} ms` : '-' }}
            </template>
          </el-table-column>
          <el-table-column prop="message_text" :label="t('status.message')" min-width="280" />
        </el-table>
      </NWCard>
    </template>
  </div>
</template>

<style scoped>
.status-page__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}

.status-page__title {
  margin: 0 0 4px;
  font-size: 20px;
  color: var(--text-main);
}

.status-page__subtitle {
  margin: 0;
  color: var(--text-secondary);
  font-size: 13px;
}

.status-page__summary {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 16px;
  margin-bottom: 16px;
}

.status-dl {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px 16px;
  margin: 0;
}

.status-dl dt {
  color: var(--text-placeholder);
  font-size: 12px;
  margin-bottom: 2px;
}

.status-dl dd {
  margin: 0;
  color: var(--text-main);
  font-size: 14px;
}

.status-ai {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.status-ai__item {
  padding: 8px 12px;
  border-radius: var(--radius-base);
  font-size: 13px;
  background-color: var(--bg-page);
}

.status-ai__item.is-up {
  background-color: var(--success-bg);
  color: var(--success-text);
}

.status-ai__item.is-degraded {
  background-color: var(--warning-bg);
  color: var(--warning-text);
}

.status-ai__item.is-down {
  background-color: var(--error-bg);
  color: var(--error-text);
}


.status-ai__hint {
  margin: 0;
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.6;
}

.status-badge {
  padding: 2px 10px;
  border-radius: var(--radius-pill);
  font-size: 12px;
  background-color: var(--border-light);
  color: var(--text-secondary);
}

.status-badge.is-up {
  background-color: var(--success-border);
  color: var(--success-text);
}

.status-badge.is-degraded {
  background-color: var(--warning-border);
  color: var(--warning-text);
}

.status-badge.is-down {
  background-color: var(--error-border);
  color: var(--error-text);
}

.status-page__error {
  background-color: var(--bg-card);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-lg);
  padding: 8px 0 24px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}
</style>

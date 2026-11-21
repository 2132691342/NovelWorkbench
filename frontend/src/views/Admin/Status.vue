<!-- 运行状态页：手动探测各中间件与模型网关的可用性。 -->
<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import { ElButton, ElCard, ElEmpty } from 'element-plus'
  import { getSystemStatus } from '@/api/model'
  import type { NWDependencyHealthVo, NWSystemStatusVo } from '@/api/model'
  import { formatTime } from '@/utils/time'

  const status = ref<NWSystemStatusVo | null>(null)
  const loading = ref(false)

  const healthBadge = (statusCode: string) => {
    if (statusCode === 'up') return 'success'
    if (statusCode === 'degraded') return 'warning'
    return 'danger'
  }

  const healthLabel = (statusCode: string) => {
    if (statusCode === 'up') return '正常'
    if (statusCode === 'degraded') return '降级'
    return '异常'
  }

  /** 运行时长：天/时/分 */
  const uptimeText = computed(() => {
    const totalSec = status.value?.server_uptime_sec ?? 0
    const days = Math.floor(totalSec / 86400)
    const hours = Math.floor((totalSec % 86400) / 3600)
    const minutes = Math.floor((totalSec % 3600) / 60)
    const parts: string[] = []
    if (days > 0) parts.push(`${days} 天`)
    if (hours > 0) parts.push(`${hours} 小时`)
    parts.push(`${minutes} 分钟`)
    return parts.join(' ')
  })

  const dependencyList = computed<NWDependencyHealthVo[]>(() => status.value?.dependency_list ?? [])

  const load = async () => {
    loading.value = true
    try {
      status.value = await getSystemStatus()
    } catch {
      // 失败提示由请求层统一弹出
    } finally {
      loading.value = false
    }
  }

  onMounted(() => {
    void load()
  })
</script>

<template>
  <div class="nw-page">
    <div class="nw-toolbar">
      <ElButton type="primary" plain :loading="loading" @click="load()">
        <Icon icon="mdi:refresh" :size="15" />
        <span class="admin-status__btn-text">刷新状态</span>
      </ElButton>
      <div class="admin-status__spacer" />
      <span v-if="status" class="nw-hint">采集于 {{ formatTime(status.server_started_at) }} 启动的实例</span>
    </div>

    <div v-loading="loading" class="admin-status__summary">
      <ElCard shadow="never" class="admin-status__card">
        <span class="admin-status__label">构建版本</span>
        <strong class="admin-status__value">{{ status?.build_version_text || '—' }}</strong>
      </ElCard>
      <ElCard shadow="never" class="admin-status__card">
        <span class="admin-status__label">Go 运行时</span>
        <strong class="admin-status__value">{{ status?.go_version_text || '—' }}</strong>
      </ElCard>
      <ElCard shadow="never" class="admin-status__card">
        <span class="admin-status__label">已运行</span>
        <strong class="admin-status__value">{{ uptimeText }}</strong>
      </ElCard>
      <ElCard shadow="never" class="admin-status__card">
        <span class="admin-status__label">对话链路</span>
        <span
          class="nw-badge"
          :class="status?.chat_ready ? 'nw-badge--success' : 'nw-badge--danger'"
        >
          {{ status?.chat_ready ? '可用' : '不可用' }}
        </span>
      </ElCard>
    </div>

    <p v-if="status?.hint_text" class="nw-hint">{{ status.hint_text }}</p>

    <div class="admin-status__deps">
      <ElCard
        v-for="item in dependencyList"
        :key="item.dependency_code"
        shadow="never"
        class="dep-card"
      >
        <div class="dep-card__head">
          <span class="dep-card__name">{{ item.dependency_name || item.dependency_code }}</span>
          <span class="nw-badge" :class="`nw-badge--${healthBadge(item.status_code)}`">
            {{ healthLabel(item.status_code) }}
          </span>
        </div>
        <p class="dep-card__message">{{ item.message_text || '—' }}</p>
        <div class="dep-card__meta">
          <span class="dep-card__code">{{ item.dependency_code }}</span>
          <span class="nw-hint">{{ item.latency_ms }} ms</span>
        </div>
      </ElCard>

      <ElEmpty
        v-if="!loading && dependencyList.length === 0"
        description="暂无依赖健康数据"
        :image-size="70"
      />
    </div>
  </div>
</template>

<style lang="less" scoped>
  .admin-status {
    &__btn-text {
      margin-left: 4px;
    }

    &__spacer {
      flex: 1;
    }

    &__summary {
      display: grid;
      gap: 14px;
      grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
    }

    &__card {
      border-radius: var(--nw-radius-lg);

      :deep(.el-card__body) {
        display: flex;
        flex-direction: column;
        gap: 6px;
        align-items: flex-start;
        padding: 14px 18px;
      }
    }

    &__label {
      font-size: 12px;
      color: var(--el-text-color-secondary);
    }

    &__value {
      font-size: 16px;
      font-weight: 700;
      color: var(--nw-ink);
      word-break: break-all;
    }

    &__deps {
      display: grid;
      gap: 14px;
      min-height: 120px;
      grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
    }
  }

  .dep-card {
    border-radius: var(--nw-radius-lg);

    &__head {
      display: flex;
      gap: 10px;
      align-items: center;
      justify-content: space-between;
    }

    &__name {
      font-size: 14px;
      font-weight: 650;
      color: var(--nw-ink);
    }

    &__message {
      margin: 8px 0 10px;
      font-size: 12.5px;
      line-height: 1.6;
      color: var(--nw-text-secondary);
    }

    &__meta {
      display: flex;
      gap: 12px;
      align-items: center;
      justify-content: space-between;
    }

    &__code {
      padding: 1px 6px;
      font-family: var(--el-font-family-mono, monospace);
      font-size: 11.5px;
      color: var(--nw-text-muted);
      background: var(--nw-surface-subtle);
      border-radius: var(--nw-radius-sm);
    }
  }
</style>

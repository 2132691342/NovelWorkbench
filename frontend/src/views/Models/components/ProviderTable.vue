<!-- 供应商表：展示供应商与连通性测试入口。 -->
<script setup lang="ts">
  import {
    ElButton,
    ElCard,
    ElEmpty,
    ElSwitch,
    ElTable,
    ElTableColumn,
    ElTag
  } from 'element-plus'
  import type { NWModelProviderVo } from '@/api/model'
  import type { ModelTestResult } from './types'

  defineProps<{
    providers: NWModelProviderVo[]
    loading: boolean
    testingUlid: string
    testMap: Record<string, ModelTestResult>
    activeProviderUlid: string
  }>()

  const emit = defineEmits<{
    create: []
    edit: [row: NWModelProviderVo]
    remove: [row: NWModelProviderVo]
    test: [row: NWModelProviderVo]
    select: [providerUlid: string]
    toggle: [row: NWModelProviderVo, enabled: boolean]
  }>()

  const testStatusType = (statusCode: string) =>
    statusCode === 'success' ? 'success' : statusCode === 'failed' ? 'danger' : 'info'

  const testStatusText = (statusCode: string) =>
    statusCode === 'success' ? '连通' : statusCode === 'failed' ? '不通' : '未测试'
</script>

<template>
  <ElCard shadow="never" class="section">
    <template #header>
      <div class="section__head">
        <span class="section__title">模型供应商</span>
        <span class="nw-hint">一套网关地址与密钥，可挂多个模型</span>
        <div class="section__spacer" />
        <ElButton type="primary" plain size="small" @click="emit('create')">
          <Icon icon="mdi:plus" :size="14" />
          <span class="ml-4px">新建供应商</span>
        </ElButton>
      </div>
    </template>

    <ElTable :data="providers" v-loading="loading" size="small">
      <ElTableColumn label="名称" min-width="140">
        <template #default="{ row }">
          <div class="cell-main">
            <span class="cell-main__title">{{ row.provider_name }}</span>
            <span v-if="row.remark_text" class="cell-main__sub">{{ row.remark_text }}</span>
          </div>
        </template>
      </ElTableColumn>
      <ElTableColumn label="协议" width="110">
        <template #default="{ row }">
          <ElTag
            size="small"
            effect="plain"
            :type="row.protocol_code === 'openai' ? 'success' : 'warning'"
          >
            {{ row.protocol_code }}
          </ElTag>
        </template>
      </ElTableColumn>
      <ElTableColumn label="网关地址" min-width="200" show-overflow-tooltip>
        <template #default="{ row }">{{ row.base_url }}</template>
      </ElTableColumn>
      <ElTableColumn label="密钥" width="130">
        <template #default="{ row }">
          <span v-if="row.api_key_configured" class="cell-code">{{ row.api_key_masked }}</span>
          <span v-else class="nw-hint">未配置</span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="模型" width="90">
        <template #default="{ row }">{{ row.enabled_model_count }}/{{ row.model_count }}</template>
      </ElTableColumn>
      <ElTableColumn label="最近测试" width="180">
        <template #default="{ row }">
          <div class="cell-main">
            <ElTag size="small" effect="plain" :type="testStatusType(row.last_test_status_code)">
              {{ testStatusText(row.last_test_status_code) }}
              <template v-if="testMap[row.provider_ulid]">
                · {{ testMap[row.provider_ulid].latency_ms }} ms
              </template>
            </ElTag>
            <span class="cell-main__sub">
              {{ testMap[row.provider_ulid]?.message_text || row.last_test_message || '尚未测试' }}
            </span>
          </div>
        </template>
      </ElTableColumn>
      <ElTableColumn label="启用" width="80">
        <template #default="{ row }">
          <ElSwitch
            :model-value="row.enabled_flag"
            size="small"
            @update:model-value="emit('toggle', row as NWModelProviderVo, Boolean($event))"
          />
        </template>
      </ElTableColumn>
      <ElTableColumn label="操作" width="250" fixed="right">
        <template #default="{ row }">
          <ElButton size="small" text :loading="testingUlid === row.provider_ulid" @click="emit('test', row as NWModelProviderVo)">
            测试
          </ElButton>
          <ElButton size="small" text @click="emit('select', row.provider_ulid)">
            {{ activeProviderUlid === row.provider_ulid ? '全部模型' : '查看模型' }}
          </ElButton>
          <ElButton size="small" text @click="emit('edit', row as NWModelProviderVo)">编辑</ElButton>
          <ElButton size="small" text type="danger" @click="emit('remove', row as NWModelProviderVo)">删除</ElButton>
        </template>
      </ElTableColumn>
      <template #empty>
        <ElEmpty description="还没有供应商，先接入一个网关" :image-size="70" />
      </template>
    </ElTable>
  </ElCard>
</template>

<style lang="less" scoped>
  .section {
    border-radius: var(--nw-radius-lg);

    &__head {
      display: flex;
      gap: 12px;
      align-items: center;
    }

    &__title {
      font-size: 14px;
      font-weight: 650;
      color: var(--nw-ink);
    }

    &__spacer {
      flex: 1;
    }
  }

  .cell-main {
    display: flex;
    flex-direction: column;
    gap: 2px;

    &__title {
      font-weight: 600;
      color: var(--nw-ink);
    }

    &__sub {
      overflow: hidden;
      font-size: 11px;
      color: var(--nw-text-muted);
      text-overflow: ellipsis;
      white-space: nowrap;
    }
  }

  .cell-code {
    font-family: var(--el-font-family-mono, monospace);
    font-size: 11px;
    color: var(--nw-text-secondary);
  }
</style>

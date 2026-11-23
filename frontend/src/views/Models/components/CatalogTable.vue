<!-- 模型目录表：维护模型条目与默认模型。 -->
<script setup lang="ts">
  import { ElButton, ElCard, ElEmpty, ElTable, ElTableColumn, ElTag } from 'element-plus'
  import type { NWModelCatalogVo, NWModelProviderVo } from '@/api/model'
  import type { ModelTestResult } from './types'

  defineProps<{
    models: NWModelCatalogVo[]
    providers: NWModelProviderVo[]
    loading: boolean
    testingUlid: string
    testMap: Record<string, ModelTestResult>
    activeProvider: NWModelProviderVo | null
  }>()

  const emit = defineEmits<{
    create: []
    edit: [row: NWModelCatalogVo]
    remove: [row: NWModelCatalogVo]
    test: [row: NWModelCatalogVo]
    setDefault: [row: NWModelCatalogVo]
    grant: [row: NWModelCatalogVo]
    clearFilter: []
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
        <span class="section__title">模型目录</span>
        <span class="nw-hint">
          {{ activeProvider ? `已筛选：${activeProvider.provider_name}` : '全部供应商的模型' }}
        </span>
        <div class="section__spacer" />
        <ElButton v-if="activeProvider" size="small" text @click="emit('clearFilter')">
          清除筛选
        </ElButton>
        <ElButton
          type="primary"
          plain
          size="small"
          :disabled="providers.length === 0"
          @click="emit('create')"
        >
          <Icon icon="mdi:plus" :size="14" />
          <span class="ml-4px">新建模型</span>
        </ElButton>
      </div>
    </template>

    <ElTable :data="models" v-loading="loading" size="small">
      <ElTableColumn label="模型" min-width="170">
        <template #default="{ row }">
          <div class="cell-main">
            <span class="cell-main__title">
              {{ row.model_label || row.model_name }}
              <ElTag v-if="row.default_flag" size="small" type="primary" effect="dark">默认</ElTag>
            </span>
            <span class="cell-main__sub">{{ row.model_name }}</span>
          </div>
        </template>
      </ElTableColumn>
      <ElTableColumn label="供应商" width="150" show-overflow-tooltip>
        <template #default="{ row }">
          <span>{{ row.provider_name }}</span>
          <ElTag v-if="!row.provider_enabled" size="small" type="info" effect="plain">已停用</ElTag>
        </template>
      </ElTableColumn>
      <ElTableColumn label="max_tokens" width="110">
        <template #default="{ row }">{{ row.max_tokens || '—' }}</template>
      </ElTableColumn>
      <ElTableColumn label="上下文窗口" width="120">
        <template #default="{ row }">
          {{ row.context_window_count ? row.context_window_count.toLocaleString() : '—' }}
        </template>
      </ElTableColumn>
      <ElTableColumn label="授权" min-width="200">
        <template #default="{ row }">
          <div class="cell-tags">
            <ElTag
              v-for="roleName in row.role_name_list"
              :key="`role-${roleName}`"
              size="small"
              effect="plain"
              type="success"
            >
              角色 · {{ roleName }}
            </ElTag>
            <ElTag
              v-for="userLabel in row.user_label_list"
              :key="`user-${userLabel}`"
              size="small"
              effect="plain"
              type="warning"
            >
              用户 · {{ userLabel }}
            </ElTag>
            <span
              v-if="row.role_name_list.length === 0 && row.user_label_list.length === 0"
              class="nw-hint"
            >
              仅管理员
            </span>
          </div>
        </template>
      </ElTableColumn>
      <ElTableColumn label="最近测试" width="150">
        <template #default="{ row }">
          <div class="cell-main">
            <ElTag size="small" effect="plain" :type="testStatusType(row.last_test_status_code)">
              {{ testStatusText(row.last_test_status_code) }}
              <template v-if="testMap[row.model_ulid]">
                · {{ testMap[row.model_ulid].latency_ms }} ms
              </template>
            </ElTag>
            <span class="cell-main__sub">
              {{ testMap[row.model_ulid]?.message_text || row.last_test_message || '尚未测试' }}
            </span>
          </div>
        </template>
      </ElTableColumn>
      <ElTableColumn label="操作" width="270" fixed="right">
        <template #default="{ row }">
          <ElButton size="small" text :loading="testingUlid === row.model_ulid" @click="emit('test', row as NWModelCatalogVo)">
            测试
          </ElButton>
          <ElButton size="small" text :disabled="row.default_flag" @click="emit('setDefault', row as NWModelCatalogVo)">
            设默认
          </ElButton>
          <ElButton size="small" text @click="emit('grant', row as NWModelCatalogVo)">授权使用</ElButton>
          <ElButton size="small" text @click="emit('edit', row as NWModelCatalogVo)">编辑</ElButton>
          <ElButton size="small" text type="danger" @click="emit('remove', row as NWModelCatalogVo)">删除</ElButton>
        </template>
      </ElTableColumn>
      <template #empty>
        <ElEmpty
          :description="providers.length === 0 ? '先创建供应商，再添加模型' : '该范围下还没有模型'"
          :image-size="70"
        />
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
      display: flex;
      gap: 6px;
      align-items: center;
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

  .cell-tags {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
  }
</style>

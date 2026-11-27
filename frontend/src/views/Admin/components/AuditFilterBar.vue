<!-- 审计筛选区：时间快捷、级别 chips（带条数）、span 类型、动作码、关键字与重置。 -->
<script setup lang="ts">
  import { ElButton, ElDatePicker, ElInput, ElOption, ElSelect } from 'element-plus'
  import type { NWAuditActionOptionVo, NWAuditListReqVo } from '@/api/admin'

  /** 筛选条件（父级持有，本组件直接改字段后请求刷新） */
  type AuditQuery = Omit<NWAuditListReqVo, 'page_num' | 'page_size'>

  const props = defineProps<{
    query: AuditQuery
    quickRange: string
    dateRange: [string, string] | null
    quickRanges: readonly { value: string; label: string }[]
    levelChips: readonly { value: string; label: string }[]
    spanOptions: readonly { value: string; label: string }[]
    actionOptions: NWAuditActionOptionVo[]
    levelCounts: Record<string, number>
    optionsLoading: boolean
    total: number
  }>()

  const emit = defineEmits<{
    refresh: []
    reset: []
    'update:quickRange': [value: string]
    'update:dateRange': [value: [string, string] | null]
  }>()

  const levelDotClass = (levelCode: string) => `audit-filter__dot is-${levelCode || 'info'}`
</script>

<template>
  <div class="nw-toolbar audit-filter">
    <div class="audit-filter__group">
      <button
        v-for="item in quickRanges"
        :key="item.value"
        type="button"
        class="audit-filter__chip"
        :class="{ 'is-active': quickRange === item.value }"
        @click="emit('update:quickRange', item.value); emit('refresh')"
      >
        {{ item.label }}
      </button>
    </div>

    <ElDatePicker
      v-if="quickRange === 'custom'"
      :model-value="dateRange"
      type="daterange"
      value-format="YYYY-MM-DD"
      start-placeholder="开始日期"
      end-placeholder="结束日期"
      class="audit-filter__date"
      @update:model-value="(value) => { emit('update:dateRange', value as [string, string] | null); emit('refresh') }"
    />

    <div class="audit-filter__group">
      <button
        v-for="item in levelChips"
        :key="item.value"
        type="button"
        class="audit-filter__chip"
        :class="{ 'is-active': query.log_level_code === item.value }"
        @click="query.log_level_code = item.value; emit('refresh')"
      >
        <span :class="levelDotClass(item.value)" />
        {{ item.label }}
        <span class="audit-filter__count">{{ levelCounts[item.value] ?? 0 }}</span>
      </button>
    </div>

    <ElSelect
      v-model="query.span_type_code"
      class="audit-filter__span"
      placeholder="全部类型"
      @change="emit('refresh')"
    >
      <ElOption v-for="item in spanOptions" :key="item.value" :label="item.label" :value="item.value" />
    </ElSelect>

    <ElSelect
      v-model="query.action_code"
      class="audit-filter__action"
      placeholder="全部动作"
      clearable
      filterable
      :loading="optionsLoading"
      @change="emit('refresh')"
    >
      <ElOption
        v-for="option in actionOptions"
        :key="option.action_code"
        :label="`${option.action_name || option.action_code}（${option.total}）`"
        :value="option.action_code"
      />
    </ElSelect>

    <ElInput
      v-model="query.keyword_text"
      placeholder="搜索动作 / 用户 / 路径 / 说明"
      clearable
      class="audit-filter__search"
      @keyup.enter="emit('refresh')"
      @clear="emit('refresh')"
    />

    <ElButton type="primary" plain @click="emit('refresh')">
      <Icon icon="mdi:magnify" :size="15" />
      <span class="audit-filter__btn-text">查询</span>
    </ElButton>
    <ElButton text @click="emit('reset')">重置</ElButton>
    <div class="audit-filter__spacer" />
    <span class="nw-hint">共 {{ total }} 条</span>
  </div>
</template>

<style lang="less" scoped>
  .audit-filter {
    gap: 8px;

    &__group {
      display: flex;
      flex-wrap: wrap;
      gap: 6px;
      align-items: center;
    }

    &__chip {
      display: inline-flex;
      gap: 6px;
      align-items: center;
      height: 30px;
      padding: 0 10px;
      font-size: 12px;
      color: var(--el-text-color-regular);
      cursor: pointer;
      background: var(--nw-surface);
      border: 1px solid var(--nw-border);
      border-radius: 999px;
      transition:
        color 0.15s ease,
        border-color 0.15s ease,
        background 0.15s ease;

      &:hover {
        color: var(--el-color-primary);
        border-color: var(--el-color-primary);
      }

      &.is-active {
        color: var(--el-color-primary);
        background: var(--nw-primary-soft);
        border-color: var(--el-color-primary);
      }
    }

    &__count {
      font-size: 11px;
      font-variant-numeric: tabular-nums;
      color: var(--el-text-color-placeholder);
    }

    &__dot {
      width: 7px;
      height: 7px;
      border-radius: 50%;
      background: var(--nw-success);

      &.is-error {
        background: var(--nw-danger);
      }

      &.is-warn {
        background: var(--nw-warning);
      }
    }

    &__date {
      width: 240px;
    }

    &__span {
      width: 130px;
    }

    &__action {
      width: 200px;
    }

    &__search {
      width: 240px;
    }

    &__btn-text {
      margin-left: 4px;
    }

    &__spacer {
      flex: 1;
    }
  }
</style>

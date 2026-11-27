<!-- 系统设置页：分组卡片、按类型渲染控件、未保存提示与逐项失败定位。 -->
<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import { onBeforeRouteLeave } from 'vue-router'
  import {
    ElAlert,
    ElButton,
    ElCard,
    ElEmpty,
    ElInput,
    ElInputNumber,
    ElMessage,
    ElMessageBox,
    ElSwitch
  } from 'element-plus'
  import { get, post } from '@/api/http'
  import type { NWSettingItemVo } from '@/api/contracts'

  const items = ref<NWSettingItemVo[]>([])
  const drafts = ref<Record<string, string>>({})
  const loading = ref(false)
  const saving = ref(false)
  /** 保存失败的键（用于在对应卡片上标红，不笼统 toast） */
  const failedKeys = ref<string[]>([])

  /** 分组：按配置键归类；未登记分组的键归入「其他」 */
  const groupMeta: Array<{ code: string; name: string; hint: string; keys: string[] }> = [
    {
      code: 'knowledge',
      name: '知识库检索',
      hint: '影响知识库问答时的召回条数，条数越多上下文越丰富、耗时与费用也越高。',
      keys: ['rag_top_k']
    },
    {
      code: 'writing',
      name: '写作目标',
      hint: '写作台与工作台的每日字数目标，用于进度提示。',
      keys: ['daily_word_goal']
    },
    {
      code: 'audit',
      name: '审计留存',
      hint: '审计日志与登录日志的保留窗口，到期由后台任务自动清理。',
      keys: ['audit_retention_days', 'audit_purge_hour']
    }
  ]

  const groups = computed(() => {
    const usedKeys = new Set<string>()
    const grouped = groupMeta
      .map((meta) => {
        const list = items.value.filter((item) => meta.keys.includes(item.setting_key))
        list.forEach((item) => usedKeys.add(item.setting_key))
        return { ...meta, list }
      })
      .filter((group) => group.list.length > 0)
    const others = items.value.filter((item) => !usedKeys.has(item.setting_key))
    if (others.length > 0) {
      grouped.push({
        code: 'other',
        name: '其他配置',
        hint: '未归入预置分组的运行时配置项。',
        keys: [],
        list: others
      })
    }
    return grouped
  })

  const dirtyItems = computed(() =>
    items.value.filter((item) => (drafts.value[item.setting_key] ?? '') !== item.setting_value)
  )
  const dirtyCount = computed(() => dirtyItems.value.length)

  const isDirty = (item: NWSettingItemVo) =>
    (drafts.value[item.setting_key] ?? '') !== item.setting_value

  /** 就地校验：整数类型要求落在后端声明区间内，布尔/文本不做范围判断 */
  const valueError = (item: NWSettingItemVo): string => {
    if (item.setting_value_type !== 'int') return ''
    const raw = drafts.value[item.setting_key] ?? ''
    if (raw.trim() === '') return '不能为空'
    if (!/^-?\d+$/.test(raw.trim())) return '请输入整数'
    const numeric = Number(raw)
    // setting_value_max 为 0 表示后端未声明区间；声明了区间则下限按 setting_value_min 严格判断（允许 0）
    if (!item.setting_value_max) return ''
    if (numeric < item.setting_value_min) return `不能小于 ${item.setting_value_min}`
    if (numeric > item.setting_value_max) return `不能大于 ${item.setting_value_max}`
    return ''
  }

  const rangeHint = (item: NWSettingItemVo) => {
    if (item.setting_value_type !== 'int') return ''
    if (!item.setting_value_max) return ''
    return `取值 ${item.setting_value_min} ~ ${item.setting_value_max}`
  }

  const load = async () => {
    loading.value = true
    try {
      items.value = await get<NWSettingItemVo[]>('/setting/list')
      drafts.value = Object.fromEntries(
        items.value.map((item) => [item.setting_key, item.setting_value])
      )
      failedKeys.value = []
    } catch {
      items.value = []
      drafts.value = {}
    } finally {
      loading.value = false
    }
  }

  /** 保存全部：逐项提交，失败项单独记录并保留其草稿，成功项同步基准值 */
  const saveAll = async () => {
    if (dirtyCount.value === 0) {
      ElMessage.info('没有需要保存的修改')
      return
    }
    const invalid = dirtyItems.value.filter((item) => valueError(item) !== '')
    if (invalid.length > 0) {
      ElMessage.warning(`有 ${invalid.length} 项取值不合法，请先修正`)
      return
    }

    saving.value = true
    failedKeys.value = []
    const changedList = [...dirtyItems.value]
    const failed: Array<{ key: string; message: string }> = []
    try {
      for (const item of changedList) {
        try {
          const saved = await post<NWSettingItemVo>('/setting/update', {
            setting_key: item.setting_key,
            setting_value: drafts.value[item.setting_key] ?? ''
          })
          // 用后端回传的同构视图替换基准，避免再拉一次列表
          const index = items.value.findIndex((row) => row.setting_key === saved.setting_key)
          if (index >= 0) items.value[index] = saved
        } catch (error) {
          const message = error instanceof Error ? error.message : '保存失败'
          failed.push({ key: item.setting_key, message })
        }
      }
      if (failed.length === 0) {
        ElMessage.success(`已保存 ${changedList.length} 项配置`)
        return
      }
      failedKeys.value = failed.map((entry) => entry.key)
      const summary = failed
        .slice(0, 3)
        .map((entry) => `${entry.key}：${entry.message}`)
        .join('；')
      ElMessage.error(
        failed.length > 3
          ? `${failed.length} 项保存失败（前 3 项）${summary}`
          : `${failed.length} 项保存失败：${summary}`
      )
    } finally {
      saving.value = false
    }
  }

  const resetDrafts = () => {
    drafts.value = Object.fromEntries(
      items.value.map((item) => [item.setting_key, item.setting_value])
    )
    failedKeys.value = []
  }

  const restoreDefault = (item: NWSettingItemVo) => {
    if (!item.setting_default_text) return
    drafts.value[item.setting_key] = item.setting_default_text
  }

  const setBoolean = (settingKey: string, checked: boolean) => {
    drafts.value[settingKey] = checked ? 'true' : 'false'
  }

  /** 离开页面前拦截未保存改动 */
  onBeforeRouteLeave(async () => {
    if (dirtyCount.value === 0) return true
    try {
      await ElMessageBox.confirm(
        `有 ${dirtyCount.value} 项配置尚未保存，离开将丢失这些修改。确定离开吗？`,
        '未保存的修改',
        { type: 'warning', confirmButtonText: '放弃修改并离开', cancelButtonText: '留在本页' }
      )
      return true
    } catch {
      return false
    }
  })

  onMounted(() => {
    void load()
  })
</script>

<template>
  <div class="nw-page">
    <div class="nw-toolbar">
      <ElButton type="primary" :loading="saving" :disabled="dirtyCount === 0" @click="saveAll()">
        <Icon icon="mdi:check" :size="15" />
        <span class="settings__btn-text">保存全部</span>
      </ElButton>
      <ElButton :disabled="dirtyCount === 0" @click="resetDrafts()">放弃修改</ElButton>
      <ElButton :loading="loading" @click="load()">
        <Icon icon="mdi:refresh" :size="15" />
      </ElButton>
      <div class="settings__spacer" />
      <span class="nw-hint">{{ dirtyCount > 0 ? `${dirtyCount} 项待保存` : '配置已同步' }}</span>
    </div>

    <ElAlert
      v-if="dirtyCount > 0"
      type="warning"
      :closable="false"
      show-icon
      :title="`有 ${dirtyCount} 项修改尚未保存`"
      description="点击「保存全部」提交；离开本页会提示确认，未保存的改动将丢失。"
    />

    <ElAlert
      v-else
      type="info"
      :closable="false"
      show-icon
      title="运行时可调参数"
      description="改动保存后立即生效，无需重启服务；数值型配置请保持在标注区间内。"
    />

    <div v-loading="loading" class="settings__groups">
      <ElCard v-for="group in groups" :key="group.code" shadow="never" class="setting-group">
        <template #header>
          <div class="setting-group__head">
            <span class="setting-group__name">{{ group.name }}</span>
            <span class="nw-badge nw-badge--neutral">{{ group.list.length }}</span>
          </div>
        </template>

        <p class="setting-group__hint">{{ group.hint }}</p>

        <div
          v-for="item in group.list"
          :key="item.setting_key"
          class="setting-row"
          :class="{ 'is-dirty': isDirty(item), 'is-failed': failedKeys.includes(item.setting_key) }"
        >
          <div class="setting-row__label">
            <span class="setting-row__desc">{{ item.setting_desc || item.setting_key }}</span>
            <div class="setting-row__meta">
              <code class="setting-row__key">{{ item.setting_key }}</code>
              <span v-if="rangeHint(item)" class="nw-hint">{{ rangeHint(item) }}</span>
              <span v-if="item.setting_default_text" class="nw-hint">
                默认 {{ item.setting_default_text }}
              </span>
            </div>
            <p v-if="valueError(item)" class="setting-row__error">{{ valueError(item) }}</p>
            <p v-else-if="failedKeys.includes(item.setting_key)" class="setting-row__error">
              该项保存失败，请修正后重试
            </p>
          </div>

          <div class="setting-row__control">
            <ElSwitch
              v-if="item.setting_value_type === 'bool'"
              :model-value="drafts[item.setting_key] === 'true'"
              @update:model-value="setBoolean(item.setting_key, Boolean($event))"
            />
            <ElInputNumber
              v-else-if="item.setting_value_type === 'int'"
              :model-value="Number(drafts[item.setting_key] ?? 0)"
              :min="item.setting_value_min || undefined"
              :max="item.setting_value_max || undefined"
              :step="1"
              step-strictly
              controls-position="right"
              class="setting-row__number"
              @update:model-value="
                drafts[item.setting_key] = $event === null || $event === undefined ? '' : String($event)
              "
            />
            <ElInput
              v-else
              v-model="drafts[item.setting_key]"
              class="setting-row__input"
              placeholder="请输入配置值"
            />
            <ElButton
              v-if="item.setting_default_text && isDirty(item)"
              size="small"
              text
              @click="restoreDefault(item)"
            >
              恢复默认
            </ElButton>
          </div>
        </div>
      </ElCard>

      <ElEmpty
        v-if="!loading && groups.length === 0"
        description="暂无可用配置项（后端尚未播种运行时配置）"
      />
    </div>
  </div>
</template>

<style lang="less" scoped>
  .settings {
    &__btn-text {
      margin-left: 4px;
    }

    &__spacer {
      flex: 1;
    }

    &__groups {
      display: grid;
      gap: 14px;
      align-items: start;
      min-height: 120px;
      grid-template-columns: repeat(auto-fill, minmax(400px, 1fr));
    }
  }

  .setting-group {
    border-radius: var(--nw-radius-lg);

    &__head {
      display: flex;
      gap: 10px;
      align-items: center;
    }

    &__name {
      font-size: 14px;
      font-weight: 650;
      color: var(--nw-ink);
    }

    &__hint {
      margin-bottom: 4px;
      font-size: 11.5px;
      line-height: 1.6;
      color: var(--el-text-color-secondary);
    }
  }

  .setting-row {
    display: flex;
    gap: 16px;
    align-items: flex-start;
    justify-content: space-between;
    padding: 12px 8px;
    margin: 0 -8px;
    border-bottom: 1px solid var(--nw-border-soft);
    border-radius: var(--nw-radius-sm);
    transition: background-color 0.15s ease;

    &:last-child {
      border-bottom: 0;
    }

    &.is-dirty {
      background: color-mix(in srgb, var(--el-color-primary) 6%, transparent);
    }

    &.is-failed {
      background: color-mix(in srgb, var(--nw-danger) 8%, transparent);
    }

    &__label {
      display: flex;
      flex-direction: column;
      gap: 4px;
      min-width: 0;
    }

    &__desc {
      font-size: 13px;
      color: var(--nw-ink);
    }

    &__meta {
      display: flex;
      flex-wrap: wrap;
      gap: 10px;
      align-items: center;
    }

    &__key {
      font-family: var(--el-font-family-mono, monospace);
      font-size: 11.5px;
      color: var(--el-text-color-placeholder);
    }

    &__error {
      font-size: 11.5px;
      color: var(--nw-danger);
    }

    &__control {
      display: flex;
      gap: 8px;
      align-items: center;
      flex: none;
    }

    &__number {
      width: 150px;
    }

    &__input {
      width: 170px;
    }
  }
</style>

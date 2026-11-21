<!-- 系统设置页：分组编辑运行配置项。 -->
<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import { ElAlert, ElButton, ElCard, ElInput, ElMessage, ElSwitch } from 'element-plus'
  import { get, post } from '@/api/http'
  import type { NWSettingItemVo } from '@/api/contracts'

  const items = ref<NWSettingItemVo[]>([])
  const drafts = ref<Record<string, string>>({})
  const loading = ref(false)
  const saving = ref(false)

  /** 分组：按配置键前缀归类，避免设置项堆成一列 */
  const groupMeta: Array<{ code: string; name: string; keys: string[] }> = [
    { code: 'chat', name: 'AI 对话', keys: ['chat_history_count'] },
    { code: 'knowledge', name: '知识库检索', keys: ['rag_top_k'] },
    { code: 'writing', name: '写作目标', keys: ['daily_word_goal'] }
  ]

  const isBooleanValue = (value: string) => value === 'true' || value === 'false'

  /** 分组后的设置项（未登记分组的键归入「其他」） */
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
      grouped.push({ code: 'other', name: '其他配置', keys: [], list: others })
    }
    return grouped
  })

  const dirtyCount = computed(
    () => items.value.filter((item) => drafts.value[item.setting_key] !== item.setting_value).length
  )

  const load = async () => {
    loading.value = true
    try {
      items.value = await get<NWSettingItemVo[]>('/setting/list')
      drafts.value = Object.fromEntries(items.value.map((item) => [item.setting_key, item.setting_value]))
    } catch {
      items.value = []
      drafts.value = {}
    } finally {
      loading.value = false
    }
  }

  /** 保存全部：逐项提交后端单键更新接口，任一失败即中断并保留未提交草稿 */
  const saveAll = async () => {
    const changed = items.value.filter(
      (item) => drafts.value[item.setting_key] !== item.setting_value
    )
    if (changed.length === 0) {
      ElMessage.info('没有需要保存的修改')
      return
    }
    saving.value = true
    try {
      for (const item of changed) {
        await post<NWSettingItemVo>('/setting/update', {
          setting_key: item.setting_key,
          setting_value: drafts.value[item.setting_key] ?? ''
        })
      }
      ElMessage.success(`已保存 ${changed.length} 项配置`)
      await load()
    } finally {
      saving.value = false
    }
  }

  const resetDrafts = () => {
    drafts.value = Object.fromEntries(items.value.map((item) => [item.setting_key, item.setting_value]))
  }

  const handleSwitchChange = (settingKey: string, checked: boolean) => {
    drafts.value[settingKey] = checked ? 'true' : 'false'
  }

  onMounted(() => {
    void load()
  })
</script>

<template>
  <div class="nw-page">
    <div class="nw-toolbar">
      <ElButton type="primary" :loading="saving" @click="saveAll()">
        <Icon icon="mdi:check" :size="15" />
        <span class="settings__btn-text">保存全部</span>
      </ElButton>
      <ElButton :disabled="dirtyCount === 0" @click="resetDrafts()">撤销修改</ElButton>
      <div class="settings__spacer" />
      <span class="nw-hint">
        {{ dirtyCount > 0 ? `${dirtyCount} 项待保存` : '配置已同步' }}
      </span>
    </div>

    <ElAlert
      type="info"
      :closable="false"
      show-icon
      title="运行时可调参数"
      description="这里的改动立即生效，无需重启服务；数值型配置请保持在说明标注的范围内。"
    />

    <div v-loading="loading" class="settings__groups">
      <ElCard v-for="group in groups" :key="group.code" shadow="never" class="setting-group">
        <template #header>
          <div class="setting-group__head">
            <span class="setting-group__name">{{ group.name }}</span>
            <span class="nw-badge nw-badge--neutral">{{ group.list.length }}</span>
          </div>
        </template>

        <div v-for="item in group.list" :key="item.setting_key" class="setting-row">
          <div class="setting-row__label">
            <span class="setting-row__desc">{{ item.setting_desc || item.setting_key }}</span>
            <code class="setting-row__key">{{ item.setting_key }}</code>
          </div>
          <ElSwitch
            v-if="isBooleanValue(item.setting_value)"
            :model-value="drafts[item.setting_key] === 'true'"
            @update:model-value="handleSwitchChange(item.setting_key, Boolean($event))"
          />
          <ElInput
            v-else
            v-model="drafts[item.setting_key]"
            class="setting-row__input"
            placeholder="请输入配置值"
          >
            <template #append>数值</template>
          </ElInput>
        </div>
      </ElCard>

      <ElAlert
        v-if="!loading && groups.length === 0"
        type="info"
        :closable="false"
        title="暂无可用配置项"
        description="后端尚未播种运行时配置。"
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
      grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
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
  }

  .setting-row {
    display: flex;
    gap: 16px;
    align-items: center;
    justify-content: space-between;
    padding: 12px 0;
    border-bottom: 1px solid var(--nw-border-soft);

    &:last-child {
      border-bottom: 0;
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

    &__key {
      font-family: var(--el-font-family-mono, monospace);
      font-size: 11.5px;
      color: var(--nw-text-muted);
    }

    &__input {
      width: 170px;
      flex: none;
    }
  }
</style>

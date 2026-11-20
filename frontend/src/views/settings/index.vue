<!--
  系统设置：运行时白名单配置，按业务分组展示。
  配置在每次请求时读取，保存后立即生效，无需重启服务。
-->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { get, post } from '@/api/http'
import type { NWSettingItemVo } from '@/api/contracts'
import NWButton from '@/components/NWButton.vue'
import NWIcon from '@/components/NWIcon.vue'
import NWPageHeader from '@/components/NWPageHeader.vue'

const { t } = useI18n()

interface SettingMeta {
  group_code: string
  title_text: string
  desc_text: string
  min_value: number
  max_value: number
  unit_text: string
}

const settingList = ref<NWSettingItemVo[]>([])
const draftMap = ref<Record<string, string>>({})
const listLoading = ref(false)
const savingKey = ref('')
const savingAll = ref(false)

const settingMetaMap = computed<Record<string, SettingMeta>>(() => ({
  chat_history_count: {
    group_code: 'ai',
    title_text: t('settings.chatHistoryTitle'),
    desc_text: t('settings.chatHistoryDesc'),
    min_value: 0,
    max_value: 50,
    unit_text: t('settings.unitMessages'),
  },
  rag_top_k: {
    group_code: 'knowledge',
    title_text: t('settings.ragTopKTitle'),
    desc_text: t('settings.ragTopKDesc'),
    min_value: 1,
    max_value: 20,
    unit_text: t('settings.unitChunks'),
  },
  daily_word_goal: {
    group_code: 'writing',
    title_text: t('settings.dailyGoalTitle'),
    desc_text: t('settings.dailyGoalDesc'),
    min_value: 100,
    max_value: 100000,
    unit_text: t('settings.unitWords'),
  },
}))

const groupMetaList = computed(() => [
  { group_code: 'writing', group_title: t('settings.groupWriting'), group_icon: 'target', group_desc: t('settings.groupWritingDesc') },
  { group_code: 'ai', group_title: t('settings.groupAi'), group_icon: 'sparkles', group_desc: t('settings.groupAiDesc') },
  { group_code: 'knowledge', group_title: t('settings.groupKnowledge'), group_icon: 'database', group_desc: t('settings.groupKnowledgeDesc') },
])

const groupedSettingList = computed(() =>
  groupMetaList.value.map((groupItem) => ({
    ...groupItem,
    item_list: settingList.value.filter(
      (settingItem) =>
        (settingMetaMap.value[settingItem.setting_key]?.group_code ?? 'writing') === groupItem.group_code,
    ),
  })).filter((groupItem) => groupItem.item_list.length > 0),
)

const dirtyKeyList = computed(() =>
  settingList.value
    .filter((settingItem) => draftMap.value[settingItem.setting_key] !== settingItem.setting_value)
    .map((settingItem) => settingItem.setting_key),
)

function metaOf(settingKey: string): SettingMeta {
  return (
    settingMetaMap.value[settingKey] ?? {
      group_code: 'writing',
      title_text: settingKey,
      desc_text: '',
      min_value: 0,
      max_value: 999999,
      unit_text: '',
    }
  )
}

async function loadSettingList(): Promise<void> {
  listLoading.value = true
  try {
    const settingData = await get<NWSettingItemVo[]>('/setting/list')
    settingList.value = Array.isArray(settingData) ? settingData : []
    draftMap.value = Object.fromEntries(
      settingList.value.map((settingItem) => [settingItem.setting_key, settingItem.setting_value]),
    )
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    listLoading.value = false
  }
}

async function saveSetting(settingKey: string): Promise<void> {
  const settingValue = draftMap.value[settingKey]
  if (settingValue === undefined) return
  savingKey.value = settingKey
  try {
    await post<null>('/setting/update', {
      setting_key: settingKey,
      setting_value: String(settingValue),
    })
    const targetItem = settingList.value.find((settingItem) => settingItem.setting_key === settingKey)
    if (targetItem) targetItem.setting_value = String(settingValue)
    ElMessage.success(t('settings.saveSuccess'))
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    savingKey.value = ''
  }
}

async function saveAllSettings(): Promise<void> {
  if (dirtyKeyList.value.length === 0) return
  savingAll.value = true
  const failedKeyList: string[] = []
  for (const settingKey of dirtyKeyList.value) {
    try {
      await post<null>('/setting/update', {
        setting_key: settingKey,
        setting_value: String(draftMap.value[settingKey]),
      })
      const targetItem = settingList.value.find((settingItem) => settingItem.setting_key === settingKey)
      if (targetItem) targetItem.setting_value = String(draftMap.value[settingKey])
    } catch {
      failedKeyList.push(settingKey)
    }
  }
  savingAll.value = false
  if (failedKeyList.length === 0) {
    ElMessage.success(t('settings.saveAllSuccess'))
  } else {
    ElMessage.error(t('settings.saveAllPartial', { count: failedKeyList.length }))
  }
}

function resetDraft(): void {
  draftMap.value = Object.fromEntries(
    settingList.value.map((settingItem) => [settingItem.setting_key, settingItem.setting_value]),
  )
}

onMounted(() => {
  void loadSettingList()
})
</script>

<template>
  <div class="nw-page settings-page">
    <NWPageHeader
      :title_text="t('settings.title')"
      :subtitle_text="t('settings.pageSubtitle')"
      icon_code="sliders"
    >
      <template #actions>
        <NWButton :disabled="dirtyKeyList.length === 0" @click="resetDraft">
          {{ t('settings.resetDraft') }}
        </NWButton>
        <NWButton
          type="primary"
          :loading="savingAll"
          :disabled="dirtyKeyList.length === 0"
          @click="saveAllSettings"
        >
          <NWIcon icon_code="save" :size_px="15" />
          {{ t('settings.saveAll', { count: dirtyKeyList.length }) }}
        </NWButton>
      </template>
    </NWPageHeader>

    <div v-if="listLoading" class="settings-page__skeleton">
      <div v-for="index in 3" :key="index" class="settings-page__skeleton-card" />
    </div>

    <template v-else>
      <section v-for="groupItem in groupedSettingList" :key="groupItem.group_code" class="nw-panel">
        <header class="nw-panel__header">
          <div class="settings-page__group-head">
            <span class="settings-page__group-icon">
              <NWIcon :icon_code="groupItem.group_icon" :size_px="17" />
            </span>
            <div>
              <h3 class="nw-panel__title">{{ groupItem.group_title }}</h3>
              <p class="nw-panel__subtitle">{{ groupItem.group_desc }}</p>
            </div>
          </div>
          <span class="nw-badge nw-badge--success">
            <span class="nw-badge__dot" />
            {{ t('settings.effectiveNow') }}
          </span>
        </header>
        <div class="nw-panel__body settings-page__group-body">
          <article
            v-for="settingItem in groupItem.item_list"
            :key="settingItem.setting_key"
            class="settings-page__row"
            :class="{ 'is-dirty': draftMap[settingItem.setting_key] !== settingItem.setting_value }"
          >
            <div class="settings-page__row-text">
              <h4 class="settings-page__row-title">{{ metaOf(settingItem.setting_key).title_text }}</h4>
              <p class="settings-page__row-desc">{{ metaOf(settingItem.setting_key).desc_text }}</p>
              <span class="settings-page__row-key">{{ settingItem.setting_key }}</span>
            </div>
            <div class="settings-page__row-control">
              <el-input-number
                :model-value="Number(draftMap[settingItem.setting_key] ?? 0)"
                :min="metaOf(settingItem.setting_key).min_value"
                :max="metaOf(settingItem.setting_key).max_value"
                :step="1"
                controls-position="right"
                class="settings-page__input-number"
                @update:model-value="(nextValue: number | undefined) => (draftMap[settingItem.setting_key] = String(nextValue ?? 0))"
              />
              <span class="settings-page__unit">{{ metaOf(settingItem.setting_key).unit_text }}</span>
              <NWButton
                size="small"
                type="primary"
                :loading="savingKey === settingItem.setting_key"
                :disabled="draftMap[settingItem.setting_key] === settingItem.setting_value"
                @click="saveSetting(settingItem.setting_key)"
              >
                {{ t('common.save') }}
              </NWButton>
            </div>
          </article>
        </div>
      </section>

      <section class="nw-panel settings-page__about">
        <div class="nw-panel__body">
          <div class="settings-page__about-icon"><NWIcon icon_code="info" :size_px="18" /></div>
          <div>
            <h4 class="settings-page__about-title">{{ t('settings.aboutTitle') }}</h4>
            <p class="settings-page__about-desc">{{ t('settings.aboutDesc') }}</p>
          </div>
        </div>
      </section>
    </template>
  </div>
</template>

<style scoped>
.settings-page {
  max-width: 1040px;
}

.settings-page__group-head {
  display: flex;
  align-items: flex-start;
  gap: 11px;
}

.settings-page__group-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  flex-shrink: 0;
  border-radius: var(--radius-md);
  background: var(--primary-soft);
  color: var(--primary);
}

.settings-page__group-body {
  padding: 6px 20px;
}

.settings-page__row {
  display: flex;
  align-items: center;
  gap: 20px;
  padding: 15px 0;
  border-bottom: 1px solid var(--border-light);
}

.settings-page__row:last-child {
  border-bottom: none;
}

.settings-page__row.is-dirty .settings-page__row-title {
  color: var(--primary);
}

.settings-page__row-text {
  flex: 1;
  min-width: 0;
}

.settings-page__row-title {
  margin: 0;
  color: var(--text-main);
  font-size: 13.5px;
  font-weight: 600;
}

.settings-page__row-desc {
  margin: 3px 0 4px;
  color: var(--text-placeholder);
  font-size: 11.5px;
  line-height: 1.6;
}

.settings-page__row-key {
  color: var(--text-placeholder);
  font-family: 'JetBrains Mono', Consolas, monospace;
  font-size: 10px;
  opacity: 0.7;
}

.settings-page__row-control {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.settings-page__input-number {
  width: 132px;
}

.settings-page__unit {
  min-width: 44px;
  color: var(--text-placeholder);
  font-size: 11.5px;
}

.settings-page__about .nw-panel__body {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}

.settings-page__about-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  flex-shrink: 0;
  border-radius: var(--radius-md);
  background: var(--info-bg);
  color: var(--info-text);
}

.settings-page__about-title {
  margin: 0 0 3px;
  color: var(--text-main);
  font-size: 13px;
  font-weight: 650;
}

.settings-page__about-desc {
  margin: 0;
  color: var(--text-secondary);
  font-size: 12px;
  line-height: 1.7;
}

.settings-page__skeleton {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.settings-page__skeleton-card {
  height: 160px;
  border-radius: var(--radius-lg);
  background: linear-gradient(90deg, var(--skeleton-base) 25%, var(--skeleton-shine) 37%, var(--skeleton-base) 63%);
  background-size: 400% 100%;
  animation: settings-shimmer 1.3s ease infinite;
}

@keyframes settings-shimmer {
  0% { background-position: 100% 50%; }
  100% { background-position: 0 50%; }
}

@media (max-width: 760px) {
  .settings-page__row {
    flex-direction: column;
    align-items: flex-start;
    gap: 10px;
  }

  .settings-page__row-control {
    width: 100%;
  }
}
</style>

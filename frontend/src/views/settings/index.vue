<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { get, post } from '@/api/http'
import type { NWSettingItemVo } from '@/api/contracts'
import NWButton from '@/components/NWButton.vue'
import NWCard from '@/components/NWCard.vue'
import NWEmpty from '@/components/NWEmpty.vue'
import NWLoading from '@/components/NWLoading.vue'

const { t } = useI18n()

const settingList = ref<NWSettingItemVo[]>([])
const listLoading = ref(false)
const savingKey = ref('')

/** 加载配置列表 */
async function loadSettingList(): Promise<void> {
  listLoading.value = true
  try {
    const settingData = await get<NWSettingItemVo[]>('/setting/list')
    settingList.value = Array.isArray(settingData) ? settingData : []
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    listLoading.value = false
  }
}

/** 保存单条配置 */
async function handleSaveSetting(settingItem: NWSettingItemVo): Promise<void> {
  savingKey.value = settingItem.setting_key
  try {
    await post<null>('/setting/update', {
      setting_key: settingItem.setting_key,
      setting_value: settingItem.setting_value,
    })
    ElMessage.success(t('settings.saveSuccess'))
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    savingKey.value = ''
  }
}

onMounted(() => {
  void loadSettingList()
})
</script>

<template>
  <div class="settings-page">
    <NWCard :title="t('settings.title')">
      <NWLoading v-if="listLoading" />
      <el-table v-else :data="settingList">
        <el-table-column prop="setting_key" :label="t('settings.settingKey')" width="220" />
        <el-table-column :label="t('settings.settingValue')" min-width="240">
          <template #default="{ row }">
            <el-input v-model="row.setting_value" />
          </template>
        </el-table-column>
        <el-table-column prop="setting_desc" :label="t('settings.settingDesc')" min-width="220" />
        <el-table-column :label="t('common.action')" width="120">
          <template #default="{ row }">
            <NWButton
              type="primary"
              size="small"
              :loading="savingKey === row.setting_key"
              @click="handleSaveSetting(row)"
            >
              {{ t('common.save') }}
            </NWButton>
          </template>
        </el-table-column>
        <template #empty>
          <NWEmpty :description_text="t('settings.emptyTip')" />
        </template>
      </el-table>
    </NWCard>
  </div>
</template>

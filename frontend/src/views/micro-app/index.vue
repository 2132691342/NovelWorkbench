<!--
  微前端管理页：登记 Qiankun 子应用入口，按需加载/卸载；配置保存在当前浏览器。
  这里只做宿主侧装载能力，不把外部入口写入业务数据库。
-->
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { loadMicroApp } from 'qiankun'
import NWButton from '@/components/NWButton.vue'
import NWCard from '@/components/NWCard.vue'
import NWEmpty from '@/components/NWEmpty.vue'

interface NWMicroAppItem {
  app_ulid: string
  app_name: string
  app_entry: string
  route_prefix: string
  enabled_flag: boolean
}

interface NWMicroAppForm {
  app_name: string
  app_entry: string
  route_prefix: string
}

const { t } = useI18n()
const storageKey = 'nw_micro_app_registry_v1'
const appList = ref<NWMicroAppItem[]>([])
const dialogVisible = ref(false)
const loadingFlag = ref(false)
const loadedAppUlid = ref('')
const mountRef = ref<HTMLDivElement | null>(null)
const formRef = ref<NWMicroAppForm>({ app_name: '', app_entry: '', route_prefix: '' })

let mountedApp: ReturnType<typeof loadMicroApp> | null = null

const sortedAppList = computed(() =>
  [...appList.value].sort((left, right) => left.app_name.localeCompare(right.app_name)),
)

function readAppList(): NWMicroAppItem[] {
  try {
    const raw = window.localStorage.getItem(storageKey)
    const parsed = raw ? JSON.parse(raw) : []
    return Array.isArray(parsed) ? (parsed as NWMicroAppItem[]) : []
  } catch {
    return []
  }
}

function persistAppList(): void {
  window.localStorage.setItem(storageKey, JSON.stringify(appList.value))
}

function makeAppUlid(): string {
  return `MAP_${Date.now().toString(36).toUpperCase()}_${Math.random().toString(36).slice(2, 8).toUpperCase()}`
}

function openCreateDialog(): void {
  formRef.value = { app_name: '', app_entry: '', route_prefix: '' }
  dialogVisible.value = true
}

function handleCreate(): void {
  const form = formRef.value
  if (!form.app_name.trim()) {
    ElMessage.warning(t('microApp.nameRequired'))
    return
  }
  if (!form.app_entry.trim()) {
    ElMessage.warning(t('microApp.entryRequired'))
    return
  }
  try {
    new URL(form.app_entry.trim())
  } catch {
    ElMessage.warning(t('microApp.entryInvalid'))
    return
  }
  if (!form.route_prefix.trim()) {
    ElMessage.warning(t('microApp.routePrefixRequired'))
    return
  }
  appList.value = [
    ...appList.value,
    {
      app_ulid: makeAppUlid(),
      app_name: form.app_name.trim(),
      app_entry: form.app_entry.trim(),
      route_prefix: form.route_prefix.trim(),
      enabled_flag: true,
    },
  ]
  persistAppList()
  dialogVisible.value = false
  ElMessage.success(t('microApp.saveSuccess'))
}

async function unloadCurrent(): Promise<void> {
  if (!mountedApp) {
    return
  }
  await mountedApp.unmount()
  mountedApp = null
  loadedAppUlid.value = ''
}

async function handleLoad(appItem: NWMicroAppItem): Promise<void> {
  if (!mountRef.value) {
    return
  }
  loadingFlag.value = true
  try {
    await unloadCurrent()
    mountedApp = loadMicroApp(
      {
        name: appItem.app_ulid,
        entry: appItem.app_entry,
        container: mountRef.value,
        props: { route_prefix: appItem.route_prefix },
      },
      { sandbox: true, singular: true },
    )
    await mountedApp.mountPromise
    loadedAppUlid.value = appItem.app_ulid
    ElMessage.success(t('microApp.loadSuccess'))
  } catch {
    mountedApp = null
    loadedAppUlid.value = ''
  } finally {
    loadingFlag.value = false
  }
}

async function handleUnload(): Promise<void> {
  await unloadCurrent()
  ElMessage.success(t('microApp.unloadSuccess'))
}

async function handleDelete(appItem: NWMicroAppItem): Promise<void> {
  try {
    await ElMessageBox.confirm(t('microApp.deleteConfirm'), t('common.confirm'), {
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel'),
      type: 'warning',
    })
  } catch {
    return
  }
  if (loadedAppUlid.value === appItem.app_ulid) {
    await unloadCurrent()
  }
  appList.value = appList.value.filter((item) => item.app_ulid !== appItem.app_ulid)
  persistAppList()
  ElMessage.success(t('microApp.deleteSuccess'))
}

onMounted(() => {
  appList.value = readAppList()
})

onBeforeUnmount(() => {
  void unloadCurrent()
})
</script>

<template>
  <div class="micro-app-page">
    <NWCard :title="t('microApp.title')" :hoverable="false">
      <p class="micro-app-page__subtitle">{{ t('microApp.subtitle') }}</p>
      <NWButton type="primary" size="small" @click="openCreateDialog">
        {{ t('microApp.createApp') }}
      </NWButton>
      <el-table :data="sortedAppList" class="micro-app-page__table">
        <el-table-column prop="app_name" :label="t('microApp.appName')" min-width="160" />
        <el-table-column prop="app_entry" :label="t('microApp.appEntry')" min-width="260" />
        <el-table-column prop="route_prefix" :label="t('microApp.appRoutePrefix')" min-width="140" />
        <el-table-column :label="t('common.action')" width="220">
          <template #default="{ row }">
            <NWButton
              type="primary"
              size="small"
              :loading="loadingFlag && loadedAppUlid !== row.app_ulid"
              @click="handleLoad(row)"
            >
              {{ t('microApp.loadApp') }}
            </NWButton>
            <NWButton size="small" @click="handleUnload">{{ t('microApp.unloadApp') }}</NWButton>
            <el-button link type="danger" size="small" @click="handleDelete(row)">
              {{ t('microApp.deleteApp') }}
            </el-button>
          </template>
        </el-table-column>
        <template #empty>
          <NWEmpty :description_text="t('microApp.emptyTip')" />
        </template>
      </el-table>
      <p class="micro-app-page__tip">
        {{ loadedAppUlid ? t('microApp.mountTip') : t('microApp.unmountedTip') }}
      </p>
      <div ref="mountRef" class="micro-app-page__mount" />
    </NWCard>

    <el-dialog v-model="dialogVisible" :title="t('microApp.createApp')" width="520px">
      <el-form :model="formRef" label-width="120px">
        <el-form-item :label="t('microApp.appName')">
          <el-input v-model="formRef.app_name" maxlength="64" />
        </el-form-item>
        <el-form-item :label="t('microApp.appEntry')">
          <el-input v-model="formRef.app_entry" placeholder="https://example.com/micro-app/" />
        </el-form-item>
        <el-form-item :label="t('microApp.appRoutePrefix')">
          <el-input v-model="formRef.route_prefix" placeholder="/novel-map" />
        </el-form-item>
      </el-form>
      <template #footer>
        <NWButton @click="dialogVisible = false">{{ t('common.cancel') }}</NWButton>
        <NWButton type="primary" @click="handleCreate">{{ t('common.save') }}</NWButton>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped src="./micro-app.css"></style>

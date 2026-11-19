<!--
  通知中心：按已读状态与类型筛选，支持批量已读、单条已读与删除。
  未读数与顶栏铃铛共用 notification store。
-->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import {
  deleteNotification,
  listNotifications,
  markNotificationsRead,
  type NWNotificationVo,
} from '@/api/notification'
import { useNotificationStore } from '@/stores/notification'
import NWButton from '@/components/NWButton.vue'
import NWCard from '@/components/NWCard.vue'
import NWEmpty from '@/components/NWEmpty.vue'
import NWLoading from '@/components/NWLoading.vue'

const { t } = useI18n()
const router = useRouter()
const notificationStore = useNotificationStore()

const activeReadStatus = ref<'all' | 'unread' | 'read'>('all')
const typeFilter = ref('')
const keywordText = ref('')
const listLoading = ref(false)
const notificationList = ref<NWNotificationVo[]>([])
const total = ref(0)
const pageNum = ref(1)
const pageSize = ref(10)

const readStatusTabList = computed(() => [
  { read_status_code: 'all' as const, label: t('notifications.tabAll') },
  { read_status_code: 'unread' as const, label: t('notifications.tabUnread') },
  { read_status_code: 'read' as const, label: t('notifications.tabRead') },
])

/** 通知类型展示文案 */
function typeLabel(typeCode: string): string {
  switch (typeCode) {
    case 'task':
      return t('notifications.typeTask')
    case 'account':
      return t('notifications.typeAccount')
    default:
      return t('notifications.typeSystem')
  }
}

/** 通知类型标签样式 */
function typeTagType(typeCode: string): 'success' | 'warning' | 'info' {
  if (typeCode === 'task') {
    return 'warning'
  }
  if (typeCode === 'account') {
    return 'success'
  }
  return 'info'
}

function formatTime(timeMs: number): string {
  return new Date(timeMs).toLocaleString()
}

/** 加载通知分页 */
async function loadNotificationList(): Promise<void> {
  listLoading.value = true
  try {
    const pageData = await listNotifications({
      page_num: pageNum.value,
      page_size: pageSize.value,
      read_status_code: activeReadStatus.value,
      notification_type_code: typeFilter.value || undefined,
      keyword_text: keywordText.value.trim() || undefined,
    })
    notificationList.value = pageData.items ?? []
    total.value = pageData.total ?? 0
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    listLoading.value = false
  }
}

function handleSearch(): void {
  pageNum.value = 1
  void loadNotificationList()
}

function handleReadStatusChange(): void {
  pageNum.value = 1
  void loadNotificationList()
}

/** 单条已读 */
async function handleRead(notificationItem: NWNotificationVo): Promise<void> {
  if (notificationItem.read_flag) {
    return
  }
  try {
    await markNotificationsRead([notificationItem.notification_ulid])
    notificationItem.read_flag = true
    await notificationStore.refreshUnread()
  } catch {
    // 失败提示由 http 层统一弹出
  }
}

/** 单条删除 */
async function handleDelete(notificationItem: NWNotificationVo): Promise<void> {
  try {
    await ElMessageBox.confirm(t('notifications.deleteConfirm'), t('common.confirm'), {
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel'),
      type: 'warning',
    })
  } catch {
    return
  }
  try {
    await deleteNotification(notificationItem.notification_ulid)
    ElMessage.success(t('notifications.deleteSuccess'))
    if (notificationList.value.length === 1 && pageNum.value > 1) {
      pageNum.value -= 1
    }
    await loadNotificationList()
    await notificationStore.refreshAll()
  } catch {
    // 失败提示由 http 层统一弹出
  }
}

/** 全部已读 */
async function handleReadAll(): Promise<void> {
  try {
    await notificationStore.markAllRead()
    ElMessage.success(t('notifications.markAllSuccess'))
    await loadNotificationList()
  } catch {
    // 失败提示由 http 层统一弹出
  }
}

/** 打开通知关联页面 */
function handleOpenLink(notificationItem: NWNotificationVo): void {
  if (!notificationItem.link_url) {
    return
  }
  void handleRead(notificationItem)
  void router.push(notificationItem.link_url)
}

function handlePageChange(page: number): void {
  pageNum.value = page
  void loadNotificationList()
}

function handleSizeChange(size: number): void {
  pageSize.value = size
  pageNum.value = 1
  void loadNotificationList()
}

onMounted(() => {
  void loadNotificationList()
  void notificationStore.refreshAll()
})
</script>

<template>
  <div class="nw-page notification-page">
    <NWCard :hoverable="false">
      <template #header>
        <div class="notification-page__header">
          <div>
            <h3 class="nw-card__title">{{ t('notifications.title') }}</h3>
            <p class="notification-page__subtitle">{{ t('notifications.subtitle') }}</p>
          </div>
          <NWButton :disabled="notificationStore.unread_count === 0" @click="handleReadAll">
            {{ t('notifications.markAllRead') }}
          </NWButton>
        </div>
      </template>

      <div class="notification-page__toolbar">
        <el-radio-group v-model="activeReadStatus" @change="handleReadStatusChange">
          <el-radio-button
            v-for="readStatusTab in readStatusTabList"
            :key="readStatusTab.read_status_code"
            :value="readStatusTab.read_status_code"
          >
            {{ readStatusTab.label }}
          </el-radio-button>
        </el-radio-group>
        <el-select
          v-model="typeFilter"
          class="notification-page__filter"
          :placeholder="t('notifications.typeAll')"
          clearable
          @change="handleSearch"
        >
          <el-option :label="t('notifications.typeSystem')" value="system" />
          <el-option :label="t('notifications.typeTask')" value="task" />
          <el-option :label="t('notifications.typeAccount')" value="account" />
        </el-select>
        <el-input
          v-model="keywordText"
          class="notification-page__keyword"
          :placeholder="t('notifications.keywordPlaceholder')"
          clearable
          @keyup.enter="handleSearch"
        />
        <NWButton type="primary" @click="handleSearch">{{ t('admin.search') }}</NWButton>
      </div>

      <NWLoading v-if="listLoading" />
      <template v-else-if="notificationList.length">
        <div class="notification-page__list">
          <article
            v-for="notificationItem in notificationList"
            :key="notificationItem.notification_ulid"
            class="notification-page__item"
            :class="{ 'is-unread': !notificationItem.read_flag }"
          >
            <span class="notification-page__dot" />
            <div class="notification-page__content">
              <div class="notification-page__title-row">
                <strong class="notification-page__title">
                  {{ notificationItem.notification_title_text }}
                </strong>
                <el-tag size="small" effect="light" :type="typeTagType(notificationItem.notification_type_code)">
                  {{ typeLabel(notificationItem.notification_type_code) }}
                </el-tag>
                <span class="notification-page__time">{{ formatTime(notificationItem.created_at_ms) }}</span>
              </div>
              <p class="notification-page__body">{{ notificationItem.notification_body_text }}</p>
              <div class="notification-page__actions">
                <NWButton
                  v-if="notificationItem.link_url"
                  type="text"
                  size="small"
                  @click="handleOpenLink(notificationItem)"
                >
                  {{ t('notifications.openLink') }}
                </NWButton>
                <NWButton
                  v-if="!notificationItem.read_flag"
                  type="text"
                  size="small"
                  @click="handleRead(notificationItem)"
                >
                  {{ t('notifications.markRead') }}
                </NWButton>
                <NWButton type="danger-text" size="small" @click="handleDelete(notificationItem)">
                  {{ t('common.delete') }}
                </NWButton>
              </div>
            </div>
          </article>
        </div>
        <el-pagination
          v-if="total > 0"
          class="notification-page__pagination"
          layout="total, prev, pager, next, sizes"
          :total="total"
          :current-page="pageNum"
          :page-size="pageSize"
          :page-sizes="[10, 20, 50]"
          @current-change="handlePageChange"
          @size-change="handleSizeChange"
        />
      </template>
      <NWEmpty v-else :description_text="t('notifications.emptyTip')" />
    </NWCard>
  </div>
</template>

<style scoped>
.notification-page {
  max-width: 980px;
  margin: 0 auto;
}

.notification-page__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  width: 100%;
}

.notification-page__subtitle {
  margin: 4px 0 0;
  color: var(--text-secondary);
  font-size: 12.5px;
}

.notification-page__toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 14px;
}

.notification-page__filter {
  width: 150px;
}

.notification-page__keyword {
  width: 220px;
}

.notification-page__list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.notification-page__item {
  display: flex;
  gap: 12px;
  padding: 14px 16px;
  border: 1px solid var(--border-light);
  border-radius: var(--radius-base);
  background-color: var(--bg-card);
  transition: border-color var(--transition-base), box-shadow var(--transition-base),
    transform var(--transition-base);
}

.notification-page__item:hover {
  border-color: var(--primary-light);
  box-shadow: var(--shadow-card);
  transform: translateY(-1px);
}

.notification-page__item.is-unread {
  border-left: 3px solid var(--primary);
  background-color: #fbfbff;
}

.notification-page__dot {
  width: 8px;
  height: 8px;
  margin-top: 7px;
  flex-shrink: 0;
  border-radius: 50%;
  background-color: var(--border);
}

.notification-page__item.is-unread .notification-page__dot {
  background-color: var(--primary);
  box-shadow: 0 0 0 3px var(--primary-light);
}

.notification-page__content {
  min-width: 0;
  flex: 1;
}

.notification-page__title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.notification-page__title {
  color: var(--text-main);
  font-size: 14px;
}

.notification-page__time {
  margin-left: auto;
  color: var(--text-placeholder);
  font-size: 12px;
}

.notification-page__body {
  margin: 6px 0 2px;
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.6;
}

.notification-page__actions {
  display: flex;
  gap: 4px;
  margin-top: 4px;
}

.notification-page__pagination {
  margin-top: 14px;
  justify-content: flex-end;
}
</style>

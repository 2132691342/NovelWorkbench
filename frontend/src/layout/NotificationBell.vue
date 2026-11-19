<!--
  顶栏通知铃铛：未读徽标 + 最近通知浮层。
  未读数每 60 秒轮询；点击单条标记已读并跳转关联页面。
-->
<script setup lang="ts">
import { onBeforeUnmount, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { useNotificationStore } from '@/stores/notification'
import NWIcon from '@/components/NWIcon.vue'

const router = useRouter()
const notificationStore = useNotificationStore()
const { t } = useI18n()

/** 轮询句柄（60 秒刷新未读数与最近通知） */
let notificationTimer: ReturnType<typeof setInterval> | null = null

/** 点击通知：先标记已读，再跳转关联页面 */
async function handleNotificationClick(notificationItem: {
  notification_ulid: string
  link_url: string
}): Promise<void> {
  try {
    await notificationStore.markRead(notificationItem.notification_ulid)
  } catch {
    // 失败提示由 http 层统一弹出
  }
  if (notificationItem.link_url) {
    void router.push(notificationItem.link_url)
  }
}

/** 全部标记已读 */
async function handleReadAll(): Promise<void> {
  try {
    await notificationStore.markAllRead()
    ElMessage.success(t('notifications.markAllSuccess'))
    await notificationStore.refreshAll()
  } catch {
    // 失败提示由 http 层统一弹出
  }
}

function formatNoticeTime(timeMs: number): string {
  return new Date(timeMs).toLocaleString()
}

onMounted(() => {
  void notificationStore.refreshAll()
  notificationTimer = setInterval(() => {
    void notificationStore.refreshAll()
  }, 60000)
})

onBeforeUnmount(() => {
  if (notificationTimer) {
    clearInterval(notificationTimer)
    notificationTimer = null
  }
})
</script>

<template>
  <el-popover placement="bottom-end" :width="360" trigger="click" popper-class="nw-notify-popper">
    <template #reference>
      <button type="button" class="admin-layout__header-btn" :aria-label="t('notifications.title')">
        <el-badge
          :value="notificationStore.unread_count"
          :max="99"
          :hidden="notificationStore.unread_count === 0"
        >
          <NWIcon icon_code="bell" :size_px="18" />
        </el-badge>
      </button>
    </template>
    <div class="nw-notify-panel">
      <div class="nw-notify-panel__header">
        <strong>{{ t('notifications.title') }}</strong>
        <button
          type="button"
          class="nw-notify-panel__action"
          :disabled="notificationStore.unread_count === 0"
          @click="handleReadAll"
        >
          {{ t('notifications.markAllRead') }}
        </button>
      </div>
      <div v-if="notificationStore.latest_list.length" class="nw-notify-panel__list">
        <button
          v-for="notificationItem in notificationStore.latest_list"
          :key="notificationItem.notification_ulid"
          type="button"
          class="nw-notify-panel__item"
          :class="{ 'is-unread': !notificationItem.read_flag }"
          @click="handleNotificationClick(notificationItem)"
        >
          <span class="nw-notify-panel__dot" />
          <span class="nw-notify-panel__main">
            <span class="nw-notify-panel__title">{{ notificationItem.notification_title_text }}</span>
            <span class="nw-notify-panel__body">{{ notificationItem.notification_body_text }}</span>
            <span class="nw-notify-panel__time">
              {{ formatNoticeTime(notificationItem.created_at_ms) }}
            </span>
          </span>
        </button>
      </div>
      <p v-else class="nw-notify-panel__empty">{{ t('notifications.emptyTip') }}</p>
      <button type="button" class="nw-notify-panel__footer" @click="router.push('/notifications')">
        {{ t('notifications.viewAll') }}
      </button>
    </div>
  </el-popover>
</template>

<!-- 通知中心：按状态筛选、标记已读与删除。 -->
<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import {
    ElButton,
    ElCard,
    ElEmpty,
    ElMessage,
    ElMessageBox,
    ElPagination,
    ElTabPane,
    ElTabs
  } from 'element-plus'
  import {
    deleteNotification,
    listNotifications,
    markAllNotificationsRead,
    markNotificationsRead,
    type NWNotificationListReqVo,
    type NWNotificationVo
  } from '@/api/notification'
  import { useNotificationStore } from '@/store/modules/notification'
  import { useRowAction } from '@/composables/useRowAction'
  import { formatRelativeTime, formatTime } from '@/utils/time'

  const notificationStore = useNotificationStore()

  type ReadFilter = NonNullable<NWNotificationListReqVo['read_status_code']>

  const items = ref<NWNotificationVo[]>([])
  const total = ref(0)
  const pageNum = ref(1)
  const pageSize = ref(20)
  const loading = ref(false)
  const activeFilter = ref<string>('all')
  const markingAll = ref(false)
  const rowAction = useRowAction()

  const filterTabs: Array<{ value: ReadFilter; label: string }> = [
    { value: 'all', label: '全部' },
    { value: 'unread', label: '未读' },
    { value: 'read', label: '已读' }
  ]

  const unreadCount = computed(() => notificationStore.unreadCount)

  const typeLabel = (typeCode: string) =>
    ({ system: '系统', task: '任务', account: '账号' })[typeCode] ?? typeCode

  const typeBadge = (typeCode: string) => {
    if (typeCode === 'task') return 'warning'
    if (typeCode === 'account') return 'success'
    return 'neutral'
  }

  const load = async () => {
    loading.value = true
    try {
      const page = await listNotifications({
        page_num: pageNum.value,
        page_size: pageSize.value,
        read_status_code: activeFilter.value as ReadFilter
      })
      items.value = page.items ?? []
      total.value = page.total ?? 0
    } catch {
      items.value = []
      total.value = 0
    } finally {
      loading.value = false
    }
  }

  /** 刷新列表与顶栏铃铛未读数 */
  const refreshAll = async () => {
    await Promise.all([load(), notificationStore.refreshUnreadCount()])
  }

  const changeFilter = async () => {
    pageNum.value = 1
    await load()
  }

  const changePage = async (nextPage: number) => {
    pageNum.value = nextPage
    await load()
  }

  const markRead = async (row: NWNotificationVo) => {
    if (row.read_flag) return
    await markNotificationsRead([row.notification_ulid])
    await refreshAll()
  }

  const markAllRead = async () => {
    markingAll.value = true
    try {
      await markAllNotificationsRead()
      ElMessage.success('已全部标记为已读')
      await refreshAll()
    } finally {
      markingAll.value = false
    }
  }

  const remove = async (row: NWNotificationVo) => {
    try {
      await ElMessageBox.confirm(
        `将删除通知「${row.notification_title_text}」，确定吗？`,
        '删除通知',
        { type: 'warning' }
      )
    } catch {
      return
    }
    await rowAction.run(row.notification_ulid, async () => {
      await deleteNotification(row.notification_ulid)
      ElMessage.success('通知已删除')
      if (items.value.length === 1 && pageNum.value > 1) {
        pageNum.value -= 1
      }
      await refreshAll()
    })
  }

  onMounted(() => {
    void refreshAll()
  })
</script>

<template>
  <div class="nw-page">
    <div class="nw-toolbar">
      <ElTabs v-model="activeFilter" class="notifications__tabs" @tab-change="changeFilter">
        <ElTabPane v-for="tab in filterTabs" :key="tab.value" :name="tab.value" :label="tab.label" />
      </ElTabs>
      <div class="notifications__spacer" />
      <span class="nw-hint">{{ unreadCount }} 条未读 · 共 {{ total }} 条</span>
      <ElButton
        type="primary"
        plain
        :disabled="unreadCount === 0"
        :loading="markingAll"
        @click="markAllRead()"
      >
        全部已读
      </ElButton>
      <ElButton @click="refreshAll()">
        <Icon icon="mdi:refresh" :size="15" />
      </ElButton>
    </div>

    <div v-loading="loading" class="notifications__list">
      <ElCard
        v-for="row in items"
        :key="row.notification_ulid"
        shadow="never"
        class="notice-card"
        :class="{ 'is-unread': !row.read_flag }"
      >
        <div class="notice-card__head">
          <div class="notice-card__title-row">
            <span v-if="!row.read_flag" class="notice-card__dot" />
            <h3 class="notice-card__title">{{ row.notification_title_text }}</h3>
            <span class="nw-badge" :class="`nw-badge--${typeBadge(row.notification_type_code)}`">
              {{ typeLabel(row.notification_type_code) }}
            </span>
          </div>
          <span class="nw-hint" :title="formatTime(row.created_at)">
            {{ formatRelativeTime(row.created_at) }}
          </span>
        </div>

        <p class="notice-card__body">{{ row.notification_body_text }}</p>

        <div class="notice-card__actions">
          <ElButton
            v-if="row.link_url"
            size="small"
            text
            type="primary"
            tag="a"
            :href="row.link_url"
          >
            查看详情
          </ElButton>
          <ElButton v-if="!row.read_flag" size="small" text @click="markRead(row)">
            标记已读
          </ElButton>
          <div class="notifications__spacer" />
          <ElButton
            size="small"
            text
            type="danger"
            :loading="rowAction.isPending(row.notification_ulid)"
            :disabled="rowAction.busyFlag.value"
            @click="remove(row)"
          >
            删除
          </ElButton>
        </div>
      </ElCard>

      <ElEmpty v-if="!loading && items.length === 0" description="没有通知" />
    </div>

    <div v-if="total > pageSize" class="notifications__pager">
      <ElPagination
        :current-page="pageNum"
        :page-size="pageSize"
        :total="total"
        layout="prev, pager, next"
        background
        @current-change="changePage"
      />
    </div>
  </div>
</template>

<style lang="less" scoped>
  .notifications {
    &__tabs {
      :deep(.el-tabs__header) {
        margin: 0;
      }

      :deep(.el-tabs__nav-wrap::after) {
        display: none;
      }
    }

    &__spacer {
      flex: 1;
    }

    &__list {
      display: flex;
      flex-direction: column;
      gap: 12px;
      min-height: 120px;
    }

    &__pager {
      display: flex;
      justify-content: flex-end;
    }
  }

  .notice-card {
    border-radius: var(--nw-radius-lg);

    &.is-unread {
      border-left: 3px solid var(--nw-primary);
    }

    &__head {
      display: flex;
      gap: 12px;
      align-items: center;
      justify-content: space-between;
    }

    &__title-row {
      display: flex;
      gap: 8px;
      align-items: center;
      min-width: 0;
    }

    &__dot {
      width: 7px;
      height: 7px;
      background: var(--nw-primary);
      border-radius: 50%;
      flex: none;
    }

    &__title {
      overflow: hidden;
      font-size: 14px;
      font-weight: 650;
      color: var(--nw-ink);
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    &__body {
      margin: 8px 0 10px;
      font-size: 12.5px;
      line-height: 1.7;
      color: var(--nw-text-secondary);
      white-space: pre-wrap;
      word-break: break-word;
    }

    &__actions {
      display: flex;
      gap: 4px;
      align-items: center;
    }
  }
</style>

<!-- 通知中心：全部/未读切换、单条与全部已读、展开正文、删除。 -->
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
  const activeFilter = ref<ReadFilter>('all')
  const markingAll = ref(false)
  const rowAction = useRowAction()
  /** 展开正文的通知 ULID 集合（默认只显示摘要） */
  const expandedSet = ref<string[]>([])

  const filterTabs: Array<{ value: ReadFilter; label: string }> = [
    { value: 'all', label: '全部' },
    { value: 'unread', label: '未读' }
  ]

  const unreadCount = computed(() => notificationStore.unreadCount)

  const typeMeta: Record<string, { label: string; icon: string; badge: string }> = {
    system: { label: '系统', icon: 'mdi:cog-outline', badge: 'neutral' },
    task: { label: '任务', icon: 'mdi:progress-clock', badge: 'warning' },
    account: { label: '账号', icon: 'mdi:account-circle-outline', badge: 'success' }
  }

  const metaOf = (typeCode: string) =>
    typeMeta[typeCode] ?? { label: typeCode, icon: 'mdi:bell-outline', badge: 'neutral' }

  /** 正文摘要：默认截断，展开后显示全文 */
  const summaryOf = (row: NWNotificationVo) =>
    row.notification_body_text?.length > 80
      ? `${row.notification_body_text.slice(0, 80)}…`
      : row.notification_body_text

  const isExpanded = (ulid: string) => expandedSet.value.includes(ulid)

  const toggleExpand = (row: NWNotificationVo) => {
    if (!row.notification_body_text) return
    expandedSet.value = isExpanded(row.notification_ulid)
      ? expandedSet.value.filter((ulid) => ulid !== row.notification_ulid)
      : [...expandedSet.value, row.notification_ulid]
  }

  const load = async () => {
    loading.value = true
    try {
      const page = await listNotifications({
        page_num: pageNum.value,
        page_size: pageSize.value,
        read_status_code: activeFilter.value
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
    expandedSet.value = []
    await load()
  }

  const changePage = async (nextPage: number) => {
    pageNum.value = nextPage
    await load()
  }

  const markRead = async (row: NWNotificationVo) => {
    if (row.read_flag) return
    await markNotificationsRead([row.notification_ulid])
    ElMessage.success('已标记为已读')
    await refreshAll()
  }

  const markAllRead = async () => {
    if (unreadCount.value === 0) return
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
        `将删除通知「${row.notification_title_text}」，删除后不可恢复。`,
        '删除通知',
        { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' }
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

  const emptyHint = computed(() =>
    activeFilter.value === 'unread' ? '没有未读通知，全部已处理完' : '暂时没有通知'
  )

  onMounted(() => {
    void refreshAll()
  })
</script>

<template>
  <div class="nw-page">
    <div class="nw-toolbar">
      <h2 class="notifications__title">
        通知中心
        <span v-if="unreadCount > 0" class="nw-badge nw-badge--warning">{{ unreadCount }} 条未读</span>
      </h2>
      <div class="notifications__spacer" />
      <ElButton
        type="primary"
        plain
        :disabled="unreadCount === 0"
        :loading="markingAll"
        @click="markAllRead()"
      >
        全部已读
      </ElButton>
      <ElButton :loading="loading" @click="refreshAll()">
        <Icon icon="mdi:refresh" :size="15" />
      </ElButton>
    </div>

    <div class="nw-toolbar">
      <ElTabs v-model="activeFilter" class="notifications__tabs" @tab-change="changeFilter">
        <ElTabPane v-for="tab in filterTabs" :key="tab.value" :name="tab.value" :label="tab.label" />
      </ElTabs>
      <div class="notifications__spacer" />
      <span class="nw-hint">共 {{ total }} 条</span>
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
            <span class="notice-card__icon" :class="`is-${metaOf(row.notification_type_code).badge}`">
              <Icon :icon="metaOf(row.notification_type_code).icon" :size="14" />
            </span>
            <span
              v-if="!row.read_flag"
              class="notice-card__dot"
              title="未读"
            />
            <h3 class="notice-card__title">{{ row.notification_title_text }}</h3>
            <span
              class="nw-badge"
              :class="`nw-badge--${metaOf(row.notification_type_code).badge}`"
            >
              {{ metaOf(row.notification_type_code).label }}
            </span>
          </div>
          <span class="nw-hint" :title="formatTime(row.created_at)">
            {{ formatRelativeTime(row.created_at) }}
          </span>
        </div>

        <p
          v-if="row.notification_body_text"
          class="notice-card__body"
          :class="{ 'is-expanded': isExpanded(row.notification_ulid) }"
          @click="toggleExpand(row)"
        >
          {{ isExpanded(row.notification_ulid) ? row.notification_body_text : summaryOf(row) }}
          <span v-if="row.notification_body_text.length > 80" class="notice-card__more">
            {{ isExpanded(row.notification_ulid) ? '收起' : '展开' }}
          </span>
        </p>

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

      <ElEmpty v-if="!loading && items.length === 0" :description="emptyHint" />
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
    &__title {
      display: flex;
      gap: 8px;
      align-items: center;
      font-size: 15px;
      font-weight: 650;
      color: var(--nw-ink);
    }

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
      min-height: 140px;
    }

    &__pager {
      display: flex;
      justify-content: flex-end;
    }
  }

  .notice-card {
    border-radius: var(--nw-radius-lg);
    transition: border-color 0.15s ease;

    &.is-unread {
      border-left: 3px solid var(--el-color-primary);
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

    &__icon {
      display: grid;
      width: 24px;
      height: 24px;
      color: var(--el-text-color-secondary);
      background: var(--el-fill-color-light);
      border-radius: 50%;
      flex: none;
      place-items: center;

      &.is-warning {
        color: #b45309;
        background: #fffbeb;
      }

      &.is-success {
        color: #047857;
        background: #ecfdf5;
      }
    }

    &__dot {
      width: 7px;
      height: 7px;
      background: var(--el-color-primary);
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
      color: var(--el-text-color-regular);
      white-space: pre-wrap;
      word-break: break-word;
      cursor: pointer;
      transition: color 0.15s ease;

      &:hover {
        color: var(--nw-ink);
      }
    }

    &__more {
      margin-left: 4px;
      color: var(--el-color-primary);
    }

    &__actions {
      display: flex;
      gap: 4px;
      align-items: center;
    }
  }
</style>

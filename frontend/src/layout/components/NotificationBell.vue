<!-- 通知铃铛：展示未读数并可展开最近通知。 -->
<script setup lang="ts">
  import { onBeforeUnmount, onMounted } from 'vue'
  import { useRouter } from 'vue-router'
  import { useNotificationStore } from '@/store/modules/notification'

  const notificationStore = useNotificationStore()
  const router = useRouter()

  let timer: ReturnType<typeof setInterval> | undefined

  onMounted(() => {
    void notificationStore.refreshUnreadCount()
    timer = setInterval(() => void notificationStore.refreshUnreadCount(), 60000)
  })

  onBeforeUnmount(() => {
    if (timer) clearInterval(timer)
  })
</script>

<template>
  <button
    type="button"
    class="nw-bell header-action"
    title="通知中心"
    @click="router.push('/notifications')"
  >
    <el-badge :value="notificationStore.unreadCount" :hidden="notificationStore.unreadCount === 0" :max="99">
      <Icon icon="mdi:bell-outline" :size="19" />
    </el-badge>
  </button>
</template>

<style lang="less" scoped>
  .nw-bell {
    display: grid;
    padding: 0 12px;
    color: var(--top-header-text-color);
    cursor: pointer;
    background: transparent;
    border: 0;
    place-items: center;
  }
</style>

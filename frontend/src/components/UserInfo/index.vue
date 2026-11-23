<!-- 用户菜单：展示当前用户并提供个人中心与退出入口。 -->
<script setup lang="ts">
  import { computed } from 'vue'
  import { useRouter } from 'vue-router'
  import { ElMessageBox } from 'element-plus'
  import { useI18n } from 'vue-i18n'
  import { useUserStore } from '@/store/modules/user'
  import { useProjectStore } from '@/store/modules/project'

  const userStore = useUserStore()
  const projectStore = useProjectStore()
  const router = useRouter()
  const { t } = useI18n()

  const avatarText = computed(() => userStore.userInfo?.user_name.slice(0, 1).toUpperCase() ?? '?')
  const displayName = computed(() => userStore.userInfo?.user_name ?? t('common.guest'))

  const logout = async () => {
    try {
      await ElMessageBox.confirm(t('common.loginOutMessage'), t('common.reminder'), {
        confirmButtonText: t('common.ok'),
        cancelButtonText: t('common.cancel'),
        type: 'warning'
      })
    } catch {
      return
    }
    projectStore.reset()
    await userStore.logout()
  }

  const toProfile = () => router.push('/profile')
</script>

<template>
  <el-dropdown class="nw-user header-action" trigger="click">
    <div class="nw-user__trigger">
      <span class="nw-user__avatar">{{ avatarText }}</span>
      <span class="nw-user__name">{{ displayName }}</span>
    </div>
    <template #dropdown>
      <el-dropdown-menu>
        <el-dropdown-item @click="toProfile">
          <Icon icon="mdi:account-circle-outline" :size="15" />
          <span class="ml-6px">{{ t('common.profile') }}</span>
        </el-dropdown-item>
        <el-dropdown-item divided @click="logout">
          <Icon icon="mdi:logout" :size="15" />
          <span class="ml-6px">{{ t('common.loginOut') }}</span>
        </el-dropdown-item>
      </el-dropdown-menu>
    </template>
  </el-dropdown>
</template>

<style lang="less" scoped>
  .nw-user {
    display: flex;
    align-items: center;
    height: 100%;
    padding: 0 12px;

    &__trigger {
      display: flex;
      gap: 8px;
      align-items: center;
      cursor: pointer;
    }

    &__avatar {
      display: grid;
      width: 28px;
      height: 28px;
      font-size: 13px;
      font-weight: 650;
      color: var(--el-color-primary);
      background: var(--nw-primary-soft);
      border-radius: 50%;
      place-items: center;
    }

    &__name {
      font-size: 13px;
      color: var(--top-header-text-color);
    }
  }

  @media (width <= 767px) {
    .nw-user__name {
      display: none;
    }
  }
</style>

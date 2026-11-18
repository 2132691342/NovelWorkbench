<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { NW_LOCALE_OPTIONS } from '@/i18n'
import NWButton from '@/components/NWButton.vue'

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()
const { t } = useI18n()

const activeMenuPath = computed(() => route.path)

/** 是否系统管理员（角色编码包含 admin 时展示系统管理入口） */
const isAdminUser = computed(() => (authStore.user_vo?.role_codes ?? []).includes('admin'))

const avatarText = computed(() => {
  const userName = authStore.user_vo?.user_name ?? ''
  return userName.length > 0 ? userName.slice(0, 1) : '?'
})

const currentLocaleLabel = computed(() => {
  const matchedOption = NW_LOCALE_OPTIONS.find(
    (optionItem) => optionItem.locale_code === appStore.locale_code,
  )
  return matchedOption?.locale_label ?? NW_LOCALE_OPTIONS[0].locale_label
})

function handleMenuSelect(menuPath: string | number): void {
  void router.push(String(menuPath))
}

function handleLocaleCommand(command: string | number | object): void {
  const localeCode = String(command)
  if (localeCode === 'zh-CN' || localeCode === 'en-US') {
    appStore.setLocale(localeCode)
  }
}

async function handleUserCommand(command: string | number | object): Promise<void> {
  if (command !== 'logout') {
    return
  }
  await authStore.logout()
  ElMessage.success(t('layout.logoutSuccess'))
  void router.push('/login')
}
</script>

<template>
  <div class="admin-layout">
    <aside class="admin-layout__aside" :class="{ 'is-collapsed': appStore.sidebar_collapsed }">
      <div class="admin-layout__brand">
        <svg class="admin-layout__brand-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
          <rect x="3" y="3" width="18" height="18" rx="5" fill="var(--primary)" />
          <path
            d="M8 8v8M8 8h2.6a2.4 2.4 0 0 1 2.4 2.4V16M16 8v8"
            stroke="#ffffff"
            stroke-width="1.8"
            stroke-linecap="round"
          />
        </svg>
        <span v-show="!appStore.sidebar_collapsed" class="admin-layout__brand-text">
          {{ t('common.appName') }}
        </span>
      </div>

      <el-menu
        class="admin-layout__menu"
        :collapse="appStore.sidebar_collapsed"
        :collapse-transition="false"
        :default-active="activeMenuPath"
        @select="handleMenuSelect"
      >
        <el-menu-item index="/">
          <svg class="admin-layout__menu-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <rect x="4" y="4" width="7" height="7" rx="1.5" stroke="currentColor" stroke-width="1.8" />
            <rect x="13" y="4" width="7" height="7" rx="1.5" stroke="currentColor" stroke-width="1.8" />
            <rect x="4" y="13" width="7" height="7" rx="1.5" stroke="currentColor" stroke-width="1.8" />
            <rect x="13" y="13" width="7" height="7" rx="1.5" stroke="currentColor" stroke-width="1.8" />
          </svg>
          <template #title>{{ t('layout.menu.dashboard') }}</template>
        </el-menu-item>

        <el-menu-item index="/settings">
          <svg class="admin-layout__menu-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <circle cx="12" cy="12" r="3" stroke="currentColor" stroke-width="1.8" />
            <path
              d="M12 3.5v2M12 18.5v2M3.5 12h2M18.5 12h2M6 6l1.4 1.4M16.6 16.6L18 18M18 6l-1.4 1.4M7.4 16.6L6 18"
              stroke="currentColor"
              stroke-width="1.8"
              stroke-linecap="round"
            />
          </svg>
          <template #title>{{ t('layout.menu.settings') }}</template>
        </el-menu-item>

        <el-menu-item index="/slash-admin">
          <svg class="admin-layout__menu-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <path
              d="M9 5L5 19M19 5l-4 14M4 10h16"
              stroke="currentColor"
              stroke-width="1.8"
              stroke-linecap="round"
            />
          </svg>
          <template #title>{{ t('layout.menu.slashAdmin') }}</template>
        </el-menu-item>

        <el-sub-menu v-if="isAdminUser" index="/admin">
          <template #title>
            <svg class="admin-layout__menu-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
              <path
                d="M12 3l7 3v5c0 4.5-2.9 8.3-7 10-4.1-1.7-7-5.5-7-10V6l7-3z"
                stroke="currentColor"
                stroke-width="1.8"
                stroke-linejoin="round"
              />
              <path
                d="M9 12l2 2 4-4"
                stroke="currentColor"
                stroke-width="1.8"
                stroke-linecap="round"
                stroke-linejoin="round"
              />
            </svg>
            <span>{{ t('layout.menu.systemAdmin') }}</span>
          </template>

          <el-menu-item index="/admin/users">
            <svg class="admin-layout__menu-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
              <circle cx="12" cy="8" r="3.5" stroke="currentColor" stroke-width="1.8" />
              <path
                d="M5 19c0-3.3 3.1-5.5 7-5.5s7 2.2 7 5.5"
                stroke="currentColor"
                stroke-width="1.8"
                stroke-linecap="round"
              />
            </svg>
            <template #title>{{ t('layout.menu.adminUsers') }}</template>
          </el-menu-item>

          <el-menu-item index="/admin/roles">
            <svg class="admin-layout__menu-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
              <circle cx="8" cy="12" r="3.5" stroke="currentColor" stroke-width="1.8" />
              <path
                d="M11.5 12H20M17 12v3M20 12v2.5"
                stroke="currentColor"
                stroke-width="1.8"
                stroke-linecap="round"
              />
            </svg>
            <template #title>{{ t('layout.menu.adminRoles') }}</template>
          </el-menu-item>

          <el-menu-item index="/admin/audit">
            <svg class="admin-layout__menu-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
              <rect x="5" y="3.5" width="14" height="17" rx="2" stroke="currentColor" stroke-width="1.8" />
              <path
                d="M8.5 8h7M8.5 12h7M8.5 16h4"
                stroke="currentColor"
                stroke-width="1.8"
                stroke-linecap="round"
              />
            </svg>
            <template #title>{{ t('layout.menu.adminAudit') }}</template>
          </el-menu-item>
        </el-sub-menu>
      </el-menu>
    </aside>

    <div class="admin-layout__body">
      <header class="admin-layout__header">
        <NWButton
          type="text"
          :aria-label="t('layout.toggleSidebar')"
          @click="appStore.toggleSidebar()"
        >
          <svg class="admin-layout__header-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <path
              d="M4 6h16M4 12h16M4 18h16"
              stroke="currentColor"
              stroke-width="1.8"
              stroke-linecap="round"
            />
          </svg>
        </NWButton>

        <div class="admin-layout__header-right">
          <el-dropdown @command="handleLocaleCommand">
            <span class="admin-layout__header-link">
              {{ currentLocaleLabel }}
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item
                  v-for="localeOption in NW_LOCALE_OPTIONS"
                  :key="localeOption.locale_code"
                  :command="localeOption.locale_code"
                >
                  {{ localeOption.locale_label }}
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>

          <el-dropdown @command="handleUserCommand">
            <span class="admin-layout__user">
              <el-avatar :size="32" :src="authStore.user_vo?.avatar_url" class="admin-layout__avatar">
                {{ avatarText }}
              </el-avatar>
              <span class="admin-layout__user-name">
                {{ authStore.user_vo?.user_name || t('layout.guest') }}
              </span>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="logout">{{ t('common.logout') }}</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </header>

      <main class="admin-layout__content">
        <router-view v-slot="{ Component }">
          <transition name="fade-slide" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </main>
    </div>
  </div>
</template>

<style scoped>
.admin-layout {
  display: flex;
  height: 100vh;
  overflow: hidden;
  background-color: var(--bg-page);
}

.admin-layout__aside {
  display: flex;
  flex-direction: column;
  width: 220px;
  flex-shrink: 0;
  background-color: var(--bg-card);
  border-right: 1px solid var(--border-light);
  transition: width 0.2s ease;
}

.admin-layout__aside.is-collapsed {
  width: 64px;
}

.admin-layout__brand {
  display: flex;
  align-items: center;
  gap: 10px;
  height: 56px;
  padding: 0 16px;
  border-bottom: 1px solid var(--border-light);
  overflow: hidden;
}

.admin-layout__brand-icon {
  width: 28px;
  height: 28px;
  flex-shrink: 0;
}

.admin-layout__brand-text {
  font-size: 16px;
  font-weight: 600;
  white-space: nowrap;
  color: var(--text-main);
}

.admin-layout__menu {
  flex: 1;
  border-right: none;
  overflow-y: auto;
}

.admin-layout__menu-icon {
  width: 18px;
  height: 18px;
  margin-right: 8px;
}

.admin-layout__body {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
}

.admin-layout__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 56px;
  padding: 0 20px;
  background-color: var(--bg-card);
  border-bottom: 1px solid var(--border-light);
}

.admin-layout__header-icon {
  width: 20px;
  height: 20px;
}

.admin-layout__header-right {
  display: flex;
  align-items: center;
  gap: 20px;
}

.admin-layout__header-link {
  color: var(--text-secondary);
  cursor: pointer;
  outline: none;
  transition: color 0.2s ease;
}

.admin-layout__header-link:hover {
  color: var(--primary);
}

.admin-layout__user {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  outline: none;
}

.admin-layout__avatar {
  background-color: var(--primary);
  color: var(--text-inverse);
  font-weight: 600;
}

.admin-layout__user-name {
  color: var(--text-main);
  font-size: 14px;
}

.admin-layout__content {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
}
</style>

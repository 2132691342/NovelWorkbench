<!-- 用户详情抽屉：只读展示账号基本信息与角色，不含任何写操作。 -->
<script setup lang="ts">
  import { computed } from 'vue'
  import type { NWUserAdminVo } from '@/api/admin'
  import { formatTime } from '@/utils/time'

  const props = defineProps<{
    visible: boolean
    user: NWUserAdminVo | null
    /** 角色编码 → 角色名称（由列表页的角色下拉映射而来，避免抽屉里只显示编码） */
    roleNameMap?: Record<string, string>
  }>()

  const emit = defineEmits<{ 'update:visible': [visible: boolean] }>()

  const roleLabel = (roleCode: string) => props.roleNameMap?.[roleCode] ?? roleCode

  const statusLabel = computed(() =>
    props.user?.user_status_code === 'active' ? '正常' : '已停用'
  )

  const statusBadge = computed(() =>
    props.user?.user_status_code === 'active' ? 'success' : 'danger'
  )
</script>

<template>
  <ElDrawer
    :model-value="visible"
    title="用户详情"
    size="420px"
    append-to-body
    destroy-on-close
    @update:model-value="emit('update:visible', $event)"
  >
    <template v-if="user">
      <div class="user-detail__head">
        <span class="user-detail__avatar">
          <img v-if="user.avatar_url" :src="user.avatar_url" alt="头像" />
          <template v-else>{{ (user.user_name || 'U').slice(0, 1).toUpperCase() }}</template>
        </span>
        <div class="user-detail__head-text">
          <h3 class="user-detail__name">{{ user.user_name }}</h3>
          <p class="nw-hint">{{ user.user_email }}</p>
        </div>
        <span class="nw-badge" :class="`nw-badge--${statusBadge}`">{{ statusLabel }}</span>
      </div>

      <dl class="user-detail__meta">
        <div class="user-detail__row">
          <dt>角色</dt>
          <dd>
            <span v-if="user.role_codes?.length" class="user-detail__roles">
              <span v-for="code in user.role_codes" :key="code" class="nw-badge nw-badge--neutral">
                {{ roleLabel(code) }}
              </span>
            </span>
            <span v-else class="nw-hint">未分配角色</span>
          </dd>
        </div>
        <div class="user-detail__row">
          <dt>备注</dt>
          <dd>{{ user.remark_text || '—' }}</dd>
        </div>
        <div class="user-detail__row">
          <dt>创建时间</dt>
          <dd>{{ formatTime(user.created_at) }}</dd>
        </div>
        <div class="user-detail__row">
          <dt>最近更新</dt>
          <dd>{{ formatTime(user.updated_at) }}</dd>
        </div>
        <div class="user-detail__row">
          <dt>用户标识</dt>
          <dd><code class="user-detail__ulid">{{ user.user_ulid }}</code></dd>
        </div>
      </dl>

      <p class="user-detail__note nw-hint">
        该账号的登录记录与操作审计请分别到「登录日志」「审计日志」按用户标识查询。
      </p>
    </template>

    <ElEmpty v-else description="未选择用户" />
  </ElDrawer>
</template>

<style lang="less" scoped>
  .user-detail {
    &__head {
      display: flex;
      gap: 12px;
      align-items: center;
      padding-bottom: 14px;
      border-bottom: 1px solid var(--nw-border-soft);
    }

    &__avatar {
      display: grid;
      width: 48px;
      height: 48px;
      overflow: hidden;
      font-size: 18px;
      font-weight: 700;
      color: var(--el-color-primary);
      background: var(--nw-primary-soft);
      border-radius: 50%;
      flex: none;
      place-items: center;

      img {
        width: 100%;
        height: 100%;
        object-fit: cover;
      }
    }

    &__head-text {
      flex: 1;
      min-width: 0;
    }

    &__name {
      font-size: 15px;
      font-weight: 650;
      color: var(--nw-ink);
    }

    &__meta {
      display: flex;
      flex-direction: column;
      gap: 10px;
      margin-top: 14px;
    }

    &__row {
      display: flex;
      gap: 12px;
      align-items: baseline;
      justify-content: space-between;
      font-size: 12.5px;

      dt {
        color: var(--el-text-color-secondary);
        flex: none;
      }

      dd {
        color: var(--nw-ink);
        text-align: right;
        word-break: break-all;
      }
    }

    &__roles {
      display: inline-flex;
      flex-wrap: wrap;
      gap: 4px;
      justify-content: flex-end;
    }

    &__ulid {
      font-family: var(--el-font-family-mono, monospace);
      font-size: 11px;
      color: var(--el-text-color-placeholder);
    }

    &__note {
      margin-top: 18px;
      padding-top: 12px;
      border-top: 1px solid var(--nw-border-soft);
    }
  }
</style>

<!-- 用户管理页：创建账号、分配角色与重置密码。 -->
<script setup lang="ts">
  import { onMounted, ref } from 'vue'
  import {
    ElButton,
    ElInput,
    ElMessage,
    ElMessageBox,
    ElOption,
    ElPagination,
    ElSelect,
    ElTable,
    ElTableColumn
  } from 'element-plus'
  import { deleteAdminUser, listAdminUsers, type NWUserAdminVo } from '@/api/admin'
  import { usePageQuery } from '@/composables/usePageQuery'
  import { useRowAction } from '@/composables/useRowAction'
  import { formatTime } from '@/utils/time'
  import { asRow } from '@/utils/table'
  import ResetPasswordDialog from './components/ResetPasswordDialog.vue'
  import UserFormDialog from './components/UserFormDialog.vue'

  const { items, total, pageNum, pageSize, loading, query, load, search, changePage } = usePageQuery(
    listAdminUsers,
    { keyword_text: '', user_status_code: '' }
  )

  const formDialog = ref<InstanceType<typeof UserFormDialog>>()
  const resetDialog = ref<InstanceType<typeof ResetPasswordDialog>>()
  const rowAction = useRowAction()

  const statusOptions = [
    { value: '', label: '全部状态' },
    { value: 'active', label: '正常' },
    { value: 'disabled', label: '禁用' }
  ]

  const statusLabel = (statusCode: string) =>
    statusOptions.find((item) => item.value === statusCode)?.label ?? statusCode

  const statusBadge = (statusCode: string) =>
    statusCode === 'active' ? 'success' : 'danger'

  /** 删除用户（禁止删除自己由后端拦截并给出提示；行内加锁防连点） */
  const remove = async (row: NWUserAdminVo) => {
    try {
      await ElMessageBox.confirm(`将永久删除用户「${row.user_name}」，确定吗？`, '删除用户', {
        type: 'warning'
      })
    } catch {
      return
    }
    await rowAction.run(row.user_ulid, async () => {
      await deleteAdminUser(row.user_ulid)
      ElMessage.success('用户已删除')
      await load()
    })
  }

  const handleSaved = () => {
    void load()
  }

  onMounted(() => {
    void load()
  })
</script>

<template>
  <div class="nw-page">
    <div class="nw-toolbar">
      <ElInput
        v-model="query.keyword_text"
        placeholder="搜索登录名或邮箱"
        clearable
        class="admin-users__search"
        @keyup.enter="search()"
        @clear="search()"
      />
      <ElSelect v-model="query.user_status_code" class="admin-users__status" @change="search()">
        <ElOption
          v-for="item in statusOptions"
          :key="item.value"
          :label="item.label"
          :value="item.value"
        />
      </ElSelect>
      <ElButton type="primary" plain @click="search()">查询</ElButton>
      <ElButton type="primary" @click="formDialog?.open()">
        <Icon icon="mdi:plus" :size="15" />
        <span class="admin-users__btn-text">新建用户</span>
      </ElButton>
      <div class="admin-users__spacer" />
      <span class="nw-hint">共 {{ total }} 个账号</span>
    </div>

    <ElTable v-loading="loading" :data="items" class="admin-users__table">
      <ElTableColumn prop="user_name" label="登录名" min-width="140">
        <template #default="{ row }">
          <span class="admin-users__name">{{ row.user_name }}</span>
          <span v-if="row.remark_text" class="admin-users__remark">{{ row.remark_text }}</span>
        </template>
      </ElTableColumn>
      <ElTableColumn prop="user_email" label="邮箱" min-width="200" show-overflow-tooltip />
      <ElTableColumn label="状态" width="90">
        <template #default="{ row }">
          <span class="nw-badge" :class="`nw-badge--${statusBadge(row.user_status_code)}`">
            {{ statusLabel(row.user_status_code) }}
          </span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="角色" min-width="160">
        <template #default="{ row }">
          <div v-if="row.role_codes?.length" class="admin-users__roles">
            <span v-for="code in row.role_codes" :key="code" class="nw-badge nw-badge--neutral">
              {{ code }}
            </span>
          </div>
          <span v-else class="nw-hint">—</span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="创建时间" width="180">
        <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
      </ElTableColumn>
      <ElTableColumn label="操作" width="200" fixed="right">
        <template #default="{ row }">
          <ElButton size="small" text type="primary" @click="formDialog?.open(asRow<NWUserAdminVo>(row))">编辑</ElButton>
          <ElButton size="small" text @click="resetDialog?.open(asRow<NWUserAdminVo>(row))">重置密码</ElButton>
          <ElButton
            size="small"
            text
            type="danger"
            :loading="rowAction.isPending(asRow<NWUserAdminVo>(row).user_ulid)"
            :disabled="rowAction.busyFlag.value"
            @click="remove(asRow<NWUserAdminVo>(row))"
          >
            删除
          </ElButton>
        </template>
      </ElTableColumn>
      <template #empty>
        <span class="nw-hint">暂无用户</span>
      </template>
    </ElTable>

    <div v-if="total > pageSize" class="admin-users__pager">
      <ElPagination
        :current-page="pageNum"
        :page-size="pageSize"
        :total="total"
        layout="prev, pager, next"
        background
        @current-change="changePage"
      />
    </div>

    <UserFormDialog ref="formDialog" @saved="handleSaved" />
    <ResetPasswordDialog ref="resetDialog" />
  </div>
</template>

<style lang="less" scoped>
  .admin-users {
    &__search {
      width: 240px;
    }

    &__status {
      width: 140px;
    }

    &__btn-text {
      margin-left: 4px;
    }

    &__spacer {
      flex: 1;
    }

    &__table {
      border-radius: var(--nw-radius-lg);
    }

    &__name {
      font-weight: 600;
      color: var(--nw-ink);
    }

    &__remark {
      margin-left: 6px;
      font-size: 11.5px;
      color: var(--nw-text-muted);
    }

    &__roles {
      display: flex;
      flex-wrap: wrap;
      gap: 4px;
    }

    &__pager {
      display: flex;
      justify-content: flex-end;
    }
  }
</style>

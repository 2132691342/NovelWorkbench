<!-- 角色管理页：维护角色与权限勾选。 -->
<script setup lang="ts">
  import { onMounted, ref } from 'vue'
  import {
    ElButton,
    ElMessage,
    ElMessageBox,
    ElPagination,
    ElTable,
    ElTableColumn
  } from 'element-plus'
  import { deleteAdminRole, listAdminRoles, type NWRoleVo } from '@/api/admin'
  import { usePageQuery } from '@/composables/usePageQuery'
  import { useRowAction } from '@/composables/useRowAction'
  import { formatTime } from '@/utils/time'
  import { asRow } from '@/utils/table'
  import RoleFormDialog from './components/RoleFormDialog.vue'

  const { items, total, pageNum, pageSize, loading, load, changePage } = usePageQuery(
    listAdminRoles,
    {}
  )

  const formDialog = ref<InstanceType<typeof RoleFormDialog>>()
  const rowAction = useRowAction()

  /** 删除角色（内置角色由后端拒绝；行内加锁防连点） */
  const remove = async (row: NWRoleVo) => {
    if (row.builtin_flag) return
    try {
      await ElMessageBox.confirm(`将删除角色「${row.role_name}」，确定吗？`, '删除角色', {
        type: 'warning'
      })
    } catch {
      return
    }
    await rowAction.run(row.role_ulid, async () => {
      await deleteAdminRole(row.role_ulid)
      ElMessage.success('角色已删除')
      await load()
    })
  }

  onMounted(() => {
    void load()
  })
</script>

<template>
  <div class="nw-page">
    <div class="nw-toolbar">
      <ElButton type="primary" @click="formDialog?.open()">
        <Icon icon="mdi:plus" :size="15" />
        <span class="admin-roles__btn-text">新建角色</span>
      </ElButton>
      <div class="admin-roles__spacer" />
      <span class="nw-hint">共 {{ total }} 个角色</span>
    </div>

    <ElTable v-loading="loading" :data="items" class="admin-roles__table">
      <ElTableColumn prop="role_name" label="角色名称" min-width="140">
        <template #default="{ row }">
          <span class="admin-roles__name">{{ row.role_name }}</span>
          <span v-if="row.builtin_flag" class="nw-badge nw-badge--neutral">内置</span>
        </template>
      </ElTableColumn>
      <ElTableColumn prop="role_code" label="角色编码" min-width="140" show-overflow-tooltip />
      <ElTableColumn label="描述" min-width="200">
        <template #default="{ row }">
          <span :class="row.remark_text ? '' : 'nw-hint'">{{ row.remark_text || '—' }}</span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="权限数" width="100" align="center">
        <template #default="{ row }">
          <span class="nw-badge">
            {{ row.perm_codes?.includes('*') ? '全部' : (row.perm_codes?.length ?? 0) }}
          </span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="创建时间" width="180">
        <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
      </ElTableColumn>
      <ElTableColumn label="操作" width="150" fixed="right">
        <template #default="{ row }">
          <ElButton size="small" text type="primary" @click="formDialog?.open(asRow<NWRoleVo>(row))">编辑</ElButton>
          <ElButton
            size="small"
            text
            type="danger"
            :loading="rowAction.isPending(row.role_ulid)"
            :disabled="row.builtin_flag || rowAction.busyFlag.value"
            @click="remove(asRow<NWRoleVo>(row))"
          >
            删除
          </ElButton>
        </template>
      </ElTableColumn>
      <template #empty>
        <span class="nw-hint">暂无角色</span>
      </template>
    </ElTable>

    <div v-if="total > pageSize" class="admin-roles__pager">
      <ElPagination
        :current-page="pageNum"
        :page-size="pageSize"
        :total="total"
        layout="prev, pager, next"
        background
        @current-change="changePage"
      />
    </div>

    <RoleFormDialog ref="formDialog" @saved="load" />
  </div>
</template>

<style lang="less" scoped>
  .admin-roles {
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
      margin-right: 8px;
      font-weight: 600;
      color: var(--nw-ink);
    }

    &__pager {
      display: flex;
      justify-content: flex-end;
    }
  }
</style>

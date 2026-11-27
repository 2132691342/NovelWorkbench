<!-- 角色权限页：角色列表（内置标记、权限数、用户数）与抽屉式权限勾选。 -->
<script setup lang="ts">
  import { onMounted, ref } from 'vue'
  import {
    ElButton,
    ElMessage,
    ElMessageBox,
    ElPagination,
    ElTable,
    ElTableColumn,
    ElTooltip
  } from 'element-plus'
  import { deleteAdminRole, listAdminRoles, type NWRoleVo } from '@/api/admin'
  import { usePageQuery } from '@/composables/usePageQuery'
  import { useRowAction } from '@/composables/useRowAction'
  import { formatTime } from '@/utils/time'
  import { asRow } from '@/utils/table'
  import RoleFormDrawer from './components/RoleFormDrawer.vue'

  const { items, total, pageNum, pageSize, loading, load, changePage } = usePageQuery(
    listAdminRoles,
    {}
  )

  const formDrawer = ref<InstanceType<typeof RoleFormDrawer>>()
  const rowAction = useRowAction()

  /** 权限数展示：通配 "*" 记为「全部」 */
  const permCountText = (row: NWRoleVo) =>
    row.perm_codes?.includes('*') ? '全部' : String(row.perm_codes?.length ?? 0)

  /** 删除角色：内置角色禁用按钮；仍有用户时后端会拒绝，消息原样透出 */
  const remove = async (row: NWRoleVo) => {
    if (row.builtin_flag) return
    const userHint =
      row.user_count > 0 ? `该角色仍挂载 ${row.user_count} 个用户，需先移除挂载。` : ''
    try {
      await ElMessageBox.confirm(
        `将删除角色「${row.role_name}」，其权限配置一并失效。${userHint}`,
        '删除角色',
        { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' }
      )
    } catch {
      return
    }
    await rowAction.run(row.role_ulid, async () => {
      await deleteAdminRole(row.role_ulid)
      ElMessage.success('角色已删除')
      if (items.value.length === 1 && pageNum.value > 1) {
        pageNum.value -= 1
      }
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
      <ElButton type="primary" @click="formDrawer?.open()">
        <Icon icon="mdi:plus" :size="15" />
        <span class="admin-roles__btn-text">新建角色</span>
      </ElButton>
      <ElButton :loading="loading" @click="load()">
        <Icon icon="mdi:refresh" :size="15" />
      </ElButton>
      <div class="admin-roles__spacer" />
      <span class="nw-hint">共 {{ total }} 个角色</span>
    </div>

    <ElTable v-loading="loading" :data="items" class="admin-roles__table" row-key="role_ulid">
      <ElTableColumn label="角色名称" min-width="170">
        <template #default="{ row }">
          <span class="admin-roles__name">{{ row.role_name }}</span>
          <span v-if="row.builtin_flag" class="nw-badge nw-badge--neutral">内置</span>
        </template>
      </ElTableColumn>
      <ElTableColumn prop="role_code" label="编码" min-width="130" show-overflow-tooltip>
        <template #default="{ row }">
          <code class="admin-roles__code">{{ row.role_code }}</code>
        </template>
      </ElTableColumn>
      <ElTableColumn label="说明" min-width="180">
        <template #default="{ row }">
          <span :class="row.remark_text ? '' : 'nw-hint'">{{ row.remark_text || '—' }}</span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="权限数" width="90" align="center">
        <template #default="{ row }">
          <span class="nw-badge">{{ permCountText(asRow<NWRoleVo>(row)) }}</span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="用户数" width="90" align="center">
        <template #default="{ row }">
          <span :class="row.user_count > 0 ? '' : 'nw-hint'">{{ row.user_count ?? 0 }}</span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="创建时间" width="180">
        <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
      </ElTableColumn>
      <ElTableColumn label="操作" width="160" fixed="right">
        <template #default="{ row }">
          <ElButton size="small" text type="primary" @click="formDrawer?.open(asRow<NWRoleVo>(row))">
            编辑
          </ElButton>
          <ElTooltip
            v-if="row.builtin_flag"
            content="内置角色不允许删除"
            placement="top"
          >
            <span>
              <ElButton size="small" text type="danger" disabled>删除</ElButton>
            </span>
          </ElTooltip>
          <ElButton
            v-else
            size="small"
            text
            type="danger"
            :loading="rowAction.isPending(asRow<NWRoleVo>(row).role_ulid)"
            :disabled="rowAction.busyFlag.value"
            @click="remove(asRow<NWRoleVo>(row))"
          >
            删除
          </ElButton>
        </template>
      </ElTableColumn>
      <template #empty>
        <span class="nw-hint">暂无角色，点击「新建角色」开始</span>
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

    <RoleFormDrawer ref="formDrawer" @saved="load" />
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

    &__code {
      font-family: var(--el-font-family-mono, monospace);
      font-size: 11.5px;
      color: var(--el-text-color-secondary);
    }

    &__pager {
      display: flex;
      justify-content: flex-end;
    }
  }
</style>

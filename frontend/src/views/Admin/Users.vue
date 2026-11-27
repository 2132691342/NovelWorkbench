<!-- 用户管理页：账号检索、状态/角色筛选、启停、重置密码、详情与删除。 -->
<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import {
    ElButton,
    ElInput,
    ElMessage,
    ElMessageBox,
    ElOption,
    ElPagination,
    ElSelect,
    ElSwitch,
    ElTable,
    ElTableColumn,
    ElTooltip
  } from 'element-plus'
  import {
    deleteAdminUser,
    listAdminRoles,
    listAdminUsers,
    updateAdminUser,
    type NWRoleVo,
    type NWUserAdminVo
  } from '@/api/admin'
  import { usePageQuery } from '@/composables/usePageQuery'
  import { useRowAction } from '@/composables/useRowAction'
  import { formatTime } from '@/utils/time'
  import { asRow } from '@/utils/table'
  import ResetPasswordDialog from './components/ResetPasswordDialog.vue'
  import UserDetailDrawer from './components/UserDetailDrawer.vue'
  import UserFormDrawer from './components/UserFormDrawer.vue'

  const { items, total, pageNum, pageSize, loading, query, load, search, changePage } = usePageQuery(
    listAdminUsers,
    { keyword_text: '', user_status_code: '', role_code: '' }
  )

  const formDrawer = ref<InstanceType<typeof UserFormDrawer>>()
  const resetDialog = ref<InstanceType<typeof ResetPasswordDialog>>()
  const rowAction = useRowAction()

  const detailVisible = ref(false)
  const detailUser = ref<NWUserAdminVo | null>(null)
  const roleOptions = ref<NWRoleVo[]>([])
  const togglingUlid = ref('')

  const statusOptions = [
    { value: '', label: '全部状态' },
    { value: 'active', label: '正常' },
    { value: 'disabled', label: '已停用' }
  ]

  const statusLabel = (statusCode: string) =>
    statusOptions.find((item) => item.value === statusCode)?.label ?? statusCode

  const hasFilter = computed(
    () => Boolean(query.value.keyword_text || query.value.user_status_code || query.value.role_code)
  )

  /** 角色下拉：编码 → 名称，列表里也用它把编码渲染成可读标签 */
  const loadRoleOptions = async () => {
    try {
      const page = await listAdminRoles({ page_num: 1, page_size: 100 })
      roleOptions.value = page.items ?? []
    } catch {
      roleOptions.value = []
    }
  }

  const roleNameOf = (roleCode: string) =>
    roleOptions.value.find((item) => item.role_code === roleCode)?.role_name ?? roleCode

  /** 角色编码 → 名称映射（详情抽屉复用同一份数据，避免再拉一次角色列表） */
  const roleNameMap = computed<Record<string, string>>(() =>
    Object.fromEntries(roleOptions.value.map((item) => [item.role_code, item.role_name]))
  )

  const resetFilters = () => {
    query.value.keyword_text = ''
    query.value.user_status_code = ''
    query.value.role_code = ''
    void search()
  }

  const openDetail = (row: NWUserAdminVo) => {
    detailUser.value = row
    detailVisible.value = true
  }

  /** 启用/停用：停用要二次确认并说明「立即无法登录」 */
  const toggleStatus = async (row: NWUserAdminVo) => {
    const nextEnabled = row.user_status_code !== 'active'
    if (!nextEnabled) {
      try {
        await ElMessageBox.confirm(
          `停用后「${row.user_name}」立即无法登录，已登录的会话会在下一次请求时被拒绝。确定停用吗？`,
          '停用账号',
          { type: 'warning', confirmButtonText: '停用', cancelButtonText: '取消' }
        )
      } catch {
        return
      }
    }
    togglingUlid.value = row.user_ulid
    try {
      await updateAdminUser({
        user_ulid: row.user_ulid,
        user_status_code: nextEnabled ? 'active' : 'disabled'
      })
      ElMessage.success(nextEnabled ? '账号已启用' : '账号已停用')
      await load()
    } finally {
      togglingUlid.value = ''
    }
  }

  /** 删除用户（禁止删除自己由后端拦截，消息原样透出；行内加锁防连点） */
  const remove = async (row: NWUserAdminVo) => {
    try {
      await ElMessageBox.confirm(
        `将永久删除用户「${row.user_name}」及其角色挂载、模型授权与站内通知，此操作不可恢复。确定删除吗？`,
        '删除用户',
        { type: 'error', confirmButtonText: '永久删除', cancelButtonText: '取消' }
      )
    } catch {
      return
    }
    await rowAction.run(row.user_ulid, async () => {
      await deleteAdminUser(row.user_ulid)
      ElMessage.success('用户已删除')
      if (items.value.length === 1 && pageNum.value > 1) {
        pageNum.value -= 1
      }
      await load()
    })
  }

  const handleSaved = () => {
    void load()
  }

  onMounted(async () => {
    await loadRoleOptions()
    await load()
  })
</script>

<template>
  <div class="nw-page">
    <div class="nw-toolbar">
      <ElInput
        v-model="query.keyword_text"
        placeholder="搜索账号或邮箱"
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
      <ElSelect
        v-model="query.role_code"
        class="admin-users__status"
        placeholder="全部角色"
        clearable
        @change="search()"
      >
        <ElOption
          v-for="item in roleOptions"
          :key="item.role_ulid"
          :label="`${item.role_name}（${item.role_code}）`"
          :value="item.role_code"
        />
      </ElSelect>
      <ElButton type="primary" plain @click="search()">查询</ElButton>
      <ElButton :disabled="!hasFilter" @click="resetFilters()">重置</ElButton>
      <ElButton type="primary" @click="formDrawer?.open()">
        <Icon icon="mdi:plus" :size="15" />
        <span class="admin-users__btn-text">新建用户</span>
      </ElButton>
      <div class="admin-users__spacer" />
      <span class="nw-hint">共 {{ total }} 个账号</span>
    </div>

    <ElTable v-loading="loading" :data="items" class="admin-users__table" row-key="user_ulid">
      <ElTableColumn prop="user_name" label="账号" min-width="180">
        <template #default="{ row }">
          <ElButton link type="primary" @click="openDetail(asRow<NWUserAdminVo>(row))">
            {{ row.user_name }}
          </ElButton>
          <span v-if="row.remark_text" class="admin-users__remark">{{ row.remark_text }}</span>
        </template>
      </ElTableColumn>
      <ElTableColumn prop="user_email" label="邮箱" min-width="200" show-overflow-tooltip />
      <ElTableColumn label="角色" min-width="170">
        <template #default="{ row }">
          <div v-if="row.role_codes?.length" class="admin-users__roles">
            <span v-for="code in row.role_codes" :key="code" class="nw-badge nw-badge--neutral">
              {{ roleNameOf(code) }}
            </span>
          </div>
          <span v-else class="nw-hint">未分配</span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="状态" width="110">
        <template #default="{ row }">
          <div class="admin-users__status-cell">
            <span
              class="admin-users__dot"
              :class="row.user_status_code === 'active' ? 'is-active' : 'is-disabled'"
            />
            <span>{{ statusLabel(row.user_status_code) }}</span>
          </div>
        </template>
      </ElTableColumn>
      <ElTableColumn label="创建时间" width="170">
        <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
      </ElTableColumn>
      <ElTableColumn label="最近更新" width="170">
        <template #default="{ row }">{{ formatTime(row.updated_at) }}</template>
      </ElTableColumn>
      <ElTableColumn label="操作" width="320" fixed="right">
        <template #default="{ row }">
          <ElButton size="small" text type="primary" @click="formDrawer?.open(asRow<NWUserAdminVo>(row))">
            编辑
          </ElButton>
          <ElButton size="small" text @click="resetDialog?.open(asRow<NWUserAdminVo>(row))">
            重置密码
          </ElButton>
          <ElTooltip
            :content="row.user_status_code === 'active' ? '停用后立即无法登录' : '恢复该账号登录'"
            placement="top"
          >
            <ElSwitch
              class="admin-users__switch"
              :model-value="row.user_status_code === 'active'"
              :loading="togglingUlid === row.user_ulid"
              :disabled="Boolean(togglingUlid)"
              @change="toggleStatus(asRow<NWUserAdminVo>(row))"
            />
          </ElTooltip>
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
        <span class="nw-hint">{{ hasFilter ? '没有符合条件的账号，试试放宽筛选' : '暂无用户' }}</span>
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

    <UserFormDrawer ref="formDrawer" @saved="handleSaved" />
    <ResetPasswordDialog ref="resetDialog" @saved="handleSaved" />
    <UserDetailDrawer
      v-model:visible="detailVisible"
      :user="detailUser"
      :role-name-map="roleNameMap"
    />
  </div>
</template>

<style lang="less" scoped>
  .admin-users {
    &__search {
      width: 220px;
    }

    &__status {
      width: 150px;
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

    &__remark {
      display: block;
      margin-top: 2px;
      font-size: 11.5px;
      color: var(--el-text-color-placeholder);
    }

    &__roles {
      display: flex;
      flex-wrap: wrap;
      gap: 4px;
    }

    &__status-cell {
      display: flex;
      gap: 6px;
      align-items: center;
      font-size: 12.5px;
    }

    &__dot {
      width: 7px;
      height: 7px;
      border-radius: 50%;
      flex: none;

      &.is-active {
        background: var(--nw-success);
      }

      &.is-disabled {
        background: var(--nw-danger);
      }
    }

    &__switch {
      margin: 0 8px;
    }

    &__pager {
      display: flex;
      justify-content: flex-end;
    }
  }
</style>

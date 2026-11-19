<!--
  用户管理页：搜索区 + 用户分页表格（状态/角色标签）+ 新建弹窗 + 编辑抽屉 + 重置密码弹窗 + 删除。
  角色选项从角色列表加载，供新建/编辑表单多选使用。
-->
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import {
  deleteAdminUser,
  listAdminRoles,
  listAdminUsers,
  type NWUserAdminVo,
} from '@/api/admin'
import NWButton from '@/components/NWButton.vue'
import NWCard from '@/components/NWCard.vue'
import NWEmpty from '@/components/NWEmpty.vue'
import NWLoading from '@/components/NWLoading.vue'
import UserFormDialogs from './UserFormDialogs.vue'

const { t } = useI18n()

const NW_USER_STATUS_LIST = ['active', 'disabled'] as const

const userList = ref<NWUserAdminVo[]>([])
const listLoading = ref(false)
const total = ref(0)
const pageNum = ref(1)
const pageSize = ref(10)
const keywordText = ref('')
const statusFilter = ref('')
const roleOptions = ref<string[]>([])

const dialogVisible = ref(false)
const dialogMode = ref<'create' | 'edit'>('create')
const activeUser = ref<NWUserAdminVo | undefined>(undefined)
const deletingKey = ref('')
const formDialogsRef = ref<InstanceType<typeof UserFormDialogs> | null>(null)

function statusLabel(statusCode: string): string {
  return statusCode === 'disabled'
    ? t('admin.users.statusDisabled')
    : t('admin.users.statusActive')
}

function formatTime(timeMs: number): string {
  return new Date(timeMs).toLocaleString()
}

async function loadUserList(): Promise<void> {
  listLoading.value = true
  try {
    const pageData = await listAdminUsers({
      page_num: pageNum.value,
      page_size: pageSize.value,
      keyword_text: keywordText.value.trim() || undefined,
      user_status_code: statusFilter.value || undefined,
    })
    userList.value = pageData.items ?? []
    total.value = pageData.total ?? 0
  } catch {
    // 失败提示由 http 层统一弹出（静默），保持当前列表
  } finally {
    listLoading.value = false
  }
}

async function loadRoleOptions(): Promise<void> {
  try {
    const pageData = await listAdminRoles({ page_num: 1, page_size: 500 })
    roleOptions.value = (pageData.items ?? []).map((roleItem) => roleItem.role_code)
  } catch {
    // 失败提示由 http 层统一弹出，选项留空不阻塞页面
  }
}

function handleSearch(): void {
  pageNum.value = 1
  void loadUserList()
}

function handleResetFilters(): void {
  keywordText.value = ''
  statusFilter.value = ''
  handleSearch()
}

function handleCreateUser(): void {
  dialogMode.value = 'create'
  activeUser.value = undefined
  dialogVisible.value = true
}

function handleEditUser(userItem: NWUserAdminVo): void {
  dialogMode.value = 'edit'
  activeUser.value = userItem
  dialogVisible.value = true
}

function handleResetPassword(userItem: NWUserAdminVo): void {
  formDialogsRef.value?.openResetPassword(userItem)
}

async function handleDeleteUser(userItem: NWUserAdminVo): Promise<void> {
  try {
    await ElMessageBox.confirm(t('admin.users.deleteConfirm'), t('common.confirm'), {
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel'),
      type: 'warning',
    })
  } catch {
    return
  }
  deletingKey.value = userItem.user_ulid
  try {
    await deleteAdminUser(userItem.user_ulid)
    ElMessage.success(t('admin.users.deleteSuccess'))
    if (userList.value.length === 1 && pageNum.value > 1) {
      pageNum.value -= 1
    }
    await loadUserList()
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    deletingKey.value = ''
  }
}

function handlePageChange(page: number): void {
  pageNum.value = page
  void loadUserList()
}

function handleSizeChange(size: number): void {
  pageSize.value = size
  pageNum.value = 1
  void loadUserList()
}

onMounted(() => {
  void loadUserList()
  void loadRoleOptions()
})
</script>

<template>
  <div class="admin-users-page">
    <NWCard :hoverable="false">
      <template #header>
        <h3 class="nw-card__title">{{ t('admin.users.title') }}</h3>
        <div class="admin-users-page__toolbar">
          <el-input
            v-model="keywordText"
            class="admin-users-page__keyword"
            :placeholder="t('admin.users.keywordPlaceholder')"
            clearable
            @keyup.enter="handleSearch"
          />
          <el-select
            v-model="statusFilter"
            class="admin-users-page__status-select"
            :placeholder="t('admin.users.statusAll')"
            clearable
          >
            <el-option
              v-for="statusCode in NW_USER_STATUS_LIST"
              :key="statusCode"
              :value="statusCode"
              :label="statusLabel(statusCode)"
            />
          </el-select>
          <NWButton type="primary" @click="handleSearch">{{ t('admin.search') }}</NWButton>
          <NWButton @click="handleResetFilters">{{ t('admin.reset') }}</NWButton>
          <NWButton type="primary" @click="handleCreateUser">{{ t('common.create') }}</NWButton>
        </div>
      </template>

      <NWLoading v-if="listLoading" />
      <template v-else>
        <el-table :data="userList" stripe>
          <el-table-column prop="user_name" :label="t('admin.users.userName')" width="150" />
          <el-table-column prop="user_email" :label="t('admin.users.userEmail')" min-width="200" />
          <el-table-column :label="t('admin.users.status')" width="100">
            <template #default="{ row }">
              <el-tag :type="row.user_status_code === 'disabled' ? 'info' : 'success'">
                {{ statusLabel(row.user_status_code) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t('admin.users.roles')" min-width="180">
            <template #default="{ row }">
              <template v-if="row.role_codes && row.role_codes.length > 0">
                <el-tag
                  v-for="roleCode in row.role_codes"
                  :key="roleCode"
                  class="admin-users-page__role-tag"
                  size="small"
                >
                  {{ roleCode }}
                </el-tag>
              </template>
              <span v-else class="admin-users-page__muted">-</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('admin.users.createdAt')" width="170">
            <template #default="{ row }">
              {{ formatTime(row.created_at_ms) }}
            </template>
          </el-table-column>
          <el-table-column :label="t('common.action')" width="270" fixed="right">
            <template #default="{ row }">
              <NWButton size="small" @click="handleEditUser(row)">{{ t('common.edit') }}</NWButton>
              <NWButton size="small" @click="handleResetPassword(row)">
                {{ t('admin.users.resetPasswordTitle') }}
              </NWButton>
              <NWButton
                type="danger-text"
                size="small"
                :loading="deletingKey === row.user_ulid"
                @click="handleDeleteUser(row)"
              >
                {{ t('common.delete') }}
              </NWButton>
            </template>
          </el-table-column>
          <template #empty>
            <NWEmpty :description_text="t('admin.users.emptyTip')" />
          </template>
        </el-table>
        <el-pagination
          v-if="total > 0"
          class="admin-users-page__pagination"
          layout="total, prev, pager, next, sizes"
          :total="total"
          :current-page="pageNum"
          :page-size="pageSize"
          :page-sizes="[10, 20, 50]"
          @current-change="handlePageChange"
          @size-change="handleSizeChange"
        />
      </template>
    </NWCard>

    <UserFormDialogs
      ref="formDialogsRef"
      v-model="dialogVisible"
      :mode="dialogMode"
      :user-item="activeUser"
      :role-options="roleOptions"
      @saved="loadUserList"
    />
  </div>
</template>
<style scoped src="./users.css"></style>

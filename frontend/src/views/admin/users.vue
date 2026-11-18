<!--
  用户管理页：搜索区 + 用户分页表格（状态/角色标签）+ 新建弹窗 + 编辑抽屉 + 重置密码弹窗 + 删除。
  角色选项从角色列表加载，供新建/编辑表单多选使用。
-->
<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { useI18n } from 'vue-i18n'
import {
  createAdminUser,
  deleteAdminUser,
  listAdminRoles,
  listAdminUsers,
  resetAdminUserPassword,
  updateAdminUser,
  type NWUserAdminVo,
} from '@/api/admin'
import NWButton from '@/components/NWButton.vue'
import NWCard from '@/components/NWCard.vue'
import NWEmpty from '@/components/NWEmpty.vue'
import NWLoading from '@/components/NWLoading.vue'

const { t } = useI18n()

// 账号状态编码（对齐后端 user_status_code 枚举）
const NW_USER_STATUS_LIST = ['active', 'disabled'] as const

interface NWUserCreateFormState {
  user_name: string
  user_email: string
  password_text: string
  role_codes: string[]
  remark_text: string
}

interface NWUserEditFormState {
  user_ulid: string
  user_name: string
  user_email: string
  avatar_url: string
  remark_text: string
  user_status_code: string
  role_codes: string[]
}

const userList = ref<NWUserAdminVo[]>([])
const listLoading = ref(false)
const total = ref(0)
const pageNum = ref(1)
const pageSize = ref(10)
const keywordText = ref('')
const statusFilter = ref('')
const roleOptions = ref<string[]>([])

const createDialogVisible = ref(false)
const editDrawerVisible = ref(false)
const resetPasswordVisible = ref(false)
const creating = ref(false)
const updating = ref(false)
const resetting = ref(false)
const deletingKey = ref('')
const resetPasswordUser = ref<NWUserAdminVo | null>(null)
const newPasswordText = ref('')

const createFormRef = ref<FormInstance>()
const editFormRef = ref<FormInstance>()

const createFormState = reactive<NWUserCreateFormState>({
  user_name: '',
  user_email: '',
  password_text: '',
  role_codes: [],
  remark_text: '',
})

const editFormState = reactive<NWUserEditFormState>({
  user_ulid: '',
  user_name: '',
  user_email: '',
  avatar_url: '',
  remark_text: '',
  user_status_code: 'active',
  role_codes: [],
})

const createFormRules = computed<FormRules>(() => ({
  user_name: [{ required: true, message: t('admin.users.ruleUserNameRequired'), trigger: 'blur' }],
  user_email: [
    { required: true, message: t('admin.users.ruleEmailRequired'), trigger: 'blur' },
    { type: 'email', message: t('admin.users.ruleEmailInvalid'), trigger: 'blur' },
  ],
  password_text: [
    { required: true, message: t('admin.users.rulePasswordRequired'), trigger: 'blur' },
    { min: 6, message: t('admin.users.rulePasswordMin'), trigger: 'blur' },
  ],
}))

const editFormRules = computed<FormRules>(() => ({
  user_email: [
    { required: true, message: t('admin.users.ruleEmailRequired'), trigger: 'blur' },
    { type: 'email', message: t('admin.users.ruleEmailInvalid'), trigger: 'blur' },
  ],
}))

const newPasswordRules = computed<FormRules>(() => ({
  new_password_text: [
    { required: true, message: t('admin.users.rulePasswordRequired'), trigger: 'blur' },
    { min: 6, message: t('admin.users.rulePasswordMin'), trigger: 'blur' },
  ],
}))

/** 状态编码转展示文案 */
function statusLabel(statusCode: string): string {
  return statusCode === 'disabled'
    ? t('admin.users.statusDisabled')
    : t('admin.users.statusActive')
}

/** 毫秒时间戳转本地时间 */
function formatTime(timeMs: number): string {
  return new Date(timeMs).toLocaleString()
}

/** 加载用户分页 */
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

/** 加载角色编码选项（供新建/编辑表单多选） */
async function loadRoleOptions(): Promise<void> {
  try {
    const pageData = await listAdminRoles({ page_num: 1, page_size: 500 })
    roleOptions.value = (pageData.items ?? []).map((roleItem) => roleItem.role_code)
  } catch {
    // 失败提示由 http 层统一弹出，选项留空不阻塞页面
  }
}

/** 搜索：回到第一页重新加载 */
function handleSearch(): void {
  pageNum.value = 1
  void loadUserList()
}

/** 重置筛选条件 */
function handleResetFilters(): void {
  keywordText.value = ''
  statusFilter.value = ''
  handleSearch()
}

/** 打开新建弹窗 */
function handleCreateUser(): void {
  createFormState.user_name = ''
  createFormState.user_email = ''
  createFormState.password_text = ''
  createFormState.role_codes = []
  createFormState.remark_text = ''
  createDialogVisible.value = true
}

/** 新建用户 */
async function handleCreateSubmit(): Promise<void> {
  if (!createFormRef.value) {
    return
  }
  try {
    await createFormRef.value.validate()
  } catch {
    return
  }
  creating.value = true
  try {
    await createAdminUser({
      user_name: createFormState.user_name,
      user_email: createFormState.user_email,
      password_text: createFormState.password_text,
      role_codes: createFormState.role_codes,
      remark_text: createFormState.remark_text || undefined,
    })
    ElMessage.success(t('admin.users.createSuccess'))
    createDialogVisible.value = false
    await loadUserList()
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    creating.value = false
  }
}

/** 打开编辑抽屉 */
function handleEditUser(userItem: NWUserAdminVo): void {
  editFormState.user_ulid = userItem.user_ulid
  editFormState.user_name = userItem.user_name
  editFormState.user_email = userItem.user_email
  editFormState.avatar_url = userItem.avatar_url
  editFormState.remark_text = userItem.remark_text
  editFormState.user_status_code = userItem.user_status_code
  editFormState.role_codes = [...userItem.role_codes]
  editDrawerVisible.value = true
}

/** 保存编辑 */
async function handleEditSubmit(): Promise<void> {
  if (!editFormRef.value) {
    return
  }
  try {
    await editFormRef.value.validate()
  } catch {
    return
  }
  updating.value = true
  try {
    await updateAdminUser({
      user_ulid: editFormState.user_ulid,
      user_email: editFormState.user_email,
      avatar_url: editFormState.avatar_url,
      remark_text: editFormState.remark_text,
      user_status_code: editFormState.user_status_code,
      role_codes: editFormState.role_codes,
    })
    ElMessage.success(t('admin.users.updateSuccess'))
    editDrawerVisible.value = false
    await loadUserList()
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    updating.value = false
  }
}

/** 打开重置密码弹窗 */
function handleResetPassword(userItem: NWUserAdminVo): void {
  resetPasswordUser.value = userItem
  newPasswordText.value = ''
  resetPasswordVisible.value = true
}

/** 重置密码（二次确认） */
async function handleResetPasswordSubmit(): Promise<void> {
  if (!resetPasswordUser.value) {
    return
  }
  if (newPasswordText.value.length < 6) {
    ElMessage.warning(t('admin.users.rulePasswordMin'))
    return
  }
  try {
    await ElMessageBox.confirm(t('admin.users.resetPasswordConfirm'), t('common.confirm'), {
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel'),
      type: 'warning',
    })
  } catch {
    return
  }
  resetting.value = true
  try {
    await resetAdminUserPassword({
      user_ulid: resetPasswordUser.value.user_ulid,
      new_password_text: newPasswordText.value,
    })
    ElMessage.success(t('admin.users.resetPasswordSuccess'))
    resetPasswordVisible.value = false
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    resetting.value = false
  }
}

/** 删除用户（二次确认） */
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
                type="danger"
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

    <!-- 新建用户弹窗 -->
    <el-dialog v-model="createDialogVisible" :title="t('admin.users.createTitle')" width="600px">
      <el-form
        ref="createFormRef"
        :model="createFormState"
        :rules="createFormRules"
        label-position="top"
      >
        <el-form-item prop="user_name" :label="t('admin.users.userName')">
          <el-input v-model="createFormState.user_name" />
        </el-form-item>
        <el-form-item prop="user_email" :label="t('admin.users.userEmail')">
          <el-input v-model="createFormState.user_email" />
        </el-form-item>
        <el-form-item prop="password_text" :label="t('admin.users.password')">
          <el-input
            v-model="createFormState.password_text"
            type="password"
            show-password
            :placeholder="t('login.passwordPlaceholder')"
          />
        </el-form-item>
        <el-form-item prop="role_codes" :label="t('admin.users.roles')">
          <el-select
            v-model="createFormState.role_codes"
            class="admin-users-page__select"
            multiple
            clearable
          >
            <el-option
              v-for="roleCode in roleOptions"
              :key="roleCode"
              :value="roleCode"
              :label="roleCode"
            />
          </el-select>
        </el-form-item>
        <el-form-item prop="remark_text" :label="t('admin.users.remark')">
          <el-input v-model="createFormState.remark_text" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <NWButton @click="createDialogVisible = false">{{ t('common.cancel') }}</NWButton>
        <NWButton type="primary" :loading="creating" @click="handleCreateSubmit">
          {{ t('common.save') }}
        </NWButton>
      </template>
    </el-dialog>

    <!-- 编辑用户抽屉 -->
    <el-drawer v-model="editDrawerVisible" :title="t('admin.users.editTitle')" size="480px">
      <el-form
        ref="editFormRef"
        :model="editFormState"
        :rules="editFormRules"
        label-position="top"
      >
        <el-form-item :label="t('admin.users.userName')">
          <el-input :model-value="editFormState.user_name" disabled />
        </el-form-item>
        <el-form-item prop="user_email" :label="t('admin.users.userEmail')">
          <el-input v-model="editFormState.user_email" />
        </el-form-item>
        <el-form-item prop="avatar_url" :label="t('admin.users.avatarUrl')">
          <el-input v-model="editFormState.avatar_url" />
        </el-form-item>
        <el-form-item prop="user_status_code" :label="t('admin.users.status')">
          <el-radio-group v-model="editFormState.user_status_code">
            <el-radio
              v-for="statusCode in NW_USER_STATUS_LIST"
              :key="statusCode"
              :value="statusCode"
            >
              {{ statusLabel(statusCode) }}
            </el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item prop="role_codes" :label="t('admin.users.roles')">
          <el-select
            v-model="editFormState.role_codes"
            class="admin-users-page__select"
            multiple
            clearable
          >
            <el-option
              v-for="roleCode in roleOptions"
              :key="roleCode"
              :value="roleCode"
              :label="roleCode"
            />
          </el-select>
        </el-form-item>
        <el-form-item prop="remark_text" :label="t('admin.users.remark')">
          <el-input v-model="editFormState.remark_text" type="textarea" :rows="3" />
        </el-form-item>
      </el-form>
      <template #footer>
        <NWButton @click="editDrawerVisible = false">{{ t('common.cancel') }}</NWButton>
        <NWButton type="primary" :loading="updating" @click="handleEditSubmit">
          {{ t('common.save') }}
        </NWButton>
      </template>
    </el-drawer>

    <!-- 重置密码弹窗 -->
    <el-dialog
      v-model="resetPasswordVisible"
      :title="t('admin.users.resetPasswordTitle')"
      width="440px"
    >
      <p class="admin-users-page__reset-tip">
        {{ t('admin.users.resetPasswordTip') }}
        <strong>{{ resetPasswordUser?.user_name }}</strong>
      </p>
      <el-form :model="{ new_password_text: newPasswordText }" :rules="newPasswordRules" label-position="top">
        <el-form-item prop="new_password_text" :label="t('admin.users.newPassword')">
          <el-input
            v-model="newPasswordText"
            type="password"
            show-password
            :placeholder="t('login.passwordPlaceholder')"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <NWButton @click="resetPasswordVisible = false">{{ t('common.cancel') }}</NWButton>
        <NWButton type="danger" :loading="resetting" @click="handleResetPasswordSubmit">
          {{ t('common.confirm') }}
        </NWButton>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.admin-users-page__toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.admin-users-page__keyword {
  width: 220px;
}

.admin-users-page__status-select {
  width: 140px;
}

.admin-users-page__select {
  width: 100%;
}

.admin-users-page__role-tag {
  margin-right: 6px;
}

.admin-users-page__muted {
  color: var(--text-secondary);
}

.admin-users-page__pagination {
  margin-top: 14px;
  justify-content: flex-end;
}

.admin-users-page__reset-tip {
  margin-bottom: 14px;
  color: var(--text-secondary);
}
</style>

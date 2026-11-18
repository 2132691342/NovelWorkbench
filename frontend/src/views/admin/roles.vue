<!--
  角色管理页：角色分页表格（权限标签/内置标签）+ 保存弹窗（权限多选支持自定义）+ 删除。
  内置角色不可删除，删除按钮禁用并以 tooltip 说明原因。
-->
<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, ElTooltip, type FormInstance, type FormRules } from 'element-plus'
import { useI18n } from 'vue-i18n'
import {
  deleteAdminRole,
  listAdminRoles,
  saveAdminRole,
  type NWRoleVo,
} from '@/api/admin'
import NWButton from '@/components/NWButton.vue'
import NWCard from '@/components/NWCard.vue'
import NWEmpty from '@/components/NWEmpty.vue'
import NWLoading from '@/components/NWLoading.vue'

const { t } = useI18n()

// 已知权限编码选项（"*" 全部 + 细粒度管理权限，支持自定义输入）
const NW_PERM_CODE_LIST = [
  '*',
  'system:user:manage',
  'system:role:manage',
  'system:audit:read',
] as const

interface NWRoleFormState {
  role_ulid: string
  role_code: string
  role_name: string
  remark_text: string
  perm_codes: string[]
}

const roleList = ref<NWRoleVo[]>([])
const listLoading = ref(false)
const total = ref(0)
const pageNum = ref(1)
const pageSize = ref(10)
const dialogVisible = ref(false)
const saving = ref(false)
const deletingKey = ref('')
const formRef = ref<FormInstance>()

const formState = reactive<NWRoleFormState>({
  role_ulid: '',
  role_code: '',
  role_name: '',
  remark_text: '',
  perm_codes: [],
})

const formRules = computed<FormRules>(() => ({
  role_code: [{ required: true, message: t('admin.roles.ruleRoleCodeRequired'), trigger: 'blur' }],
  role_name: [{ required: true, message: t('admin.roles.ruleRoleNameRequired'), trigger: 'blur' }],
}))

/** 权限编码转展示文案 */
function permLabel(permCode: string): string {
  switch (permCode) {
    case '*':
      return t('admin.roles.permAll')
    case 'system:user:manage':
      return t('admin.roles.permUserManage')
    case 'system:role:manage':
      return t('admin.roles.permRoleManage')
    case 'system:audit:read':
      return t('admin.roles.permAuditRead')
    default:
      return permCode
  }
}

/** 加载角色分页 */
async function loadRoleList(): Promise<void> {
  listLoading.value = true
  try {
    const pageData = await listAdminRoles({
      page_num: pageNum.value,
      page_size: pageSize.value,
    })
    roleList.value = pageData.items ?? []
    total.value = pageData.total ?? 0
  } catch {
    // 失败提示由 http 层统一弹出（静默），保持当前列表
  } finally {
    listLoading.value = false
  }
}

/** 打开新建弹窗 */
function handleCreateRole(): void {
  formState.role_ulid = ''
  formState.role_code = ''
  formState.role_name = ''
  formState.remark_text = ''
  formState.perm_codes = []
  dialogVisible.value = true
}

/** 打开编辑弹窗 */
function handleEditRole(roleItem: NWRoleVo): void {
  formState.role_ulid = roleItem.role_ulid
  formState.role_code = roleItem.role_code
  formState.role_name = roleItem.role_name
  formState.remark_text = roleItem.remark_text
  formState.perm_codes = [...roleItem.perm_codes]
  dialogVisible.value = true
}

/** 保存（新建/编辑共用，role_ulid 为空表示新建） */
async function handleSaveRole(): Promise<void> {
  if (!formRef.value) {
    return
  }
  try {
    await formRef.value.validate()
  } catch {
    return
  }
  saving.value = true
  try {
    await saveAdminRole({
      role_ulid: formState.role_ulid || undefined,
      role_code: formState.role_code,
      role_name: formState.role_name,
      remark_text: formState.remark_text || undefined,
      perm_codes: formState.perm_codes,
    })
    ElMessage.success(t('admin.roles.saveSuccess'))
    dialogVisible.value = false
    await loadRoleList()
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    saving.value = false
  }
}

/** 删除角色（二次确认，内置角色由按钮禁用拦截） */
async function handleDeleteRole(roleItem: NWRoleVo): Promise<void> {
  try {
    await ElMessageBox.confirm(t('admin.roles.deleteConfirm'), t('common.confirm'), {
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel'),
      type: 'warning',
    })
  } catch {
    return
  }
  deletingKey.value = roleItem.role_ulid
  try {
    await deleteAdminRole(roleItem.role_ulid)
    ElMessage.success(t('admin.roles.deleteSuccess'))
    if (roleList.value.length === 1 && pageNum.value > 1) {
      pageNum.value -= 1
    }
    await loadRoleList()
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    deletingKey.value = ''
  }
}

function handlePageChange(page: number): void {
  pageNum.value = page
  void loadRoleList()
}

function handleSizeChange(size: number): void {
  pageSize.value = size
  pageNum.value = 1
  void loadRoleList()
}

onMounted(() => {
  void loadRoleList()
})
</script>

<template>
  <div class="admin-roles-page">
    <NWCard :hoverable="false">
      <template #header>
        <h3 class="nw-card__title">{{ t('admin.roles.title') }}</h3>
        <NWButton type="primary" @click="handleCreateRole">{{ t('common.create') }}</NWButton>
      </template>

      <NWLoading v-if="listLoading" />
      <template v-else>
        <el-table :data="roleList" stripe>
          <el-table-column prop="role_code" :label="t('admin.roles.roleCode')" width="180" />
          <el-table-column prop="role_name" :label="t('admin.roles.roleName')" width="150" />
          <el-table-column :label="t('admin.roles.permCodes')" min-width="240">
            <template #default="{ row }">
              <template v-if="row.perm_codes && row.perm_codes.length > 0">
                <el-tag
                  v-for="permCode in row.perm_codes"
                  :key="permCode"
                  class="admin-roles-page__perm-tag"
                  size="small"
                  :title="permLabel(permCode)"
                >
                  {{ permCode }}
                </el-tag>
              </template>
              <span v-else class="admin-roles-page__muted">-</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('admin.roles.builtinFlag')" width="90">
            <template #default="{ row }">
              <el-tag :type="row.builtin_flag ? 'warning' : 'primary'">
                {{ row.builtin_flag ? t('admin.roles.builtinTag') : t('admin.roles.customTag') }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="remark_text" :label="t('admin.roles.remark')" min-width="160" />
          <el-table-column :label="t('common.action')" width="180">
            <template #default="{ row }">
              <NWButton size="small" @click="handleEditRole(row)">{{ t('common.edit') }}</NWButton>
              <ElTooltip
                :content="t('admin.roles.builtinDeleteDisabled')"
                :disabled="!row.builtin_flag"
                placement="top"
              >
                <span class="admin-roles-page__delete-wrap">
                  <NWButton
                    type="danger"
                    size="small"
                    :disabled="row.builtin_flag"
                    :loading="deletingKey === row.role_ulid"
                    @click="handleDeleteRole(row)"
                  >
                    {{ t('common.delete') }}
                  </NWButton>
                </span>
              </ElTooltip>
            </template>
          </el-table-column>
          <template #empty>
            <NWEmpty :description_text="t('admin.roles.emptyTip')" />
          </template>
        </el-table>
        <el-pagination
          v-if="total > 0"
          class="admin-roles-page__pagination"
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

    <!-- 保存角色弹窗 -->
    <el-dialog
      v-model="dialogVisible"
      :title="formState.role_ulid ? t('admin.roles.editTitle') : t('admin.roles.createTitle')"
      width="600px"
    >
      <el-form ref="formRef" :model="formState" :rules="formRules" label-position="top">
        <el-form-item prop="role_code" :label="t('admin.roles.roleCode')">
          <el-input v-model="formState.role_code" :disabled="Boolean(formState.role_ulid)" />
        </el-form-item>
        <el-form-item prop="role_name" :label="t('admin.roles.roleName')">
          <el-input v-model="formState.role_name" />
        </el-form-item>
        <el-form-item prop="perm_codes" :label="t('admin.roles.permCodes')">
          <el-select
            v-model="formState.perm_codes"
            class="admin-roles-page__select"
            multiple
            filterable
            allow-create
            default-first-option
            clearable
          >
            <el-option
              v-for="permCode in NW_PERM_CODE_LIST"
              :key="permCode"
              :value="permCode"
              :label="`${permLabel(permCode)} (${permCode})`"
            />
          </el-select>
        </el-form-item>
        <el-form-item prop="remark_text" :label="t('admin.roles.remark')">
          <el-input v-model="formState.remark_text" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <NWButton @click="dialogVisible = false">{{ t('common.cancel') }}</NWButton>
        <NWButton type="primary" :loading="saving" @click="handleSaveRole">
          {{ t('common.save') }}
        </NWButton>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.admin-roles-page__select {
  width: 100%;
}

.admin-roles-page__perm-tag {
  margin-right: 6px;
}

.admin-roles-page__muted {
  color: var(--text-secondary);
}

.admin-roles-page__delete-wrap {
  display: inline-block;
}

.admin-roles-page__pagination {
  margin-top: 14px;
  justify-content: flex-end;
}
</style>

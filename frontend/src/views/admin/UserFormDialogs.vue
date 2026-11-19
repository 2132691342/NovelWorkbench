<script setup lang="ts">

// 用户表单对话框：创建/编辑用户与重置密码的弹窗表单。
import { computed, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { useI18n } from 'vue-i18n'
import {
  createAdminUser,
  resetAdminUserPassword,
  updateAdminUser,
  type NWUserAdminVo,
} from '@/api/admin'
import NWButton from '@/components/NWButton.vue'

const props = defineProps<{
  modelValue: boolean
  mode: 'create' | 'edit'
  userItem?: NWUserAdminVo
  roleOptions: string[]
}>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  saved: []
}>()

const { t } = useI18n()

const NW_USER_STATUS_LIST = ['active', 'disabled'] as const

const creating = ref(false)
const updating = ref(false)
const resetting = ref(false)
const createFormRef = ref<FormInstance>()
const editFormRef = ref<FormInstance>()
const resetFormRef = ref<FormInstance>()

const createFormState = reactive({
  user_name: '',
  user_email: '',
  password_text: '',
  role_codes: [] as string[],
  remark_text: '',
})

const editFormState = reactive({
  user_ulid: '',
  user_name: '',
  user_email: '',
  avatar_url: '',
  remark_text: '',
  user_status_code: 'active',
  role_codes: [] as string[],
})
watch(
  () => props.userItem,
  (userItem) => {
    if (!userItem) {
      return
    }
    editFormState.user_ulid = userItem.user_ulid
    editFormState.user_name = userItem.user_name
    editFormState.user_email = userItem.user_email
    editFormState.avatar_url = userItem.avatar_url
    editFormState.remark_text = userItem.remark_text
    editFormState.user_status_code = userItem.user_status_code
    editFormState.role_codes = [...userItem.role_codes]
  },
  { immediate: true },
)
watch(
  () => props.modelValue && props.mode === 'create',
  (visible) => {
    if (!visible) {
      return
    }
    createFormState.user_name = ''
    createFormState.user_email = ''
    createFormState.password_text = ''
    createFormState.role_codes = []
    createFormState.remark_text = ''
  },
)

defineExpose({ openResetPassword })
const newPasswordText = ref('')

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

const createVisible = computed({
  get: () => props.modelValue && props.mode === 'create',
  set: (value: boolean) => emit('update:modelValue', value),
})
const editVisible = computed({
  get: () => props.modelValue && props.mode === 'edit',
  set: (value: boolean) => emit('update:modelValue', value),
})
const resetVisible = ref(false)

function statusLabel(statusCode: string): string {
  return statusCode === 'disabled'
    ? t('admin.users.statusDisabled')
    : t('admin.users.statusActive')
}

const resetUser = ref<NWUserAdminVo | null>(null)

function openResetPassword(userItem: NWUserAdminVo): void {
  resetUser.value = userItem
  newPasswordText.value = ''
  resetVisible.value = true
}

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
    createVisible.value = false
    emit('saved')
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    creating.value = false
  }
}

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
    editVisible.value = false
    emit('saved')
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    updating.value = false
  }
}

async function handleResetPasswordSubmit(): Promise<void> {
  if (!resetUser.value || !resetFormRef.value) {
    return
  }
  try {
    await resetFormRef.value.validate()
  } catch {
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
      user_ulid: resetUser.value.user_ulid,
      new_password_text: newPasswordText.value,
    })
    ElMessage.success(t('admin.users.resetPasswordSuccess'))
    resetVisible.value = false
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    resetting.value = false
  }
}
</script>

<template>
  <el-dialog v-model="createVisible" :title="t('admin.users.createTitle')" width="600px">
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
      <NWButton @click="createVisible = false">{{ t('common.cancel') }}</NWButton>
      <NWButton type="primary" :loading="creating" @click="handleCreateSubmit">
        {{ t('common.save') }}
      </NWButton>
    </template>
  </el-dialog>

  <el-drawer v-model="editVisible" :title="t('admin.users.editTitle')" size="480px">
    <el-form
      ref="editFormRef"
      :model="editFormState"
      :rules="editFormRules"
      label-position="top"
    >
      <el-form-item :label="t('admin.users.userName')">
        <el-input :model-value="userItem?.user_name ?? ''" disabled />
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
      <NWButton @click="editVisible = false">{{ t('common.cancel') }}</NWButton>
      <NWButton type="primary" :loading="updating" @click="handleEditSubmit">
        {{ t('common.save') }}
      </NWButton>
    </template>
  </el-drawer>

  <el-dialog
    v-model="resetVisible"
    :title="t('admin.users.resetPasswordTitle')"
    width="440px"
  >
    <p class="admin-users-page__reset-tip">
      {{ t('admin.users.resetPasswordTip') }}
      <strong>{{ resetUser?.user_name }}</strong>
    </p>
    <el-form
      ref="resetFormRef"
      :model="{ new_password_text: newPasswordText }"
      :rules="newPasswordRules"
      label-position="top"
    >
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
      <NWButton @click="resetVisible = false">{{ t('common.cancel') }}</NWButton>
      <NWButton type="danger" :loading="resetting" @click="handleResetPasswordSubmit">
        {{ t('common.confirm') }}
      </NWButton>
    </template>
  </el-dialog>
</template>
<style scoped>
.admin-users-page__select {
  width: 100%;
}

.admin-users-page__reset-tip {
  margin-bottom: 14px;
  color: var(--text-secondary);
}
</style>

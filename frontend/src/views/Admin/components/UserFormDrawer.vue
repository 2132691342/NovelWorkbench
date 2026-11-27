<!-- 用户编辑抽屉：新建账号（初始密码 + 强度提示）与编辑资料/角色/状态。 -->
<script setup lang="ts">
  import { computed, reactive, ref } from 'vue'
  import {
    ElAlert,
    ElButton,
    ElDrawer,
    ElForm,
    ElFormItem,
    ElInput,
    ElMessage,
    ElOption,
    ElSelect,
    type FormInstance,
    type FormRules
  } from 'element-plus'
  import {
    createAdminUser,
    listAdminRoles,
    updateAdminUser,
    type NWRoleVo,
    type NWUserAdminVo
  } from '@/api/admin'
  import { checkPasswordStrength } from './passwordStrength'

  const emit = defineEmits<{ saved: [] }>()

  const visible = ref(false)
  const saving = ref(false)
  const formRef = ref<FormInstance>()
  const editing = ref<NWUserAdminVo | null>(null)
  const roleOptions = ref<NWRoleVo[]>([])
  const roleLoading = ref(false)

  const form = reactive({
    user_name: '',
    user_email: '',
    password_text: '',
    user_status_code: 'active',
    role_codes: [] as string[],
    remark_text: ''
  })

  const title = computed(() => (editing.value ? '编辑用户' : '新建用户'))
  const isEdit = computed(() => editing.value !== null)

  const statusOptions = [
    { value: 'active', label: '正常' },
    { value: 'disabled', label: '禁用' }
  ]

  /** 密码强度：新建时必填校验，编辑时不展示密码字段 */
  const strength = computed(() => checkPasswordStrength(form.password_text))

  const rules = computed<FormRules>(() => ({
    user_name: isEdit.value
      ? []
      : [
          { required: true, message: '请输入登录名', trigger: 'blur' },
          { min: 3, max: 64, message: '长度 3-64 个字符', trigger: 'blur' }
        ],
    user_email: [
      { required: true, message: '请输入邮箱', trigger: 'blur' },
      { type: 'email', message: '邮箱格式不正确', trigger: 'blur' }
    ],
    password_text: isEdit.value
      ? []
      : [
          { required: true, message: '请输入初始密码', trigger: 'blur' },
          {
            validator: (_rule, value: string, callback: (error?: Error) => void) => {
              if (!checkPasswordStrength(value).passed) {
                callback(new Error('密码至少 8 位，且同时包含字母与数字'))
                return
              }
              callback()
            },
            trigger: 'blur'
          }
        ]
  }))

  /** 角色下拉（一次取满，用于把角色编码映射成可读标签） */
  const loadRoleOptions = async () => {
    roleLoading.value = true
    try {
      const page = await listAdminRoles({ page_num: 1, page_size: 100 })
      roleOptions.value = page.items ?? []
    } catch {
      roleOptions.value = []
    } finally {
      roleLoading.value = false
    }
  }

  const resetForm = (user?: NWUserAdminVo) => {
    form.user_name = user?.user_name ?? ''
    form.user_email = user?.user_email ?? ''
    form.password_text = ''
    form.user_status_code = user?.user_status_code || 'active'
    form.remark_text = user?.remark_text ?? ''
    form.role_codes = [...(user?.role_codes ?? [])]
  }

  /** 打开抽屉：传用户为编辑，不传为新建 */
  const open = async (user?: NWUserAdminVo) => {
    editing.value = user ?? null
    resetForm(user)
    visible.value = true
    formRef.value?.clearValidate()
    await loadRoleOptions()
  }

  const submit = async () => {
    const valid = await formRef.value?.validate().catch(() => false)
    if (!valid) return

    saving.value = true
    try {
      if (editing.value) {
        await updateAdminUser({
          user_ulid: editing.value.user_ulid,
          user_email: form.user_email,
          user_status_code: form.user_status_code,
          role_codes: form.role_codes,
          remark_text: form.remark_text
        })
        ElMessage.success('用户已更新')
      } else {
        await createAdminUser({
          user_name: form.user_name,
          user_email: form.user_email,
          password_text: form.password_text,
          role_codes: form.role_codes,
          remark_text: form.remark_text
        })
        ElMessage.success('用户已创建')
      }
      visible.value = false
      emit('saved')
    } finally {
      saving.value = false
    }
  }

  defineExpose({ open })
</script>

<template>
  <ElDrawer v-model="visible" :title="title" size="480px" append-to-body destroy-on-close>
    <ElForm ref="formRef" :model="form" :rules="rules" label-position="top">
      <ElFormItem label="登录名" prop="user_name">
        <ElInput
          v-model="form.user_name"
          :disabled="isEdit"
          maxlength="64"
          placeholder="用于登录的唯一账号"
        />
      </ElFormItem>
      <ElFormItem label="邮箱" prop="user_email">
        <ElInput v-model="form.user_email" maxlength="128" placeholder="name@example.com" />
      </ElFormItem>

      <ElFormItem v-if="!isEdit" label="初始密码" prop="password_text">
        <ElInput
          v-model="form.password_text"
          type="password"
          show-password
          maxlength="64"
          placeholder="至少 8 位，含字母与数字"
        />
        <div v-if="form.password_text" class="user-form__strength">
          <span class="user-form__strength-bar">
            <i
              v-for="level in 3"
              :key="level"
              class="user-form__strength-cell"
              :class="
                level <= strength.score
                  ? `is-filled is-${strength.tone}`
                  : ''
              "
            />
          </span>
          <span class="nw-hint">{{ strength.hint }}</span>
        </div>
        <p v-else class="user-form__tip">密码至少 8 位，且同时包含字母与数字。</p>
      </ElFormItem>

      <ElFormItem v-if="isEdit" label="账号状态">
        <ElSelect v-model="form.user_status_code" class="user-form__field">
          <ElOption
            v-for="item in statusOptions"
            :key="item.value"
            :label="item.label"
            :value="item.value"
          />
        </ElSelect>
        <p class="user-form__tip">停用后该账号立即无法登录；重新启用即可恢复。</p>
      </ElFormItem>

      <ElFormItem label="角色">
        <ElSelect
          v-model="form.role_codes"
          multiple
          collapse-tags
          collapse-tags-tooltip
          :loading="roleLoading"
          class="user-form__field"
          placeholder="不选则默认普通用户"
        >
          <ElOption
            v-for="item in roleOptions"
            :key="item.role_ulid"
            :label="`${item.role_name}（${item.role_code}）`"
            :value="item.role_code"
          />
        </ElSelect>
        <p class="user-form__tip">不选择任何角色时，新账号默认挂载普通用户角色。</p>
      </ElFormItem>

      <ElFormItem label="备注">
        <ElInput v-model="form.remark_text" type="textarea" :rows="3" maxlength="256" show-word-limit />
      </ElFormItem>
    </ElForm>

    <ElAlert
      v-if="isEdit && form.user_status_code === 'disabled'"
      type="warning"
      :closable="false"
      show-icon
      title="保存后该账号将被立即停用"
      description="已登录的会话会在下一次请求时被拒绝，需要重新启用才能登录。"
    />

    <template #footer>
      <ElButton @click="visible = false">取消</ElButton>
      <ElButton type="primary" :loading="saving" @click="submit">
        {{ isEdit ? '保存修改' : '创建用户' }}
      </ElButton>
    </template>
  </ElDrawer>
</template>

<style lang="less" scoped>
  .user-form {
    &__field {
      width: 100%;
    }

    &__tip {
      margin-top: 4px;
      font-size: 11.5px;
      line-height: 1.6;
      color: var(--el-text-color-secondary);
    }

    &__strength {
      display: flex;
      gap: 8px;
      align-items: center;
      margin-top: 6px;
    }

    &__strength-bar {
      display: flex;
      gap: 3px;
    }

    &__strength-cell {
      width: 26px;
      height: 4px;
      background: var(--el-fill-color);
      border-radius: 2px;
      transition: background-color 0.15s ease;

      &.is-filled.is-weak {
        background: var(--nw-danger);
      }

      &.is-filled.is-medium {
        background: var(--nw-warning);
      }

      &.is-filled.is-strong {
        background: var(--nw-success);
      }
    }
  }
</style>

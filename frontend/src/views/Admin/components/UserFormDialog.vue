<!-- 用户编辑弹窗：维护账号资料与角色分配。 -->
<script setup lang="ts">
  import { computed, reactive, ref } from 'vue'
  import {
    ElButton,
    ElDialog,
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

  const emit = defineEmits<{ saved: [] }>()

  const visible = ref(false)
  const saving = ref(false)
  const formRef = ref<FormInstance>()
  const editing = ref<NWUserAdminVo | null>(null)
  const roleOptions = ref<NWRoleVo[]>([])

  const form = reactive({
    user_name: '',
    user_email: '',
    password_text: '',
    user_status_code: 'active',
    role_ulids: [] as string[],
    remark_text: ''
  })

  const title = computed(() => (editing.value ? '编辑用户' : '新建用户'))

  const statusOptions = [
    { value: 'active', label: '正常' },
    { value: 'disabled', label: '禁用' }
  ]

  const rules = computed<FormRules>(() => ({
    user_name: editing.value
      ? []
      : [
          { required: true, message: '请输入登录名', trigger: 'blur' },
          { min: 3, max: 64, message: '长度 3-64 个字符', trigger: 'blur' }
        ],
    user_email: [
      { required: true, message: '请输入邮箱', trigger: 'blur' },
      { type: 'email', message: '邮箱格式不正确', trigger: 'blur' }
    ],
    password_text: editing.value
      ? []
      : [
          { required: true, message: '请输入初始密码', trigger: 'blur' },
          { min: 6, max: 64, message: '长度 6-64 个字符', trigger: 'blur' }
        ]
  }))

  /** 角色下拉数据（分页接口一次取满） */
  const loadRoleOptions = async () => {
    try {
      const page = await listAdminRoles({ page_num: 1, page_size: 100 })
      roleOptions.value = page.items ?? []
    } catch {
      roleOptions.value = []
    }
  }

  /** 角色 ULID 映射为后端需要的角色编码 */
  const selectedRoleCodes = () =>
    roleOptions.value
      .filter((item) => form.role_ulids.includes(item.role_ulid))
      .map((item) => item.role_code)

  const resetForm = (user?: NWUserAdminVo) => {
    form.user_name = user?.user_name ?? ''
    form.user_email = user?.user_email ?? ''
    form.password_text = ''
    form.user_status_code = user?.user_status_code || 'active'
    form.remark_text = user?.remark_text ?? ''
    form.role_ulids = user
      ? roleOptions.value
          .filter((item) => (user.role_codes ?? []).includes(item.role_code))
          .map((item) => item.role_ulid)
      : []
  }

  /** 打开弹窗：传用户为编辑，不传为新建 */
  const open = async (user?: NWUserAdminVo) => {
    editing.value = user ?? null
    visible.value = true
    if (roleOptions.value.length === 0) {
      await loadRoleOptions()
    }
    resetForm(user)
    formRef.value?.clearValidate()
  }

  const submit = async () => {
    const valid = await formRef.value?.validate().catch(() => false)
    if (!valid) return

    saving.value = true
    try {
      const roleCodes = selectedRoleCodes()
      if (editing.value) {
        await updateAdminUser({
          user_ulid: editing.value.user_ulid,
          user_email: form.user_email,
          user_status_code: form.user_status_code,
          role_codes: roleCodes,
          remark_text: form.remark_text
        })
      } else {
        await createAdminUser({
          user_name: form.user_name,
          user_email: form.user_email,
          password_text: form.password_text,
          role_codes: roleCodes,
          remark_text: form.remark_text
        })
      }
      ElMessage.success(editing.value ? '用户已更新' : '用户已创建')
      visible.value = false
      emit('saved')
    } finally {
      saving.value = false
    }
  }

  defineExpose({ open })
</script>

<template>
  <ElDialog v-model="visible" :title="title" width="520px" append-to-body destroy-on-close>
    <ElForm ref="formRef" :model="form" :rules="rules" label-position="top">
      <ElFormItem label="登录名" prop="user_name">
        <ElInput
          v-model="form.user_name"
          :disabled="Boolean(editing)"
          maxlength="64"
          placeholder="用于登录的唯一账号"
        />
      </ElFormItem>
      <ElFormItem label="邮箱" prop="user_email">
        <ElInput v-model="form.user_email" maxlength="128" placeholder="name@example.com" />
      </ElFormItem>
      <ElFormItem v-if="!editing" label="初始密码" prop="password_text">
        <ElInput
          v-model="form.password_text"
          type="password"
          show-password
          maxlength="64"
          placeholder="至少 6 位"
        />
      </ElFormItem>
      <ElFormItem v-if="editing" label="账号状态">
        <ElSelect v-model="form.user_status_code" class="user-form__select">
          <ElOption
            v-for="item in statusOptions"
            :key="item.value"
            :label="item.label"
            :value="item.value"
          />
        </ElSelect>
      </ElFormItem>
      <ElFormItem label="角色">
        <ElSelect
          v-model="form.role_ulids"
          multiple
          collapse-tags
          collapse-tags-tooltip
          class="user-form__select"
          placeholder="不选则默认普通用户"
        >
          <ElOption
            v-for="item in roleOptions"
            :key="item.role_ulid"
            :label="`${item.role_name}（${item.role_code}）`"
            :value="item.role_ulid"
          />
        </ElSelect>
      </ElFormItem>
      <ElFormItem label="备注">
        <ElInput v-model="form.remark_text" type="textarea" :rows="2" maxlength="256" />
      </ElFormItem>
    </ElForm>
    <template #footer>
      <ElButton @click="visible = false">取消</ElButton>
      <ElButton type="primary" :loading="saving" @click="submit">保存</ElButton>
    </template>
  </ElDialog>
</template>

<style lang="less" scoped>
  .user-form {
    &__select {
      width: 100%;
    }
  }
</style>

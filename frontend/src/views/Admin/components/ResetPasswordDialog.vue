<!-- 重置密码弹窗：管理员为用户设置新密码。 -->
<script setup lang="ts">
  import { reactive, ref } from 'vue'
  import {
    ElButton,
    ElDialog,
    ElForm,
    ElFormItem,
    ElInput,
    ElMessage,
    type FormInstance,
    type FormRules
  } from 'element-plus'
  import { resetAdminUserPassword, type NWUserAdminVo } from '@/api/admin'

  const emit = defineEmits<{ saved: [] }>()

  const visible = ref(false)
  const saving = ref(false)
  const formRef = ref<FormInstance>()
  const target = ref<NWUserAdminVo | null>(null)

  const form = reactive({
    new_password_text: '',
    confirm_password_text: ''
  })

  const rules: FormRules = {
    new_password_text: [
      { required: true, message: '请输入新密码', trigger: 'blur' },
      { min: 6, max: 64, message: '长度 6-64 个字符', trigger: 'blur' }
    ],
    confirm_password_text: [
      { required: true, message: '请再次输入新密码', trigger: 'blur' },
      {
        validator: (_rule, value: string, callback: (error?: Error) => void) => {
          if (value !== form.new_password_text) {
            callback(new Error('两次输入的密码不一致'))
            return
          }
          callback()
        },
        trigger: 'blur'
      }
    ]
  }

  /** 打开重置密码弹窗 */
  const open = (user: NWUserAdminVo) => {
    target.value = user
    form.new_password_text = ''
    form.confirm_password_text = ''
    visible.value = true
    formRef.value?.clearValidate()
  }

  const submit = async () => {
    const valid = await formRef.value?.validate().catch(() => false)
    if (!valid || !target.value) return

    saving.value = true
    try {
      await resetAdminUserPassword({
        user_ulid: target.value.user_ulid,
        new_password_text: form.new_password_text
      })
      ElMessage.success('密码已重置')
      visible.value = false
      emit('saved')
    } finally {
      saving.value = false
    }
  }

  defineExpose({ open })
</script>

<template>
  <ElDialog v-model="visible" title="重置密码" width="420px" append-to-body destroy-on-close>
    <p class="reset-password__hint">
      将为「{{ target?.user_name }}」设置新密码，对方下次登录即生效。
    </p>
    <ElForm ref="formRef" :model="form" :rules="rules" label-position="top">
      <ElFormItem label="新密码" prop="new_password_text">
        <ElInput
          v-model="form.new_password_text"
          type="password"
          show-password
          maxlength="64"
          placeholder="至少 6 位"
        />
      </ElFormItem>
      <ElFormItem label="确认新密码" prop="confirm_password_text">
        <ElInput
          v-model="form.confirm_password_text"
          type="password"
          show-password
          maxlength="64"
        />
      </ElFormItem>
    </ElForm>
    <template #footer>
      <ElButton @click="visible = false">取消</ElButton>
      <ElButton type="primary" :loading="saving" @click="submit">确认重置</ElButton>
    </template>
  </ElDialog>
</template>

<style lang="less" scoped>
  .reset-password {
    &__hint {
      margin-bottom: 14px;
      font-size: 12.5px;
      color: var(--nw-text-secondary);
    }
  }
</style>

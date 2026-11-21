<!-- 个人中心：查看账号信息、编辑资料与修改密码。 -->
<script setup lang="ts">
  import { onMounted, reactive, ref } from 'vue'
  import {
    ElAlert,
    ElButton,
    ElCard,
    ElForm,
    ElFormItem,
    ElInput,
    ElMessage,
    type FormInstance,
    type FormRules
  } from 'element-plus'
  import { changePassword, fetchProfileDetail, updateProfile } from '@/api/profile'
  import type { NWProfileVo } from '@/api/profile'
  import { useUserStore } from '@/store/modules/user'
  import { formatTime } from '@/utils/time'

  const userStore = useUserStore()

  const profile = ref<NWProfileVo | null>(null)
  const loading = ref(false)
  const savingProfile = ref(false)
  const savingPassword = ref(false)

  const profileFormRef = ref<FormInstance>()
  const passwordFormRef = ref<FormInstance>()

  const profileForm = reactive({
    user_email: '',
    avatar_url: '',
    remark_text: ''
  })

  const passwordForm = reactive({
    old_password_text: '',
    new_password_text: '',
    confirm_password_text: ''
  })

  const profileRules: FormRules = {
    user_email: [
      { required: true, message: '请输入邮箱', trigger: 'blur' },
      { type: 'email', message: '邮箱格式不正确', trigger: 'blur' }
    ]
  }

  const passwordRules: FormRules = {
    old_password_text: [{ required: true, message: '请输入当前密码', trigger: 'blur' }],
    new_password_text: [
      { required: true, message: '请输入新密码', trigger: 'blur' },
      { min: 6, max: 64, message: '长度 6-64 个字符', trigger: 'blur' }
    ],
    confirm_password_text: [
      { required: true, message: '请再次输入新密码', trigger: 'blur' },
      {
        validator: (_rule, value: string, callback: (error?: Error) => void) => {
          if (value !== passwordForm.new_password_text) {
            callback(new Error('两次输入的新密码不一致'))
            return
          }
          callback()
        },
        trigger: 'blur'
      }
    ]
  }

  const load = async () => {
    loading.value = true
    try {
      profile.value = await fetchProfileDetail()
      profileForm.user_email = profile.value.user_email
      profileForm.avatar_url = profile.value.avatar_url
      profileForm.remark_text = profile.value.remark_text
    } catch {
      profile.value = null
    } finally {
      loading.value = false
    }
  }

  const saveProfile = async () => {
    const valid = await profileFormRef.value?.validate().catch(() => false)
    if (!valid) return

    savingProfile.value = true
    try {
      profile.value = await updateProfile({
        user_email: profileForm.user_email,
        avatar_url: profileForm.avatar_url,
        remark_text: profileForm.remark_text
      })
      await userStore.fetchProfile()
      ElMessage.success('资料已更新')
    } finally {
      savingProfile.value = false
    }
  }

  const savePassword = async () => {
    const valid = await passwordFormRef.value?.validate().catch(() => false)
    if (!valid) return

    savingPassword.value = true
    try {
      await changePassword({
        old_password_text: passwordForm.old_password_text,
        new_password_text: passwordForm.new_password_text
      })
      ElMessage.success('密码已修改，请牢记新密码')
      passwordForm.old_password_text = ''
      passwordForm.new_password_text = ''
      passwordForm.confirm_password_text = ''
      passwordFormRef.value?.clearValidate()
    } finally {
      savingPassword.value = false
    }
  }

  onMounted(() => {
    void load()
  })
</script>

<template>
  <div class="nw-page">
    <div v-loading="loading" class="profile__grid">
      <ElCard shadow="never" class="profile__card">
        <template #header>
          <span class="profile__title">账号信息</span>
        </template>
        <div v-if="profile" class="profile__account">
          <span class="profile__avatar">
            <img v-if="profile.avatar_url" :src="profile.avatar_url" alt="头像" />
            <template v-else>{{ (profile.user_name || 'U').slice(0, 1).toUpperCase() }}</template>
          </span>
          <div class="profile__account-text">
            <h3 class="profile__name">{{ profile.user_name }}</h3>
            <p class="nw-hint">{{ profile.user_email }}</p>
          </div>
          <span
            class="nw-badge"
            :class="profile.user_status_code === 'active' ? 'nw-badge--success' : 'nw-badge--danger'"
          >
            {{ profile.user_status_code === 'active' ? '正常' : '已禁用' }}
          </span>
        </div>

        <dl v-if="profile" class="profile__meta">
          <div class="profile__meta-row">
            <dt>角色</dt>
            <dd>
              <span v-if="profile.role_codes?.length" class="profile__roles">
                <span v-for="code in profile.role_codes" :key="code" class="nw-badge nw-badge--neutral">
                  {{ code }}
                </span>
              </span>
              <span v-else class="nw-hint">—</span>
            </dd>
          </div>
          <div class="profile__meta-row">
            <dt>注册时间</dt>
            <dd>{{ formatTime(profile.created_at) }}</dd>
          </div>
          <div class="profile__meta-row">
            <dt>最近更新</dt>
            <dd>{{ formatTime(profile.updated_at) }}</dd>
          </div>
          <div class="profile__meta-row">
            <dt>用户标识</dt>
            <dd><code class="profile__ulid">{{ profile.user_ulid }}</code></dd>
          </div>
        </dl>
      </ElCard>

      <ElCard shadow="never" class="profile__card">
        <template #header>
          <span class="profile__title">编辑资料</span>
        </template>
        <ElForm ref="profileFormRef" :model="profileForm" :rules="profileRules" label-position="top">
          <ElFormItem label="登录名">
            <ElInput :model-value="profile?.user_name ?? ''" disabled />
          </ElFormItem>
          <ElFormItem label="邮箱" prop="user_email">
            <ElInput v-model="profileForm.user_email" maxlength="128" placeholder="name@example.com" />
          </ElFormItem>
          <ElFormItem label="头像地址">
            <ElInput v-model="profileForm.avatar_url" maxlength="256" placeholder="头像图片地址" />
          </ElFormItem>
          <ElFormItem label="备注">
            <ElInput v-model="profileForm.remark_text" type="textarea" :rows="2" maxlength="256" />
          </ElFormItem>
        </ElForm>
        <div class="profile__actions">
          <ElButton type="primary" :loading="savingProfile" @click="saveProfile()">保存资料</ElButton>
        </div>
      </ElCard>

      <ElCard shadow="never" class="profile__card">
        <template #header>
          <span class="profile__title">修改密码</span>
        </template>
        <ElAlert
          type="warning"
          :closable="false"
          show-icon
          title="修改后需使用新密码重新登录其他设备"
        />
        <ElForm
          ref="passwordFormRef"
          :model="passwordForm"
          :rules="passwordRules"
          label-position="top"
          class="profile__password-form"
        >
          <ElFormItem label="当前密码" prop="old_password_text">
            <ElInput
              v-model="passwordForm.old_password_text"
              type="password"
              show-password
              maxlength="64"
            />
          </ElFormItem>
          <ElFormItem label="新密码" prop="new_password_text">
            <ElInput
              v-model="passwordForm.new_password_text"
              type="password"
              show-password
              maxlength="64"
              placeholder="至少 6 位"
            />
          </ElFormItem>
          <ElFormItem label="确认新密码" prop="confirm_password_text">
            <ElInput
              v-model="passwordForm.confirm_password_text"
              type="password"
              show-password
              maxlength="64"
            />
          </ElFormItem>
        </ElForm>
        <div class="profile__actions">
          <ElButton type="primary" :loading="savingPassword" @click="savePassword()">
            修改密码
          </ElButton>
        </div>
      </ElCard>
    </div>
  </div>
</template>

<style lang="less" scoped>
  .profile {
    &__grid {
      display: grid;
      gap: 14px;
      align-items: start;
      grid-template-columns: repeat(auto-fit, minmax(340px, 1fr));
    }

    &__card {
      border-radius: var(--nw-radius-lg);
    }

    &__title {
      font-size: 14px;
      font-weight: 650;
    }

    &__account {
      display: flex;
      gap: 12px;
      align-items: center;
      padding-bottom: 14px;
      border-bottom: 1px solid var(--nw-border-soft);
    }

    &__avatar {
      display: grid;
      width: 52px;
      height: 52px;
      overflow: hidden;
      font-size: 20px;
      font-weight: 700;
      color: var(--el-color-primary);
      background: var(--nw-primary-soft);
      border-radius: 50%;
      place-items: center;

      img {
        width: 100%;
        height: 100%;
        object-fit: cover;
      }
    }

    &__account-text {
      flex: 1;
      min-width: 0;
    }

    &__name {
      font-size: 16px;
      font-weight: 650;
      color: var(--nw-ink);
    }

    &__meta {
      display: flex;
      flex-direction: column;
      gap: 10px;
      margin-top: 14px;
    }

    &__meta-row {
      display: flex;
      gap: 12px;
      align-items: baseline;
      justify-content: space-between;
      font-size: 12.5px;

      dt {
        color: var(--nw-text-secondary);
      }

      dd {
        color: var(--nw-ink);
        text-align: right;
        word-break: break-all;
      }
    }

    &__roles {
      display: inline-flex;
      flex-wrap: wrap;
      gap: 4px;
      justify-content: flex-end;
    }

    &__ulid {
      font-family: var(--el-font-family-mono, monospace);
      font-size: 11px;
      color: var(--nw-text-muted);
    }

    &__password-form {
      margin-top: 12px;
    }

    &__actions {
      display: flex;
      justify-content: flex-end;
      margin-top: 4px;
    }
  }
</style>

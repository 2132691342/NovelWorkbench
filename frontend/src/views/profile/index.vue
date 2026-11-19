<!--
  个人中心：资料卡 + 基本资料表单 + 密码修改 + 账号信息。
  仅本人可访问；保存成功后同步刷新顶栏用户信息。
-->
<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { changePassword, fetchProfileDetail, updateProfile, type NWProfileVo } from '@/api/profile'
import { useAuthStore } from '@/stores/auth'
import NWButton from '@/components/NWButton.vue'
import NWCard from '@/components/NWCard.vue'
import NWLoading from '@/components/NWLoading.vue'

const { t } = useI18n()
const authStore = useAuthStore()

const profileLoading = ref(false)
const profileSaving = ref(false)
const passwordSaving = ref(false)
const activeTab = ref('profile')
const profileVo = ref<NWProfileVo | null>(null)
const profileFormRef = ref<FormInstance>()
const passwordFormRef = ref<FormInstance>()

/** 基本资料表单 */
const profileForm = reactive({
  user_email: '',
  avatar_url: '',
  remark_text: '',
})

/** 密码修改表单 */
const passwordForm = reactive({
  old_password_text: '',
  new_password_text: '',
  confirm_password_text: '',
})

const profileRules = computed<FormRules>(() => ({
  user_email: [
    { required: true, message: t('profile.ruleEmailRequired'), trigger: 'blur' },
    { type: 'email', message: t('profile.ruleEmailInvalid'), trigger: 'blur' },
  ],
}))

const passwordRules = computed<FormRules>(() => ({
  old_password_text: [
    { required: true, message: t('profile.ruleOldPasswordRequired'), trigger: 'blur' },
  ],
  new_password_text: [
    { required: true, message: t('profile.ruleNewPasswordRequired'), trigger: 'blur' },
    { min: 6, max: 64, message: t('profile.rulePasswordLength'), trigger: 'blur' },
  ],
  confirm_password_text: [
    { required: true, message: t('profile.ruleConfirmRequired'), trigger: 'blur' },
    {
      validator: (_rule, value: string, callback) => {
        if (value !== passwordForm.new_password_text) {
          callback(new Error(t('profile.ruleConfirmMismatch')))
          return
        }
        callback()
      },
      trigger: 'blur',
    },
  ],
}))

const avatarText = computed(() => (profileVo.value?.user_name ?? '').slice(0, 1).toUpperCase() || '?')
const roleText = computed(() => (profileVo.value?.role_codes ?? []).join(' / ') || '-')

function formatTime(timeMs: number): string {
  return new Date(timeMs).toLocaleString()
}

/** 加载个人资料并回填表单 */
async function loadProfile(): Promise<void> {
  profileLoading.value = true
  try {
    const data = await fetchProfileDetail()
    profileVo.value = data
    profileForm.user_email = data.user_email
    profileForm.avatar_url = data.avatar_url
    profileForm.remark_text = data.remark_text
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    profileLoading.value = false
  }
}

/** 保存基本资料 */
async function handleProfileSave(): Promise<void> {
  if (!profileFormRef.value) {
    return
  }
  try {
    await profileFormRef.value.validate()
  } catch {
    return
  }
  profileSaving.value = true
  try {
    const data = await updateProfile({
      user_email: profileForm.user_email.trim(),
      avatar_url: profileForm.avatar_url.trim(),
      remark_text: profileForm.remark_text.trim(),
    })
    profileVo.value = data
    await authStore.fetchProfile()
    ElMessage.success(t('profile.saveSuccess'))
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    profileSaving.value = false
  }
}

/** 修改密码：成功后清空表单 */
async function handlePasswordSave(): Promise<void> {
  if (!passwordFormRef.value) {
    return
  }
  try {
    await passwordFormRef.value.validate()
  } catch {
    return
  }
  passwordSaving.value = true
  try {
    await changePassword({
      old_password_text: passwordForm.old_password_text,
      new_password_text: passwordForm.new_password_text,
    })
    passwordForm.old_password_text = ''
    passwordForm.new_password_text = ''
    passwordForm.confirm_password_text = ''
    passwordFormRef.value.clearValidate()
    ElMessage.success(t('profile.passwordSuccess'))
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    passwordSaving.value = false
  }
}

onMounted(() => {
  void loadProfile()
})
</script>

<template>
  <div class="nw-page profile-page">
    <NWLoading v-if="profileLoading" />
    <template v-else-if="profileVo">
      <section class="profile-page__hero">
        <div class="profile-page__identity">
          <el-avatar :size="72" :src="profileVo.avatar_url" class="profile-page__avatar">
            {{ avatarText }}
          </el-avatar>
          <div class="profile-page__meta">
            <h2 class="profile-page__name">{{ profileVo.user_name }}</h2>
            <p class="profile-page__email">{{ profileVo.user_email }}</p>
            <div class="profile-page__tags">
              <el-tag size="small" effect="light">{{ roleText }}</el-tag>
              <el-tag v-if="profileVo.user_status_code === 'active'" size="small" type="success" effect="light">
                {{ t('profile.statusActive') }}
              </el-tag>
              <el-tag v-else size="small" type="danger" effect="light">
                {{ t('profile.statusDisabled') }}
              </el-tag>
            </div>
          </div>
        </div>
        <div class="profile-page__hero-side">
          <span class="profile-page__hero-label">{{ t('profile.joinedAt') }}</span>
          <strong>{{ formatTime(profileVo.created_at_ms) }}</strong>
        </div>
      </section>

      <el-tabs v-model="activeTab" class="profile-page__tabs">
        <el-tab-pane :label="t('profile.tabProfile')" name="profile">
          <NWCard :hoverable="false">
            <el-form
              ref="profileFormRef"
              :model="profileForm"
              :rules="profileRules"
              label-position="top"
              class="profile-page__form"
            >
              <el-form-item :label="t('profile.userEmail')" prop="user_email">
                <el-input v-model="profileForm.user_email" maxlength="128" clearable />
              </el-form-item>
              <el-form-item :label="t('profile.avatarUrl')" prop="avatar_url">
                <el-input
                  v-model="profileForm.avatar_url"
                  maxlength="256"
                  clearable
                  :placeholder="t('profile.avatarUrlPlaceholder')"
                />
              </el-form-item>
              <el-form-item :label="t('profile.remark')" prop="remark_text">
                <el-input
                  v-model="profileForm.remark_text"
                  type="textarea"
                  :rows="3"
                  maxlength="256"
                  show-word-limit
                  :placeholder="t('profile.remarkPlaceholder')"
                />
              </el-form-item>
              <div class="profile-page__actions">
                <NWButton type="primary" :loading="profileSaving" @click="handleProfileSave">
                  {{ t('common.save') }}
                </NWButton>
                <NWButton @click="loadProfile">{{ t('profile.reload') }}</NWButton>
              </div>
            </el-form>
          </NWCard>
        </el-tab-pane>

        <el-tab-pane :label="t('profile.tabSecurity')" name="security">
          <NWCard :hoverable="false">
            <el-alert
              :title="t('profile.passwordHint')"
              type="info"
              :closable="false"
              show-icon
              class="profile-page__alert"
            />
            <el-form
              ref="passwordFormRef"
              :model="passwordForm"
              :rules="passwordRules"
              label-position="top"
              class="profile-page__form"
            >
              <el-form-item :label="t('profile.oldPassword')" prop="old_password_text">
                <el-input
                  v-model="passwordForm.old_password_text"
                  type="password"
                  show-password
                  autocomplete="current-password"
                />
              </el-form-item>
              <el-form-item :label="t('profile.newPassword')" prop="new_password_text">
                <el-input
                  v-model="passwordForm.new_password_text"
                  type="password"
                  show-password
                  autocomplete="new-password"
                />
              </el-form-item>
              <el-form-item :label="t('profile.confirmPassword')" prop="confirm_password_text">
                <el-input
                  v-model="passwordForm.confirm_password_text"
                  type="password"
                  show-password
                  autocomplete="new-password"
                />
              </el-form-item>
              <div class="profile-page__actions">
                <NWButton type="primary" :loading="passwordSaving" @click="handlePasswordSave">
                  {{ t('profile.changePassword') }}
                </NWButton>
              </div>
            </el-form>
          </NWCard>
        </el-tab-pane>

        <el-tab-pane :label="t('profile.tabAccount')" name="account">
          <NWCard :hoverable="false">
            <el-descriptions :column="1" border>
              <el-descriptions-item :label="t('profile.userUlid')">
                {{ profileVo.user_ulid }}
              </el-descriptions-item>
              <el-descriptions-item :label="t('profile.userName')">
                {{ profileVo.user_name }}
              </el-descriptions-item>
              <el-descriptions-item :label="t('profile.roles')">
                {{ roleText }}
              </el-descriptions-item>
              <el-descriptions-item :label="t('profile.status')">
                {{ profileVo.user_status_code === 'active' ? t('profile.statusActive') : t('profile.statusDisabled') }}
              </el-descriptions-item>
              <el-descriptions-item :label="t('profile.createdAt')">
                {{ formatTime(profileVo.created_at_ms) }}
              </el-descriptions-item>
              <el-descriptions-item :label="t('profile.updatedAt')">
                {{ formatTime(profileVo.updated_at_ms) }}
              </el-descriptions-item>
            </el-descriptions>
          </NWCard>
        </el-tab-pane>
      </el-tabs>
    </template>
  </div>
</template>

<style scoped>
.profile-page {
  max-width: 920px;
  margin: 0 auto;
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.profile-page__hero {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  padding: 24px;
  border-radius: var(--radius-lg);
  background: linear-gradient(135deg, var(--saas_dashboard-color-5) 0%, var(--saas_dashboard-color-1) 100%);
  color: var(--text-inverse);
  box-shadow: var(--shadow-card);
}

.profile-page__identity {
  display: flex;
  align-items: center;
  gap: 18px;
}

.profile-page__avatar {
  background-color: rgba(255, 255, 255, 0.18);
  color: var(--text-inverse);
  font-size: 26px;
  font-weight: 700;
}

.profile-page__name {
  margin: 0 0 4px;
  font-size: 22px;
  font-weight: 700;
  letter-spacing: -0.01em;
}

.profile-page__email {
  margin: 0 0 10px;
  font-size: 13px;
  opacity: 0.8;
}

.profile-page__tags {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.profile-page__hero-side {
  display: flex;
  flex-direction: column;
  gap: 4px;
  text-align: right;
  font-size: 13px;
}

.profile-page__hero-label {
  opacity: 0.7;
  font-size: 12px;
}

.profile-page__tabs {
  padding: 6px 18px 18px;
  border: 1px solid var(--border-light);
  border-radius: var(--radius-lg);
  background-color: var(--bg-card);
  box-shadow: var(--shadow-card);
}

.profile-page__form {
  max-width: 520px;
}

.profile-page__actions {
  display: flex;
  gap: 10px;
  margin-top: 8px;
}

.profile-page__alert {
  margin-bottom: 16px;
}

@media (max-width: 720px) {
  .profile-page__hero {
    flex-direction: column;
    align-items: flex-start;
  }

  .profile-page__hero-side {
    text-align: left;
  }
}
</style>

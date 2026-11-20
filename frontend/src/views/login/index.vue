<!--
  登录页：仅登录（系统不开放注册，账号由管理员在「用户管理」中分配）。
  左侧品牌叙事 + 右侧登录表单，含字段校验、提交加载态与登录失败提示。
-->
<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useMenuStore } from '@/stores/menu'
import NWButton from '@/components/NWButton.vue'

const router = useRouter()
const authStore = useAuthStore()
const menuStore = useMenuStore()
const { t } = useI18n()

const submitting = ref(false)
const formRef = ref<FormInstance>()
const formState = reactive({
  account_text: '',
  password_text: '',
})

const formRules: FormRules = {
  account_text: [{ required: true, message: t('login.ruleAccountRequired'), trigger: 'blur' }],
  password_text: [
    { required: true, message: t('login.rulePasswordRequired'), trigger: 'blur' },
    { min: 6, message: t('login.rulePasswordMin'), trigger: 'blur' },
  ],
}

/** 登录并预加载导航（进入业务页前菜单与权限就绪） */
async function handleSubmit(): Promise<void> {
  if (!formRef.value) {
    return
  }
  try {
    await formRef.value.validate()
  } catch {
    return
  }
  submitting.value = true
  try {
    await authStore.login({
      account_text: formState.account_text.trim(),
      password_text: formState.password_text,
    })
    await menuStore.loadNav()
    ElMessage.success(t('login.loginSuccess'))
    void router.push('/')
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <section class="login-page__brand">
      <div class="login-page__brand-inner">
        <span class="login-page__brand-mark">N</span>
        <h1 class="login-page__brand-title">{{ t('login.brandTitle') }}</h1>
        <p class="login-page__brand-desc">{{ t('login.brandDesc') }}</p>
        <ul class="login-page__brand-list">
          <li>{{ t('login.brandPointProject') }}</li>
          <li>{{ t('login.brandPointAi') }}</li>
          <li>{{ t('login.brandPointKnowledge') }}</li>
        </ul>
      </div>
    </section>

    <section class="login-page__panel">
      <div class="login-page__card">
        <header class="login-page__header">
          <h2 class="login-page__title">{{ t('login.title') }}</h2>
          <p class="login-page__subtitle">{{ t('login.subtitle') }}</p>
        </header>

        <el-form
          ref="formRef"
          class="login-page__form"
          :model="formState"
          :rules="formRules"
          label-position="top"
          @submit.prevent="handleSubmit"
        >
          <el-form-item prop="account_text" :label="t('login.account')">
            <el-input
              v-model="formState.account_text"
              :placeholder="t('login.accountPlaceholder')"
              size="large"
              autocomplete="username"
            />
          </el-form-item>

          <el-form-item prop="password_text" :label="t('login.password')">
            <el-input
              v-model="formState.password_text"
              type="password"
              show-password
              :placeholder="t('login.passwordPlaceholder')"
              size="large"
              autocomplete="current-password"
              @keyup.enter="handleSubmit"
            />
          </el-form-item>

          <NWButton
            type="primary"
            size="large"
            class="login-page__submit"
            :loading="submitting"
            @click="handleSubmit"
          >
            {{ t('login.submitLogin') }}
          </NWButton>
        </el-form>

        <p class="login-page__hint">{{ t('login.noRegisterHint') }}</p>
      </div>
    </section>
  </div>
</template>

<style scoped>
.login-page {
  display: flex;
  min-height: 100vh;
  background-color: var(--bg-page);
}

.login-page__brand {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: 1.1;
  padding: 48px;
  background: linear-gradient(150deg, var(--saas_dashboard-color-5) 0%, var(--saas_dashboard-color-1) 100%);
  color: var(--text-inverse);
}

.login-page__brand-inner {
  max-width: 420px;
}

.login-page__brand-mark {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  margin-bottom: 24px;
  border-radius: var(--radius-xl);
  background-color: rgba(255, 255, 255, 0.16);
  font-size: 24px;
  font-weight: 700;
}

.login-page__brand-title {
  margin: 0 0 12px;
  font-size: 30px;
  font-weight: 700;
  line-height: 1.3;
  letter-spacing: -0.02em;
}

.login-page__brand-desc {
  margin: 0 0 24px;
  font-size: 14px;
  line-height: 1.8;
  opacity: 0.82;
}

.login-page__brand-list {
  margin: 0;
  padding-left: 18px;
  font-size: 13.5px;
  line-height: 2;
  opacity: 0.86;
}

.login-page__panel {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: 1;
  padding: 48px 32px;
}

.login-page__card {
  width: 100%;
  max-width: 380px;
}

.login-page__header {
  margin-bottom: 28px;
}

.login-page__title {
  margin: 0 0 6px;
  font-size: 24px;
  font-weight: 650;
  letter-spacing: -0.01em;
  color: var(--text-main);
}

.login-page__subtitle {
  margin: 0;
  font-size: 13.5px;
  color: var(--text-secondary);
}

.login-page__submit {
  width: 100%;
  margin-top: 10px;
}

.login-page__hint {
  margin: 20px 0 0;
  padding: 10px 12px;
  border-radius: var(--radius-base);
  background-color: var(--primary-light);
  color: var(--primary-active);
  font-size: 12.5px;
  line-height: 1.7;
}

@media (max-width: 900px) {
  .login-page__brand {
    display: none;
  }
}
</style>

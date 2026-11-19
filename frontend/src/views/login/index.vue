<script setup lang="ts">

// 登录注册页：登录与注册双表单，含字段校验与提交加载态。
import { computed, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import NWButton from '@/components/NWButton.vue'

const router = useRouter()
const authStore = useAuthStore()
const { t } = useI18n()

interface NWLoginFormState {
  account_text: string
  user_name: string
  user_email: string
  password_text: string
}

const activeTab = ref<string>('login')
const submitting = ref(false)
const formRef = ref<FormInstance>()
const formState = reactive<NWLoginFormState>({
  account_text: '',
  user_name: '',
  user_email: '',
  password_text: '',
})

// 登录/注册共用一个表单，校验规则按当前 Tab 组装
const formRules = computed<FormRules>(() => {
  const rules: FormRules = {
    password_text: [
      { required: true, message: t('login.rulePasswordRequired'), trigger: 'blur' },
      { min: 6, message: t('login.rulePasswordMin'), trigger: 'blur' },
    ],
  }
  if (activeTab.value === 'login') {
    rules.account_text = [{ required: true, message: t('login.ruleAccountRequired'), trigger: 'blur' }]
  } else {
    rules.user_name = [{ required: true, message: t('login.ruleUserNameRequired'), trigger: 'blur' }]
    rules.user_email = [
      { required: true, message: t('login.ruleEmailRequired'), trigger: 'blur' },
      { type: 'email', message: t('login.ruleEmailInvalid'), trigger: 'blur' },
    ]
  }
  return rules
})

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
    if (activeTab.value === 'login') {
      await authStore.login({
        account_text: formState.account_text,
        password_text: formState.password_text,
      })
      ElMessage.success(t('login.loginSuccess'))
      void router.push('/')
    } else {
      await authStore.register({
        user_name: formState.user_name,
        user_email: formState.user_email,
        password_text: formState.password_text,
      })
      ElMessage.success(t('login.registerSuccess'))
      activeTab.value = 'login'
    }
  } catch {
    // 失败提示由 http 层统一弹出
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <section class="login-page__card">
      <header class="login-page__header">
        <svg class="login-page__logo" viewBox="0 0 24 24" fill="none" aria-hidden="true">
          <rect x="3" y="3" width="18" height="18" rx="5" fill="var(--primary)" />
          <path
            d="M8 8v8M8 8h2.6a2.4 2.4 0 0 1 2.4 2.4V16M16 8v8"
            stroke="#ffffff"
            stroke-width="1.8"
            stroke-linecap="round"
          />
        </svg>
        <h1 class="login-page__title">{{ t('login.title') }}</h1>
        <p class="login-page__subtitle">{{ t('login.subtitle') }}</p>
      </header>

      <div class="login-page__tabs">
        <NWButton
          class="login-page__tab"
          :class="{ 'is-active': activeTab === 'login' }"
          @click="activeTab = 'login'"
        >
          {{ t('login.tabLogin') }}
        </NWButton>
        <NWButton
          class="login-page__tab"
          :class="{ 'is-active': activeTab === 'register' }"
          @click="activeTab = 'register'"
        >
          {{ t('login.tabRegister') }}
        </NWButton>
      </div>

      <el-form
        ref="formRef"
        class="login-page__form"
        :model="formState"
        :rules="formRules"
        label-position="top"
        @submit.prevent
      >
        <el-form-item v-if="activeTab === 'login'" prop="account_text" :label="t('login.account')">
          <el-input
            v-model="formState.account_text"
            :placeholder="t('login.accountPlaceholder')"
            size="large"
          />
        </el-form-item>

        <el-form-item v-if="activeTab === 'register'" prop="user_name" :label="t('login.userName')">
          <el-input v-model="formState.user_name" size="large" />
        </el-form-item>

        <el-form-item v-if="activeTab === 'register'" prop="user_email" :label="t('login.userEmail')">
          <el-input v-model="formState.user_email" size="large" />
        </el-form-item>

        <el-form-item prop="password_text" :label="t('login.password')">
          <el-input
            v-model="formState.password_text"
            type="password"
            show-password
            :placeholder="t('login.passwordPlaceholder')"
            size="large"
          />
        </el-form-item>

        <NWButton
          type="primary"
          size="large"
          class="login-page__submit"
          :loading="submitting"
          @click="handleSubmit"
        >
          {{ activeTab === 'login' ? t('login.submitLogin') : t('login.submitRegister') }}
        </NWButton>
      </el-form>
    </section>
  </div>
</template>

<style scoped>
.login-page {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  background-color: var(--bg-page);
  background-image: radial-gradient(circle at 20% 20%, var(--primary-light) 0, transparent 45%),
    radial-gradient(circle at 80% 80%, var(--primary-light) 0, transparent 45%);
}

.login-page__card {
  width: 420px;
  padding: 36px 40px;
  background-color: var(--bg-card);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-card-hover);
}

.login-page__header {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  margin-bottom: 24px;
}

.login-page__logo {
  width: 48px;
  height: 48px;
}

.login-page__title {
  margin: 8px 0 0;
  font-size: 22px;
  font-weight: 600;
  color: var(--text-main);
}

.login-page__subtitle {
  margin: 0;
  font-size: 13px;
  color: var(--text-secondary);
}

.login-page__tabs {
  display: flex;
  gap: 8px;
  margin-bottom: 20px;
  padding: 4px;
  background-color: var(--bg-page);
  border-radius: var(--radius-base);
}

.login-page__tab.nw-button {
  flex: 1;
  height: 34px;
  border: none;
  background-color: transparent;
  color: var(--text-secondary);
}

.login-page__tab.nw-button:hover {
  color: var(--primary);
}

.login-page__tab.nw-button.is-active {
  background-color: var(--primary);
  color: var(--text-inverse);
  box-shadow: var(--shadow-button-hover);
}


.login-page__submit {
  width: 100%;
  margin-top: 8px;
}
</style>

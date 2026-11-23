<!-- 登录页：账号密码登录并跳转回原目标。 -->
<script setup lang="ts">
  import { computed, ref } from 'vue'
  import { ElButton, ElCheckbox, ElForm, ElFormItem, ElInput } from 'element-plus'
  import { useRouter } from 'vue-router'
  import type { LoginParams } from '@/api/login'
  import { useI18n } from 'vue-i18n'
  import { required, useForm } from '@novelworkbench/hooks'
  import { ThemeSwitch } from '@novelworkbench/components'
  import { useUserStore } from '@/store/modules/user'
  import { useAppStore } from '@/store/modules/app'
  import { appConfig } from '@/config/app'

  const emit = defineEmits<{ success: [] }>()

  const { t } = useI18n()
  const router = useRouter()
  const userStore = useUserStore()
  const appStore = useAppStore()

  const { state, actions } = useForm<LoginParams>({
    initialValues: { account_text: '', password_text: '' },
    rules: {
      account_text: required(() => t('common.required')),
      password_text: required(() => t('common.required'))
    }
  })

  const values = computed(() => state.values.value)
  const errors = computed(() => state.errors.value)
  const submitting = computed(() => state.submitting.value)
  const title = appConfig.title

  /** 记住我：勾选把令牌写入 localStorage，否则只保留到关闭标签页 */
  const rememberFlag = ref(true)

  const signIn = () =>
    actions.submit(async (formData) => {
      await userStore.login(formData, rememberFlag.value)
      await userStore.loadNav()
      emit('success')
      const redirectPath = String(router.currentRoute.value.query.redirect ?? '')
      await router.replace(redirectPath.startsWith('/') ? redirectPath : '/')
    })
</script>

<template>
  <main class="login-page">
    <header class="login-page__brand">
      <span class="login-page__mark">N</span>
      <span class="login-page__name">{{ title }}</span>
      <ThemeSwitch v-model="appStore.isDark" class="login-page__theme" />
    </header>

    <section class="login-page__visual">
      <p class="login-page__kicker">AI NOVEL WORKBENCH</p>
      <h1>{{ t('login.welcome') }}</h1>
      <p class="login-page__desc">{{ t('login.message') }}</p>
    </section>

    <section class="login-page__stage">
      <ElForm
        :model="values"
        label-position="top"
        hide-required-asterisk
        size="large"
        class="login-form"
        @submit.prevent="signIn()"
      >
        <h2 class="login-form__title">{{ t('login.login') }}</h2>

        <ElFormItem :label="t('login.account')" :error="errors.account_text?.[0]">
          <ElInput
            v-model="values.account_text"
            :placeholder="t('login.accountPlaceholder')"
            autocomplete="username"
            :disabled="submitting"
            @input="actions.clearErrors('account_text')"
          />
        </ElFormItem>

        <ElFormItem :label="t('login.password')" :error="errors.password_text?.[0]">
          <ElInput
            v-model="values.password_text"
            type="password"
            show-password
            :placeholder="t('login.passwordPlaceholder')"
            autocomplete="current-password"
            :disabled="submitting"
            @input="actions.clearErrors('password_text')"
            @keyup.enter="signIn()"
          />
        </ElFormItem>

        <ElCheckbox v-model="rememberFlag" class="login-form__remember" :disabled="submitting">
          {{ t('login.remember') }}
        </ElCheckbox>

        <ElButton
          native-type="submit"
          type="primary"
          class="login-form__submit"
          :loading="submitting"
        >
          {{ t('login.login') }}
        </ElButton>

        <p class="login-form__hint">{{ t('login.noRegisterHint') }}</p>
      </ElForm>
    </section>
  </main>
</template>

<style lang="less" scoped>
  .login-page {
    display: grid;
    height: 100vh;
    height: 100dvh;
    color: var(--nw-ink);
    background: var(--app-content-bg-color);
    grid-template-columns: minmax(0, 1.05fr) minmax(420px, 0.95fr);

    &__brand {
      position: absolute;
      top: 26px;
      left: 34px;
      z-index: 3;
      display: flex;
      gap: 10px;
      align-items: center;
    }

    &__mark {
      display: grid;
      width: 34px;
      height: 34px;
      font-size: 17px;
      font-weight: 700;
      color: #fff;
      background: linear-gradient(135deg, var(--saas_dashboard-color-1), var(--saas_dashboard-color-2));
      border-radius: 9px;
      place-items: center;
    }

    &__name {
      font-size: 15px;
      font-weight: 650;
    }

    &__theme {
      margin-left: 6px;
    }

    &__visual {
      display: flex;
      flex-direction: column;
      justify-content: center;
      padding: 90px clamp(38px, 6vw, 96px);
      color: #fff;
      background:
        radial-gradient(circle at 18% 22%, rgb(129 140 248 / 32%), transparent 42%),
        radial-gradient(circle at 78% 78%, rgb(99 102 241 / 28%), transparent 46%),
        linear-gradient(150deg, #1e293b, #111a2e 62%, #0f172a);

      h1 {
        max-width: 520px;
        margin: 18px 0 16px;
        font-size: clamp(32px, 3.4vw, 50px);
        font-weight: 700;
        line-height: 1.14;
        letter-spacing: -0.03em;
      }

      .login-page__brand & {
        color: #fff;
      }
    }

    &__kicker {
      font-family: SFMono-Regular, Consolas, monospace;
      font-size: 11px;
      font-weight: 600;
      letter-spacing: 0.16em;
      color: rgb(199 210 254 / 76%);
    }

    &__desc {
      max-width: 480px;
      margin: 0;
      font-size: 15px;
      line-height: 1.75;
      color: rgb(226 232 240 / 74%);
    }

    &__stage {
      display: grid;
      padding: 80px clamp(24px, 5vw, 64px);
      background: var(--nw-surface);
      place-items: center;
    }
  }

  .login-form {
    width: min(100%, 380px);

    &__title {
      margin: 0 0 26px;
      font-size: 26px;
      font-weight: 700;
      letter-spacing: -0.02em;
    }

    &__remember {
      margin: -6px 0 18px;
    }

    &__submit {
      width: 100%;
      min-height: 46px;
      font-size: 14px;
      font-weight: 600;
      letter-spacing: 0.03em;
    }

    &__hint {
      margin: 18px 0 0;
      font-size: 12px;
      line-height: 1.7;
      color: var(--el-text-color-secondary);
    }

    :deep(.el-form-item__label) {
      font-size: 12.5px;
      font-weight: 600;
      color: var(--nw-ink);
    }

    :deep(.el-input__wrapper) {
      min-height: 44px;
      border-radius: var(--nw-radius-md);
    }
  }

  @media (width <= 900px) {
    .login-page {
      display: flex;
      flex-direction: column;

      &__visual {
        flex: none;
        padding: 96px 24px 30px;

        h1 {
          font-size: 28px;
        }
      }

      &__desc {
        font-size: 13px;
      }

      &__stage {
        flex: 1;
        align-items: start;
        padding: 26px 22px 40px;
      }
    }
  }
</style>

<script setup lang="ts">
import { reactive } from 'vue'

import type { LoginCredentials } from '@/types'

defineProps<{
  busy: boolean
  error: string
}>()

const emit = defineEmits<{
  submit: [credentials: LoginCredentials]
}>()

const form = reactive<LoginCredentials>({
  email: '',
  password: '',
})

function submit(): void {
  emit('submit', {
    email: form.email.trim(),
    password: form.password,
  })
}
</script>

<template>
  <section class="login-shell">
    <div class="login-glow" aria-hidden="true"></div>
    <div class="login-card">
      <div class="brand-mark" aria-hidden="true">HS</div>
      <p class="eyebrow">OPERATOR CONSOLE</p>
      <h1 class="login-title">让不确定内容<br />停在发布之前</h1>
      <p class="login-copy">
        登录 HateSentry 人工复核控制台，处理策略标记为 review 的内容。
      </p>

      <form class="login-form" @submit.prevent="submit">
        <div class="field-group">
          <label class="field-label" for="email">管理员邮箱</label>
          <input
            id="email"
            v-model="form.email"
            class="field-input"
            name="email"
            type="email"
            autocomplete="username"
            required
            :disabled="busy"
          />
        </div>

        <div class="field-group">
          <label class="field-label" for="password">密码</label>
          <input
            id="password"
            v-model="form.password"
            class="field-input"
            name="password"
            type="password"
            autocomplete="current-password"
            required
            :disabled="busy"
          />
        </div>

        <p v-if="error" class="form-error" role="alert">{{ error }}</p>

        <button class="primary-button" type="submit" :disabled="busy">
          {{ busy ? '正在验证…' : '进入复核队列' }}
        </button>
      </form>

      <p class="security-note">会话仅保存在当前浏览器标签页中。</p>
    </div>
  </section>
</template>

<style scoped>
@reference "../../styles.css";

.login-shell {
  @apply relative flex min-h-screen items-center justify-center overflow-hidden px-5 py-10;
  background:
    radial-gradient(circle at 15% 18%, rgba(255, 190, 92, 0.18), transparent 28rem),
    linear-gradient(145deg, #111712 0%, #18221a 45%, #0d120f 100%);
}

.login-glow {
  @apply absolute h-[34rem] w-[34rem] rounded-full blur-3xl;
  right: -15rem;
  bottom: -18rem;
  background: rgba(79, 129, 87, 0.22);
}

.login-card {
  @apply relative z-10 w-full max-w-lg rounded-[2rem] border border-white/10 bg-[#f7f2e8] p-7 shadow-2xl md:p-11;
}

.brand-mark {
  @apply mb-8 grid h-12 w-12 place-items-center rounded-2xl bg-[#1d2b20] text-sm font-black tracking-[0.16em] text-[#ffd27a];
}

.eyebrow {
  @apply text-xs font-bold tracking-[0.24em] text-[#637164];
}

.login-title {
  @apply mt-4 text-4xl font-semibold leading-[1.05] tracking-[-0.04em] text-[#172119] md:text-5xl;
}

.login-copy {
  @apply mt-5 max-w-md text-sm leading-6 text-[#667168] md:text-base;
}

.login-form {
  @apply mt-9 space-y-5;
}

.field-group {
  @apply space-y-2;
}

.field-label {
  @apply block text-sm font-semibold text-[#27352a];
}

.field-input {
  @apply h-12 w-full rounded-xl border border-[#c9c7ba] bg-white/70 px-4 text-base text-[#172119] outline-none transition focus:border-[#456b4d] focus:ring-4 focus:ring-[#456b4d]/15 disabled:cursor-not-allowed disabled:opacity-60;
}

.form-error {
  @apply rounded-xl border border-[#bf4d3e]/20 bg-[#bf4d3e]/10 px-4 py-3 text-sm text-[#8c2f25];
}

.primary-button {
  @apply h-12 w-full rounded-xl bg-[#1d2b20] px-5 text-sm font-bold text-[#fff4d8] transition hover:bg-[#2b422f] focus:outline-none focus:ring-4 focus:ring-[#456b4d]/25 disabled:cursor-not-allowed disabled:opacity-60;
}

.security-note {
  @apply mt-5 text-center text-xs text-[#7c827b];
}
</style>

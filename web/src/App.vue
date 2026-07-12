<script setup lang="ts">
import { shallowRef } from 'vue'

import LoginForm from '@/components/auth/LoginForm.vue'
import ReviewWorkspace from '@/components/reviews/ReviewWorkspace.vue'
import { useSession } from '@/composables/useSession'
import type { LoginCredentials } from '@/types'

const { session, isLoggingIn, login, logout } = useSession()
const loginError = shallowRef('')

async function handleLogin(credentials: LoginCredentials): Promise<void> {
  loginError.value = ''
  try {
    await login(credentials)
  } catch (error) {
    loginError.value = error instanceof Error ? error.message : '登录失败，请稍后重试'
  }
}
</script>

<template>
  <main class="min-h-screen">
    <LoginForm
      v-if="!session"
      :busy="isLoggingIn"
      :error="loginError"
      @submit="handleLogin"
    />
    <ReviewWorkspace
      v-else
      :session="session"
      @logout="logout"
    />
  </main>
</template>

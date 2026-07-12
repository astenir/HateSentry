import { readonly, shallowRef } from 'vue'

import { login as loginRequest } from '@/api'
import type { LoginCredentials, Session } from '@/types'

const storageKey = 'hatesentry-operator-session'

function readStoredSession(): Session | null {
  const stored = sessionStorage.getItem(storageKey)
  if (!stored) return null

  try {
    const session = JSON.parse(stored) as Session
    if (!session.token || !session.user || session.user.role !== 'admin') return null
    return session
  } catch {
    return null
  }
}

export function useSession() {
  const session = shallowRef<Session | null>(readStoredSession())
  const isLoggingIn = shallowRef(false)

  async function login(credentials: LoginCredentials): Promise<void> {
    isLoggingIn.value = true
    try {
      const nextSession = await loginRequest(credentials)
      if (nextSession.user.role !== 'admin') {
        throw new Error('当前账号没有人工复核权限')
      }
      session.value = nextSession
      sessionStorage.setItem(storageKey, JSON.stringify(nextSession))
    } finally {
      isLoggingIn.value = false
    }
  }

  function logout(): void {
    session.value = null
    sessionStorage.removeItem(storageKey)
  }

  return {
    session: readonly(session),
    isLoggingIn: readonly(isLoggingIn),
    login,
    logout,
  }
}

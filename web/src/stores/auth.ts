import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { authApi } from '@/services/api'
import type { LoginRequest } from '@/types'

export const useAuthStore = defineStore('auth', () => {
  // State
  const token = ref<string | null>(localStorage.getItem('auth_token'))
  const user = ref<{ id: number; username: string } | null>(
    JSON.parse(localStorage.getItem('auth_user') || 'null')
  )
  const loading = ref(false)
  const error = ref<string | null>(null)

  // Getters
  const isAuthenticated = computed(() => !!token.value)
  const username = computed(() => user.value?.username || '')

  // Actions
  async function login(credentials: LoginRequest) {
    loading.value = true
    error.value = null
    
    try {
      const response = await authApi.login(credentials)
      
      token.value = response.token
      user.value = response.user
      
      localStorage.setItem('auth_token', response.token)
      localStorage.setItem('auth_user', JSON.stringify(response.user))
      
      return true
    } catch (err: unknown) {
      const axiosError = err as { response?: { data?: { message?: string } } }
      error.value = axiosError.response?.data?.message || 'Login failed'
      return false
    } finally {
      loading.value = false
    }
  }

  async function logout() {
    try {
      await authApi.logout()
    } catch {
      // Ignore logout errors
    } finally {
      token.value = null
      user.value = null
      localStorage.removeItem('auth_token')
      localStorage.removeItem('auth_user')
    }
  }

  function clearError() {
    error.value = null
  }

  return {
    // State
    token,
    user,
    loading,
    error,
    // Getters
    isAuthenticated,
    username,
    // Actions
    login,
    logout,
    clearError
  }
})

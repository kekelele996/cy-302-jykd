import { defineStore } from 'pinia'
import { authApi } from '../api'
import type { UserProfile } from '../types'

function readUser(): UserProfile | null {
  const raw = localStorage.getItem('gbexam_user')
  if (!raw) return null
  try {
    return JSON.parse(raw) as UserProfile
  } catch {
    return null
  }
}

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: localStorage.getItem('gbexam_token') || '',
    user: readUser()
  }),
  getters: {
    isLoggedIn: (state) => !!state.token,
    role: (state) => state.user?.role || ''
  },
  actions: {
    async login(username: string, password: string) {
      const res = await authApi.login({ username, password })
      this.token = res.token
      this.user = res.user
      localStorage.setItem('gbexam_token', res.token)
      localStorage.setItem('gbexam_user', JSON.stringify(res.user))
    },
    async fetchProfile() {
      const user = await authApi.profile()
      this.user = user
      localStorage.setItem('gbexam_user', JSON.stringify(user))
    },
    async logout() {
      try {
        await authApi.logout()
      } catch {
        // ignore network errors during logout
      }
      this.token = ''
      this.user = null
      localStorage.removeItem('gbexam_token')
      localStorage.removeItem('gbexam_user')
    }
  }
})

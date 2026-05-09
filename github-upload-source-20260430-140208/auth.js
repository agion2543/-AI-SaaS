import { defineStore } from 'pinia'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: localStorage.getItem('admin_token') || '',
    admin: JSON.parse(localStorage.getItem('admin_profile') || 'null')
  }),
  actions: {
    setAuth(payload) {
      this.token = payload.token
      this.admin = payload.admin
      localStorage.setItem('admin_token', payload.token)
      localStorage.setItem('admin_profile', JSON.stringify(payload.admin))
    },
    logout() {
      this.token = ''
      this.admin = null
      localStorage.removeItem('admin_token')
      localStorage.removeItem('admin_profile')
    }
  }
})

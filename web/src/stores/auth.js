import { defineStore } from 'pinia'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: localStorage.getItem('admin_token') || '',
    admin: JSON.parse(localStorage.getItem('admin_profile') || 'null')
  }),
  getters: {
    isSuperAdmin: (state) => state.admin?.role?.code === 'super_admin',
    permissions: (state) => Array.isArray(state.admin?.role?.permissions) ? state.admin.role.permissions : [],
    hasPermission: (state) => (code) => state.admin?.role?.code === 'super_admin' ||
      (Array.isArray(state.admin?.role?.permissions) && state.admin.role.permissions.includes(code))
  },
  actions: {
    setAuth(payload) {
      this.token = payload.token
      localStorage.setItem('admin_token', payload.token)
      this.setAdmin(payload.admin)
    },
    setAdmin(admin) {
      this.admin = admin
      localStorage.setItem('admin_profile', JSON.stringify(admin))
    },
    logout() {
      this.token = ''
      this.admin = null
      localStorage.removeItem('admin_token')
      localStorage.removeItem('admin_profile')
    }
  }
})

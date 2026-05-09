import { defineStore } from 'pinia'

export const useUserAuthStore = defineStore('userAuth', {
  state: () => ({
    token: localStorage.getItem('user_token') || '',
    profile: JSON.parse(localStorage.getItem('user_profile') || 'null')
  }),
  actions: {
    setAuth(payload) {
      this.token = payload.token
      this.profile = payload.user
      localStorage.setItem('user_token', payload.token)
      localStorage.setItem('user_profile', JSON.stringify(payload.user))
    },
    setProfile(profile) {
      this.profile = profile
      localStorage.setItem('user_profile', JSON.stringify(profile))
    },
    logout() {
      this.token = ''
      this.profile = null
      localStorage.removeItem('user_token')
      localStorage.removeItem('user_profile')
    }
  }
})

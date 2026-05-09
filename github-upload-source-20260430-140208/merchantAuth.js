import { defineStore } from 'pinia'

export const useMerchantAuthStore = defineStore('merchantAuth', {
  state: () => ({
    token: localStorage.getItem('merchant_token') || '',
    profile: JSON.parse(localStorage.getItem('merchant_profile') || 'null'),
    merchant: JSON.parse(localStorage.getItem('merchant_current') || 'null')
  }),
  actions: {
    setAuth(payload) {
      this.token = payload.token
      this.profile = payload.user
      this.merchant = payload.merchant
      localStorage.setItem('merchant_token', payload.token)
      localStorage.setItem('merchant_profile', JSON.stringify(payload.user))
      localStorage.setItem('merchant_current', JSON.stringify(payload.merchant))
    },
    logout() {
      this.token = ''
      this.profile = null
      this.merchant = null
      localStorage.removeItem('merchant_token')
      localStorage.removeItem('merchant_profile')
      localStorage.removeItem('merchant_current')
    }
  }
})

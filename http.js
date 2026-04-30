import axios from 'axios'

const http = axios.create({
  baseURL: import.meta.env.VITE_API_BASE || 'http://localhost:8080/api/v1',
  timeout: 10000
})

http.interceptors.request.use((config) => {
  const path = window.location.pathname
  let token = localStorage.getItem('admin_token')
  if (path.startsWith('/portal') || path.startsWith('/payment')) {
    token = localStorage.getItem('user_token')
  }
  if (path.startsWith('/merchant')) {
    token = localStorage.getItem('merchant_token')
  }
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

http.interceptors.response.use((response) => response.data, (error) => {
  const message = error.response?.data?.message || ''
  const path = window.location.pathname

  if (error.response?.status === 401 || message === 'invalid token' || message === 'missing token') {
    if (path.startsWith('/merchant')) {
      localStorage.removeItem('merchant_token')
      localStorage.removeItem('merchant_profile')
      localStorage.removeItem('merchant_current')
      if (path !== '/merchant/login') {
        window.location.href = `/merchant/login?redirect=${encodeURIComponent(path)}`
      }
    } else if (path.startsWith('/portal')) {
      localStorage.removeItem('user_token')
      localStorage.removeItem('user_profile')
      if (path !== '/portal/login') {
        window.location.href = `/portal/login?redirect=${encodeURIComponent(path)}`
      }
    } else {
      localStorage.removeItem('admin_token')
      localStorage.removeItem('admin_profile')
      if (path !== '/login') {
        window.location.href = `/login?redirect=${encodeURIComponent(path)}`
      }
    }
  }

  if (error.response?.status === 403 && message.includes('订阅已过期') && path !== '/merchant/subscription') {
    sessionStorage.setItem('merchant_subscription_message', message)
    window.location.href = '/merchant/subscription'
  }

  return Promise.reject(error)
})

export default http

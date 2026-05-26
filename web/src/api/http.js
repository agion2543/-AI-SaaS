import axios from 'axios'

// 根据当前页面判断是否为移动端
const isMobile = window.location.pathname.includes('mobile.html') ||
  window.location.pathname.startsWith('/m/')

// 移动端使用相对路径（走 Vite 代理，避免 CORS）
// 桌面端使用绝对路径
const http = axios.create({
  baseURL: isMobile
    ? '/api/v1'
    : (import.meta.env.VITE_API_BASE || 'http://localhost:8080/api/v1'),
  timeout: 10000
})

http.interceptors.request.use((config) => {
  const path = window.location.pathname
  let token = localStorage.getItem('admin_token')

  // 桌面端：根据路径前缀判断
  // 移动端：根据 hash 或 pathname 判断
  const isMobilePage = path.includes('mobile.html') ||
    path.startsWith('/m/') ||
    (window.location.hash && window.location.hash.includes('/m/'))

  if ((path.startsWith('/portal') || path.startsWith('/payment')) ||
      (isMobilePage && window.location.hash.includes('/customer'))) {
    token = localStorage.getItem('user_token')
  }
  if (path.startsWith('/merchant') ||
      (isMobilePage && (window.location.hash.includes('/merchant') || !window.location.hash || window.location.hash === '#/'))) {
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
  const isMobilePage = path.includes('mobile.html') ||
    path.startsWith('/m/') ||
    (window.location.hash && window.location.hash.includes('/m/'))

  if (error.response?.status === 401 || message === 'invalid token' || message === 'missing token') {
    if (path.startsWith('/merchant') ||
        (isMobilePage && window.location.hash.includes('/merchant'))) {
      localStorage.removeItem('merchant_token')
      localStorage.removeItem('merchant_profile')
      localStorage.removeItem('merchant_current')
      if (!isMobilePage && path !== '/merchant/login') {
        window.location.href = `/merchant/login?redirect=${encodeURIComponent(path)}`
      } else if (isMobilePage) {
        window.location.hash = '#/m/merchant/login'
      }
    } else if (path.startsWith('/portal') ||
             (isMobilePage && window.location.hash.includes('/customer'))) {
      localStorage.removeItem('user_token')
      localStorage.removeItem('user_profile')
      if (!isMobilePage && path !== '/portal/login') {
        window.location.href = `/portal/login?redirect=${encodeURIComponent(path)}`
      }
    } else {
      localStorage.removeItem('admin_token')
      localStorage.removeItem('admin_profile')
      if (!isMobilePage && path !== '/login') {
        window.location.href = `/login?redirect=${encodeURIComponent(path)}`
      }
    }
  }

  if (error.response?.status === 403 && message.includes('订阅已过期')) {
    sessionStorage.setItem('merchant_subscription_message', message)
    if (isMobilePage) {
      window.location.hash = '#/m/merchant/subscription'
    } else if (path !== '/merchant/subscription') {
      window.location.href = '/merchant/subscription'
    }
  }

  return Promise.reject(error)
})

export default http

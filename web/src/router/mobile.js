// Mobile Router - 移动端路由配置
import { createRouter, createWebHashHistory } from 'vue-router'

// 商家端页面（需要认证）
const MerchantLayout = () => import('../mobile/merchant/MerchantLayout.vue')
const MerchantHome = () => import('../mobile/merchant/MerchantHome.vue')
const MerchantOrders = () => import('../mobile/merchant/MerchantOrders.vue')
const MerchantOrderDetail = () => import('../mobile/merchant/MerchantOrderDetail.vue')
const MerchantProducts = () => import('../mobile/merchant/MerchantProducts.vue')
const MerchantStores = () => import('../mobile/merchant/MerchantStores.vue')
const MerchantFinance = () => import('../mobile/merchant/MerchantFinance.vue')
const MerchantCoupons = () => import('../mobile/merchant/MerchantCoupons.vue')
const MerchantAI = () => import('../mobile/merchant/MerchantAI.vue')
const MerchantSettings = () => import('../mobile/merchant/MerchantSettings.vue')

// 顾客端页面（无需登录）
const CustomerStore = () => import('../mobile/customer/CustomerStore.vue')
const CustomerCart = () => import('../mobile/customer/CustomerCart.vue')
const CustomerOrderConfirm = () => import('../mobile/customer/CustomerOrderConfirm.vue')
const CustomerOrderStatus = () => import('../mobile/customer/CustomerOrderStatus.vue')

// 商家登录/注册页面
const MerchantLogin = () => import('../mobile/merchant/MerchantLogin.vue')
const MerchantRegister = () => import('../mobile/merchant/MerchantRegister.vue')

const routes = [
  // 根路径重定向到商家登录
  { path: '/', redirect: '/m/merchant/login' },

  // ===== 商家端移动端 =====
  {
    path: '/m/merchant',
    component: MerchantLayout,
    meta: { requiresAuth: true },
    children: [
      { path: '', redirect: '/m/merchant/home' },
      { path: 'home', component: MerchantHome, name: 'MerchantHome' },
      { path: 'orders', component: MerchantOrders, name: 'MerchantOrders' },
      { path: 'orders/:id', component: MerchantOrderDetail, name: 'MerchantOrderDetail' },
      { path: 'products', component: MerchantProducts, name: 'MerchantProducts' },
      { path: 'stores', component: MerchantStores, name: 'MerchantStores' },
      { path: 'finance', component: MerchantFinance, name: 'MerchantFinance' },
      { path: 'coupons', component: MerchantCoupons, name: 'MerchantCoupons' },
      { path: 'ai', component: MerchantAI, name: 'MerchantAI' },
      { path: 'settings', component: MerchantSettings, name: 'MerchantSettings' },
    ]
  },
  { path: '/m/merchant/login', component: MerchantLogin, name: 'MerchantLogin' },
  { path: '/m/merchant/register', component: MerchantRegister, name: 'MerchantRegister' },

  // ===== 顾客端移动端（扫码点单） =====
  { path: '/m/store/:storeId', component: CustomerStore, name: 'CustomerStore' },
  { path: '/m/cart/:storeId', component: CustomerCart, name: 'CustomerCart' },
  { path: '/m/order/confirm', component: CustomerOrderConfirm, name: 'CustomerOrderConfirm' },
  { path: '/m/order/:orderNo', component: CustomerOrderStatus, name: 'CustomerOrderStatus' },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes
})

// 路由守卫
router.beforeEach((to, from, next) => {
  // 商家端需要登录验证
  if (to.meta.requiresAuth) {
    const token = localStorage.getItem('merchant_token')
    if (!token) {
      next({ path: '/m/merchant/login', query: { redirect: to.fullPath } })
      return
    }
  }
  next()
})

export default router

import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useUserAuthStore } from '../stores/userAuth'
import { useMerchantAuthStore } from '../stores/merchantAuth'
import AdminLayout from '../layout/AdminLayout.vue'
import MerchantLayout from '../layout/MerchantLayout.vue'
import UserLayout from '../layout/UserLayout.vue'
import LoginView from '../views/auth/LoginView.vue'
import ForgotPasswordView from '../views/auth/ForgotPasswordView.vue'
import UserLoginView from '../views/auth/UserLoginView.vue'
import DashboardView from '../views/dashboard/DashboardView.vue'
import UsersView from '../views/users/UsersView.vue'
import UserProfileView from '../views/users/UserProfileView.vue'
import PackagesView from '../views/packages/PackagesView.vue'
import UserPackagesView from '../views/packages/UserPackagesView.vue'
import OrdersView from '../views/orders/OrdersView.vue'
import UserOrdersView from '../views/orders/UserOrdersView.vue'
import UserPayView from '../views/orders/UserPayView.vue'
import PaymentReturnView from '../views/orders/PaymentReturnView.vue'
import CardsView from '../views/cards/CardsView.vue'
import SystemView from '../views/system/SystemView.vue'
import AuditLogsView from '../views/system/AuditLogsView.vue'
import MerchantsView from '../views/merchants/MerchantsView.vue'
import MerchantDetailView from '../views/merchants/MerchantDetailView.vue'
import MerchantLoginView from '../views/merchants/MerchantLoginView.vue'
import MerchantDashboardView from '../views/merchants/MerchantDashboardView.vue'
import MerchantProfileView from '../views/merchants/MerchantProfileView.vue'
import MerchantAIView from '../views/merchants/MerchantAIView.vue'
import MerchantPromotionsView from '../views/merchants/MerchantPromotionsView.vue'
import MerchantStoresView from '../views/merchants/MerchantStoresView.vue'
import MerchantStoreProductsView from '../views/merchants/MerchantStoreProductsView.vue'
import MerchantOrdersView from '../views/merchants/MerchantOrdersView.vue'
import MerchantOrderDetailView from '../views/merchants/MerchantOrderDetailView.vue'
import MerchantFinanceView from '../views/merchants/MerchantFinanceView.vue'
import MerchantPaymentSettingsView from '../views/merchants/MerchantPaymentSettingsView.vue'
import MerchantSubscriptionView from '../views/merchants/MerchantSubscriptionView.vue'
import MerchantRegisterView from '../views/merchants/MerchantRegisterView.vue'
import CustomerStoreView from '../views/customer/CustomerStoreView.vue'
import CustomerOrderSuccessView from '../views/customer/CustomerOrderSuccessView.vue'
import { checkMerchantSubscription } from '../api/modules'

const routes = [
  { path: '/', redirect: '/admin/dashboard' },
  { path: '/login', component: LoginView },
  { path: '/dashboard', redirect: '/admin/dashboard' },
  { path: '/users', redirect: '/admin/users' },
  { path: '/packages', redirect: '/admin/packages' },
  { path: '/orders', redirect: '/admin/orders' },
  { path: '/cards', redirect: '/admin/cards' },
  { path: '/system', redirect: '/admin/system' },
  {
    path: '/admin',
    component: AdminLayout,
    children: [
      { path: '', redirect: '/admin/dashboard' },
      { path: 'dashboard', component: DashboardView },
      { path: 'merchants', component: MerchantsView },
      { path: 'merchants/:id', component: MerchantDetailView },
      { path: 'users', component: UsersView },
      { path: 'packages', component: PackagesView },
      { path: 'orders', component: OrdersView },
      { path: 'cards', component: CardsView },
      { path: 'audit-logs', component: AuditLogsView },
      { path: 'system', component: SystemView }
    ]
  },
  {
    path: '/merchant',
    children: [
      { path: 'login', component: MerchantLoginView },
      { path: 'register', component: MerchantRegisterView },
      {
        path: '',
        component: MerchantLayout,
        children: [
          { path: 'dashboard', component: MerchantDashboardView, meta: { requiresMerchantAuth: true } },
          { path: 'promotions', component: MerchantPromotionsView, meta: { requiresMerchantAuth: true } },
          { path: 'ai', component: MerchantAIView, meta: { requiresMerchantAuth: true } },
          { path: 'subscription', component: MerchantSubscriptionView, meta: { requiresMerchantAuth: true, allowExpiredSubscription: true } },
          { path: 'profile', component: MerchantProfileView, meta: { requiresMerchantAuth: true } },
          { path: 'stores', component: MerchantStoresView, meta: { requiresMerchantAuth: true } },
          { path: 'orders', component: MerchantOrdersView, meta: { requiresMerchantAuth: true } },
          { path: 'orders/:id', component: MerchantOrderDetailView, meta: { requiresMerchantAuth: true } },
          { path: 'finance', component: MerchantFinanceView, meta: { requiresMerchantAuth: true } },
          { path: 'payment-settings', component: MerchantPaymentSettingsView, meta: { requiresMerchantAuth: true } },
          { path: 'stores/:storeId/products', component: MerchantStoreProductsView, meta: { requiresMerchantAuth: true } }
        ]
      }
    ]
  },
  { path: '/customer/store/:storeId', component: CustomerStoreView },
  { path: '/customer/order-success', component: CustomerOrderSuccessView },
  { path: '/customer/orders/:orderNo', component: CustomerOrderSuccessView },
  {
    path: '/portal',
    component: UserLayout,
    children: [
      { path: '', redirect: '/portal/packages' },
      { path: 'login', component: UserLoginView },
      { path: 'forgot-password', component: ForgotPasswordView },
      { path: 'packages', component: UserPackagesView },
      { path: 'pay', component: UserPayView, meta: { requiresUserAuth: true } },
      { path: 'orders', component: UserOrdersView, meta: { requiresUserAuth: true } },
      { path: 'profile', component: UserProfileView, meta: { requiresUserAuth: true } }
    ]
  },
  { path: '/payment/return', component: PaymentReturnView, meta: { requiresUserAuth: true } }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach(async (to) => {
  const adminStore = useAuthStore()
  const userStore = useUserAuthStore()
  const merchantStore = useMerchantAuthStore()

  if (to.meta.requiresUserAuth && !userStore.token) {
    return '/portal/login'
  }

  if (to.meta.requiresMerchantAuth && !merchantStore.token) {
    return { path: '/merchant/login', query: { redirect: to.fullPath } }
  }
  if (
    to.meta.requiresMerchantAuth &&
    !to.meta.allowExpiredSubscription
  ) {
    try {
      const res = await checkMerchantSubscription()
      if (!res.data.valid) {
        return { path: '/merchant/subscription', query: { redirect: to.fullPath } }
      }
    } catch {
      return { path: '/merchant/subscription', query: { redirect: to.fullPath } }
    }
  }

  if (to.path.startsWith('/portal')) {
    if (to.path === '/portal/login' && userStore.token) {
      return '/portal/packages'
    }
    return
  }

  if (to.path.startsWith('/merchant')) {
    if (to.path === '/merchant/login' && merchantStore.token) {
      return '/merchant/dashboard'
    }
    if (to.path === '/merchant/register' && merchantStore.token) {
      return '/merchant/dashboard'
    }
    return
  }

  if (to.path.startsWith('/customer')) {
    return
  }

  if (to.path !== '/login' && !adminStore.token) {
    return '/login'
  }
  if (to.path === '/login' && adminStore.token) {
    return '/admin/dashboard'
  }
})

export default router

import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useUserAuthStore } from '../stores/userAuth'
import { useMerchantAuthStore } from '../stores/merchantAuth'
import { fetchMerchantInfo, fetchAdminSession } from '../api/modules'
import { merchantSubscriptionAccessState } from '../utils/merchantSubscriptionState'
const AdminLayout = () => import('../layout/AdminLayout.vue')
const MerchantLayout = () => import('../layout/MerchantLayout.vue')
const UserLayout = () => import('../layout/UserLayout.vue')
const LoginView = () => import('../views/auth/LoginView.vue')
const ForgotPasswordView = () => import('../views/auth/ForgotPasswordView.vue')
const UserLoginView = () => import('../views/auth/UserLoginView.vue')
const DashboardView = () => import('../views/dashboard/DashboardView.vue')
const UsersView = () => import('../views/users/UsersView.vue')
const UserProfileView = () => import('../views/users/UserProfileView.vue')
const PackagesView = () => import('../views/packages/PackagesView.vue')
const UserPackagesView = () => import('../views/packages/UserPackagesView.vue')
const OrdersView = () => import('../views/orders/OrdersView.vue')
const SubscriptionRevenueView = () => import('../views/orders/SubscriptionRevenueView.vue')
const UserOrdersView = () => import('../views/orders/UserOrdersView.vue')
const UserPayView = () => import('../views/orders/UserPayView.vue')
const PaymentReturnView = () => import('../views/orders/PaymentReturnView.vue')
const CardsView = () => import('../views/cards/CardsView.vue')
const SystemView = () => import('../views/system/SystemView.vue')
const PlatformCostView = () => import('../views/system/PlatformCostView.vue')
const AnnouncementsView = () => import('../views/system/AnnouncementsView.vue')
const MaintenancePlansView = () => import('../views/system/MaintenancePlansView.vue')
const MerchantAnnouncementsView = () => import('../views/merchants/MerchantAnnouncementsView.vue')
const AuditLogsView = () => import('../views/system/AuditLogsView.vue')
const FollowUpTodosView = () => import('../views/system/FollowUpTodosView.vue')
const AdminAccessView = () => import('../views/system/AdminAccessView.vue')
const AdminAccessRestrictedView = () => import('../views/system/AdminAccessRestrictedView.vue')
const MerchantsView = () => import('../views/merchants/MerchantsView.vue')
const GroupBuyLinksView = () => import('../views/platform/GroupBuyLinksView.vue')
const RefundReconciliationView = () => import('../views/platform/RefundReconciliationView.vue')
const RefundReviewView = () => import('../views/platform/RefundReviewView.vue')
const MerchantReviewView = () => import('../views/platform/MerchantReviewView.vue')
const SupportTicketsView = () => import('../views/platform/SupportTicketsView.vue')
const MerchantTicketsView = () => import('../views/merchants/MerchantTicketsView.vue')
const MerchantDetailView = () => import('../views/merchants/MerchantDetailView.vue')
const MerchantLoginView = () => import('../views/merchants/MerchantLoginView.vue')
const MerchantDashboardView = () => import('../views/merchants/MerchantDashboardView.vue')
const MerchantProfileView = () => import('../views/merchants/MerchantProfileView.vue')
const MerchantAIView = () => import('../views/merchants/MerchantAIView.vue')
const MerchantOpsDashboardView = () => import('../views/merchants/MerchantOpsDashboardView.vue')
const MerchantPromotionsView = () => import('../views/merchants/MerchantPromotionsView.vue')
const MerchantPromotionCalculatorView = () => import('../views/merchants/MerchantPromotionCalculatorView.vue')
const MerchantGroupBuyView = () => import('../views/merchants/MerchantGroupBuyView.vue')
const MerchantCouponsView = () => import('../views/merchants/MerchantCouponsView.vue')
const MerchantShareView = () => import('../views/merchants/MerchantShareView.vue')
const MerchantStoresView = () => import('../views/merchants/MerchantStoresView.vue')
const MerchantStoreProductsView = () => import('../views/merchants/MerchantStoreProductsView.vue')
const MerchantOrdersView = () => import('../views/merchants/MerchantOrdersView.vue')
const MerchantOrderDetailView = () => import('../views/merchants/MerchantOrderDetailView.vue')
const MerchantFinanceView = () => import('../views/merchants/MerchantFinanceView.vue')
const MerchantPaymentSettingsView = () => import('../views/merchants/MerchantPaymentSettingsView.vue')
const MerchantSubscriptionView = () => import('../views/merchants/MerchantSubscriptionView.vue')
const MerchantRegisterView = () => import('../views/merchants/MerchantRegisterView.vue')
const CustomerStoreView = () => import('../views/customer/CustomerStoreView.vue')
const CustomerOrderSuccessView = () => import('../views/customer/CustomerOrderSuccessView.vue')

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
      { path: 'todos', component: FollowUpTodosView },
      { path: 'merchants', component: MerchantsView },
      { path: 'group-buy-links', component: GroupBuyLinksView },
      { path: 'merchants/:id', component: MerchantDetailView },
      { path: 'users', component: UsersView },
      { path: 'packages', component: PackagesView },
      { path: 'subscriptions', component: SubscriptionRevenueView },
      { path: 'orders', component: OrdersView },
      { path: 'refund-reconciliations', component: RefundReconciliationView },
      { path: 'refund-reviews', component: RefundReviewView },
      { path: 'merchant-reviews', component: MerchantReviewView },
      { path: 'support-tickets', component: SupportTicketsView },
      { path: 'cards', component: CardsView },
      { path: 'audit-logs', component: AuditLogsView },
      { path: 'system', component: SystemView },
      { path: 'platform-costs', component: PlatformCostView },
      { path: 'announcements', component: AnnouncementsView },
      { path: 'maintenance-plans', component: MaintenancePlansView },
      { path: 'access', component: AdminAccessView },
      { path: 'limited-access', component: AdminAccessRestrictedView }
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
          { path: 'dashboard', component: MerchantDashboardView, meta: { requiresMerchantAuth: true, allowExpiredReadOnly: true } },
          { path: 'promotions', component: MerchantPromotionsView, meta: { requiresMerchantAuth: true, allowExpiredReadOnly: true } },
          { path: 'promotion-calculator', component: MerchantPromotionCalculatorView, meta: { requiresMerchantAuth: true, allowExpiredReadOnly: true } },
          { path: 'group-buy', component: MerchantGroupBuyView, meta: { requiresMerchantAuth: true, allowExpiredReadOnly: true } },
          { path: 'ai', component: MerchantAIView, meta: { requiresMerchantAuth: true } },
          { path: 'ops-dashboard', component: MerchantOpsDashboardView, meta: { requiresMerchantAuth: true } },
          { path: 'announcements', component: MerchantAnnouncementsView, meta: { requiresMerchantAuth: true, allowExpiredReadOnly: true } },
          { path: 'tickets', component: MerchantTicketsView, meta: { requiresMerchantAuth: true, allowExpiredReadOnly: true } },
          { path: 'share', component: MerchantShareView, meta: { requiresMerchantAuth: true } },
          { path: 'subscription', component: MerchantSubscriptionView, meta: { requiresMerchantAuth: true, allowExpiredSubscription: true } },
          { path: 'profile', component: MerchantProfileView, meta: { requiresMerchantAuth: true, allowExpiredReadOnly: true } },
          { path: 'stores', component: MerchantStoresView, meta: { requiresMerchantAuth: true, allowExpiredReadOnly: true } },
          { path: 'orders', component: MerchantOrdersView, meta: { requiresMerchantAuth: true, allowExpiredReadOnly: true } },
          { path: 'orders/:id', component: MerchantOrderDetailView, meta: { requiresMerchantAuth: true, allowExpiredReadOnly: true } },
          { path: 'finance', component: MerchantFinanceView, meta: { requiresMerchantAuth: true, allowExpiredReadOnly: true } },
          { path: 'coupons', component: MerchantCouponsView, meta: { requiresMerchantAuth: true } },
          { path: 'payment-settings', component: MerchantPaymentSettingsView, meta: { requiresMerchantAuth: true, allowExpiredReadOnly: true } },
          { path: 'stores/:storeId/products', component: MerchantStoreProductsView, meta: { requiresMerchantAuth: true, allowExpiredReadOnly: true } }
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
      const res = await fetchMerchantInfo()
      const merchant = res.data?.merchant || res.data || {}
      // Pending onboarding: only profile, subscription and announcements
      // are reachable; everything else goes to the profile page, where the
      // review status and the rejection reason are shown.
      if (merchant.status === 'pending') {
        const pendingAllowed = ['/merchant/profile', '/merchant/subscription', '/merchant/announcements', '/merchant/tickets']
        if (pendingAllowed.includes(to.path)) return
        return { path: '/merchant/profile', query: { redirect: to.fullPath } }
      }
      const accessState = merchantSubscriptionAccessState(merchant)
      if (accessState === 'active') return
      if (accessState === 'expired' && to.meta.allowExpiredReadOnly) return
      return { path: '/merchant/subscription', query: { redirect: to.fullPath } }
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

  if (to.path.startsWith('/admin') && adminStore.token) {
    try {
      const { data } = await fetchAdminSession()
      adminStore.setAdmin(data.admin)
    } catch (error) {
      if ([401, 403].includes(error?.response?.status)) {
        adminStore.logout()
        return '/login'
      }
    }
    const isSuperAdmin = adminStore.isSuperAdmin
    const hasAny = (...codes) => isSuperAdmin || codes.some(code => adminStore.hasPermission(code))
    const firstAllowedPath = () => {
      if (isSuperAdmin || adminStore.hasPermission('read_only')) return '/admin/dashboard'
      if (adminStore.hasPermission('operations')) return '/admin/todos'
      if (adminStore.hasPermission('finance')) return '/admin/packages'
      if (adminStore.hasPermission('system_admin')) return '/admin/audit-logs'
      return '/admin/limited-access'
    }
    let allowed = false
    if (to.path === '/admin/limited-access') allowed = true
    else if (to.path === '/admin/access') allowed = isSuperAdmin
    else if (to.path === '/admin/platform-costs') allowed = isSuperAdmin
    else if (to.path === '/admin/announcements' || to.path === '/admin/maintenance-plans') allowed = isSuperAdmin
    else if (to.path === '/admin/refund-reviews') allowed = isSuperAdmin
    else if (to.path === '/admin/merchant-reviews') allowed = isSuperAdmin
    else if (to.path === '/admin/support-tickets') allowed = isSuperAdmin
    else if (to.path === '/admin/dashboard') allowed = hasAny('read_only')
    else if (to.path === '/admin/todos') allowed = hasAny('operations')
    else if (/^\/admin\/(merchants|group-buy-links|users)(\/|$)/.test(to.path)) allowed = hasAny('read_only', 'operations')
    else if (/^\/admin\/(packages|subscriptions|orders|refund-reconciliations|cards)(\/|$)/.test(to.path)) allowed = hasAny('finance')
    else if (/^\/admin\/(audit-logs|system)(\/|$)/.test(to.path)) allowed = hasAny('system_admin')
    if (!allowed) return firstAllowedPath()
  }

  if (to.path === '/login' && adminStore.token) {
    if (adminStore.isSuperAdmin || adminStore.hasPermission('read_only')) return '/admin/dashboard'
    if (adminStore.hasPermission('operations')) return '/admin/todos'
    if (adminStore.hasPermission('finance')) return '/admin/packages'
    if (adminStore.hasPermission('system_admin')) return '/admin/audit-logs'
    return '/admin/limited-access'
  }
})

export default router

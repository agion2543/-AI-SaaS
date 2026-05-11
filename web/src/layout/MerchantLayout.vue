<template>
  <div class="merchant-layout">
    <aside class="merchant-sidebar">
      <div class="merchant-brand">
        <div class="merchant-brand-title">Merchant</div>
        <div class="merchant-brand-subtitle">{{ text.subtitle }}</div>
      </div>

      <el-menu
        router
        :default-active="$route.path"
        class="merchant-menu"
        background-color="transparent"
        text-color="#cfe4ef"
        active-text-color="#ffffff"
      >
        <div class="menu-section">{{ text.sectionOperation }}</div>
        <el-menu-item index="/merchant/dashboard" :disabled="navigationLocked">{{ text.dashboard }}</el-menu-item>
        <el-menu-item index="/merchant/stores" :disabled="navigationLocked">{{ text.stores }}</el-menu-item>
        <el-menu-item index="/merchant/orders" :disabled="navigationLocked">{{ text.orders }}</el-menu-item>
        <el-menu-item index="/merchant/finance" :disabled="navigationLocked">{{ text.finance }}</el-menu-item>
        <el-menu-item index="/merchant/promotions" :disabled="navigationLocked">{{ text.promotions }}</el-menu-item>
        <el-menu-item index="/merchant/coupons" :disabled="navigationLocked">{{ text.coupons }}</el-menu-item>
        <div class="menu-section">{{ text.sectionAI }}</div>
        <el-menu-item index="/merchant/ai" :disabled="navigationLocked">{{ text.ai }}</el-menu-item>
        <div class="menu-section">{{ text.sectionSettings }}</div>
        <el-menu-item index="/merchant/subscription">{{ text.subscription }}</el-menu-item>
        <el-menu-item index="/merchant/payment-settings" :disabled="navigationLocked">{{ text.paymentSettings }}</el-menu-item>
        <el-menu-item index="/merchant/profile" :disabled="navigationLocked">{{ text.profile }}</el-menu-item>
      </el-menu>

      <div v-if="navigationLocked" class="subscription-lock">
        {{ text.lockTip }}
      </div>
    </aside>

    <main class="merchant-main">
      <header class="merchant-header page-card">
        <div>
          <div class="merchant-title">{{ text.title }}</div>
          <div class="merchant-subtitle">{{ text.description }}</div>
        </div>
        <el-button type="primary" plain @click="logout">{{ text.logout }}</el-button>
      </header>

      <section class="merchant-content">
        <router-view :key="routeViewKey" />
      </section>
    </main>
  </div>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { checkMerchantSubscription } from '../api/modules'
import { useMerchantAuthStore } from '../stores/merchantAuth'

const text = {
  subtitle: '\u5546\u5bb6\u5de5\u4f5c\u53f0',
  sectionOperation: '\u7ecf\u8425',
  sectionAI: 'AI \u80fd\u529b',
  sectionSettings: '\u8bbe\u7f6e',
  dashboard: '\u7ecf\u8425\u770b\u677f',
  stores: '\u95e8\u5e97\u7ba1\u7406',
  orders: '\u8ba2\u5355\u7ba1\u7406',
  finance: '\u8d22\u52a1\u5bf9\u8d26',
  promotions: '\u4f18\u60e0\u6d3b\u52a8',
  coupons: '\u5238\u5305/\u6838\u9500',
  ai: 'AI \u7ecf\u8425\u5206\u6790',
  subscription: '\u8ba2\u9605\u4ed8\u8d39',
  paymentSettings: '\u6536\u6b3e\u8bbe\u7f6e',
  profile: '\u5546\u5bb6\u4fe1\u606f',
  lockTip: '\u5b8c\u6210\u8ba2\u9605\u540e\u5373\u53ef\u89e3\u9501\u5de5\u4f5c\u53f0\u529f\u80fd',
  title: '\u5546\u5bb6\u5de5\u4f5c\u53f0',
  description: '\u67e5\u770b\u5546\u5bb6\u4fe1\u606f\u4e0e\u540e\u7eed\u7ecf\u8425\u80fd\u529b\u7684\u7edf\u4e00\u5165\u53e3',
  logout: '\u9000\u51fa\u767b\u5f55'
}

const router = useRouter()
const route = useRoute()
const authStore = useMerchantAuthStore()
const subscriptionValid = ref(false)
const navigationLocked = computed(() => !subscriptionValid.value && route.path === '/merchant/subscription')
const routeViewKey = computed(() => route.fullPath)

const refreshSubscriptionState = async () => {
  if (!authStore.token) {
    subscriptionValid.value = false
    return
  }
  try {
    const res = await checkMerchantSubscription()
    subscriptionValid.value = Boolean(res.data?.valid)
  } catch {
    subscriptionValid.value = false
  }
}

const logout = () => {
  authStore.logout()
  router.push('/merchant/login')
}

watch(() => route.fullPath, refreshSubscriptionState)
onMounted(refreshSubscriptionState)
</script>

<style scoped>
.merchant-layout {
  display: grid;
  grid-template-columns: 248px minmax(0, 1fr);
  min-height: 100vh;
  background:
    radial-gradient(circle at top left, rgba(14, 165, 233, 0.16), transparent 24%),
    linear-gradient(135deg, #f8fbff 0%, #eef6fb 100%);
}

.merchant-sidebar {
  padding: 24px 16px;
  background:
    radial-gradient(circle at top, rgba(14, 165, 233, 0.18), transparent 28%),
    linear-gradient(180deg, #0f2534 0%, #143447 100%);
  color: #fff;
  border-right: 1px solid rgba(148, 163, 184, 0.12);
}

.merchant-brand {
  padding: 6px 10px 16px;
}

.merchant-brand-title {
  font-size: 30px;
  font-weight: 900;
}

.merchant-brand-subtitle {
  margin-top: 8px;
  font-size: 13px;
  color: #9ec5d8;
}

.merchant-menu {
  border: none;
}

.merchant-menu :deep(.el-menu-item) {
  height: 46px;
  margin: 4px 0;
  border-radius: 8px;
  font-size: 15px;
  font-weight: 700;
}

.merchant-menu :deep(.el-menu-item:hover) {
  background: rgba(14, 165, 233, 0.14);
  color: #ffffff !important;
}

.merchant-menu :deep(.el-menu-item.is-active) {
  background: linear-gradient(90deg, #0ea5e9 0%, #38bdf8 100%);
  color: #ffffff !important;
  box-shadow: 0 12px 24px rgba(14, 165, 233, 0.24);
}

.merchant-menu :deep(.el-menu-item.is-disabled) {
  cursor: not-allowed;
  opacity: 0.38;
  background: transparent !important;
}

.subscription-lock {
  margin: 18px 10px 0;
  padding: 12px;
  border: 1px solid rgba(125, 211, 252, 0.22);
  border-radius: 12px;
  background: rgba(14, 165, 233, 0.1);
  color: #d7eef8;
  font-size: 13px;
  line-height: 1.6;
}

.menu-section {
  margin: 18px 10px 8px;
  color: #79aabd;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.14em;
}

.merchant-main {
  padding: 20px;
  display: grid;
  gap: 20px;
}

.merchant-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  padding: 20px 24px;
}

.merchant-title {
  font-size: 28px;
  font-weight: 900;
}

.merchant-subtitle {
  margin-top: 6px;
  color: var(--muted);
}

.merchant-content {
  display: grid;
  gap: 20px;
}

@media (max-width: 960px) {
  .merchant-layout {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .merchant-header {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>

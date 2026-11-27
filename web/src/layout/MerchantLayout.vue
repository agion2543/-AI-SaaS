<template>
  <div class="merchant-layout">
    <aside class="merchant-sidebar">
      <div class="merchant-brand">
        <div class="merchant-brand-title">Merchant</div>
        <div class="merchant-brand-subtitle">{{ text.subtitle }}</div>
      </div>

      <el-menu
        router
        :mode="compactNavigation ? 'horizontal' : 'vertical'"
        :default-active="activeMenuPath"
        class="merchant-menu"
        background-color="transparent"
        text-color="#cfe4ef"
        active-text-color="#ffffff"
      >
        <div class="menu-section">{{ text.sectionOrders }}</div>
        <el-menu-item index="/merchant/orders" :disabled="navigationLocked">{{ text.orders }}</el-menu-item>
        <el-menu-item index="/merchant/finance" :disabled="navigationLocked">{{ text.finance }}</el-menu-item>
        <div class="menu-section">{{ text.sectionStores }}</div>
        <el-menu-item index="/merchant/stores" :disabled="navigationLocked">{{ text.stores }}</el-menu-item>
        <div class="menu-section">{{ text.sectionActivity }}</div>
        <el-menu-item index="/merchant/dashboard" :disabled="navigationLocked">{{ text.dashboard }}</el-menu-item>
        <el-menu-item index="/merchant/promotions" :disabled="navigationLocked">{{ text.promotions }}</el-menu-item>
        <el-menu-item index="/merchant/share" :disabled="navigationLocked">{{ text.share }}</el-menu-item>
        <el-menu-item index="/merchant/coupons" :disabled="navigationLocked">{{ text.coupons }}</el-menu-item>
        <div class="menu-section">{{ text.sectionSubscription }}</div>
        <el-menu-item index="/merchant/subscription">{{ text.subscription }}</el-menu-item>
        <el-sub-menu index="merchant-more" :disabled="navigationLocked">
          <template #title>{{ text.more }}</template>
          <el-menu-item index="/merchant/group-buy" :disabled="navigationLocked">{{ text.groupBuy }}</el-menu-item>
          <el-menu-item index="/merchant/promotion-calculator" :disabled="navigationLocked">{{ text.promotionCalculator }}</el-menu-item>
          <el-menu-item index="/merchant/ai" :disabled="navigationLocked">{{ text.ai }}</el-menu-item>
          <el-menu-item index="/merchant/announcements" :disabled="navigationLocked">{{ text.announcements }}</el-menu-item>
          <el-menu-item index="/merchant/payment-settings" :disabled="navigationLocked">{{ text.paymentSettings }}</el-menu-item>
          <el-menu-item index="/merchant/profile" :disabled="navigationLocked">{{ text.profile }}</el-menu-item>
        </el-sub-menu>
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
        <el-alert
          v-if="maintenanceNotice"
          class="maintenance-notice"
          type="warning"
          show-icon
          :closable="false"
          :title="maintenanceNotice.title"
          :description="maintenanceNoticeDescription"
        />
        <el-alert
          v-if="subscriptionExpired && route.path !== '/merchant/subscription'"
          type="warning"
          show-icon
          :closable="false"
          title="订阅已到期：现有经营数据可只读查看，新建、修改、删除和业务处理操作已锁定。续费后恢复。"
        />
        <div v-if="subscriptionPaused && route.path !== '/merchant/subscription'" class="pause-lock-screen">
          <h2>商家业务已被平台安全暂停</h2>
          <p>暂停期间不能查看或操作经营数据。请前往订阅页查看状态及恢复方式。</p>
          <el-button type="primary" @click="router.push('/merchant/subscription')">查看订阅状态与恢复入口</el-button>
        </div>
        <router-view v-else :key="routeViewKey" />
      </section>
    </main>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { fetchMerchantInfo, fetchMerchantMaintenanceNotice } from '../api/modules'
import { useMerchantAuthStore } from '../stores/merchantAuth'
import { merchantSubscriptionAccessState } from '../utils/merchantSubscriptionState'
import { getMerchantPcActivePath } from '../utils/navigation'

const text = {
  subtitle: '商家工作台',
  sectionOrders: '订单',
  sectionStores: '门店与商品',
  sectionActivity: '活动与效果报告',
  sectionSubscription: '订阅',
  more: '更多功能',
  dashboard: '经营概览',
  stores: '门店管理',
  orders: '订单管理',
  finance: '财务对账',
  promotions: '优惠活动',
  promotionCalculator: '促销测算',
  groupBuy: '外部团购跳转入口',
  coupons: '券包 / 核销',
  ai: 'AI 经营分析',
  announcements: '平台公告',
  share: '裂变效果报告',
  subscription: '订阅状态 / 续费',
  paymentSettings: '收款设置',
  profile: '商家信息',
  lockTip: '平台已暂停商家业务权限，请查看订阅状态和恢复方式',
  title: '商家工作台',
  description: '查看商家信息与后续经营能力的统一入口',
  logout: '退出登录'
}

const router = useRouter()
const route = useRoute()
const authStore = useMerchantAuthStore()
const merchant = ref({})
const maintenanceNotice = ref(null)
let maintenanceNoticeTimer
const subscriptionState = computed(() => merchantSubscriptionAccessState(merchant.value))
const subscriptionExpired = computed(() => subscriptionState.value === 'expired')
const subscriptionPaused = computed(() => subscriptionState.value === 'paused')
const navigationLocked = computed(() => subscriptionPaused.value)
const routeViewKey = computed(() => route.fullPath)
const activeMenuPath = computed(() => getMerchantPcActivePath(route.path))
const maintenanceNoticeDescription = computed(() => {
  const notice = maintenanceNotice.value
  if (!notice) return ''
  return `维护窗口（北京时间）：${notice.window_start.replace('T', ' ')} 至 ${notice.window_end.replace('T', ' ')}。可能影响：${notice.expected_impact} 商家准备事项：${notice.preparation}`
})
const compactNavigation = ref(false)
const updateNavigationMode = () => {
  compactNavigation.value = window.matchMedia('(max-width: 768px)').matches
}

const refreshSubscriptionState = async () => {
  if (!authStore.token) {
    merchant.value = {}
    return
  }
  try {
    const res = await fetchMerchantInfo()
    merchant.value = res.data?.merchant || res.data || {}
  } catch {
    merchant.value = {}
  }
}

const refreshMaintenanceNotice = async () => {
  try {
    const res = await fetchMerchantMaintenanceNotice()
    maintenanceNotice.value = res.data?.notice || null
  } catch {
    // 不让公告请求故障影响商家日常操作；保留已展示的上一条公告。
  }
}

const logout = () => {
  authStore.logout()
  router.push('/merchant/login')
}

watch(() => route.fullPath, refreshSubscriptionState)
onMounted(() => {
  updateNavigationMode()
  window.addEventListener('resize', updateNavigationMode, { passive: true })
  refreshSubscriptionState()
  refreshMaintenanceNotice()
  maintenanceNoticeTimer = window.setInterval(refreshMaintenanceNotice, 5 * 60 * 1000)
})
onBeforeUnmount(() => {
  window.removeEventListener('resize', updateNavigationMode)
  window.clearInterval(maintenanceNoticeTimer)
})
</script>

<style scoped>
.merchant-layout {
  display: grid;
  grid-template-columns: 248px minmax(0, 1fr);
  min-height: 100vh;
  background:
    radial-gradient(circle at top left, rgba(37, 99, 235, 0.16), transparent 24%),
    radial-gradient(circle at bottom right, rgba(6, 182, 212, 0.12), transparent 26%),
    linear-gradient(135deg, #f8fbff 0%, #eef4fb 100%);
}

.merchant-sidebar {
  padding: 24px 16px;
  background:
    radial-gradient(circle at top, rgba(37, 99, 235, 0.28), transparent 30%),
    linear-gradient(180deg, #07182e 0%, #0f2747 100%);
  color: #fff;
  border-right: 1px solid rgba(148, 163, 184, 0.12);
}

.merchant-brand {
  padding: 6px 10px 16px;
}

.merchant-brand-title {
  font-size: 30px;
  font-weight: 900;
  letter-spacing: -0.04em;
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
  height: 48px;
  margin: 5px 0;
  border-radius: 14px;
  font-size: 15px;
  font-weight: 700;
}

.merchant-menu :deep(.el-sub-menu__title) {
  height: 48px;
  margin: 5px 0;
  border-radius: 14px;
  font-size: 15px;
  font-weight: 700;
}

.merchant-menu :deep(.el-menu-item:hover) {
  background: rgba(37, 99, 235, 0.18);
  color: #ffffff !important;
}

.merchant-menu :deep(.el-menu-item.is-active) {
  background: linear-gradient(90deg, #2563eb 0%, #06b6d4 100%);
  color: #ffffff !important;
  box-shadow: 0 14px 30px rgba(37, 99, 235, 0.3);
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

.pause-lock-screen {
  display: grid;
  justify-items: start;
  gap: 12px;
  padding: 28px;
  border: 1px solid #fecaca;
  border-radius: 20px;
  background: #fff7f7;
  color: #7f1d1d;
}

.pause-lock-screen h2,
.pause-lock-screen p {
  margin: 0;
}

.pause-lock-screen p {
  color: #7f1d1d;
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
  padding: 20px 24px 30px;
  display: grid;
  gap: 20px;
}

.merchant-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  padding: 20px 24px;
  border-radius: 26px;
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

@media (max-width: 768px) {
  .merchant-layout {
    display: flex;
    flex-direction: column;
  }

  .merchant-sidebar {
    position: sticky;
    top: 0;
    z-index: 20;
    padding: 8px 12px 6px;
  }

  .merchant-brand {
    padding: 0 6px 4px;
  }

  .merchant-brand-title {
    font-size: 16px;
    line-height: 1.3;
  }

  .merchant-brand-subtitle {
    display: none;
  }

  .merchant-menu {
    width: 100%;
    min-width: 0;
    overflow-x: auto;
    overflow-y: hidden;
    white-space: nowrap;
    scrollbar-width: none;
  }

  .merchant-menu::-webkit-scrollbar {
    display: none;
  }

  .merchant-menu.el-menu--horizontal {
    border-bottom: 0;
  }

  .merchant-menu :deep(.el-menu-item),
  .merchant-menu :deep(.el-sub-menu__title) {
    flex: 0 0 auto;
    height: 40px;
    margin: 0 4px 0 0;
    padding: 0 12px;
    border-radius: 12px;
    font-size: 13px;
    border-bottom: 0;
  }

  .merchant-menu :deep(.el-sub-menu) {
    flex: 0 0 auto;
  }

  .menu-section {
    display: none;
  }

  .subscription-lock {
    margin: 5px 4px 0;
    padding: 6px 8px;
    font-size: 11px;
    line-height: 1.35;
  }

  .merchant-main {
    min-width: 0;
    padding: 10px 12px 24px;
    gap: 12px;
  }

  .merchant-header {
    flex-direction: column;
    align-items: flex-start;
    padding: 14px;
    border-radius: 18px;
  }

  .merchant-title {
    font-size: 20px;
  }

  .merchant-subtitle {
    font-size: 12px;
    line-height: 1.4;
  }
}
</style>

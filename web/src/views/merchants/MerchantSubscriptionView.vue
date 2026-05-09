<template>
  <div class="subscription-page">
    <section class="subscription-hero">
      <div>
        <div class="eyebrow">SAAS SUBSCRIPTION</div>
        <h1>订阅付费</h1>
        <p>开通商家工作台后，可持续使用经营看板、扫码点单、优惠活动、订单管理和 AI 经营分析。</p>
      </div>
      <div class="hero-actions">
        <el-tag size="large" :type="isActive ? 'success' : 'danger'">
          {{ isActive ? '订阅有效' : '订阅未开通或已过期' }}
        </el-tag>
        <el-button v-if="isActive" type="success" @click="enterWorkspace">进入工作台</el-button>
      </div>
    </section>

    <el-alert v-if="notice" :title="notice" type="warning" show-icon :closable="false" />

    <section class="page-card current-card">
      <div class="card-toolbar">
        <div>
          <h2 class="page-title">当前订阅</h2>
          <p class="muted">这里展示当前商家服务状态，套餐内容由平台后台“平台套餐”统一维护。</p>
        </div>
      </div>
      <div class="current-grid">
        <div>
          <span>商家名称</span>
          <strong>{{ merchant.name || '-' }}</strong>
        </div>
        <div>
          <span>当前套餐</span>
          <strong>{{ currentPlanName }}</strong>
        </div>
        <div>
          <span>到期时间</span>
          <strong>{{ formatDateTime(merchant.subscription_expire_at) }}</strong>
        </div>
        <div>
          <span>商家状态</span>
          <strong>{{ statusLabel(merchant.status) }}</strong>
        </div>
      </div>
    </section>

    <section class="benefit-grid">
      <article class="benefit-card">
        <strong>经营看板</strong>
        <span>查看销售、订单、顾客和转化数据。</span>
      </article>
      <article class="benefit-card">
        <strong>扫码点单</strong>
        <span>顾客扫码浏览商品、下单和支付。</span>
      </article>
      <article class="benefit-card">
        <strong>AI 经营分析</strong>
        <span>识别热销商品、顾客标签和活动建议。</span>
      </article>
      <article class="benefit-card">
        <strong>营销活动</strong>
        <span>创建满减、折扣和 AI 生成活动草稿。</span>
      </article>
    </section>

    <section class="plan-grid">
      <article v-for="plan in plans" :key="plan.id" class="plan-card page-card">
        <div class="plan-top">
          <h3>{{ plan.name }}</h3>
          <el-tag>{{ plan.duration_days }} 天</el-tag>
        </div>
        <div class="price">¥{{ formatMoney(plan.price_cents) }}</div>
        <div class="pay-actions">
          <el-button type="primary" :loading="payingPlanId === plan.id" @click="subscribe(plan, 'page')">支付宝支付</el-button>
          <el-button :loading="payingPlanId === plan.id" @click="subscribe(plan, 'qr')">扫码支付</el-button>
        </div>
      </article>
    </section>

    <section v-if="payment" class="page-card payment-card">
      <h3>支付信息</h3>
      <p class="muted">请完成支付宝付款，系统会自动轮询订单状态。支付成功后会回到原页面。</p>
      <div class="payment-actions">
        <el-button v-if="payment.payment_url" type="primary" @click="openPayment">打开支付宝支付页</el-button>
        <el-input v-if="payment.qr_code" :model-value="payment.qr_code" readonly />
        <el-tag v-if="polling">{{ pollingText }}</el-tag>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  createMerchantSubscriptionOrder,
  fetchMerchantSubscription,
  fetchMerchantSubscriptionOrder
} from '../../api/modules'
import { useMerchantAuthStore } from '../../stores/merchantAuth'

const route = useRoute()
const router = useRouter()
const authStore = useMerchantAuthStore()
const merchant = ref({})
const plans = ref([])
const payment = ref(null)
const currentOrderNo = ref('')
const payingPlanId = ref(null)
const polling = ref(false)
const pollingText = ref('等待支付结果...')
const notice = ref(sessionStorage.getItem('merchant_subscription_message') || '')
let pollTimer = null
sessionStorage.removeItem('merchant_subscription_message')

const isActive = computed(() => merchant.value.subscription_expire_at && new Date(merchant.value.subscription_expire_at).getTime() > Date.now())
const currentPlanName = computed(() => {
  const found = plans.value.find((item) => item.id === merchant.value.subscription_plan_id)
  return found?.name || merchant.value.subscription_plan || '未开通'
})

const load = async () => {
  const res = await fetchMerchantSubscription()
  merchant.value = res.data.merchant || {}
  plans.value = res.data.plans || []
  authStore.merchant = merchant.value
  localStorage.setItem('merchant_current', JSON.stringify(merchant.value))
}

const enterWorkspace = () => {
  router.push(route.query.redirect || '/merchant/dashboard')
}

const subscribe = async (plan, payMode) => {
  payingPlanId.value = plan.id
  try {
    const res = await createMerchantSubscriptionOrder({
      plan_id: plan.id,
      payment_channel: 'alipay',
      pay_mode: payMode
    })
    payment.value = res.data.payment
    currentOrderNo.value = res.data.order.order_no
    ElMessage.success('订阅订单已创建，请完成支付')
    if (payment.value?.payment_url) {
      window.open(payment.value.payment_url, '_blank')
    }
    startPolling()
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '创建订阅订单失败')
  } finally {
    payingPlanId.value = null
  }
}

const startPolling = () => {
  stopPolling()
  polling.value = true
  pollTimer = window.setInterval(async () => {
    if (!currentOrderNo.value) return
    try {
      const res = await fetchMerchantSubscriptionOrder(currentOrderNo.value)
      if (res.data.paid && res.data.valid) {
        stopPolling()
        await load()
        ElMessage.success('订阅成功')
        router.push(route.query.redirect || '/merchant/dashboard')
      }
    } catch {
      pollingText.value = '正在等待支付回调...'
    }
  }, 3000)
}

const stopPolling = () => {
  if (pollTimer) {
    window.clearInterval(pollTimer)
    pollTimer = null
  }
  polling.value = false
}

const openPayment = () => {
  if (payment.value?.payment_url) {
    window.open(payment.value.payment_url, '_blank')
  }
}

const formatMoney = (value) => (Number(value || 0) / 100).toFixed(2)
const formatDateTime = (value) => value ? String(value).replace('T', ' ').slice(0, 19) : '未开通'
const statusLabel = (status) => ({ active: '正常', pending: '待审核', suspended: '已禁用' }[status] || status || '-')

onMounted(load)
onUnmounted(stopPolling)
</script>

<style scoped>
.subscription-page {
  display: grid;
  gap: 20px;
}

.subscription-hero {
  display: flex;
  justify-content: space-between;
  gap: 20px;
  align-items: flex-start;
  padding: 26px;
  border-radius: 24px;
  background:
    radial-gradient(circle at 84% 18%, rgba(34, 197, 94, 0.26), transparent 28%),
    linear-gradient(135deg, #052e16, #166534);
  color: #fff;
}

.subscription-hero h1 {
  margin: 8px 0;
  font-size: 32px;
}

.subscription-hero p {
  max-width: 720px;
  margin: 0;
  color: #dcfce7;
  line-height: 1.7;
}

.eyebrow {
  color: #86efac;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.16em;
}

.card-toolbar {
  display: flex;
  justify-content: space-between;
  gap: 16px;
}

.muted {
  color: #64748b;
  line-height: 1.7;
  margin: 0;
}

.current-card,
.payment-card {
  padding: 22px;
}

.current-grid,
.benefit-grid,
.plan-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 16px;
}

.current-grid {
  margin-top: 16px;
}

.current-grid div,
.benefit-card {
  padding: 16px;
  border-radius: 14px;
  border: 1px solid #e2e8f0;
  background: #f8fafc;
}

.current-grid span,
.benefit-card span {
  display: block;
  color: #64748b;
  font-size: 13px;
}

.current-grid strong,
.benefit-card strong {
  display: block;
  margin-top: 8px;
}

.plan-card {
  padding: 24px;
  border: 1px solid #dcfce7;
}

.plan-top {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: center;
}

.plan-card h3,
.payment-card h3 {
  margin: 0;
}

.price {
  margin: 18px 0 8px;
  color: #16a34a;
  font-size: 38px;
  font-weight: 900;
}

.hero-actions,
.pay-actions,
.payment-actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.pay-actions,
.payment-actions {
  margin-top: 18px;
}

.hero-actions {
  justify-content: flex-end;
  align-items: center;
}

@media (max-width: 720px) {
  .subscription-hero,
  .card-toolbar {
    flex-direction: column;
  }
}
</style>

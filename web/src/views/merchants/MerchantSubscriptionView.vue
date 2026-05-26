<template>
  <div class="subscription-page">
    <section class="subscription-hero">
      <div>
        <div class="eyebrow">SAAS SUBSCRIPTION</div>
        <h1>{{ isActive ? '订阅续费与升级' : '开通商家工作台' }}</h1>
        <p>用一套轻量工具跑通扫码点单、订单处理、财务对账、优惠活动和 AI 经营建议，先把日常经营闭环搭起来。</p>
      </div>
      <div class="hero-actions">
        <el-tag size="large" :type="isActive ? 'success' : 'danger'">
          {{ isActive ? '订阅有效' : '订阅未开通或已过期' }}
        </el-tag>
        <el-button v-if="isActive" type="success" @click="enterWorkspace">进入工作台</el-button>
      </div>
    </section>

    <el-alert v-if="notice" :title="notice" type="warning" show-icon :closable="false" />

    <section class="flow-strip">
      <article v-for="step in payFlowSteps" :key="step.title">
        <span>{{ step.label }}</span>
        <strong>{{ step.title }}</strong>
        <p>{{ step.text }}</p>
      </article>
    </section>

    <section class="value-grid">
      <article v-for="item in valueCards" :key="item.title" class="value-card">
        <span>{{ item.label }}</span>
        <strong>{{ item.title }}</strong>
        <p>{{ item.text }}</p>
      </article>
    </section>

    <section class="page-card current-card">
      <div class="card-toolbar">
        <div>
          <h2 class="page-title">当前订阅</h2>
          <p class="muted">这里展示当前商家服务状态，也可以使用平台发放的卡密直接兑换订阅时长。</p>
        </div>
      </div>
      <div class="current-grid">
        <div><span>商家名称</span><strong>{{ merchant.name || '-' }}</strong></div>
        <div><span>当前套餐</span><strong>{{ currentPlanName }}</strong></div>
        <div><span>服务状态</span><strong>{{ subscriptionState.title }}</strong><small>{{ subscriptionState.hint }}</small></div>
        <div><span>剩余时长</span><strong>{{ remainingDaysText }}</strong></div>
        <div><span>到期时间</span><strong>{{ formatDateTime(merchant.subscription_expire_at) }}</strong></div>
        <div><span>商家状态</span><strong>{{ statusLabel(merchant.status) }}</strong></div>
      </div>
    </section>

    <section class="page-card redeem-card">
      <div>
        <div class="eyebrow blue">REDEEM CODE</div>
        <h2 class="page-title">卡密兑换</h2>
        <p class="muted">输入平台发放的订阅卡密后，会按卡密绑定套餐天数自动延长商家订阅。</p>
      </div>
      <div class="redeem-form">
        <el-input v-model="cardCode" placeholder="请输入订阅卡密，例如 CARD-xxxx" clearable />
        <el-button type="primary" :loading="redeeming" @click="redeemCard">兑换订阅</el-button>
      </div>
    </section>

    <section class="plan-grid">
      <article v-for="plan in plans" :key="plan.id" class="plan-card page-card" :class="{ selected: selectedPlan?.id === plan.id }">
        <div class="plan-top">
          <h3>{{ displayPlanName(plan) }}</h3>
          <div class="plan-tags">
            <el-tag type="success">完整功能</el-tag>
            <el-tag>{{ planBadge(plan) }}</el-tag>
          </div>
        </div>
        <div class="price">¥{{ formatMoney(plan.price_cents) }}</div>
        <p class="plan-desc">{{ planPitch(plan) }}</p>
        <p class="plan-scope">当前付费版本均可使用完整商家工作台，差异主要是服务周期、付款频率和人工支持方式。</p>
        <ul class="feature-list">
          <li v-for="feature in planFeatures(plan)" :key="feature">{{ feature }}</li>
        </ul>
        <div class="pay-actions">
          <el-button type="primary" :loading="payingPlanId === plan.id" @click="subscribe(plan)">生成订阅付款单</el-button>
        </div>
      </article>
    </section>

    <section v-if="payment" class="page-card payment-card">
      <div class="payment-head">
        <div>
          <div class="eyebrow blue">PLATFORM QR PAYMENT</div>
          <h3>平台收款码订阅</h3>
          <p class="muted">订单 {{ currentOrderNo }} 已创建，请用平台收款码付款。平台确认到账后会自动开通或续期。</p>
        </div>
        <el-tag type="warning">{{ paymentStatusText }}</el-tag>
      </div>
      <div class="platform-pay-grid">
        <article v-for="item in platformQrItems" :key="item.key" class="platform-qr-card" :class="{ empty: !item.value }">
          <strong>{{ item.label }}</strong>
          <div class="platform-qr-box">
            <img v-if="item.value" :src="qrDisplayUrl(item.value)" :alt="item.label" />
            <span v-else>平台暂未配置{{ item.label }}</span>
          </div>
        </article>
        <article class="pay-summary">
          <span>选择版本</span>
          <strong class="summary-plan">{{ selectedPlan ? displayPlanName(selectedPlan) : '-' }}</strong>
          <span>应付金额</span>
          <strong>¥{{ formatMoney(selectedPlan?.price_cents) }}</strong>
          <p>{{ platformPayment.platform_subscription_note || '付款后请等待平台确认到账。' }}</p>
          <el-alert
            v-if="manualPaidMarked"
            type="success"
            show-icon
            :closable="false"
            title="已提交付款提醒，平台核对到账后会自动开通或续期。"
          />
          <el-button type="primary" :disabled="manualPaidMarked" :loading="markingPaid" @click="markManualPaid">
            {{ manualPaidMarked ? '已提交付款提醒' : '我已付款，等待平台确认' }}
          </el-button>
          <el-tag v-if="polling">{{ pollingText }}</el-tag>
        </article>
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
  fetchMerchantSubscriptionOrder,
  markMerchantSubscriptionOrderPaid,
  redeemMerchantSubscriptionCard
} from '../../api/modules'
import { useMerchantAuthStore } from '../../stores/merchantAuth'

const route = useRoute()
const router = useRouter()
const authStore = useMerchantAuthStore()
const merchant = ref({})
const plans = ref([])
const payment = ref(null)
const platformPayment = ref({})
const selectedPlan = ref(null)
const currentOrderNo = ref('')
const payingPlanId = ref(null)
const polling = ref(false)
const pollingText = ref('等待平台确认到账...')
const markingPaid = ref(false)
const manualPaidMarked = ref(false)
const notice = ref(sessionStorage.getItem('merchant_subscription_message') || '')
const cardCode = ref('')
const redeeming = ref(false)
let pollTimer = null
sessionStorage.removeItem('merchant_subscription_message')

const isActive = computed(() => merchant.value.subscription_expire_at && new Date(merchant.value.subscription_expire_at).getTime() > Date.now())
const currentPlanName = computed(() => {
  const found = plans.value.find((item) => item.id === merchant.value.subscription_plan_id)
  if (found) return displayPlanName(found)
  return merchant.value.subscription_plan ? displayPlanName({ name: merchant.value.subscription_plan }) : '未开通'
})
const remainingDays = computed(() => {
  if (!merchant.value.subscription_expire_at) return null
  const diff = new Date(merchant.value.subscription_expire_at).getTime() - Date.now()
  return Math.ceil(diff / 86400000)
})
const remainingDaysText = computed(() => {
  if (remainingDays.value === null) return '未开通'
  if (remainingDays.value < 0) return '已过期'
  if (remainingDays.value === 0) return '今天到期'
  return `${remainingDays.value} 天`
})
const subscriptionState = computed(() => {
  if (!merchant.value.subscription_expire_at) {
    return { title: '未开通', hint: '选择服务版本并完成付款后开通' }
  }
  if (remainingDays.value < 0) {
    return { title: '已过期', hint: '续费后可恢复商家工作台服务' }
  }
  if (remainingDays.value <= 7) {
    return { title: '即将到期', hint: '建议提前续费，避免影响使用' }
  }
  return { title: '服务中', hint: '当前订阅可正常使用' }
})
const paymentStatusText = computed(() => currentOrderNo.value ? '待平台确认到账' : '等待生成付款单')
const payFlowSteps = [
  { label: '01', title: '选择服务版本', text: '按月、按年或技术支持方式开通，当前付费版本均保留完整功能。' },
  { label: '02', title: '扫码付款', text: '使用平台支付宝或微信收款码付款，系统保留订阅付款单。' },
  { label: '03', title: '平台确认', text: '平台确认到账后，订阅订单自动变为已支付并开通或续期。' },
  { label: '04', title: '进入工作台', text: '商家可继续使用扫码下单、订单管理、财务对账和经营工具。' }
]
const valueCards = [
  { label: 'ORDER', title: '扫码下单与订单处理', text: '顾客扫码选商品，商家端集中处理待付款、待确认收款、接单和完成。' },
  { label: 'FINANCE', title: '经营看板与财务对账', text: '区分应收、待确认、实收、退款和净收入，每天收店前更容易核账。' },
  { label: 'GROWTH', title: '优惠活动与 AI 建议', text: '从商品、活动、复盘建议和营销文案入手，帮助小商家低成本试运营。' }
]
const platformQrItems = computed(() => [
  { key: 'alipay', label: '支付宝收款码', value: platformPayment.value.platform_alipay_qr_code || '' },
  { key: 'wechat', label: '微信收款码', value: platformPayment.value.platform_wechat_qr_code || '' }
])
const displayPlanName = (plan = {}) => {
  const duration = Number(plan.duration_days || 0)
  const name = String(plan.name || '')
  if (duration >= 365 || name.includes('年')) return '年付服务版'
  if (duration >= 30 || name.includes('月')) return '月付服务版'
  if (name.includes('技术')) return '技术支持版'
  return name || '技术支持版'
}

const load = async () => {
  const res = await fetchMerchantSubscription()
  merchant.value = res.data.merchant || {}
  plans.value = res.data.plans || []
  platformPayment.value = res.data.platform_payment || {}
  authStore.merchant = merchant.value
  localStorage.setItem('merchant_current', JSON.stringify(merchant.value))
}

const enterWorkspace = () => {
  router.push(route.query.redirect || '/merchant/dashboard')
}

const redeemCard = async () => {
  if (!cardCode.value.trim()) {
    ElMessage.warning('请输入卡密')
    return
  }
  redeeming.value = true
  try {
    const res = await redeemMerchantSubscriptionCard({ code: cardCode.value.trim() })
    merchant.value = res.data.merchant || merchant.value
    authStore.merchant = merchant.value
    localStorage.setItem('merchant_current', JSON.stringify(merchant.value))
    cardCode.value = ''
    ElMessage.success('卡密兑换成功，订阅已更新')
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '卡密兑换失败')
  } finally {
    redeeming.value = false
  }
}

const subscribe = async (plan) => {
  payingPlanId.value = plan.id
  try {
    selectedPlan.value = plan
    const res = await createMerchantSubscriptionOrder({
      plan_id: plan.id,
      payment_channel: 'platform_qr',
      pay_mode: 'qr'
    })
    payment.value = res.data.payment || { mode: 'platform_qr' }
    currentOrderNo.value = res.data.order.order_no
    manualPaidMarked.value = false
    ElMessage.success('订阅订单已创建，请扫码付款')
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
      if (res.data.marked_paid) {
        manualPaidMarked.value = true
        pollingText.value = '已提交付款提醒，等待平台核对到账...'
      }
      if (res.data.paid && res.data.valid) {
        stopPolling()
        await load()
        ElMessage.success('订阅成功')
        router.push(route.query.redirect || '/merchant/dashboard')
      }
    } catch {
      pollingText.value = '正在等待平台确认到账...'
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

const markManualPaid = async () => {
  if (!currentOrderNo.value) {
    ElMessage.warning('请先生成订阅付款单')
    return
  }
  markingPaid.value = true
  try {
    await markMerchantSubscriptionOrderPaid(currentOrderNo.value)
    manualPaidMarked.value = true
    pollingText.value = '已提交付款提醒，等待平台核对到账...'
    ElMessage.success('已提交付款提醒，请等待平台确认到账后开通订阅')
    startPolling()
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '提交付款提醒失败')
  } finally {
    markingPaid.value = false
  }
}

const formatMoney = (value) => (Number(value || 0) / 100).toFixed(2)
const formatDateTime = (value) => value ? String(value).replace('T', ' ').slice(0, 19) : '未开通'
const statusLabel = (status) => ({ active: '正常', pending: '待审核', suspended: '已禁用' }[status] || status || '-')
const planBadge = (plan) => Number(plan.duration_days || 0) > 0 ? `${plan.duration_days} 天` : '人工服务'
const planPitch = (plan) => Number(plan.duration_days || 0) >= 365
  ? '适合稳定经营门店，按年开通完整商家工作台服务，减少续费打扰。'
  : '适合先按月试用，完整体验扫码点单、订单管理和经营对账。'
const planFeatures = (plan) => {
  const base = ['扫码点单', '订单管理', '财务对账', '商品管理', '优惠活动', 'AI 经营建议']
  if (Number(plan.duration_days || 0) >= 365) return [...base, '年度续费更省心']
  return [...base, '按月灵活续费']
}
const buildQrImage = (value) => `https://api.qrserver.com/v1/create-qr-code/?size=260x260&data=${encodeURIComponent(value)}`
const qrDisplayUrl = (value) => {
  const text = String(value || '').trim()
  if (!text) return ''
  if (text.startsWith('data:image/')) return text
  if (/\.(png|jpe?g|webp|gif|svg)(\?.*)?$/i.test(text)) return text
  return buildQrImage(text)
}

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

.eyebrow.blue {
  color: #2563eb;
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

.flow-strip {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.flow-strip article {
  min-height: 118px;
  padding: 16px;
  border: 1px solid #dbeafe;
  border-radius: 16px;
  background: #ffffff;
}

.flow-strip span,
.flow-strip strong,
.flow-strip p {
  display: block;
}

.flow-strip span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
}

.flow-strip strong {
  margin-top: 8px;
  color: #0f2747;
}

.flow-strip p {
  margin: 8px 0 0;
  color: #64748b;
  line-height: 1.55;
}

.current-card,
.payment-card,
.redeem-card {
  padding: 22px;
}

.redeem-card {
  display: grid;
  grid-template-columns: 1.2fr 1fr;
  gap: 18px;
  align-items: end;
  border: 1px solid #bfdbfe;
  background:
    radial-gradient(circle at top right, rgba(59, 130, 246, 0.12), transparent 28%),
    #ffffff;
}

.redeem-form {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 10px;
}

.value-grid,
.current-grid,
.plan-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 16px;
}

.value-card {
  min-height: 128px;
  padding: 18px;
  border: 1px solid #dbeafe;
  border-radius: 18px;
  background: linear-gradient(135deg, #eff6ff, #ffffff);
}

.value-card span,
.value-card strong,
.value-card p {
  display: block;
}

.value-card span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.08em;
}

.value-card strong {
  margin-top: 8px;
  color: #0f2747;
  font-size: 18px;
}

.value-card p {
  margin: 8px 0 0;
  color: #64748b;
  line-height: 1.65;
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

.current-grid small {
  display: block;
  margin-top: 6px;
  color: #64748b;
  line-height: 1.5;
}

.plan-card {
  padding: 24px;
  border: 1px solid #dcfce7;
}

.plan-card.selected {
  border-color: #2563eb;
  box-shadow: 0 16px 36px rgba(37, 99, 235, 0.12);
}

.plan-top {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: center;
}

.plan-tags {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;
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

.plan-desc {
  margin: 0 0 12px;
  color: #64748b;
  line-height: 1.6;
}

.plan-scope {
  margin: 0;
  padding: 10px 12px;
  border-radius: 12px;
  background: #f0fdf4;
  color: #166534;
  line-height: 1.55;
}

.feature-list {
  display: grid;
  gap: 8px;
  min-height: 132px;
  padding: 0;
  margin: 14px 0;
  list-style: none;
}

.feature-list li {
  color: #334155;
}

.feature-list li::before {
  content: "✓";
  margin-right: 8px;
  color: #16a34a;
  font-weight: 900;
}

.payment-head {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: flex-start;
}

.platform-pay-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14px;
  margin-top: 16px;
}

.platform-qr-card,
.pay-summary {
  min-height: 230px;
  padding: 16px;
  border: 1px solid #dbeafe;
  border-radius: 18px;
  background: #f8fbff;
}

.platform-qr-card.empty {
  border-color: #e5e7eb;
  background: #f8fafc;
}

.platform-qr-box {
  display: grid;
  place-items: center;
  min-height: 180px;
  margin-top: 12px;
  border: 1px dashed #bfdbfe;
  border-radius: 14px;
  background: #ffffff;
  color: #94a3b8;
}

.platform-qr-box img {
  width: 168px;
  height: 168px;
  object-fit: contain;
}

.pay-summary span,
.pay-summary strong,
.pay-summary p {
  display: block;
}

.pay-summary span {
  color: #64748b;
  font-weight: 800;
}

.pay-summary strong {
  margin-top: 8px;
  color: #16a34a;
  font-size: 34px;
}

.pay-summary .summary-plan {
  margin: 8px 0 14px;
  color: #0f2747;
  font-size: 20px;
}

.pay-summary p {
  color: #64748b;
  line-height: 1.6;
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
  .card-toolbar,
  .payment-head {
    flex-direction: column;
  }

  .flow-strip,
  .redeem-card,
  .redeem-form,
  .platform-pay-grid {
    grid-template-columns: 1fr;
  }
}
</style>

<template>
  <div class="subscription-page">
    <van-nav-bar title="订阅与业务状态" left-arrow fixed placeholder @click-left="router.back()" />

    <main class="page-content">
      <section class="hero-card" :class="`state-${state}`">
        <span class="eyebrow">商家服务状态</span>
        <h1>{{ stateTitle }}</h1>
        <p>{{ stateDescription }}</p>
        <div v-if="merchant.subscription_expire_at || merchant.subscription_expired_at" class="expiry-line">
          当前到期时间：{{ formatDate(merchant.subscription_expire_at || merchant.subscription_expired_at) }}
        </div>
      </section>

      <section v-if="loading" class="state-card" aria-live="polite">
        <van-loading type="spinner" color="#168f8c">正在核对订阅状态…</van-loading>
      </section>

      <section v-else-if="state === 'error'" class="state-card notice-card" role="alert">
        <van-icon name="warning-o" />
        <h2>暂时无法确认订阅状态</h2>
        <p>{{ errorMessage }}</p>
        <van-button type="primary" round block :loading="loading" @click="loadStatus">重新检查状态</van-button>
      </section>

      <section v-else-if="state === 'unknown'" class="state-card notice-card" role="status">
        <van-icon name="warning-o" />
        <h2>当前状态尚无法判定</h2>
        <p>系统未返回可确认的订阅状态。为保护经营数据，本页暂不开放业务读取或写入；请重新检查，或联系平台支持核对。</p>
        <van-button type="primary" round block @click="loadStatus">重新检查状态</van-button>
      </section>

      <template v-else>
        <section v-if="merchant.status === 'pending'" class="state-card pending-note" role="status">
          <div class="pending-head">
            <van-icon name="info-o" />
            <div>
              <strong>{{ isRejected ? '审核未通过' : '入驻审核中' }}</strong>
              <p v-if="isRejected">拒绝原因：{{ merchant.review_reason }}。修改商家资料后可重新提交审核。</p>
              <p v-else>账号正在审核中，通过后可开通订阅并营业；审核通过前无法生成订阅付款单或兑换卡密。</p>
            </div>
          </div>
          <van-button v-if="isRejected" type="primary" round block :loading="resubmitting" @click="resubmitOnboarding">重新提交审核</van-button>
          <van-button plain round block :loading="loading" @click="loadStatus">重新检查状态</van-button>
        </section>

        <section v-if="state === 'expired'" class="state-card expired-note" role="status">
          <van-icon name="info-o" />
          <div>
            <strong>到期后仅保留只读查看</strong>
            <p>已有订单、门店、商品与团购数据可按系统允许范围查看；不能处理订单、增改门店或商品，也不能修改团购配置。旧的已审核团购入口按现有规则继续有效。</p>
          </div>
        </section>

        <section v-if="state === 'paused'" class="state-card paused-note" role="alert">
          <van-icon name="pause-circle-o" />
          <div>
            <strong>{{ merchant.status === 'suspended' ? '商家业务已暂停' : '订阅已被平台暂停' }}</strong>
            <p>{{ merchant.subscription_pause_reason || (merchant.status === 'suspended' ? '商家账号当前处于平台暂停状态。' : '平台未提供具体暂停原因。') }} 暂停期间不开放经营数据读取或业务操作；请通过现有平台支持渠道核实并处理。本页没有自助申诉或解封入口，续费也不会自动解除暂停。</p>
          </div>
        </section>

        <section v-if="state === 'active'" class="state-card active-note" role="status">
          <van-icon name="checked" />
          <div>
            <strong>订阅有效</strong>
            <p>商家工作台当前可用。你也可以在到期前续费；开通状态以服务端订阅记录为准。</p>
          </div>
        </section>

        <section v-if="state === 'paused'" class="state-card recovery-card">
          <h2>恢复路径</h2>
          <p>暂停原因以服务端返回内容为准。请联系平台支持/运营核对；平台解除暂停后，重新检查本页状态。此页不会解除暂停，也不会开放业务写入。</p>
          <van-button plain round block :loading="loading" @click="loadStatus">重新检查状态</van-button>
        </section>

        <section v-if="actions.canRenew" class="plan-section" id="renewal-plans">
          <header class="section-heading">
            <div>
              <span class="eyebrow dark">续费与付款</span>
              <h2>选择服务版本</h2>
            </div>
            <van-button size="small" plain round :loading="loading" @click="loadStatus">刷新状态</van-button>
          </header>
          <p class="section-intro">通过现有订阅接口创建付款单；请按页面展示的收款码付款。提交付款提醒不等于平台已确认到账，只有服务端订阅状态更新后才会恢复服务。</p>

          <div v-if="plans.length" class="plan-list">
            <article v-for="plan in plans" :key="plan.id" class="plan-card">
              <div>
                <h3>{{ displayPlanName(plan) }}</h3>
                <span>{{ Number(plan.duration_days || 0) }} 天</span>
              </div>
              <strong class="plan-price">¥{{ formatMoney(plan.price_cents) }}</strong>
              <van-button type="primary" round block :loading="payingPlanId === plan.id" @click="createOrder(plan)">
                生成订阅付款单
              </van-button>
            </article>
          </div>
          <div v-else class="state-card"><p>暂未获取到可用套餐，请重新检查订阅状态或联系平台支持。</p></div>
        </section>
      </template>

      <section v-if="paymentOrder" class="state-card payment-card" aria-live="polite">
        <div class="payment-heading">
          <div>
            <span class="eyebrow dark">付款单 {{ paymentOrder.orderNo }}</span>
            <h2>平台收款码付款</h2>
          </div>
          <van-tag type="warning">{{ paymentOrder.markedPaid ? '等待平台确认' : '待付款/确认' }}</van-tag>
        </div>
        <p>付款单已创建。平台核对到账并更新服务端订阅状态前，当前状态不会改变。</p>
        <div class="qr-grid">
          <div v-for="item in qrItems" :key="item.key" class="qr-item">
            <strong>{{ item.label }}</strong>
            <img v-if="qrImage(item.value)" :src="qrImage(item.value)" :alt="item.label" />
            <span v-else>平台暂未配置此收款码</span>
          </div>
        </div>
        <van-button v-if="!paymentOrder.markedPaid" type="primary" plain round block :loading="markingPaid" @click="markPaid">
          我已付款，提交平台核对
        </van-button>
        <van-button plain round block :loading="checkingOrder" @click="checkOrderStatus">重新检查付款与订阅状态</van-button>
        <p class="payment-footnote">付款提醒只表示已提交核对请求，不是到账凭证，也不会自行开通订阅。</p>
      </section>

      <section v-if="actions.canRenew" class="redeem-card">
        <div>
          <span class="eyebrow dark">卡密兑换</span>
          <h2>使用订阅卡密</h2>
          <p>输入平台发放的订阅卡密。兑换结果仍以服务端返回的订阅状态为准。</p>
        </div>
        <van-field v-model="cardCode" label="卡密" placeholder="请输入订阅卡密" clearable />
        <van-button type="primary" plain round block :disabled="!cardCode.trim()" :loading="redeeming" @click="redeemCard">兑换并重新核对状态</van-button>
      </section>

      <van-button v-if="state === 'active'" class="workspace-button" type="success" round block @click="enterWorkspace">进入移动工作台</van-button>
    </main>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { showFailToast, showSuccessToast } from 'vant'
import QRCode from 'qrcode'
import {
  createMerchantSubscriptionOrder,
  fetchMerchantSubscription,
  fetchMerchantSubscriptionOrder,
  markMerchantSubscriptionOrderPaid,
  redeemMerchantSubscriptionCard,
  resubmitMerchantOnboarding
} from '@/api/modules'
import { merchantFromSubscriptionPayload, mobileMerchantSubscriptionState, mobileSubscriptionActions } from '@/utils/mobileMerchantSubscription'
import { isMerchantOnboardingRejected } from '@/utils/merchantOnboarding'

const router = useRouter()
const route = useRoute()
const loading = ref(true)
const state = ref('loading')
const merchant = ref({})
const plans = ref([])
const platformPayment = ref({})
const errorMessage = ref('请稍后重试。')
const paymentOrder = ref(null)
const payingPlanId = ref(null)
const markingPaid = ref(false)
const checkingOrder = ref(false)
const redeeming = ref(false)
const resubmitting = ref(false)
const cardCode = ref('')
let pollTimer = null
const actions = computed(() => mobileSubscriptionActions(state.value))
const isRejected = computed(() => isMerchantOnboardingRejected(merchant.value))
const stateTitle = computed(() => ({
  loading: '正在核对状态', active: '服务正常', expired: '订阅已到期', paused: '业务访问已锁定', pending: '入驻审核中', unknown: '订阅状态待确认', error: '暂时无法确认状态'
}[state.value] || '订阅状态待确认'))
const stateDescription = computed(() => ({
  loading: '正在向服务端读取最新订阅记录。',
  active: '服务端订阅记录显示当前有效。',
  expired: '订阅已到期；工作台只允许读取系统明确开放的旧数据。',
  paused: '平台暂停期间，经营数据读取和业务操作均被锁定。',
  unknown: '无法从服务端确认订阅状态，经营功能将保持关闭。',
  pending: '账号正在审核中，通过后可开通订阅并营业。',
  error: '订阅服务暂时不可用，经营功能将保持关闭。'
}[state.value] || '经营功能将保持关闭。'))
const qrItems = computed(() => [
  { key: 'alipay', label: '支付宝收款码', value: platformPayment.value.platform_alipay_qr_code || '' },
  { key: 'wechat', label: '微信收款码', value: platformPayment.value.platform_wechat_qr_code || '' }
])

const saveMerchantSnapshot = (value) => {
  merchant.value = value || {}
  state.value = mobileMerchantSubscriptionState({ merchant: merchant.value })
  // A pending merchant has no active subscription, so the subscription
  // state alone would read as unknown; keep the onboarding state explicit.
  if (merchant.value.status === 'pending') state.value = 'pending'
  if (!mobileSubscriptionActions(state.value).canRenew) {
    paymentOrder.value = null
    stopPolling()
  }
  if (state.value === 'unknown') {
    localStorage.removeItem('merchant_current')
    return
  }
  localStorage.setItem('merchant_current', JSON.stringify(merchant.value))
}

const loadStatus = async () => {
  loading.value = true
  state.value = 'loading'
  errorMessage.value = '请稍后重试。'
  try {
    const response = await fetchMerchantSubscription()
    const payload = response?.data
    const currentMerchant = merchantFromSubscriptionPayload(payload)
    plans.value = Array.isArray(payload?.plans) ? payload.plans : []
    platformPayment.value = payload?.platform_payment && typeof payload.platform_payment === 'object' ? payload.platform_payment : {}
    if (!currentMerchant) {
      saveMerchantSnapshot(null)
      state.value = 'unknown'
      errorMessage.value = '接口未返回订阅商家信息。'
      return
    }
    saveMerchantSnapshot(currentMerchant)
  } catch (error) {
    merchant.value = {}
    state.value = 'error'
    paymentOrder.value = null
    stopPolling()
    errorMessage.value = error.response?.data?.message || '订阅状态读取失败，请稍后重试。'
    localStorage.removeItem('merchant_current')
  } finally {
    loading.value = false
  }
}

const resubmitOnboarding = async () => {
  if (resubmitting.value) return
  resubmitting.value = true
  try {
    await resubmitMerchantOnboarding()
    showSuccessToast('已重新提交审核，请耐心等待')
    await loadStatus()
  } catch (error) {
    showFailToast(error.response?.data?.message || '重新提交失败')
  } finally {
    resubmitting.value = false
  }
}

const displayPlanName = (plan = {}) => {
  const name = String(plan.name || '')
  const days = Number(plan.duration_days || 0)
  if (days >= 365 || name.includes('年')) return '年付服务版'
  if (days >= 30 || name.includes('月')) return '月付服务版'
  return name || '商家服务版'
}
const formatMoney = (cents) => (Number(cents || 0) / 100).toFixed(2)
const formatDate = (value) => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '未设置'
const buildQR = (value) => QRCode.toDataURL(String(value), { width: 220, margin: 2, errorCorrectionLevel: 'M' })
const qrImageUrl = (value) => {
  if (!value) return ''
  const text = String(value).trim()
  if (text.startsWith('data:image/')) return text
  if (/\.(png|jpe?g|webp|gif|svg)(\?.*)?$/i.test(text)) return text
  return ''
}

const createOrder = async (plan) => {
  if (!actions.value.canRenew) return
  payingPlanId.value = plan.id
  try {
    const response = await createMerchantSubscriptionOrder({ plan_id: plan.id, payment_channel: 'platform_qr', pay_mode: 'qr' })
    const orderNo = response?.data?.order?.order_no
    if (!orderNo) throw new Error('订阅接口未返回付款单号')
    const qrPayloads = [platformPayment.value.platform_alipay_qr_code, platformPayment.value.platform_wechat_qr_code].filter(Boolean)
    const qrData = {}
    for (const value of qrPayloads) {
      if (!qrImageUrl(value)) qrData[value] = await buildQR(value)
    }
    paymentOrder.value = {
      orderNo,
      planName: displayPlanName(plan),
      markedPaid: false,
      qrData
    }
    startPolling()
  } catch (error) {
    showFailToast(error.response?.data?.message || error.message || '创建订阅付款单失败')
  } finally {
    payingPlanId.value = null
  }
}

const qrImage = (value) => paymentOrder.value?.qrData?.[value] || qrImageUrl(value)
const markPaid = async () => {
  if (!paymentOrder.value?.orderNo) return
  markingPaid.value = true
  try {
    await markMerchantSubscriptionOrderPaid(paymentOrder.value.orderNo)
    paymentOrder.value.markedPaid = true
    showSuccessToast('已提交付款提醒；等待平台核对到账')
    await checkOrderStatus()
  } catch (error) {
    showFailToast(error.response?.data?.message || '提交付款提醒失败')
  } finally {
    markingPaid.value = false
  }
}

const checkOrderStatus = async () => {
  if (!paymentOrder.value?.orderNo || checkingOrder.value) return
  checkingOrder.value = true
  try {
    const response = await fetchMerchantSubscriptionOrder(paymentOrder.value.orderNo)
    const order = response?.data || {}
    if (order.marked_paid) paymentOrder.value.markedPaid = true
    if (order.paid && order.valid) {
      await loadStatus()
      if (state.value === 'active') {
        stopPolling()
        showSuccessToast('服务端已确认订阅有效')
      } else {
        showFailToast('付款状态已更新，但订阅状态尚未变更；请稍后重新检查')
      }
    }
  } catch {
    // Keep the payment status pending; the merchant can retry without assuming success.
  } finally {
    checkingOrder.value = false
  }
}
const startPolling = () => {
  stopPolling()
  pollTimer = window.setInterval(checkOrderStatus, 5000)
}
const stopPolling = () => {
  if (pollTimer) window.clearInterval(pollTimer)
  pollTimer = null
}

const redeemCard = async () => {
  if (!cardCode.value.trim() || !actions.value.canRenew) return
  redeeming.value = true
  try {
    const response = await redeemMerchantSubscriptionCard({ code: cardCode.value.trim() })
    const redeemedMerchant = response?.data?.merchant
    if (redeemedMerchant) saveMerchantSnapshot(redeemedMerchant)
    cardCode.value = ''
    await loadStatus()
    if (state.value === 'active') showSuccessToast('服务端订阅状态已更新')
    else showSuccessToast('卡密兑换请求已处理，请以当前服务端订阅状态为准')
  } catch (error) {
    showFailToast(error.response?.data?.message || '卡密兑换失败')
  } finally {
    redeeming.value = false
  }
}
const enterWorkspace = () => router.push(route.query.redirect || '/m/merchant/home')

onMounted(loadStatus)
onUnmounted(stopPolling)
</script>

<style scoped>
.subscription-page { min-height: 100vh; color: #18313c; background: #f4f7f8; }
.page-content { display: grid; gap: 14px; max-width: 580px; margin: 0 auto; padding: 14px 14px calc(92px + env(safe-area-inset-bottom)); }
.hero-card { padding: 22px 20px; border-radius: 20px; color: #fff; background: linear-gradient(140deg, #102e43, #145966 58%, #168c88); box-shadow: 0 12px 26px rgba(19, 70, 82, .14); }
.hero-card.state-expired { background: linear-gradient(140deg, #614315, #8d6828); }
.hero-card.state-paused, .hero-card.state-error, .hero-card.state-unknown { background: linear-gradient(140deg, #542d34, #93454a); }
.eyebrow { color: #b8e7df; font-size: 11px; font-weight: 800; letter-spacing: .12em; }
.eyebrow.dark { color: #32716b; }
.hero-card h1 { margin: 9px 0 7px; font-size: 24px; }
.hero-card p { margin: 0; color: rgba(255,255,255,.86); font-size: 13px; line-height: 1.65; }
.expiry-line { margin-top: 12px; padding-top: 10px; border-top: 1px solid rgba(255,255,255,.18); font-size: 12px; }
.state-card, .plan-card, .redeem-card { padding: 16px; border: 1px solid #e2ebec; border-radius: 17px; background: #fff; box-shadow: 0 5px 18px rgba(26,55,67,.035); }
.state-card { display: grid; gap: 10px; }
.state-card h2, .redeem-card h2 { margin: 0; font-size: 17px; }
.state-card p, .redeem-card p, .section-intro { margin: 0; color: #6d8188; font-size: 12px; line-height: 1.65; }
.notice-card { justify-items: start; }
.notice-card > .van-icon { color: #b77735; font-size: 25px; }
.expired-note, .paused-note, .active-note { display: flex; align-items: flex-start; gap: 10px; }
.pending-note { color: #826021; border-color: #ead8b0; background: #fff9ec; }
.pending-head { display: flex; align-items: flex-start; gap: 10px; }
.pending-head > .van-icon { flex: 0 0 auto; margin-top: 1px; font-size: 20px; color: #b77735; }
.pending-head strong { display: block; font-size: 14px; }
.pending-head p { margin-top: 5px; color: #6d797b; font-size: 12px; line-height: 1.65; }
.expired-note { color: #826021; border-color: #ead8b0; background: #fff9ec; }
.paused-note { color: #8d4146; border-color: #efcfd0; background: #fff4f4; }
.active-note { color: #24756a; border-color: #cce8df; background: #f0faf6; }
.expired-note > .van-icon, .paused-note > .van-icon, .active-note > .van-icon { flex: 0 0 auto; margin-top: 1px; font-size: 20px; }
.expired-note strong, .paused-note strong, .active-note strong { display: block; font-size: 14px; }
.expired-note p, .paused-note p, .active-note p { margin-top: 5px; color: #6d797b; font-size: 12px; line-height: 1.65; }
.section-heading { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.section-heading h2 { margin: 5px 0 0; font-size: 19px; }
.plan-section { display: grid; gap: 12px; }
.plan-list { display: grid; gap: 10px; }
.plan-card { display: grid; grid-template-columns: 1fr auto; align-items: center; gap: 12px; }
.plan-card h3 { margin: 0; font-size: 16px; }
.plan-card span { display: block; margin-top: 4px; color: #829399; font-size: 11px; }
.plan-price { color: #15776f; font-size: 21px; }
.plan-card .van-button { grid-column: 1 / -1; }
.payment-card { border-color: #bbded7; }
.payment-heading { display: flex; justify-content: space-between; gap: 10px; }
.payment-heading h2 { margin: 5px 0 0; font-size: 18px; }
.qr-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; }
.qr-item { display: grid; justify-items: center; gap: 8px; min-height: 150px; padding: 12px 8px; border: 1px solid #e4ecee; border-radius: 13px; color: #3b6266; font-size: 12px; text-align: center; }
.qr-item img { width: 118px; height: 118px; object-fit: contain; }
.qr-item span { align-self: center; color: #95a3a7; line-height: 1.5; }
.payment-footnote { color: #8b6a37 !important; }
.redeem-card { display: grid; gap: 10px; }
.redeem-card .van-field { padding: 8px 0; }
.workspace-button { margin-top: 2px; }
@media (max-width: 360px) { .qr-grid { grid-template-columns: 1fr; } }
</style>

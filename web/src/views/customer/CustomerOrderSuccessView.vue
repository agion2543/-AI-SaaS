<template>
  <main class="order-page">
    <section class="phone-shell">
      <header class="status-hero" :class="heroClass">
        <div class="hero-top">
          <span>ORDER STATUS</span>
          <el-button text class="refresh-btn" :loading="loading" @click="load">刷新</el-button>
        </div>
        <div class="status-icon">{{ statusIcon }}</div>
        <h1>{{ statusTitle }}</h1>
        <p>{{ statusTip }}</p>
      </header>

      <section v-if="loading && !order.order_no" class="panel">
        <el-skeleton :rows="8" animated />
      </section>

      <template v-else>
        <section class="panel summary-panel">
          <div class="summary-line">
            <span>订单号</span>
            <strong>{{ order.order_no || '-' }}</strong>
          </div>
          <div class="summary-grid">
            <div>
              <span>门店</span>
              <strong>{{ order.store?.name || '-' }}</strong>
            </div>
            <div>
              <span>实付金额</span>
              <strong>{{ yuan(order.total_amount || order.amount) }}</strong>
            </div>
            <div>
              <span>优惠金额</span>
              <strong>-{{ yuan(order.discount_amount) }}</strong>
            </div>
            <div>
              <span>已退金额</span>
              <strong>{{ yuan(order.refunded_amount) }}</strong>
            </div>
          </div>

          <div v-if="canRetry" class="retry-box">
            <el-radio-group v-model="payMode" class="pay-mode">
              <el-radio-button label="page">跳转支付</el-radio-button>
              <el-radio-button label="qr">扫码支付</el-radio-button>
            </el-radio-group>
            <el-button type="primary" size="large" :loading="retrying" @click="retryPay">
              {{ status === 'failed' ? '重新支付' : '继续支付' }}
            </el-button>
          </div>
        </section>

        <section class="panel">
          <div class="section-title">订单进度</div>
          <div class="timeline">
            <div v-for="step in timeline" :key="step.key" :class="['step', { active: step.active, done: step.done }]">
              <div class="dot"></div>
              <div>
                <strong>{{ step.title }}</strong>
                <p>{{ step.text }}</p>
                <span>{{ step.time }}</span>
              </div>
            </div>
          </div>
        </section>

        <section class="panel">
          <div class="section-title">商品明细</div>
          <div v-if="items.length" class="items">
            <div v-for="item in items" :key="`${item.product_id || item.name}-${item.quantity}`" class="item-row">
              <div>
                <strong>{{ item.name }}</strong>
                <span>{{ yuan(item.price) }} x {{ item.quantity }}</span>
              </div>
              <b>{{ yuan(item.line_amount || item.price * item.quantity) }}</b>
            </div>
          </div>
          <el-empty v-else description="暂无商品明细" />
        </section>

        <section class="panel two-cols">
          <div>
            <span>顾客备注</span>
            <p>{{ order.customer_note || '暂无备注' }}</p>
          </div>
          <div>
            <span>门店联系</span>
            <p>{{ contactPhone || '暂无联系方式' }}</p>
          </div>
        </section>

        <div class="bottom-actions">
          <el-button type="primary" size="large" @click="backStore">返回门店</el-button>
          <el-button size="large" @click="callStore" :disabled="!contactPhone">联系门店</el-button>
        </div>
      </template>
    </section>

    <el-dialog v-model="qrDialogVisible" title="支付宝扫码支付" width="92%" class="qr-dialog">
      <div class="qr-wrap">
        <img :src="qrImageUrl" alt="支付宝扫码支付" />
        <p>请使用支付宝扫码完成支付，支付成功后本页会自动刷新订单状态。</p>
      </div>
    </el-dialog>
  </main>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { fetchCustomerOrder, retryCustomerOrderPayment } from '../../api/modules'

const route = useRoute()
const router = useRouter()
const loading = ref(false)
const retrying = ref(false)
const order = ref({})
const payMode = ref('page')
const qrDialogVisible = ref(false)
const qrImageUrl = ref('')
let pollTimer = null

const orderNo = computed(() => route.params.orderNo || route.query.orderNo || '')
const status = computed(() => order.value.status || 'pending')
const canRetry = computed(() => ['pending', 'failed'].includes(status.value))
const contactPhone = computed(() => order.value.store?.contact_phone || order.value.merchant?.contact_phone || '')
const items = computed(() => Array.isArray(order.value.items) ? order.value.items : [])
const isRefunded = computed(() => Number(order.value.refunded_amount || 0) > 0)

const statusTextMap = {
  pending: {
    title: '等待支付',
    tip: '订单已创建，请在有效时间内完成支付。',
    icon: '¥'
  },
  failed: {
    title: '支付未完成',
    tip: '支付失败或已中断，可以重新发起支付。',
    icon: '!'
  },
  received: {
    title: '下单成功，等待接单',
    tip: '商家已收到订单，会尽快确认接单。',
    icon: '✓'
  },
  accepted: {
    title: '商家已接单',
    tip: '订单正在处理，请留意门店通知。',
    icon: '店'
  },
  completed: {
    title: '订单已完成',
    tip: '感谢下单，欢迎再次光临。',
    icon: '好'
  },
  closed: {
    title: '订单已关闭',
    tip: '订单已关闭，如有疑问请联系门店。',
    icon: '×'
  }
}

const statusTitle = computed(() => {
  if (isRefunded.value && order.value.refund_status === 'full') return '订单已退款'
  return statusTextMap[status.value]?.title || statusTextMap.received.title
})

const statusTip = computed(() => {
  if (isRefunded.value && order.value.refund_status === 'full') {
    return '退款已记录，请以实际支付渠道到账为准。'
  }
  return statusTextMap[status.value]?.tip || statusTextMap.received.tip
})

const statusIcon = computed(() => statusTextMap[status.value]?.icon || '✓')

const heroClass = computed(() => ({
  waiting: status.value === 'pending',
  failed: status.value === 'failed' || status.value === 'closed',
  done: status.value === 'completed',
  refunded: isRefunded.value
}))

const timeline = computed(() => {
  const rank = { pending: 1, failed: 1, received: 2, accepted: 3, completed: 4, closed: 4 }
  const current = rank[status.value] || 1
  return [
    {
      key: 'created',
      title: '订单创建',
      text: '顾客已提交订单。',
      time: formatTime(order.value.created_at),
      done: current >= 1,
      active: current === 1
    },
    {
      key: 'paid',
      title: '支付完成',
      text: '支付成功后订单进入待接单。',
      time: order.value.paid_at ? formatTime(order.value.paid_at) : '-',
      done: current >= 2,
      active: current === 2
    },
    {
      key: 'accepted',
      title: '商家接单',
      text: '商家确认接单并开始处理。',
      time: logTime('accepted'),
      done: current >= 3,
      active: current === 3
    },
    {
      key: 'completed',
      title: status.value === 'closed' ? '订单关闭' : '订单完成',
      text: status.value === 'closed' ? '订单已关闭或取消。' : '服务或商品已完成交付。',
      time: logTime(status.value === 'closed' ? 'closed' : 'completed'),
      done: current >= 4,
      active: current === 4
    }
  ]
})

const load = async () => {
  if (!orderNo.value) return
  loading.value = true
  try {
    const res = await fetchCustomerOrder(orderNo.value)
    order.value = res.data.order || {}
    if (!canRetry.value) {
      qrDialogVisible.value = false
      stopPolling()
    }
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '订单加载失败')
  } finally {
    loading.value = false
  }
}

const retryPay = async () => {
  if (!orderNo.value) return
  retrying.value = true
  try {
    const res = await retryCustomerOrderPayment(orderNo.value, { pay_mode: payMode.value })
    order.value = res.data.order || order.value
    const payment = res.data.payment
    if (payment?.mode === 'qr' && payment.qr_code) {
      qrImageUrl.value = buildQrImage(payment.qr_code)
      qrDialogVisible.value = true
      startPolling()
      return
    }
    const paymentUrl = payment?.payment_url || payment?.payment_form
    if (paymentUrl) submitAlipayPage(paymentUrl)
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '重新支付失败')
  } finally {
    retrying.value = false
  }
}

const submitAlipayPage = (paymentUrl) => {
  if (paymentUrl.trim().startsWith('<form')) {
    document.open()
    document.write(paymentUrl)
    document.close()
    document.querySelector('form')?.submit()
    return
  }
  window.location.href = paymentUrl
}

const startPolling = () => {
  stopPolling()
  pollTimer = window.setInterval(load, 3000)
}

const stopPolling = () => {
  if (pollTimer) {
    window.clearInterval(pollTimer)
    pollTimer = null
  }
}

const backStore = () => {
  const storeId = order.value.store_id || order.value.store?.id
  if (storeId) router.push(`/customer/store/${storeId}`)
}

const callStore = () => {
  if (contactPhone.value) window.location.href = `tel:${contactPhone.value}`
}

const logTime = (action) => {
  const logs = Array.isArray(order.value.operation_logs) ? order.value.operation_logs : []
  const log = logs.find((item) => item.action === action)
  return log?.time ? formatTime(log.time) : '-'
}

const buildQrImage = (value) => `https://api.qrserver.com/v1/create-qr-code/?size=260x260&data=${encodeURIComponent(value)}`
const yuan = (value) => `¥${(Number(value || 0) / 100).toFixed(2)}`
const formatTime = (value) => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '-'

onMounted(() => {
  load()
  if (orderNo.value) startPolling()
})
onBeforeUnmount(stopPolling)
</script>

<style scoped>
.order-page {
  min-height: 100vh;
  padding: 16px 0 28px;
  color: #172033;
  background:
    radial-gradient(circle at 14% 0%, rgba(251, 146, 60, 0.28), transparent 30%),
    radial-gradient(circle at 94% 8%, rgba(34, 197, 94, 0.22), transparent 32%),
    linear-gradient(180deg, #fffaf1 0%, #eef7ff 100%);
}

.phone-shell {
  width: min(430px, calc(100vw - 24px));
  margin: 0 auto;
  display: grid;
  gap: 12px;
}

.status-hero,
.panel {
  border-radius: 28px;
  background: rgba(255, 255, 255, 0.94);
  box-shadow: 0 18px 56px rgba(15, 23, 42, 0.12);
}

.status-hero {
  padding: 22px;
  color: #fff;
  background:
    radial-gradient(circle at 88% 12%, rgba(255, 255, 255, 0.24), transparent 30%),
    linear-gradient(135deg, #0f172a, #2563eb);
}

.status-hero.waiting { background: linear-gradient(135deg, #7c2d12, #f97316); }
.status-hero.failed { background: linear-gradient(135deg, #450a0a, #ef4444); }
.status-hero.done { background: linear-gradient(135deg, #064e3b, #22c55e); }
.status-hero.refunded { background: linear-gradient(135deg, #312e81, #6366f1); }

.hero-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.hero-top span {
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.14em;
  opacity: 0.78;
}

.refresh-btn {
  color: #fff;
}

.status-icon {
  width: 62px;
  height: 62px;
  margin-top: 22px;
  border-radius: 24px;
  display: grid;
  place-items: center;
  color: #0f172a;
  background: #fff;
  font-size: 28px;
  font-weight: 900;
}

h1 {
  margin: 16px 0 8px;
  font-size: 30px;
}

p {
  margin: 0;
  color: #64748b;
  line-height: 1.65;
}

.status-hero p {
  color: rgba(255, 255, 255, 0.78);
}

.panel {
  padding: 16px;
}

.summary-line {
  display: grid;
  gap: 7px;
  padding-bottom: 12px;
  border-bottom: 1px dashed #cbd5e1;
}

.summary-line span,
.summary-grid span,
.two-cols span,
.item-row span {
  color: #64748b;
  font-size: 12px;
}

.summary-line strong {
  word-break: break-all;
}

.summary-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  margin-top: 12px;
}

.summary-grid div,
.two-cols div {
  padding: 12px;
  border-radius: 16px;
  background: #f8fafc;
}

.summary-grid strong,
.summary-grid span,
.two-cols span,
.two-cols p {
  display: block;
}

.summary-grid strong {
  margin-top: 6px;
  font-size: 17px;
}

.retry-box {
  display: grid;
  gap: 10px;
  margin-top: 14px;
}

.pay-mode {
  width: 100%;
}

.section-title {
  margin-bottom: 12px;
  font-size: 18px;
  font-weight: 900;
}

.timeline {
  display: grid;
  gap: 14px;
}

.step {
  display: grid;
  grid-template-columns: 20px 1fr;
  gap: 10px;
  opacity: 0.52;
}

.step.done,
.step.active {
  opacity: 1;
}

.dot {
  width: 12px;
  height: 12px;
  margin-top: 5px;
  border-radius: 50%;
  background: #cbd5e1;
}

.step.done .dot {
  background: #22c55e;
}

.step.active .dot {
  box-shadow: 0 0 0 6px rgba(34, 197, 94, 0.14);
}

.step strong {
  display: block;
}

.step span {
  display: block;
  margin-top: 4px;
  color: #94a3b8;
  font-size: 12px;
}

.items {
  display: grid;
  gap: 10px;
}

.item-row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 12px;
  border-radius: 16px;
  background: #f8fafc;
}

.item-row strong,
.item-row span {
  display: block;
}

.two-cols {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}

.bottom-actions {
  position: sticky;
  bottom: 12px;
  z-index: 3;
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  padding: 10px;
  border-radius: 22px;
  background: rgba(255, 255, 255, 0.82);
  box-shadow: 0 14px 36px rgba(15, 23, 42, 0.18);
  backdrop-filter: blur(16px);
}

.qr-wrap {
  display: grid;
  justify-items: center;
  gap: 12px;
}

.qr-wrap img {
  width: 240px;
  height: 240px;
  border-radius: 18px;
}

@media (max-width: 430px) {
  .order-page {
    padding-top: 10px;
  }

  .status-hero,
  .panel {
    border-radius: 22px;
  }

  .two-cols {
    grid-template-columns: 1fr;
  }
}
</style>

<template>
  <div class="pay-grid">
    <div class="page-card">
      <div class="eyebrow">{{ text.eyebrow }}</div>
      <h1>{{ text.title }}</h1>
      <div class="detail-row"><span>{{ text.orderNo }}</span><strong>{{ orderNo }}</strong></div>
      <div class="detail-row"><span>{{ text.method }}</span><strong>{{ text.alipay }}</strong></div>
      <div class="detail-row"><span>{{ text.mode }}</span><strong>{{ modeLabel }}</strong></div>
      <div class="detail-row"><span>{{ text.status }}</span><strong :class="statusClass">{{ orderStatusText }}</strong></div>
      <div class="pay-actions">
        <el-button v-if="paymentUrl" type="primary" @click="goPay">{{ text.openAlipay }}</el-button>
        <el-button v-if="qrCode" @click="copyQrCode">{{ text.copyQr }}</el-button>
        <el-button v-if="orderStatus === 'pending'" @click="cancelOrder">{{ text.cancelOrder }}</el-button>
      </div>
      <p class="hint">{{ text.pollHint }}</p>
      <p v-if="!paymentUrl && !qrCode" class="hint">{{ text.missingPaymentHint }}</p>
    </div>

    <div class="page-card qr-panel">
      <div class="eyebrow">{{ text.qrEyebrow }}</div>
      <h2>{{ text.qrTitle }}</h2>
      <div v-if="qrCode" class="qr-wrap">
        <img :src="qrImageUrl" alt="payment qr code" class="qr-image" />
      </div>
      <div v-else class="empty-qrcode">{{ text.pageModeOnly }}</div>
      <el-input v-if="qrCode" :model-value="qrCode" type="textarea" :rows="4" readonly />
    </div>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import QRCode from 'qrcode'
import { cancelUserOrder, fetchUserOrderDetail } from '../../api/modules'

const text = {
  eyebrow: '\u652f\u4ed8\u4e2d\u5fc3',
  title: '\u5b8c\u6210\u8ba2\u5355\u652f\u4ed8',
  orderNo: '\u8ba2\u5355\u53f7',
  method: '\u652f\u4ed8\u65b9\u5f0f',
  alipay: '\u652f\u4ed8\u5b9d',
  mode: '\u652f\u4ed8\u6a21\u5f0f',
  status: '\u5f53\u524d\u72b6\u6001',
  openAlipay: '\u6253\u5f00\u652f\u4ed8\u5b9d\u652f\u4ed8\u9875',
  copyQr: '\u590d\u5236\u4e8c\u7ef4\u7801\u94fe\u63a5',
  cancelOrder: '\u53d6\u6d88\u8ba2\u5355',
  pollHint: '\u9875\u9762\u4f1a\u81ea\u52a8\u8f6e\u8be2\u8ba2\u5355\u72b6\u6001\u3002\u652f\u4ed8\u6210\u529f\u540e\u4f1a\u81ea\u52a8\u8df3\u8f6c\u5230\u652f\u4ed8\u7ed3\u679c\u9875\u3002',
  missingPaymentHint: '\u5f53\u524d\u6ca1\u6709\u627e\u5230\u8fd9\u7b14\u8ba2\u5355\u7684\u672c\u5730\u652f\u4ed8\u4fe1\u606f\u7f13\u5b58\u3002\u5982\u679c\u8ba2\u5355\u662f\u5728\u522b\u7684\u6d4f\u89c8\u5668\u4f1a\u8bdd\u521b\u5efa\u7684\uff0c\u8bf7\u91cd\u65b0\u4e0b\u5355\uff0c\u6216\u5728\u521b\u5efa\u8ba2\u5355\u540e\u7acb\u5373\u6253\u5f00\u652f\u4ed8\u9875\u3002',
  qrEyebrow: '\u626b\u7801\u652f\u4ed8',
  qrTitle: '\u652f\u4ed8\u5b9d\u626b\u7801\u4ed8\u6b3e',
  pageModeOnly: '\u5f53\u524d\u8ba2\u5355\u4f7f\u7528\u7684\u662f\u652f\u4ed8\u5b9d\u8df3\u8f6c\u9875\u652f\u4ed8\u6a21\u5f0f\u3002',
  qrMode: '\u4e8c\u7ef4\u7801\u652f\u4ed8',
  pageMode: '\u8df3\u8f6c\u9875\u652f\u4ed8',
  paid: '\u5df2\u652f\u4ed8',
  closed: '\u5df2\u5173\u95ed',
  failed: '\u652f\u4ed8\u5931\u8d25',
  pending: '\u5f85\u652f\u4ed8',
  copySuccess: '\u4e8c\u7ef4\u7801\u94fe\u63a5\u5df2\u590d\u5236',
  copyFailed: '\u590d\u5236\u5931\u8d25',
  qrFailed: '\u4e8c\u7ef4\u7801\u751f\u6210\u5931\u8d25',
  cancelSuccess: '\u8ba2\u5355\u5df2\u53d6\u6d88',
  cancelFailed: '\u53d6\u6d88\u8ba2\u5355\u5931\u8d25'
}

const route = useRoute()
const router = useRouter()
const PAYMENT_CACHE_KEY = 'user_payment_cache'
const orderNo = computed(() => route.query.orderNo || '')
const paymentPayload = ref({})
const orderStatus = ref('pending')
const qrImageUrl = ref('')
let timer = null

const paymentUrl = computed(() => route.query.paymentUrl || paymentPayload.value.payment_url || '')
const qrCode = computed(() => route.query.qrCode || paymentPayload.value.qr_code || '')
const mode = computed(() => route.query.mode || paymentPayload.value.mode || 'page')
const modeLabel = computed(() => (mode.value === 'qr' ? text.qrMode : text.pageMode))
const orderStatusText = computed(() => {
  if (orderStatus.value === 'paid') return text.paid
  if (orderStatus.value === 'closed') return text.closed
  if (orderStatus.value === 'failed') return text.failed
  return text.pending
})
const statusClass = computed(() => ({
  paid: orderStatus.value === 'paid',
  closed: orderStatus.value === 'closed',
  failed: orderStatus.value === 'failed'
}))

const goPay = () => {
  if (paymentUrl.value) {
    window.open(paymentUrl.value, '_blank', 'noopener,noreferrer')
  }
}

const copyQrCode = async () => {
  try {
    await navigator.clipboard.writeText(qrCode.value)
    ElMessage.success(text.copySuccess)
  } catch (error) {
    ElMessage.error(text.copyFailed)
  }
}

const renderQrCode = async () => {
  if (!qrCode.value) {
    qrImageUrl.value = ''
    return
  }

  try {
    qrImageUrl.value = await QRCode.toDataURL(qrCode.value, {
      width: 240,
      margin: 1,
      color: {
        dark: '#0f172a',
        light: '#ffffff'
      }
    })
  } catch (error) {
    qrImageUrl.value = ''
    ElMessage.error(text.qrFailed)
  }
}

const poll = async () => {
  if (!orderNo.value) return
  const res = await fetchUserOrderDetail(orderNo.value)
  orderStatus.value = res.data.status
  if (orderStatus.value === 'paid') {
    stopPolling()
    router.replace({ path: '/payment/return', query: { orderNo: orderNo.value } })
  }
}

const startPolling = () => {
  stopPolling()
  timer = window.setInterval(() => {
    poll().catch(() => {})
  }, 3000)
}

const stopPolling = () => {
  if (timer) {
    window.clearInterval(timer)
    timer = null
  }
}

const cancelOrder = async () => {
  try {
    await cancelUserOrder(orderNo.value)
    orderStatus.value = 'closed'
    removePaymentCache()
    stopPolling()
    ElMessage.success(text.cancelSuccess)
  } catch (error) {
    ElMessage.error(error.response?.data?.message || text.cancelFailed)
  }
}

const loadPaymentCache = () => {
  if (!orderNo.value) return
  const cache = JSON.parse(window.sessionStorage.getItem(PAYMENT_CACHE_KEY) || '{}')
  paymentPayload.value = cache[orderNo.value] || {}
}

const removePaymentCache = () => {
  if (!orderNo.value) return
  const cache = JSON.parse(window.sessionStorage.getItem(PAYMENT_CACHE_KEY) || '{}')
  delete cache[orderNo.value]
  window.sessionStorage.setItem(PAYMENT_CACHE_KEY, JSON.stringify(cache))
}

onMounted(async () => {
  loadPaymentCache()
  await renderQrCode()
  await poll()
  if (orderStatus.value === 'pending') {
    startPolling()
  } else if (orderStatus.value === 'paid' || orderStatus.value === 'closed') {
    removePaymentCache()
  }
})

watch(qrCode, () => {
  renderQrCode().catch(() => {})
})

onBeforeUnmount(stopPolling)
</script>

<style scoped>
.pay-grid {
  display: grid;
  grid-template-columns: minmax(320px, 1.15fr) minmax(320px, 0.85fr);
  gap: 20px;
}

.detail-row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 0;
  border-bottom: 1px solid #eef2f7;
}

.pay-actions {
  display: flex;
  gap: 12px;
  margin-top: 20px;
  flex-wrap: wrap;
}

.hint {
  color: var(--muted);
  margin-top: 16px;
}

.qr-panel {
  text-align: center;
}

.qr-wrap {
  display: grid;
  place-items: center;
  margin: 16px 0;
}

.qr-image {
  width: 240px;
  height: 240px;
  border-radius: 18px;
  border: 1px solid #e5edf9;
  background: #fff;
  padding: 12px;
}

.empty-qrcode {
  min-height: 240px;
  display: grid;
  place-items: center;
  color: var(--muted);
}

.paid {
  color: var(--success);
}

.closed,
.failed {
  color: #dc2626;
}

.eyebrow {
  color: var(--brand);
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0.15em;
  margin-bottom: 8px;
}

@media (max-width: 900px) {
  .pay-grid {
    grid-template-columns: 1fr;
  }
}
</style>

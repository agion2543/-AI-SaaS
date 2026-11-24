<template>
  <div class="customer-order-status">
    <!-- 顶部导航 -->
    <van-nav-bar
      title="订单详情"
      left-arrow
      fixed
      placeholder
      @click-left="goBack"
    />

    <van-notice-bar v-if="isSyntheticPreview" type="warning" left-icon="info-o" :scrollable="false">
      合成演示数据 · 非真实交易
    </van-notice-bar>
    <van-notice-bar v-if="paymentSetupUnavailable && !isConfirmedPaid && !isTableOrder" type="warning" left-icon="warning-o" :scrollable="false">
      支付入口暂不可用 · 仍未确认收款
    </van-notice-bar>

    <!-- 加载状态 -->
    <div v-if="loading" class="loading-container">
      <van-loading type="spinner" size="40px">加载中...</van-loading>
    </div>

    <template v-else-if="order">
      <!-- 订单状态 -->
      <div class="order-status-card" :class="statusClass">
        <div class="status-icon">
          <van-icon :name="statusIcon" size="48" />
        </div>
        <div class="status-text">{{ statusText }}</div>
        <div class="status-desc">{{ statusDesc }}</div>
      </div>

      <!-- 订单信息 -->
      <van-cell-group inset title="订单信息">
        <van-cell title="订单编号">
          <template #value>
            <span class="order-no">{{ order.order_no }}</span>
            <van-button size="mini" plain type="primary" @click="copyOrderNo">复制安全链接</van-button>
          </template>
        </van-cell>
        <van-cell v-if="Number(order.table_no) > 0" title="桌位" :value="order.table_label || `${order.table_no}号桌`" />
        <van-cell title="下单时间" :value="formatTime(order.created_at)" />
        <van-cell title="门店" :value="order.store_name" />
      </van-cell-group>

      <!-- 商品明细 -->
      <van-cell-group inset title="商品明细">
        <van-cell
          v-for="item in order.items"
          :key="item.product_id"
          :title="item.product_name"
          :label="orderItemLabel(item)"
        >
          <template #value>
            <span class="item-price">¥{{ formatPrice(item.price * item.quantity) }}</span>
          </template>
        </van-cell>
        <van-cell :title="isConfirmedPaid ? '实付金额' : '应付金额'">
          <template #value>
            <span class="final-price">¥{{ formatPrice(payableAmount) }}</span>
          </template>
        </van-cell>
        <van-cell title="收款状态" :value="paymentStatusText" />
      </van-cell-group>

      <!-- 订单备注 -->
      <van-cell-group v-if="order.customer_note || order.remark" inset title="备注">
        <van-cell :value="order.customer_note || order.remark" />
      </van-cell-group>

      <!-- 支付信息 -->
      <van-cell-group v-if="order.paid_at" inset title="支付信息">
        <van-cell title="支付方式" :value="getPayTypeText(order.payment_channel || order.pay_type)" />
        <van-cell title="支付时间" :value="formatTime(order.paid_at)" />
      </van-cell-group>

      <van-cell-group v-if="canUseMerchantQr" inset title="商户自有收款码">
        <van-cell title="应付金额" :value="`¥${formatPrice(payableAmount)}`" />
        <div class="merchant-qr-options">
          <div v-for="option in merchantPayOptions" :key="option.key" class="merchant-qr-option">
            <strong>{{ option.label }}</strong>
            <img :src="option.value" :alt="option.label" />
          </div>
        </div>
        <p class="manual-payment-note">这是门店配置的商户自有收款码，款项直接付给门店，平台不代收。付款后点击下方按钮仅通知门店核对；门店确认实际到账前，系统不会把订单计为已收款。</p>
        <van-button type="primary" block :loading="paying" @click="markPaid">已转账，通知门店核对</van-button>
      </van-cell-group>
      <van-cell-group v-else-if="showMerchantQrMissing" inset title="门店收款">
        <van-cell :title="isTableOrder ? '桌位收款方式待核实' : '收款码未配置'" :value="isTableOrder ? '请联系门店确认，勿使用平台支付入口' : '请联系门店核对付款方式'" />
        <p v-if="isTableOrder" class="manual-payment-note">当前订单未返回可用的商家收款码。请联系门店核对；在商家确认前，系统不会把订单计为已收款。</p>
      </van-cell-group>

      <!-- 倒计时（待支付状态） -->
      <van-count-down
        v-if="isUnpaidCheckout && order.expire_time && !checkoutDeadlineReached"
        :time="getExpireTime()"
        format="剩余支付时间: HH:mm:ss"
        class="countdown"
        @finish="onCountdownFinish"
      />
      <div v-else-if="order.expired_unpaid" class="countdown-expired" role="status">
        支付期限已过，订单已关闭且未付款；如仍需购买，请重新下单。
      </div>
      <div v-else-if="checkoutDeadlineReached" class="countdown-expired" role="status">
        支付时间已到，正在同步订单关闭状态；请勿再次发起付款。
      </div>

      <!-- 操作按钮 -->
      <div class="action-buttons">
        <template v-if="order.status === 'pending' && canInitiatePayment">
          <van-button type="primary" block :loading="paying" @click="handlePayment">继续支付</van-button>
        </template>
        <template v-else-if="order.status === 'payment_confirming'">
          <van-button type="warning" plain block disabled>等待门店确认到账</van-button>
        </template>
        <template v-else-if="order.status === 'failed' && canInitiatePayment">
          <van-button type="primary" block :loading="paying" @click="handlePayment">重新发起支付</van-button>
        </template>
        <template v-else-if="checkoutDeadlineReached">
          <van-button type="warning" plain block disabled>支付期限已到，正在确认订单状态</van-button>
        </template>
        <template v-else-if="order.status === 'completed'">
          <van-button type="primary" block @click="viewStore">再来一单</van-button>
          <van-button plain block @click="rateOrder">评价</van-button>
        </template>
        <template v-else-if="['closed', 'cancelled', 'refunded'].includes(order.status)">
          <van-button type="primary" block @click="viewStore">返回门店</van-button>
        </template>
        <van-button v-if="order.store_id && !['completed', 'closed', 'cancelled'].includes(order.status)" plain block @click="viewStore">联系门店 / 查看门店</van-button>
      </div>
    </template>

    <van-empty v-else description="订单不存在" />
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { showToast, showSuccessToast, showFailToast, showConfirmDialog } from 'vant'
import { fetchCustomerOrder, markCustomerOrderPaid, retryCustomerOrderPayment } from '@/api/modules'
import { captureCustomerOrderAccessToken, getCustomerOrderAccessToken, mobileCustomerOrderAccessPath } from '@/utils/customerOrderAccess'
import { isCustomerOrderPaid } from '@/utils/mobileCustomerOrder'
import { customerOrderPollIntervalMs, isCustomerOrderStatusPollable } from '@/utils/customerOrderPolling'
import { isCheckoutDeadlineReached, isUnpaidCheckoutOrder, remainingCheckoutTime } from '@/utils/customerOrderExpiry'

const route = useRoute()
const router = useRouter()
const orderNo = route.params.orderNo
const isSyntheticPreview = import.meta.env.DEV && route.query.synthetic === '1'
const paymentSetupUnavailable = ref(route.query.payment_setup_unavailable === '1')

// 状态
const loading = ref(true)
const order = ref(null)
const paying = ref(false)
const paymentConfig = ref({})
const checkoutExpiredLocally = ref(false)
let pollTimer = null
let pollAttempts = 0
let orderRequestInFlight = false

// 计算属性
const payableAmount = computed(() => Number(order.value?.total_amount ?? order.value?.amount ?? 0))
const isConfirmedPaid = computed(() => isCustomerOrderPaid(order.value) && payableAmount.value > 0)
const isUnpaidCheckout = computed(() => isUnpaidCheckoutOrder(order.value))
const checkoutDeadlineReached = computed(() => isUnpaidCheckout.value && (checkoutExpiredLocally.value || isCheckoutDeadlineReached(order.value)))
const isTableOrder = computed(() => Number(order.value?.table_no || 0) > 0)
const isPendingMerchantQrTableOrder = computed(() => isTableOrder.value && order.value?.payment_channel === 'merchant_qr' && order.value?.status === 'pending')
const canInitiatePayment = computed(() => isUnpaidCheckout.value && !checkoutDeadlineReached.value && !isTableOrder.value)
const paymentStatusText = computed(() => {
  if (order.value?.expired_unpaid) return '已过期 · 未付款'
  if (checkoutDeadlineReached.value) return '支付时间已到，正在同步关闭状态'
  if (payableAmount.value <= 0) return '无需付款'
  if (order.value?.status === 'payment_confirming') return '等待门店确认到账'
  if (isConfirmedPaid.value) return '已确认收款'
  if (order.value?.status === 'failed') return '支付未完成'
  if (isPendingMerchantQrTableOrder.value) return '待支付 · 商家收款码'
  if (['pending', 'submitted', 'preparing'].includes(order.value?.status)) return '待付款或核对'
  return '待核对'
})
const merchantPayOptions = computed(() => {
  const options = []
  if (paymentConfig.value?.alipay_qr_code) options.push({ key: 'alipay', label: '商户支付宝收款码', value: paymentConfig.value.alipay_qr_code })
  if (paymentConfig.value?.wechat_qr_code) options.push({ key: 'wechat', label: '商户微信收款码', value: paymentConfig.value.wechat_qr_code })
  return options
})
const canUseMerchantQr = computed(() => payableAmount.value > 0 && (['submitted', 'preparing'].includes(order.value?.status) || isPendingMerchantQrTableOrder.value) && !order.value?.paid_at && merchantPayOptions.value.length > 0)
const showMerchantQrMissing = computed(() => {
  if (payableAmount.value <= 0 || order.value?.paid_at) return false
  if (['submitted', 'preparing'].includes(order.value?.status)) return merchantPayOptions.value.length === 0
  return isTableOrder.value && order.value?.status === 'pending' && (!isPendingMerchantQrTableOrder.value || merchantPayOptions.value.length === 0)
})

const statusClass = computed(() => {
  if (checkoutDeadlineReached.value) return 'status-cancelled'
  const statusMap = {
    pending: 'status-pending',
    submitted: 'status-pending',
    payment_confirming: 'status-pending',
    paid: 'status-paid',
    received: 'status-paid',
    accepted: 'status-paid',
    preparing: 'status-preparing',
    ready: 'status-ready',
    completed: 'status-completed',
    cancelled: 'status-cancelled',
    closed: 'status-cancelled',
    failed: 'status-cancelled',
    refunded: 'status-refunded'
  }
  return statusMap[order.value?.status] || ''
})

const statusIcon = computed(() => {
  if (checkoutDeadlineReached.value) return 'cross'
  const iconMap = {
    pending: 'clock-o',
    submitted: 'clock-o',
    payment_confirming: 'clock-o',
    paid: 'paid',
    received: 'paid',
    accepted: 'orders-o',
    preparing: 'orders-o',
    ready: 'bag-o',
    completed: 'success',
    cancelled: 'cross',
    closed: 'cross',
    failed: 'warning-o',
    refunded: 'refund-o'
  }
  return iconMap[order.value?.status] || 'info-o'
})

const statusText = computed(() => {
  if (order.value?.expired_unpaid) return '订单已过期'
  if (checkoutDeadlineReached.value) return '支付时间已到'
  const textMap = {
    pending: '待支付',
    submitted: payableAmount.value <= 0 ? '订单已提交，无需付款' : '订单已提交',
    payment_confirming: '等待门店确认收款',
    paid: '已确认收款',
    received: '已确认收款，待接单',
    accepted: '商家已接单',
    preparing: '制作中',
    ready: '待取餐',
    completed: '已完成',
    cancelled: '已取消',
    closed: '订单已关闭',
    failed: '支付未完成',
    refunded: '已退款'
  }
  return textMap[order.value?.status] || '订单处理中'
})

const statusDesc = computed(() => {
  if (order.value?.expired_unpaid) return '订单已超过30分钟支付期限并关闭，尚未确认收款；如仍需购买，请重新下单。'
  if (checkoutDeadlineReached.value) return '支付期限已结束，正在同步订单关闭状态；请勿再次付款。'
  if (isPendingMerchantQrTableOrder.value) return '请使用门店配置的商家收款码付款；付款后由门店人工核对到账，页面返回本身不代表已收款。'
  if (paymentSetupUnavailable.value && !isConfirmedPaid.value && payableAmount.value > 0 && !isTableOrder.value) {
    return '支付宝付款入口暂不可用；订单尚未确认收款，可稍后重试或联系门店核对。'
  }
  const descMap = {
    pending: '请尽快完成支付',
    submitted: payableAmount.value <= 0 ? '本单应付金额为 0 元，不计入实收' : '订单已提交，请按门店提供的方式付款或核对',
    payment_confirming: '你已通知门店核对到账；门店确认前，系统不会计为已收款',
    paid: '门店已确认收款',
    received: '门店已确认收款，等待接单',
    accepted: '门店已接单，正在处理',
    preparing: '您的订单正在制作中',
    ready: '订单已准备好，请取餐',
    completed: '感谢您的光临，欢迎再次光临',
    cancelled: '订单已取消',
    closed: '订单已关闭',
    failed: '尚未确认付款；可重新发起支付',
    refunded: '退款已原路返回'
  }
  return descMap[order.value?.status] || ''
})

// 方法
function formatPrice(price) {
  return (Number(price ?? 0) / 100).toFixed(2)
}

function orderItemLabel(item) {
  const unit = item.sales_unit ? `/${item.sales_unit}` : ''
  const quantityUnit = item.sales_unit ? ` ${item.sales_unit}` : ''
  const details = []
  if (item.specification) details.push(item.specification)
  if (Number(item.minimum_order_quantity || 1) > 1) details.push(`下单时最低 ${item.minimum_order_quantity}${item.sales_unit || '件'}`)
  if (Number(item.applied_tier_min_quantity || 0) > 0) details.push(`应用 ${item.applied_tier_min_quantity}${item.sales_unit || '件'}档`)
  details.push(`¥${formatPrice(item.price)}${unit} × ${item.quantity}${quantityUnit}`)
  return details.join(' · ')
}

function formatTime(time) {
  if (!time) return '-'
  return new Date(time).toLocaleString('zh-CN')
}

function getExpireTime() {
  return remainingCheckoutTime(order.value) ?? 0
}

function getPayTypeText(payType) {
  const typeMap = {
    wechat: '微信支付',
    alipay: '支付宝',
    merchant_qr: '商户自有收款码（直接付给门店）',
    mock: '模拟支付'
  }
  return typeMap[payType] || payType || '-'
}

function copyOrderNo() {
  const accessToken = getCustomerOrderAccessToken(order.value?.order_no)
  if (!accessToken) {
    showToast('缺少订单访问凭证，无法生成安全链接')
    return
  }
  const path = mobileCustomerOrderAccessPath(order.value.order_no, accessToken)
  const secureLink = `${window.location.origin}${window.location.pathname}#${path}`
  navigator.clipboard.writeText(secureLink).then(() => {
    showToast('安全订单链接已复制，请勿公开分享')
  }).catch(() => {
    showToast('复制失败')
  })
}

function goBack() {
  if (order.value?.store_id) {
    router.replace(`/m/store/${order.value.store_id}`)
  } else {
    router.back()
  }
}

function onCountdownFinish() {
  checkoutExpiredLocally.value = true
  showToast('支付时间已到，正在同步订单状态')
  loadOrderData()
}

async function loadOrderData({ silent = false } = {}) {
  if (orderRequestInFlight) return
  orderRequestInFlight = true
  if (!order.value) loading.value = true
  try {
    const accessToken = getCustomerOrderAccessToken(orderNo)
    if (!accessToken) {
      order.value = null
      showToast('订单链接缺少访问凭证，旧的仅含订单号链接已停用')
      return
    }
    const res = await fetchCustomerOrder(orderNo)
    order.value = res.data.order || res.data
    paymentConfig.value = res.data.payment_config || {}
    if (!isCustomerOrderStatusPollable(order.value?.status)) stopPolling()
  } catch (e) {
    console.error('加载订单失败:', e)
    if (!silent) showToast(e.response?.status === 404 ? '订单不存在或访问凭证无效' : '加载失败')
  } finally {
    loading.value = false
    orderRequestInFlight = false
  }
}

function scheduleNextPoll() {
  if (pollTimer || document.visibilityState !== 'visible' || !isCustomerOrderStatusPollable(order.value?.status)) return
  pollTimer = setTimeout(async () => {
    pollTimer = null
    if (document.visibilityState !== 'visible' || !isCustomerOrderStatusPollable(order.value?.status)) {
      stopPolling()
      return
    }
    pollAttempts += 1
    await loadOrderData({ silent: true })
    scheduleNextPoll()
  }, customerOrderPollIntervalMs(pollAttempts))
}

function startPolling(resetAttempts = true) {
  stopPolling(false)
  if (resetAttempts) pollAttempts = 0
  scheduleNextPoll()
}

function stopPolling(resetAttempts = true) {
  if (pollTimer) {
    clearTimeout(pollTimer)
    pollTimer = null
  }
  if (resetAttempts) pollAttempts = 0
}

async function onVisibilityChange() {
  if (document.visibilityState !== 'visible') {
    stopPolling(false)
    return
  }
  if (!isCustomerOrderStatusPollable(order.value?.status)) return
  await loadOrderData({ silent: true })
  scheduleNextPoll()
}

async function handlePayment() {
  if (!orderNo || !canInitiatePayment.value || paying.value) return
  paying.value = true
  try {
    const res = await retryCustomerOrderPayment(orderNo, { pay_mode: 'page' })
    order.value = res.data.order || order.value
    const payment = res.data.payment || {}
    const paymentUrl = payment.payment_url || payment.payment_form || payment.form
    if (paymentUrl) {
      if (paymentUrl.trim().startsWith('<form')) {
        document.open()
        document.write(paymentUrl)
        document.close()
        document.querySelector('form')?.submit()
      } else {
        window.location.href = paymentUrl
      }
      return
    }
    showFailToast('支付渠道未返回有效付款入口；订单仍未确认收款')
  } catch (e) {
    showFailToast(e.response?.data?.message || '发起支付失败，订单仍未确认收款')
  } finally {
    paying.value = false
  }
}

async function markPaid() {
  if (!canUseMerchantQr.value || paying.value) return
  try {
    await showConfirmDialog({
      title: '通知门店核对到账',
      message: `请确认已完成转账，金额为 ¥${formatPrice(payableAmount.value)}。提交后只会通知门店核对，不会自动认定收款。`,
      confirmButtonText: '确认已转账',
      cancelButtonText: '返回检查'
    })
  } catch {
    return
  }
  paying.value = true
  try {
    const res = await markCustomerOrderPaid(orderNo)
    order.value = res.data.order || order.value
    paymentConfig.value = res.data.payment_config || paymentConfig.value
    showSuccessToast('已通知门店核对到账；门店确认前不会计为实收')
    startPolling()
  } catch (e) {
    showFailToast(e.response?.data?.message || '通知门店失败')
  } finally {
    paying.value = false
  }
}

function removeAccessTokenFromRoute() {
  if (!Object.prototype.hasOwnProperty.call(route.query, 'access_token')) return
  const query = { ...route.query }
  delete query.access_token
  router.replace({ path: route.path, query, hash: route.hash }).catch(() => {})
}

function viewStore() {
  if (order.value?.store_id) {
    router.push(`/m/store/${order.value.store_id}`)
  }
}

function rateOrder() {
  showToast('评价功能开发中')
}

onMounted(async () => {
  document.addEventListener('visibilitychange', onVisibilityChange)
  const accessToken = captureCustomerOrderAccessToken(orderNo, route)
  if (!accessToken) {
    await loadOrderData()
    return
  }
  removeAccessTokenFromRoute()
  await loadOrderData()
  startPolling()
})

onUnmounted(() => {
  document.removeEventListener('visibilitychange', onVisibilityChange)
  stopPolling()
})
</script>

<style scoped>
.customer-order-status {
  min-height: 100vh;
  background: #f7f8fa;
  padding-bottom: 80px;
}

.loading-container {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 50vh;
}

.order-status-card {
  padding: 30px 20px;
  text-align: center;
  color: white;
}

.status-pending {
  background: linear-gradient(135deg, #ff976a 0%, #ed6a0c 100%);
}

.status-paid,
.status-preparing,
.status-ready {
  background: linear-gradient(135deg, #07c160 0%, #06ad56 100%);
}

.status-completed {
  background: linear-gradient(135deg, #1989fa 0%, #096dd9 100%);
}

.status-cancelled,
.status-refunded {
  background: linear-gradient(135deg, #999 0%, #666 100%);
}

.status-icon {
  margin-bottom: 10px;
}

.status-text {
  font-size: 22px;
  font-weight: bold;
  margin-bottom: 5px;
}

.status-desc {
  font-size: 14px;
  opacity: 0.9;
}

.order-no {
  font-family: monospace;
  font-size: 12px;
  cursor: pointer;
}

.item-price {
  color: #666;
}

.final-price {
  color: #ee0a24;
  font-weight: bold;
  font-size: 16px;
}

.countdown {
  display: block;
  text-align: center;
  padding: 16px;
  background: #fff;
  margin: 12px 16px;
  border-radius: 8px;
  font-size: 16px;
}

.countdown-expired {
  margin: 12px 16px;
  padding: 14px 16px;
  border-radius: 8px;
  background: #fff7e8;
  color: #8a5b13;
  font-size: 14px;
  line-height: 1.6;
  text-align: center;
}

.action-buttons {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.merchant-qr-options {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  padding: 12px 16px;
}

.merchant-qr-option {
  flex: 1 1 140px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}

.merchant-qr-option img {
  width: 180px;
  max-width: 100%;
  height: auto;
  object-fit: contain;
}

.manual-payment-note {
  padding: 0 16px;
  color: #8a5a00;
  font-size: 13px;
  line-height: 1.6;
}
</style>

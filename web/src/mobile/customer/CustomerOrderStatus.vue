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
            <span class="order-no" @click="copyOrderNo">{{ order.order_no }}</span>
          </template>
        </van-cell>
        <van-cell title="下单时间" :value="formatTime(order.created_at)" />
        <van-cell title="门店" :value="order.store_name" />
      </van-cell-group>

      <!-- 商品明细 -->
      <van-cell-group inset title="商品明细">
        <van-cell
          v-for="item in order.items"
          :key="item.id"
          :title="item.product_name"
          :label="`¥${formatPrice(item.price)} x ${item.quantity}`"
        >
          <template #value>
            <span class="item-price">¥{{ formatPrice(item.price * item.quantity) }}</span>
          </template>
        </van-cell>
        <van-cell title="实付金额">
          <template #value>
            <span class="final-price">¥{{ formatPrice(order.amount) }}</span>
          </template>
        </van-cell>
      </van-cell-group>

      <!-- 订单备注 -->
      <van-cell-group v-if="order.remark" inset title="备注">
        <van-cell :value="order.remark" />
      </van-cell-group>

      <!-- 支付信息 -->
      <van-cell-group v-if="order.paid_at" inset title="支付信息">
        <van-cell title="支付方式" :value="getPayTypeText(order.pay_type)" />
        <van-cell title="支付时间" :value="formatTime(order.paid_at)" />
      </van-cell-group>

      <!-- 倒计时（待支付状态） -->
      <van-count-down
        v-if="order.status === 'pending' && order.expire_time"
        :time="getExpireTime(order.expire_time)"
        format="剩余支付时间: HH:mm:ss"
        class="countdown"
        @finish="onCountdownFinish"
      />

      <!-- 操作按钮 -->
      <div class="action-buttons">
        <!-- 待支付状态 -->
        <template v-if="order.status === 'pending'">
          <van-button type="primary" block @click="handlePayment">立即支付</van-button>
          <van-button plain block @click="cancelOrder">取消订单</van-button>
        </template>

        <!-- 已支付状态 -->
        <template v-else-if="order.status === 'paid'">
          <van-button type="primary" plain block @click="viewStore">再来一单</van-button>
        </template>

        <!-- 已完成状态 -->
        <template v-else-if="order.status === 'completed'">
          <van-button type="primary" block @click="viewStore">再来一单</van-button>
          <van-button plain block @click="rateOrder">评价</van-button>
        </template>

        <!-- 已取消/已退款 -->
        <template v-else-if="['cancelled', 'refunded'].includes(order.status)">
          <van-button type="primary" block @click="viewStore">重新下单</van-button>
        </template>
      </div>
    </template>

    <van-empty v-else description="订单不存在" />

    <!-- 取消订单弹窗 -->
    <van-dialog
      v-model:show="showCancelDialog"
      title="取消订单"
      show-cancel-button
      @confirm="confirmCancel"
    >
      <div class="cancel-dialog-content">
        <p>确定要取消该订单吗？</p>
        <p v-if="order && order.status === 'pending'" class="warning-text">请在支付前完成取消</p>
      </div>
    </van-dialog>

    <!-- 支付弹窗（模拟支付） -->
    <van-popup v-model:show="showPaymentDialog" position="bottom" style="height: 50%;">
      <div class="payment-dialog">
        <div class="payment-header">
          <span>确认支付</span>
          <van-icon name="cross" @click="showPaymentDialog = false" />
        </div>
        <div class="payment-amount">
          <span class="currency">¥</span>
          <span class="amount">{{ formatPrice(order?.amount || 0) }}</span>
        </div>
        <div class="payment-methods">
          <van-radio-group v-model="selectedPayMethod">
            <van-cell-group>
              <van-cell title="微信支付" clickable @click="selectedPayMethod = 'wechat'">
                <template #icon>
                  <van-icon name="wechat-pay" size="20" color="#07c169" />
                </template>
                <template #right-icon>
                  <van-radio name="wechat" />
                </template>
              </van-cell>
              <van-cell title="支付宝" clickable @click="selectedPayMethod = 'alipay'">
                <template #icon>
                  <van-icon name="../../assets/alipay.png" size="20" />
                </template>
                <template #right-icon>
                  <van-radio name="alipay" />
                </template>
              </van-cell>
            </van-cell-group>
          </van-radio-group>
        </div>
        <div class="payment-confirm">
          <van-button type="primary" block :loading="paying" @click="confirmPayment">
            确认支付
          </van-button>
        </div>
      </div>
    </van-popup>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { showToast, showSuccessToast, showFailToast, showDialog } from 'vant'
import { fetchCustomerOrder, createCustomerStoreOrder } from '@/api/modules'

const route = useRoute()
const router = useRouter()
const orderNo = route.params.orderNo

// 状态
const loading = ref(true)
const order = ref(null)
const showCancelDialog = ref(false)
const showPaymentDialog = ref(false)
const selectedPayMethod = ref('wechat')
const paying = ref(false)
let pollTimer = null

const defaultProductImg = 'https://via.placeholder.com/100x100?text=商品'

// 计算属性
const statusClass = computed(() => {
  const statusMap = {
    pending: 'status-pending',
    paid: 'status-paid',
    preparing: 'status-preparing',
    ready: 'status-ready',
    completed: 'status-completed',
    cancelled: 'status-cancelled',
    refunded: 'status-refunded'
  }
  return statusMap[order.value?.status] || ''
})

const statusIcon = computed(() => {
  const iconMap = {
    pending: 'clock-o',
    paid: 'paid',
    preparing: 'orders-o',
    ready: 'bag-o',
    completed: 'success',
    cancelled: 'cross',
    refunded: 'refund-o'
  }
  return iconMap[order.value?.status] || 'info-o'
})

const statusText = computed(() => {
  const textMap = {
    pending: '待支付',
    paid: '已支付',
    preparing: '制作中',
    ready: '待取餐',
    completed: '已完成',
    cancelled: '已取消',
    refunded: '已退款'
  }
  return textMap[order.value?.status] || '未知状态'
})

const statusDesc = computed(() => {
  const descMap = {
    pending: '请尽快完成支付',
    paid: '商家正在准备中',
    preparing: '您的订单正在制作中',
    ready: '订单已准备好，请取餐',
    completed: '感谢您的光临，欢迎再次光临',
    cancelled: '订单已取消',
    refunded: '退款已原路返回'
  }
  return descMap[order.value?.status] || ''
})

// 方法
function formatPrice(price) {
  return parseFloat(price || 0).toFixed(2)
}

function formatTime(time) {
  if (!time) return '-'
  return new Date(time).toLocaleString('zh-CN')
}

function getExpireTime(expireTime) {
  const expire = new Date(expireTime).getTime()
  const now = Date.now()
  return Math.max(0, expire - now)
}

function getPayTypeText(payType) {
  const typeMap = {
    wechat: '微信支付',
    alipay: '支付宝',
    mock: '模拟支付'
  }
  return typeMap[payType] || payType || '-'
}

function copyOrderNo() {
  navigator.clipboard.writeText(order.value.order_no).then(() => {
    showToast('已复制订单号')
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
  showToast('订单已过期')
  loadOrderData()
}

async function loadOrderData() {
  loading.value = true
  try {
    const res = await fetchCustomerOrder(orderNo)
    order.value = res.data
    // 如果订单已支付，停止轮询
    if (order.value?.status !== 'pending') {
      stopPolling()
    }
  } catch (e) {
    console.error('加载订单失败:', e)
    showToast('加载失败')
  } finally {
    loading.value = false
  }
}

function startPolling() {
  stopPolling()
  pollTimer = setInterval(() => {
    if (order.value?.status === 'pending') {
      loadOrderData()
    }
  }, 5000) // 每5秒轮询一次
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

function handlePayment() {
  showPaymentDialog.value = true
}

async function confirmPayment() {
  paying.value = true
  try {
    // 模拟支付成功
    // 实际项目中这里应该调用微信/支付宝 SDK
    await new Promise(resolve => setTimeout(resolve, 1500))

    showSuccessToast('支付成功')
    showPaymentDialog.value = false
    await loadOrderData()
    startPolling()
  } catch (e) {
    showFailToast('支付失败，请重试')
  } finally {
    paying.value = false
  }
}

function cancelOrder() {
  showCancelDialog.value = true
}

async function confirmCancel() {
  try {
    // 调用取消订单接口
    await new Promise(resolve => setTimeout(resolve, 500))
    showSuccessToast('订单已取消')
    await loadOrderData()
  } catch (e) {
    showFailToast('取消失败')
  }
}

function viewStore() {
  if (order.value?.store_id) {
    router.push(`/m/store/${order.value.store_id}`)
  }
}

function rateOrder() {
  showToast('评价功能开发中')
}

onMounted(() => {
  loadOrderData()
  // 如果是待支付状态，开始轮询
  startPolling()
})

onUnmounted(() => {
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

.action-buttons {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.cancel-dialog-content {
  padding: 20px;
  text-align: center;
}

.warning-text {
  color: #ee0a24;
  font-size: 12px;
  margin-top: 8px;
}

.payment-dialog {
  padding: 20px;
}

.payment-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-weight: bold;
  font-size: 16px;
  margin-bottom: 20px;
}

.payment-amount {
  text-align: center;
  padding: 20px 0;
}

.payment-amount .currency {
  font-size: 20px;
  color: #333;
}

.payment-amount .amount {
  font-size: 36px;
  font-weight: bold;
  color: #333;
}

.payment-methods {
  margin-bottom: 20px;
}

.payment-confirm {
  padding-top: 10px;
}
</style>

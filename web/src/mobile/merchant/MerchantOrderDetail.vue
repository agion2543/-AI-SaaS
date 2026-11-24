<template>
  <div class="merchant-order-detail">
    <!-- 顶部导航 -->
    <van-nav-bar
      title="订单详情"
      left-arrow
      fixed
      placeholder
      @click-left="router.back()"
    />

    <div v-if="!canWrite" class="readonly-banner" role="status">
      订阅已到期：订单详情仅供查看，不能确认收款、接单、拒单、完成或处理退款。
      <van-button size="small" type="warning" plain @click="router.push('/m/merchant/subscription')">查看续费</van-button>
    </div>

    <!-- 加载状态 -->
    <div v-if="loading" class="loading-container">
      <van-loading type="spinner" size="40px">加载中...</van-loading>
    </div>

    <template v-else-if="order">
      <!-- 订单状态 -->
      <div class="order-status" :class="statusClass">
        <div class="status-icon">
          <van-icon :name="statusIcon" size="40" />
        </div>
        <div class="status-text">{{ statusText }}</div>
      </div>
      <p v-if="order.status === 'payment_confirming'" class="payment-confirmation-note">
        顾客的操作只会通知门店核对，不代表平台已验证到账。请先在商户自己的收款账户核实实际到账，再手动确认；系统只记录商户的确认。
      </p>

      <!-- 订单信息 -->
      <van-cell-group inset title="订单信息">
        <van-cell title="订单编号" :value="order.order_no" />
        <van-cell v-if="Number(order.table_no) > 0" title="桌号" :value="order.table_label || `${order.table_no}号桌`" />
        <van-cell title="下单时间" :value="formatTime(order.created_at)" />
        <van-cell title="门店" :value="order.store?.name || '-'" />
        <van-cell
          title="顾客电话"
          :value="order.customer_phone || '未留手机号'"
          :is-link="Boolean(order.customer_phone)"
          @click="callCustomer"
        />
        <van-cell v-if="order.payment_channel || order.pay_type" title="支付方式" :value="getPayTypeText(order.payment_channel || order.pay_type)" />
        <van-cell v-if="order.paid_at" title="支付时间" :value="formatTime(order.paid_at)" />
      </van-cell-group>

      <!-- 商品明细 -->
      <van-cell-group inset title="商品明细">
        <van-cell
          v-for="item in order.items"
          :key="item.id"
        >
          <template #title>
            <div class="product-info">
              <span class="product-name">{{ item.product_name || item.name }}</span>
              <span class="product-price">{{ formatCents(item.price) }}<span v-if="item.sales_unit"> / {{ item.sales_unit }}</span></span>
            </div>
          </template>
          <template #label>
            <span class="product-quantity">x{{ item.quantity }}{{ item.sales_unit ? ` ${item.sales_unit}` : '' }}</span>
            <span v-if="item.specification" class="product-snapshot">规格: {{ item.specification }}</span>
            <span v-if="item.minimum_order_quantity > 1" class="product-snapshot">下单时最低 {{ item.minimum_order_quantity }}{{ item.sales_unit || '件' }}</span>
            <span v-if="item.applied_tier_min_quantity > 0" class="product-snapshot">实际按 {{ item.applied_tier_min_quantity }}{{ item.sales_unit || '件' }}档计价</span>
            <span class="product-subtotal">小计: {{ formatCents(item.line_amount ?? (item.price * item.quantity)) }}</span>
          </template>
        </van-cell>

        <van-cell title="应付金额">
          <template #value>
            <span class="total-price">{{ formatCents(order.total_amount ?? order.amount ?? 0) }}</span>
          </template>
        </van-cell>
        <van-cell title="已确认实收">
          <template #value>
            <span class="final-price">{{ formatCents(isConfirmedPaid ? (order.total_amount ?? order.amount ?? 0) : 0) }}</span>
          </template>
        </van-cell>
        <van-cell v-if="order.discount_amount > 0" title="优惠">
          <template #value>
            <span class="discount">-{{ formatCents(order.discount_amount) }}</span>
          </template>
        </van-cell>
        <van-cell v-if="Number(order.refunded_amount || 0) > 0" title="订单中已记录的退款金额">
          <template #value>
            <span class="refund-price">-{{ formatCents(order.refunded_amount) }}</span>
          </template>
        </van-cell>
      </van-cell-group>

      <!-- 备注信息 -->
      <van-cell-group v-if="order.customer_note" inset title="顾客备注">
        <van-cell>
          <van-icon name="comment-o" />
          {{ order.customer_note }}
        </van-cell>
      </van-cell-group>

      <!-- 操作按钮 -->
      <div class="action-buttons">
        <van-button
          v-if="canWrite && order.status === 'payment_confirming'"
          type="warning"
          block
          @click="confirmPayment"
        >
          核实到账后，确认收款
        </van-button>
        <van-button
          v-if="canWrite && ['submitted', 'received'].includes(order.status) && (order.status === 'submitted' || isConfirmedPaid || Number(order.total_amount ?? order.amount ?? 0) === 0)"
          type="primary"
          block
          @click="acceptOrder"
        >
          接单
        </van-button>
        <van-button
          v-if="canWrite && ['submitted', 'received', 'payment_confirming'].includes(order.status)"
          type="danger"
          plain
          block
          @click="showRejectDialog = true"
        >
          拒单
        </van-button>
        <van-button
          v-if="canWrite && ['accepted', 'preparing'].includes(order.status)"
          type="success"
          block
          @click="completeOrder"
        >
          完成订单
        </van-button>
        <van-button
          v-if="canWrite && canRefund"
          type="warning"
          block
          @click="showRefundDialog = true"
        >
          处理退款
        </van-button>
      </div>
    </template>

    <van-empty v-else description="订单不存在" />

    <!-- 拒单弹窗 -->
    <van-dialog
      v-model:show="showRejectDialog"
      title="拒单原因"
      show-cancel-button
      @confirm="confirmReject"
    >
      <van-field
        v-model="rejectReason"
        type="textarea"
        placeholder="请输入拒单原因"
        rows="3"
      />
    </van-dialog>

    <!-- 退款弹窗 -->
    <van-dialog
      v-model:show="showRefundDialog"
      title="处理退款"
      show-cancel-button
      @confirm="confirmRefund"
    >
      <div style="padding: 16px;">
        <p>订单金额: {{ formatCents(order?.total_amount ?? order?.amount ?? 0) }}</p>
        <van-radio-group v-model="refundAction">
          <van-cell-group>
            <van-cell title="同意退款" clickable @click="refundAction = 'approve'">
              <template #right-icon>
                <van-radio name="approve" />
              </template>
            </van-cell>
            <van-cell title="拒绝退款" clickable @click="refundAction = 'reject'">
              <template #right-icon>
                <van-radio name="reject" />
              </template>
            </van-cell>
          </van-cell-group>
        </van-radio-group>
        <van-field
          v-if="refundAction === 'reject'"
          v-model="refundReason"
          type="textarea"
          placeholder="请输入拒绝原因"
          rows="2"
          style="margin-top: 12px;"
        />
      </div>
    </van-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { showToast, showSuccessToast, showFailToast } from 'vant'
import {
  fetchMerchantOrderDetail,
  acceptMerchantOrder,
  confirmMerchantOrderPayment,
  completeMerchantOrder,
  closeMerchantOrder,
  refundMerchantOrder
} from '@/api/modules'
import { clearRefundIdempotency, getOrCreateRefundIdempotency } from '@/utils/refundIdempotency'
import { formatCents } from '@/utils/productPricing'
import { cachedMobileMerchantSubscriptionState, mobileSubscriptionActions } from '@/utils/mobileMerchantSubscription'

const route = useRoute()
const router = useRouter()
const orderId = route.params.id
const canWrite = computed(() => mobileSubscriptionActions(cachedMobileMerchantSubscriptionState()).canWriteBusiness)

const loading = ref(true)
const order = ref(null)
const showRejectDialog = ref(false)
const rejectReason = ref('')
const showRefundDialog = ref(false)
const refundAction = ref('approve')
const refundReason = ref('')
const refundIdempotencyKey = ref('')
const refundIdempotencyPayload = ref('')
const isConfirmedPaid = computed(() => Boolean(order.value?.paid_at) && Number(order.value?.total_amount ?? order.value?.amount ?? 0) > 0)
const canRefund = computed(() => Boolean(order.value?.can_refund))

const statusClass = computed(() => {
  const map = {
    pending: 'status-pending',
    failed: 'status-pending',
    payment_confirming: 'status-pending',
    submitted: 'status-paid',
    received: 'status-paid',
    accepted: 'status-preparing',
    preparing: 'status-preparing',
    completed: 'status-completed',
    closed: 'status-cancelled'
  }
  return map[order.value?.status] || ''
})

const statusIcon = computed(() => {
  const map = {
    pending: 'clock-o',
    failed: 'cross',
    payment_confirming: 'question-o',
    submitted: 'orders-o',
    received: 'paid',
    accepted: 'orders-o',
    preparing: 'orders-o',
    completed: 'success',
    closed: 'cross'
  }
  return map[order.value?.status] || 'info-o'
})

const statusText = computed(() => {
  const map = {
    pending: '等待顾客付款',
    failed: '支付未完成',
    payment_confirming: '待核对收款',
    submitted: Number(order.value?.total_amount ?? order.value?.amount ?? 0) === 0 ? '无需付款，等待商家处理' : '已提交，稍后结算',
    received: '已确认收款，等待商家接单',
    accepted: '商家已接单',
    preparing: '商家处理中',
    completed: '已完成',
    closed: '已关闭'
  }
  return map[order.value?.status] || '未知状态'
})

function formatTime(time) {
  if (!time) return '-'
  return new Date(time).toLocaleString('zh-CN')
}

function getPayTypeText(payType) {
  const map = {
    wechat: '微信支付',
    alipay: '支付宝',
    merchant_qr: '商户自有收款码（直接付给门店）',
    mock: '模拟支付'
  }
  return map[payType] || payType || '-'
}

async function loadOrder() {
  loading.value = true
  try {
    const res = await fetchMerchantOrderDetail(orderId)
    order.value = res.data.order || res.data
  } catch (e) {
    console.error('加载订单失败:', e)
    showToast('加载失败')
  } finally {
    loading.value = false
  }
}

function callCustomer() {
  if (!order.value?.customer_phone) return
  window.location.href = `tel:${order.value.customer_phone}`
}

async function acceptOrder() {
  try {
    await acceptMerchantOrder(orderId)
    showSuccessToast('已接单')
    loadOrder()
  } catch (e) {
    showFailToast(e.response?.data?.message || '操作失败')
  }
}

async function confirmPayment() {
  try {
    await confirmMerchantOrderPayment(orderId)
    showSuccessToast('已确认收款')
    loadOrder()
  } catch (e) {
    showFailToast(e.response?.data?.message || '操作失败')
  }
}

function confirmReject() {
  if (!rejectReason.value.trim()) {
    showToast('请输入拒单原因')
    return
  }
  doReject()
}

async function doReject() {
  try {
    await closeMerchantOrder(orderId)
    showSuccessToast('已拒单')
    router.back()
  } catch (e) {
    showFailToast(e.response?.data?.message || '操作失败')
  }
}

async function completeOrder() {
  try {
    await completeMerchantOrder(orderId)
    showSuccessToast('订单已完成')
    loadOrder()
  } catch (e) {
    showFailToast(e.response?.data?.message || '操作失败')
  }
}

function confirmRefund() {
  doRefund()
}

async function doRefund() {
  try {
    if (refundAction.value === 'approve') {
      const requestPayload = { full: true, reason: '同意退款' }
      const idempotency = getOrCreateRefundIdempotency(orderId, requestPayload, refundIdempotencyKey.value, refundIdempotencyPayload.value)
      if (idempotency.conflict) {
        showToast('上一笔退款请求仍待核查或请求内容已改变，请先完成渠道对账。')
        return
      }
      refundIdempotencyKey.value = idempotency.key
      refundIdempotencyPayload.value = idempotency.fingerprint
      await refundMerchantOrder(orderId, { ...requestPayload, idempotency_key: idempotency.key })
      showSuccessToast('退款渠道已确认处理')
      clearRefundIdempotency(orderId)
      refundIdempotencyKey.value = ''
      refundIdempotencyPayload.value = ''
    } else {
      if (!refundReason.value.trim()) {
        showToast('请输入拒绝原因')
        return
      }
      await closeMerchantOrder(orderId)
      showSuccessToast('已拒绝退款')
    }
    router.back()
  } catch (e) {
    const message = String(e.response?.data?.message || e.message || '')
    if (message.toLowerCase().includes('definitively rejected')) {
      clearRefundIdempotency(orderId)
      refundIdempotencyKey.value = ''
      refundIdempotencyPayload.value = ''
    }
    showFailToast(message || '操作失败')
  }
}

onMounted(() => {
  loadOrder()
})
</script>

<style scoped>
.merchant-order-detail {
  min-height: 100vh;
  background: #f7f8fa;
  padding-bottom: 80px;
}
.readonly-banner { display: flex; align-items: center; justify-content: space-between; gap: 10px; margin: 10px 12px; padding: 10px 12px; border: 1px solid #f0d49b; border-radius: 12px; color: #80591b; background: #fff8e8; font-size: 12px; line-height: 1.5; }

.loading-container {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 50vh;
}

.order-status {
  padding: 30px 20px;
  text-align: center;
  color: white;
}

.status-pending {
  background: linear-gradient(135deg, #ff976a 0%, #ed6a0c 100%);
}

.status-paid {
  background: linear-gradient(135deg, #07c160 0%, #06ad56 100%);
}

.status-preparing {
  background: linear-gradient(135deg, #1989fa 0%, #096dd9 100%);
}

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

.status-text {
  font-size: 16px;
  margin-top: 8px;
}

.payment-confirmation-note {
  margin: 12px 16px;
  padding: 12px;
  border-radius: 8px;
  background: #fff7e8;
  color: #8a5a00;
  font-size: 13px;
  line-height: 1.6;
}

.product-info {
  display: flex;
  justify-content: space-between;
  width: 100%;
}

.product-name {
  flex: 1;
}

.product-price {
  margin-left: 12px;
}

.product-quantity {
  margin-right: 12px;
  color: #666;
}

.product-snapshot {
  display: block;
  margin-top: 4px;
  color: #806217;
}

.product-subtotal {
  color: #ee0a24;
}

.total-price {
  text-decoration: line-through;
  color: #999;
}

.discount {
  color: #07c160;
}
.refund-price { color: #ee0a24; }

.final-price {
  color: #ee0a24;
  font-weight: bold;
  font-size: 16px;
}

.action-buttons {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
</style>
